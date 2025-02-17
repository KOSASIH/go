package ingest

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	logpkg "github.com/stellar/go/support/log"
	"github.com/stellar/go/toid"
)

const reapLookupTablesBatchSize = 1000

// Reaper represents the history reaping subsystem of horizon.
type Reaper struct {
	historyQ  history.IngestionQ
	reapLockQ history.IngestionQ
	pending   atomic.Bool
	config    ReapConfig
	logger    *logpkg.Entry

	totalDuration       *prometheus.SummaryVec
	totalDeleted        *prometheus.SummaryVec
	deleteBatchDuration prometheus.Summary
	rowsInBatchDeleted  prometheus.Summary
}

type ReapConfig struct {
	Frequency      uint
	RetentionCount uint32
	BatchSize      uint32
}

// NewReaper creates a new Reaper instance
func NewReaper(config ReapConfig, dbSession db.SessionInterface) *Reaper {
	return newReaper(config, &history.Q{dbSession.Clone()}, &history.Q{dbSession.Clone()})
}

func newReaper(config ReapConfig, historyQ, reapLockQ history.IngestionQ) *Reaper {
	return &Reaper{
		historyQ:  historyQ,
		reapLockQ: reapLockQ,
		config:    config,
		deleteBatchDuration: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace: "horizon", Subsystem: "reap", Name: "batch_duration",
			Help:       "reap batch duration in seconds, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}),
		rowsInBatchDeleted: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace: "horizon", Subsystem: "reap", Name: "batch_rows_deleted",
			Help:       "rows deleted during reap batch, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}),
		totalDuration: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "horizon", Subsystem: "reap", Name: "duration",
			Help:       "reap invocation duration in seconds, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"complete"}),
		totalDeleted: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "horizon", Subsystem: "reap", Name: "rows_deleted",
			Help:       "rows deleted during reap invocation, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"complete"}),
		logger: log.WithField("subservice", "reaper"),
	}
}

// DeleteUnretainedHistory removes all data associated with unretained ledgers.
func (r *Reaper) DeleteUnretainedHistory(ctx context.Context) error {
	// RetentionCount of 0 indicates "keep all history"
	if r.config.RetentionCount == 0 {
		return nil
	}

	// check if reap is already in progress on this horizon node
	if !r.pending.CompareAndSwap(false, true) {
		r.logger.Infof("existing reap already in progress, skipping request to start a new one")
		return nil
	}
	defer r.pending.Store(false)

	if err := r.reapLockQ.Begin(ctx); err != nil {
		return errors.Wrap(err, "error while starting reaper lock transaction")
	}
	defer func() {
		if err := r.reapLockQ.Rollback(); err != nil {
			r.logger.WithField("error", err).Error("failed to release reaper lock")
		}
	}()
	// check if reap is already in progress on another horizon node
	if acquired, err := r.reapLockQ.TryReaperLock(ctx); err != nil {
		return errors.Wrap(err, "error while acquiring reaper database lock")
	} else if !acquired {
		r.logger.Info("reap already in progress on another node")
		return nil
	}

	latest, err := r.historyQ.GetLatestHistoryLedger(ctx)
	if err != nil {
		return errors.Wrap(err, "error fetching latest history ledger")
	}
	var oldest uint32
	err = r.historyQ.ElderLedger(ctx, &oldest)
	if err != nil {
		return errors.Wrap(err, "error fetching elder ledger")
	}

	targetElder := latest - r.config.RetentionCount + 1
	if latest <= r.config.RetentionCount || targetElder < oldest {
		r.logger.
			WithField("latest", latest).
			WithField("oldest", oldest).
			WithField("retention_count", r.config.RetentionCount).
			Info("not enough history to reap")
		return nil
	}

	startTime := time.Now()
	var totalDeleted int64
	var complete bool
	totalDeleted, err = r.clearBefore(ctx, oldest, targetElder)
	elapsedSeconds := time.Since(startTime).Seconds()
	logger := r.logger.
		WithField("duration", elapsedSeconds).
		WithField("rows_deleted", totalDeleted)

	if err != nil {
		logger.WithError(err).Warn("reaper failed")
	} else {
		complete = true
		logger.
			WithField("new_elder", targetElder).
			Info("reaper succeeded")
	}

	labels := prometheus.Labels{
		"complete": strconv.FormatBool(complete),
	}
	r.totalDeleted.With(labels).Observe(float64(totalDeleted))
	r.totalDuration.With(labels).Observe(elapsedSeconds)
	return err
}

// RegisterMetrics registers the prometheus metrics
func (s *Reaper) RegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(
		s.deleteBatchDuration,
		s.rowsInBatchDeleted,
		s.totalDuration,
		s.totalDeleted,
	)
}

// Work in 50k (by default, otherwise configurable via the CLI) ledger
// blocks to prevent using all the CPU.
//
// By default, this runs every 720 ledgers (approximately 1 hour), so we
// need to make sure it doesn't run for longer than
// an hour.
//
// Current ledger at 2024-04-04s is 51,092,283, so 50k means 1021 batches. At 1
// batch/second, that seems like a reasonable balance between running under an
// hour, and slowing it down enough to leave some CPU for other processes.
var sleep = 1 * time.Second

func (r *Reaper) clearBefore(ctx context.Context, startSeq, endSeq uint32) (int64, error) {
	batchSize := r.config.BatchSize
	var sum int64
	if batchSize <= 0 {
		return sum, fmt.Errorf("invalid batch size for reaping (%d)", batchSize)
	}

	r.logger.WithField("start_ledger", startSeq).
		WithField("end_ledger", endSeq).
		WithField("batch_size", batchSize).
		Info("deleting history outside retention window")

	for batchStartSeq := startSeq; batchStartSeq < endSeq; {
		batchEndSeq := batchStartSeq + batchSize
		if batchEndSeq >= endSeq {
			batchEndSeq = endSeq - 1
		}

		count, err := r.deleteBatch(ctx, batchStartSeq, batchEndSeq)
		if err != nil {
			return sum, err
		}
		sum += count
		if count == 0 {
			next, ok, err := r.historyQ.GetNextLedgerSequence(ctx, batchStartSeq)
			if err != nil {
				return sum, errors.Wrapf(err, "could not find next ledger sequence after %d", batchStartSeq)
			}
			if !ok {
				break
			}
			batchStartSeq = next
		} else {
			batchStartSeq += batchSize + 1
		}
		time.Sleep(sleep)
	}

	return sum, nil
}

func (r *Reaper) deleteBatch(ctx context.Context, batchStartSeq, batchEndSeq uint32) (int64, error) {
	batchStart, batchEnd, err := toid.LedgerRangeInclusive(int32(batchStartSeq), int32(batchEndSeq))
	if err != nil {
		return 0, err
	}

	startTime := time.Now()
	err = r.historyQ.Begin(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "Error in begin")
	}
	defer r.historyQ.Rollback()

	count, err := r.historyQ.DeleteRangeAll(ctx, batchStart, batchEnd)
	if err != nil {
		return 0, errors.Wrap(err, "Error in DeleteRangeAll")
	}

	err = r.historyQ.Commit()
	if err != nil {
		return 0, errors.Wrap(err, "Error in commit")
	}

	elapsedSeconds := time.Since(startTime).Seconds()
	r.logger.WithField("start_ledger", batchStartSeq).
		WithField("end_ledger", batchEndSeq).
		WithField("rows_deleted", strconv.FormatInt(count, 10)).
		WithField("duration", elapsedSeconds).
		Info("successfully deleted batch")

	r.rowsInBatchDeleted.Observe(float64(count))
	r.deleteBatchDuration.Observe(elapsedSeconds)
	return count, nil
}

type lookupTableReaper struct {
	historyQ  history.IngestionQ
	reapLockQ history.IngestionQ
	pending   atomic.Bool
	logger    *logpkg.Entry

	reapDurationByLookupTable *prometheus.SummaryVec
	rowsReapedByLookupTable   *prometheus.SummaryVec
}

func newLookupTableReaper(dbSession db.SessionInterface) *lookupTableReaper {
	return &lookupTableReaper{
		historyQ:  &history.Q{dbSession.Clone()},
		reapLockQ: &history.Q{dbSession.Clone()},
		pending:   atomic.Bool{},
		logger:    log.WithField("subservice", "lookuptable-reaper"),
		reapDurationByLookupTable: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "horizon", Subsystem: "ingest", Name: "reap_lookup_tables_duration_seconds",
			Help:       "reap lookup tables durations, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"table", "type"}),
		rowsReapedByLookupTable: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "horizon", Subsystem: "ingest", Name: "reap_lookup_tables_rows_reaped",
			Help:       "rows deleted during lookup tables reap, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"table"}),
	}
}

func (r *lookupTableReaper) RegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(
		r.reapDurationByLookupTable,
		r.rowsReapedByLookupTable,
	)
}

func (r *lookupTableReaper) deleteOrphanedRows(ctx context.Context) error {
	// check if reap is already in progress on this horizon node
	if !r.pending.CompareAndSwap(false, true) {
		r.logger.Infof("existing reap already in progress, skipping request to start a new one")
		return nil
	}
	defer r.pending.Store(false)

	if err := r.reapLockQ.Begin(ctx); err != nil {
		return errors.Wrap(err, "error while starting reaper lock transaction")
	}
	defer func() {
		if err := r.reapLockQ.Rollback(); err != nil {
			r.logger.WithField("error", err).Error("failed to release reaper lock")
		}
	}()
	// check if reap is already in progress on another horizon node
	if acquired, err := r.reapLockQ.TryLookupTableReaperLock(ctx); err != nil {
		return errors.Wrap(err, "error while acquiring reaper database lock")
	} else if !acquired {
		r.logger.Info("reap already in progress on another node")
		return nil
	}

	reapStart := time.Now()
	var totalQueryDuration, totalDeleteDuration time.Duration
	var totalDeleted int64
	for _, table := range []string{
		"history_accounts", "history_claimable_balances",
		"history_assets", "history_liquidity_pools",
	} {
		startTime := time.Now()
		ids, offset, err := r.historyQ.FindLookupTableRowsToReap(ctx, table, reapLookupTablesBatchSize)
		if err != nil {
			r.logger.WithField("table", table).WithError(err).Warn("Error finding orphaned rows")
			return err
		}
		queryDuration := time.Since(startTime)
		totalQueryDuration += queryDuration

		deleteStartTime := time.Now()
		var rowsDeleted int64
		rowsDeleted, err = r.historyQ.ReapLookupTable(ctx, table, ids, offset)
		if err != nil {
			r.logger.WithField("table", table).WithError(err).Warn("Error deleting orphaned rows")
			return err
		}
		deleteDuration := time.Since(deleteStartTime)
		totalDeleteDuration += deleteDuration
		totalDeleted += rowsDeleted

		r.rowsReapedByLookupTable.With(prometheus.Labels{"table": table}).
			Observe(float64(rowsDeleted))
		r.reapDurationByLookupTable.With(prometheus.Labels{"table": table, "type": "query"}).
			Observe(float64(queryDuration.Seconds()))
		r.reapDurationByLookupTable.With(prometheus.Labels{"table": table, "type": "delete"}).
			Observe(float64(deleteDuration.Seconds()))
		r.reapDurationByLookupTable.With(prometheus.Labels{"table": table, "type": "total"}).
			Observe(float64((queryDuration + deleteDuration).Seconds()))

		r.logger.WithField("table", table).
			WithField("offset", offset).
			WithField("rows_deleted", rowsDeleted).
			WithField("query_duration", queryDuration.Seconds()).
			WithField("delete_duration", deleteDuration.Seconds()).
			Info("Reaper deleted rows from lookup tables")
	}

	r.rowsReapedByLookupTable.With(prometheus.Labels{"table": "total"}).
		Observe(float64(totalDeleted))
	r.reapDurationByLookupTable.With(prometheus.Labels{"table": "total", "type": "query"}).
		Observe(float64(totalQueryDuration.Seconds()))
	r.reapDurationByLookupTable.With(prometheus.Labels{"table": "total", "type": "delete"}).
		Observe(float64(totalDeleteDuration.Seconds()))
	r.reapDurationByLookupTable.With(prometheus.Labels{"table": "total", "type": "total"}).
		Observe(time.Since(reapStart).Seconds())
	return nil
}

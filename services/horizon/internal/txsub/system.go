package txsub

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/stellar/go/services/horizon/internal/ledger"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/log"
	"github.com/stellar/go/xdr"
)

type HorizonDB interface {
	GetLatestHistoryLedger(ctx context.Context) (uint32, error)
	PreFilteredTransactionByHash(ctx context.Context, dest interface{}, hash string) error
	TransactionByHash(ctx context.Context, dest interface{}, hash string) error
	AllTransactionsByHashesSinceLedger(ctx context.Context, hashes []string, sinceLedgerSeq uint32) ([]history.Transaction, error)
	GetSequenceNumbers(ctx context.Context, addresses []string) (map[string]uint64, error)
	BeginTx(context.Context, *sql.TxOptions) error
	Rollback() error
	NoRows(error) bool
}

// System represents a completely configured transaction submission system.
// Its methods tie together the various pieces used to reliably submit transactions
// to a stellar-core instance.
type System struct {
	initializer sync.Once

	tickMutex      sync.Mutex
	tickInProgress bool

	accountSeqPollInterval time.Duration

	DB                func(context.Context) HorizonDB
	Pending           OpenSubmissionList
	Submitter         Submitter
	SubmissionTimeout time.Duration
	Log               *log.Entry
	LedgerState       ledger.StateInterface

	Metrics struct {
		// OpenSubmissionsGauge tracks the count of "open" submissions (i.e.
		// submissions whose transactions haven't been confirmed successful or failed
		OpenSubmissionsGauge prometheus.Gauge

		// FailedSubmissionsCounter tracks the rate of failed transactions that have
		// been submitted to this process
		FailedSubmissionsCounter prometheus.Counter

		// SuccessfulSubmissionsCounter tracks the rate of successful transactions that
		// have been submitted to this process
		SuccessfulSubmissionsCounter prometheus.Counter
	}
}

// RegisterMetrics registers the prometheus metrics
func (sys *System) RegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(sys.Metrics.OpenSubmissionsGauge)
	registry.MustRegister(sys.Metrics.FailedSubmissionsCounter)
	registry.MustRegister(sys.Metrics.SuccessfulSubmissionsCounter)
}

// Submit submits the provided base64 encoded transaction envelope to the
// network using this submission system.
func (sys *System) Submit(
	ctx context.Context,
	rawTx string,
	envelope xdr.TransactionEnvelope,
	hash string,
) (resultReadCh <-chan Result) {
	sys.Init()
	resultCh := make(chan Result, 1)
	resultReadCh = resultCh

	db := sys.DB(ctx)
	// The database doesn't (yet) store muxed accounts, so we query
	// the corresponding AccountId
	sourceAccount := envelope.SourceAccount().ToAccountId()
	sourceAddress := sourceAccount.Address()

	sys.Log.Ctx(ctx).WithFields(log.F{
		"hash":    hash,
		"tx_type": envelope.Type.String(),
		"tx":      rawTx,
	}).Info("Processing transaction")

	seqNum := envelope.SeqNum()
	minSeqNum := envelope.MinSeqNum()
	// Ensure sequence numbers make sense
	if seqNum < 0 || (minSeqNum != nil && (*minSeqNum < 0 || *minSeqNum >= seqNum)) {
		sys.finish(ctx, hash, resultCh, Result{Err: ErrBadSequence})
		return
	}

	tx, err := txResultByHash(ctx, db, hash)
	if err == nil {
		sys.Log.Ctx(ctx).WithField("hash", hash).Info("Found submission result in a DB")
		sys.finish(ctx, hash, resultCh, Result{Transaction: tx})
		return
	}
	if err != ErrNoResults {
		sys.Log.Ctx(ctx).WithField("hash", hash).Info("Error getting submission result from a DB")
		sys.finish(ctx, hash, resultCh, Result{Transaction: tx, Err: err})
		return
	}

	sr := sys.submitOnce(ctx, rawTx)

	if sr.Err != nil {
		// any error other than "txBAD_SEQ" is a failure
		isBad, err := sr.IsBadSeq()
		if err != nil {
			sys.finish(ctx, hash, resultCh, Result{Err: err})
			return
		}
		if !isBad {
			sys.finish(ctx, hash, resultCh, Result{Err: sr.Err})
			return
		}

		// Even if a transaction is successfully submitted to core, Horizon ingestion might
		// be lagging behind leading to txBAD_SEQ. This function will block a txsub request
		// until the request times out or account sequence is bumped to txn sequence.
		if err = sys.waitUntilAccountSequence(ctx, db, sourceAddress, uint64(envelope.SeqNum())); err != nil {
			sys.finish(ctx, hash, resultCh, Result{Err: err})
			return
		}

		// If error is txBAD_SEQ, check for the result again
		tx, err = txResultByHash(ctx, db, hash)
		if err != nil {
			// finally, return the bad_seq error if no result was found on 2nd attempt
			sys.finish(ctx, hash, resultCh, Result{Err: sr.Err})
			return
		}
		// If we found the result, use it as the result
		sys.finish(ctx, hash, resultCh, Result{Transaction: tx})
		return
	}

	// Add transaction to open list of pending txns: the transaction has been successfully submitted to core
	// but that does not mean it is included in the ledger. The txn status remains pending
	// until we see an ingestion in the db.
	sys.Pending.Add(hash, resultCh)
	return
}

// waitUntilAccountSequence blocks until either the context times out or the sequence number of the
// given source account is greater than or equal to `seq`
func (sys *System) waitUntilAccountSequence(ctx context.Context, db HorizonDB, sourceAddress string, seq uint64) error {
	timer := time.NewTimer(sys.accountSeqPollInterval)
	defer timer.Stop()

	for {
		sequenceNumbers, err := db.GetSequenceNumbers(ctx, []string{sourceAddress})
		if err != nil {
			sys.Log.Ctx(ctx).
				WithError(err).
				WithField("sourceAddress", sourceAddress).
				Warn("cannot fetch sequence number")
		} else {
			num, ok := sequenceNumbers[sourceAddress]
			if !ok {
				sys.Log.Ctx(ctx).
					WithField("sequenceNumbers", sequenceNumbers).
					WithField("sourceAddress", sourceAddress).
					Warn("missing sequence number for account")
			}
			if num >= seq || sys.isSyncedUp() {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return sys.deriveTxSubError(ctx)
		case <-timer.C:
			timer.Reset(sys.accountSeqPollInterval)
		}
	}
}

// isSyncedUp Check if Horizon and Core have synced up: If yes, then no need to wait for account sequence
// and send txBAD_SEQ right away.
func (sys *System) isSyncedUp() bool {
	currentStatus := sys.LedgerState.CurrentStatus()
	return int(currentStatus.CoreLatest) <= int(currentStatus.HistoryLatest)
}

func (sys *System) deriveTxSubError(ctx context.Context) error {
	if ctx.Err() == context.Canceled {
		return ErrCanceled
	}
	return ErrTimeout
}

// Submit submits the provided base64 encoded transaction envelope to the
// network using this submission system.
func (sys *System) submitOnce(ctx context.Context, rawTx string) SubmissionResult {
	// submit to stellar-core
	sr := sys.Submitter.Submit(ctx, rawTx)

	if sr.Err == nil {
		sys.Metrics.SuccessfulSubmissionsCounter.Inc()
	} else {
		sys.Metrics.FailedSubmissionsCounter.Inc()
	}

	return sr
}

// setTickInProgress sets `tickInProgress` to `true` if it's
// `false`. Returns `true` if `tickInProgress` has been switched
// to `true` inside this method and `Tick()` should continue.
func (sys *System) setTickInProgress(ctx context.Context) bool {
	sys.tickMutex.Lock()
	defer sys.tickMutex.Unlock()

	if sys.tickInProgress {
		logger := log.Ctx(ctx)
		logger.Info("ticking in progress")
		return false
	}

	sys.tickInProgress = true
	return true
}

func (sys *System) unsetTickInProgress() {
	sys.tickMutex.Lock()
	defer sys.tickMutex.Unlock()
	sys.tickInProgress = false
}

// Tick triggers the system to update itself with any new data available.
func (sys *System) Tick(ctx context.Context) {
	sys.Init()
	logger := log.Ctx(ctx)

	// Make sure Tick is not run concurrently
	if !sys.setTickInProgress(ctx) {
		return
	}

	defer sys.unsetTickInProgress()

	logger.Debug("ticking txsub system")

	db := sys.DB(ctx)
	options := &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}
	if err := db.BeginTx(ctx, options); err != nil {
		logger.WithError(err).Error("could not start repeatable read transaction for txsub tick")
		return
	}
	defer db.Rollback()

	pending := sys.Pending.Pending()

	if len(pending) > 0 {
		latestLedger, err := db.GetLatestHistoryLedger(ctx)
		if err != nil {
			logger.WithError(err).Error("error getting latest history ledger")
			return
		}

		// In Tick we only check txs in a queue so those which did not have results before Tick
		// so we check for them in the last 5 mins of ledgers: 60.
		sinceLedgerSeq := int32(latestLedger) - 60
		if sinceLedgerSeq < 0 {
			sinceLedgerSeq = 0
		}

		txs, err := db.AllTransactionsByHashesSinceLedger(ctx, pending, uint32(sinceLedgerSeq))
		if err != nil && !db.NoRows(err) {
			logger.WithError(err).Error("error getting transactions by hashes")
			return
		}

		txMap := make(map[string]history.Transaction, len(txs))
		for _, tx := range txs {
			txMap[tx.TransactionHash] = tx
			if tx.InnerTransactionHash.Valid {
				txMap[tx.InnerTransactionHash.String] = tx
			}
		}

		for _, hash := range pending {
			tx, found := txMap[hash]
			if !found {
				continue
			}
			_, err := txResultFromHistory(tx)

			if err == nil {
				logger.WithField("hash", hash).Debug("finishing open submission")
				sys.Pending.Finish(hash, Result{Transaction: tx})
				continue
			}

			if _, ok := err.(*FailedTransactionError); ok {
				logger.WithField("hash", hash).Debug("finishing open submission")
				sys.Pending.Finish(hash, Result{Transaction: tx, Err: err})
				continue
			}

			if err != nil {
				logger.WithStack(err).Error(err)
			}
		}
	}

	stillOpen := sys.Pending.Clean(sys.SubmissionTimeout)
	sys.Metrics.OpenSubmissionsGauge.Set(float64(stillOpen))
}

// Init initializes `sys`
func (sys *System) Init() {
	sys.initializer.Do(func() {
		sys.Log = log.DefaultLogger.WithField("service", "txsub.System")

		sys.Metrics.FailedSubmissionsCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "horizon", Subsystem: "txsub", Name: "failed",
		})
		sys.Metrics.SuccessfulSubmissionsCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "horizon", Subsystem: "txsub", Name: "succeeded",
		})
		sys.Metrics.OpenSubmissionsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "horizon", Subsystem: "txsub", Name: "open",
		})

		sys.accountSeqPollInterval = time.Second

		if sys.SubmissionTimeout == 0 {
			// HTTP clients in SDKs usually timeout in 60 seconds. We want SubmissionTimeout
			// to be lower than that to make sure that they read the response before the client
			// timeout.
			// 30 seconds is 6 ledgers (with avg. close time = 5 sec), enough for stellar-core
			// to drop the transaction if not added to the ledger and ask client to try again
			// by sending a Timeout response.
			sys.SubmissionTimeout = 30 * time.Second
		}
	})
}

func (sys *System) finish(ctx context.Context, hash string, response chan<- Result, r Result) {
	sys.Log.Ctx(ctx).
		WithField("result", fmt.Sprintf("%+v", r)).
		WithField("hash", hash).
		Info("Submission system result")
	response <- r
	close(response)
}

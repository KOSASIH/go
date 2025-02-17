package ingest

import (
	"fmt"
	"math"
	"sync"

	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	logpkg "github.com/stellar/go/support/log"
	"github.com/stellar/go/support/ordered"
)

type rangeError struct {
	err         error
	ledgerRange history.LedgerRange
}

func (e rangeError) Error() string {
	return fmt.Sprintf("error when processing [%d, %d] range: %s", e.ledgerRange.StartSequence, e.ledgerRange.EndSequence, e.err)
}

type ParallelSystems struct {
	config        Config
	workerCount   uint
	minBatchSize  uint
	maxBatchSize  uint
	systemFactory func(Config) (System, error)
}

func NewParallelSystems(config Config, workerCount uint, minBatchSize, maxBatchSize uint) (*ParallelSystems, error) {
	// Leaving this because used in tests, will update after a code review.
	return newParallelSystems(config, workerCount, minBatchSize, maxBatchSize, NewSystem)
}

// private version of NewParallel systems, allowing to inject a mock system
func newParallelSystems(config Config, workerCount uint, minBatchSize, maxBatchSize uint, systemFactory func(Config) (System, error)) (*ParallelSystems, error) {
	if workerCount < 1 {
		return nil, errors.New("workerCount must be > 0")
	}
	if minBatchSize != 0 && minBatchSize < HistoryCheckpointLedgerInterval {
		return nil, fmt.Errorf("minBatchSize must be at least the %d", HistoryCheckpointLedgerInterval)
	}
	if minBatchSize != 0 && maxBatchSize != 0 && maxBatchSize < minBatchSize {
		return nil, errors.New("maxBatchSize cannot be less than minBatchSize")
	}
	return &ParallelSystems{
		config:        config,
		workerCount:   workerCount,
		maxBatchSize:  maxBatchSize,
		minBatchSize:  minBatchSize,
		systemFactory: systemFactory,
	}, nil
}

func (ps *ParallelSystems) Shutdown() {
	log.Info("Shutting down parallel ingestion system...")
	if ps.config.HistorySession != nil {
		ps.config.HistorySession.Close()
	}
}

func (ps *ParallelSystems) runReingestWorker(s System, stop <-chan struct{}, reingestJobQueue <-chan history.LedgerRange) rangeError {

	for {
		select {
		case <-stop:
			return rangeError{}
		case reingestRange := <-reingestJobQueue:
			err := s.ReingestRange([]history.LedgerRange{reingestRange}, false, false)
			if err != nil {
				return rangeError{
					err:         err,
					ledgerRange: reingestRange,
				}
			}
			log.WithFields(logpkg.F{"from": reingestRange.StartSequence, "to": reingestRange.EndSequence}).Info("successfully reingested range")
		}
	}
}

func (ps *ParallelSystems) rebuildTradeAggRanges(ledgerRanges []history.LedgerRange) error {
	s, err := ps.systemFactory(ps.config)
	if err != nil {
		return err
	}

	for _, cur := range ledgerRanges {
		err := s.RebuildTradeAggregationBuckets(cur.StartSequence, cur.EndSequence)
		if err != nil {
			return errors.Wrapf(err, "Error rebuilding trade aggregations for range start=%v, stop=%v", cur.StartSequence, cur.EndSequence)
		}
	}
	return nil
}

// returns the lowest ledger to start from of all ledgerRanges
func enqueueReingestTasks(ledgerRanges []history.LedgerRange, batchSize uint32, stop <-chan struct{}, reingestJobQueue chan<- history.LedgerRange) uint32 {
	lowestLedger := uint32(math.MaxUint32)
	for _, cur := range ledgerRanges {
		for subRangeFrom := cur.StartSequence; subRangeFrom < cur.EndSequence; {
			// job queuing
			subRangeTo := subRangeFrom + (batchSize - 1) // we subtract one because both from and to are part of the batch
			if subRangeTo > cur.EndSequence {
				subRangeTo = cur.EndSequence
			}
			select {
			case <-stop:
				return lowestLedger
			case reingestJobQueue <- history.LedgerRange{StartSequence: subRangeFrom, EndSequence: subRangeTo}:
			}
			if subRangeFrom < lowestLedger {
				lowestLedger = subRangeFrom
			}
			subRangeFrom = subRangeTo + 1
		}
	}
	return lowestLedger
}
func (ps *ParallelSystems) calculateParallelLedgerBatchSize(rangeSize uint32) uint32 {
	// calculate the initial batch size based on available workers
	batchSize := rangeSize / uint32(ps.workerCount)

	// ensure the batch size meets min threshold
	if ps.minBatchSize > 0 {
		batchSize = ordered.Max(batchSize, uint32(ps.minBatchSize))
	}

	// ensure the batch size does not exceed max threshold
	if ps.maxBatchSize > 0 {
		batchSize = ordered.Min(batchSize, uint32(ps.maxBatchSize))
	}

	// round down to the nearest multiple of HistoryCheckpointLedgerInterval
	if batchSize > uint32(HistoryCheckpointLedgerInterval) {
		return batchSize / uint32(HistoryCheckpointLedgerInterval) * uint32(HistoryCheckpointLedgerInterval)
	}

	//  HistoryCheckpointLedgerInterval is the minimum batch size.
	return uint32(HistoryCheckpointLedgerInterval)
}

func totalRangeSize(ledgerRanges []history.LedgerRange) uint32 {
	var sum uint32
	for _, ledgerRange := range ledgerRanges {
		sum += ledgerRange.EndSequence - ledgerRange.StartSequence + 1
	}
	return sum
}

func (ps *ParallelSystems) ReingestRange(ledgerRanges []history.LedgerRange) error {
	var (
		batchSize        = ps.calculateParallelLedgerBatchSize(totalRangeSize(ledgerRanges))
		reingestJobQueue = make(chan history.LedgerRange)
		wg               sync.WaitGroup

		// stopOnce is used to close the stop channel once: closing a closed channel panics and it can happen in case
		// of errors in multiple ranges.
		stopOnce sync.Once
		stop     = make(chan struct{})

		lowestRangeErrMutex sync.Mutex
		// lowestRangeErr is an error of the failed range with the lowest starting ledger sequence that is used to tell
		// the user which range to reingest in case of errors. We use that fact that System.ReingestRange is blocking,
		// jobs are sent to a queue (unbuffered channel) in sequence and there is a WaitGroup waiting for all the workers
		// to exit.
		// Because of this when we reach `wg.Wait()` all jobs previously sent to a channel are processed (either success
		// or failure). In case of a failure we save the range with the smallest sequence number because this is where
		// the user needs to start again to prevent the gaps.
		lowestRangeErr *rangeError
	)

	defer ps.Shutdown()

	if err := validateRanges(ledgerRanges); err != nil {
		return err
	}

	for i := uint(0); i < ps.workerCount; i++ {
		wg.Add(1)
		s, err := ps.systemFactory(ps.config)
		if err != nil {
			return errors.Wrap(err, "error creating new system")
		}
		go func() {
			defer wg.Done()
			rangeErr := ps.runReingestWorker(s, stop, reingestJobQueue)
			if rangeErr.err != nil {
				log.WithError(rangeErr).Error("error in reingest worker")
				lowestRangeErrMutex.Lock()
				if lowestRangeErr == nil || lowestRangeErr.ledgerRange.StartSequence > rangeErr.ledgerRange.StartSequence {
					lowestRangeErr = &rangeErr
				}
				lowestRangeErrMutex.Unlock()
				stopOnce.Do(func() {
					close(stop)
				})
				return
			}
		}()
	}

	lowestLedger := enqueueReingestTasks(ledgerRanges, batchSize, stop, reingestJobQueue)

	stopOnce.Do(func() {
		close(stop)
	})
	wg.Wait()
	close(reingestJobQueue)

	if lowestRangeErr != nil {
		lastLedger := ledgerRanges[len(ledgerRanges)-1].EndSequence
		if err := ps.rebuildTradeAggRanges([]history.LedgerRange{{StartSequence: lowestLedger, EndSequence: lowestRangeErr.ledgerRange.StartSequence}}); err != nil {
			log.WithError(err).Errorf("error when trying to rebuild trade agg for partially completed portion of overall parallel reingestion range, start=%v, stop=%v", lowestLedger, lowestRangeErr.ledgerRange.StartSequence)
		}
		return errors.Wrapf(lowestRangeErr, "job failed, recommended restart range: [%d, %d]", lowestRangeErr.ledgerRange.StartSequence, lastLedger)
	}
	if err := ps.rebuildTradeAggRanges(ledgerRanges); err != nil {
		return err
	}
	return nil
}

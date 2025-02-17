package ingest

import (
	"fmt"
	"time"

	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/support/errors"
	logpkg "github.com/stellar/go/support/log"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

type reingestHistoryRangeState struct {
	fromLedger uint32
	toLedger   uint32
	force      bool
}

func (h reingestHistoryRangeState) String() string {
	return fmt.Sprintf(
		"reingestHistoryRange(fromLedger=%d, toLedger=%d, force=%t)",
		h.fromLedger,
		h.toLedger,
		h.force,
	)
}

func (reingestHistoryRangeState) GetState() State {
	return ReingestHistoryRange
}

func (h reingestHistoryRangeState) ingestRange(s *system, fromLedger, toLedger uint32, execBatchInTx bool) error {
	if s.maxLedgerPerFlush < 1 {
		return errors.New("invalid maxLedgerPerFlush, must be greater than 0")
	}

	// Clear history data before ingesting - used in `reingest range` command.
	start, end, err := toid.LedgerRangeInclusive(
		int32(fromLedger),
		int32(toLedger),
	)
	if err != nil {
		return errors.Wrap(err, "Invalid range")
	}

	_, err = s.historyQ.DeleteRangeAll(s.ctx, start, end)
	if err != nil {
		return errors.Wrap(err, "error in DeleteRangeAll")
	}

	// s.maxLedgerPerFlush has been validated to be at least 1
	ledgers := make([]xdr.LedgerCloseMeta, 0, s.maxLedgerPerFlush)

	for cur := fromLedger; cur <= toLedger; cur++ {
		var ledgerCloseMeta xdr.LedgerCloseMeta

		log.WithField("sequence", cur).Info("Waiting for ledger to be available in the backend...")
		startTime := time.Now()
		ledgerCloseMeta, err = s.ledgerBackend.GetLedger(s.ctx, cur)

		if err != nil {
			return errors.Wrap(err, "error getting ledger")
		}

		log.WithFields(logpkg.F{
			"sequence": cur,
			"duration": time.Since(startTime).Seconds(),
		}).Info("Ledger returned from the backend")

		ledgers = append(ledgers, ledgerCloseMeta)

		if len(ledgers)%int(s.maxLedgerPerFlush) == 0 {
			if err = s.runner.RunTransactionProcessorsOnLedgers(ledgers, execBatchInTx); err != nil {
				return errors.Wrapf(err, "error processing ledger range %d - %d", ledgers[0].LedgerSequence(), ledgers[len(ledgers)-1].LedgerSequence())
			}
			ledgers = ledgers[0:0]
		}
	}

	if len(ledgers) > 0 {
		if err = s.runner.RunTransactionProcessorsOnLedgers(ledgers, execBatchInTx); err != nil {
			return errors.Wrapf(err, "error processing ledger range %d - %d", ledgers[0].LedgerSequence(), ledgers[len(ledgers)-1].LedgerSequence())
		}
	}

	return nil
}

func (h reingestHistoryRangeState) prepareRange(s *system) (transition, error) {
	log.WithFields(logpkg.F{
		"from": h.fromLedger,
		"to":   h.toLedger,
	}).Info("Preparing ledger backend to retrieve range")
	startTime := time.Now()

	err := s.ledgerBackend.PrepareRange(s.ctx, ledgerbackend.BoundedRange(h.fromLedger, h.toLedger))
	if err != nil {
		return stop(), errors.Wrap(err, "error preparing range")
	}

	log.WithFields(logpkg.F{
		"from":     h.fromLedger,
		"to":       h.toLedger,
		"duration": time.Since(startTime).Seconds(),
	}).Info("Range ready")

	return transition{}, nil
}

// reingestHistoryRangeState is used as a command to reingest historical data
func (h reingestHistoryRangeState) run(s *system) (transition, error) {
	if h.fromLedger == 0 || h.toLedger == 0 ||
		h.fromLedger > h.toLedger {
		return stop(), errors.Errorf("invalid range: [%d, %d]", h.fromLedger, h.toLedger)
	}

	if h.fromLedger == 1 {
		log.Warn("Ledger 1 is pregenerated and not available, starting from ledger 2.")
		h.fromLedger = 2
	}

	var startTime time.Time

	if h.force {
		if t, err := h.prepareRange(s); err != nil {
			return t, err
		}

		startTime = time.Now()
		if err := s.historyQ.Begin(s.ctx); err != nil {
			return stop(), errors.Wrap(err, "Error starting a transaction")
		}
		defer s.historyQ.Rollback()

		// acquire distributed lock so no one else can perform ingestion operations.
		if _, err := s.historyQ.GetLastLedgerIngest(s.ctx); err != nil {
			return stop(), errors.Wrap(err, getLastIngestedErrMsg)
		}

		if ingestErr := h.ingestRange(s, h.fromLedger, h.toLedger, false); ingestErr != nil {
			if err := s.historyQ.Commit(); err != nil {
				return stop(), errors.Wrap(ingestErr, commitErrMsg)
			}
			return stop(), ingestErr
		}

		if err := s.historyQ.Commit(); err != nil {
			return stop(), errors.Wrap(err, commitErrMsg)
		}
	} else {
		lastIngestedLedger, err := s.historyQ.GetLastLedgerIngestNonBlocking(s.ctx)
		if err != nil {
			return stop(), errors.Wrap(err, getLastIngestedErrMsg)
		}

		if lastIngestedLedger > 0 && h.toLedger >= lastIngestedLedger {
			return stop(), ErrReingestRangeConflict{lastIngestedLedger}
		}

		// Only prepare the range after checking the bounds to enable an early error return
		var t transition
		if t, err = h.prepareRange(s); err != nil {
			return t, err
		}

		startTime = time.Now()
		if e := h.ingestRange(s, h.fromLedger, h.toLedger, true); e != nil {
			return stop(), e
		}
	}

	log.WithFields(logpkg.F{
		"from":     h.fromLedger,
		"to":       h.toLedger,
		"duration": time.Since(startTime).Seconds(),
	}).Info("Reingestion done")

	return stop(), nil
}

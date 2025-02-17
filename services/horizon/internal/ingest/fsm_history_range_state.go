package ingest

import (
	"fmt"
	"time"

	"github.com/stellar/go/support/errors"
	logpkg "github.com/stellar/go/support/log"
	"github.com/stellar/go/xdr"
)

type historyRangeState struct {
	fromLedger uint32
	toLedger   uint32
}

func (h historyRangeState) String() string {
	return fmt.Sprintf(
		"historyRange(fromLedger=%d, toLedger=%d)",
		h.fromLedger,
		h.toLedger,
	)
}

func (historyRangeState) GetState() State {
	return HistoryRange
}

// historyRangeState is used when catching up history data
func (h historyRangeState) run(s *system) (transition, error) {
	if h.fromLedger == 0 || h.toLedger == 0 ||
		h.fromLedger > h.toLedger {
		return start(), errors.Errorf("invalid range: [%d, %d]", h.fromLedger, h.toLedger)
	}

	if s.maxLedgerPerFlush < 1 {
		return start(), errors.New("invalid maxLedgerPerFlush, must be greater than 0")
	}

	err := s.maybePrepareRange(s.ctx, h.fromLedger)
	if err != nil {
		return start(), err
	}

	if err = s.historyQ.Begin(s.ctx); err != nil {
		return start(), errors.Wrap(err, "Error starting a transaction")
	}
	defer s.historyQ.Rollback()

	// acquire distributed lock so no one else can perform ingestion operations.
	if _, err = s.historyQ.GetLastLedgerIngest(s.ctx); err != nil {
		return start(), errors.Wrap(err, getLastIngestedErrMsg)
	}

	lastHistoryLedger, err := s.historyQ.GetLatestHistoryLedger(s.ctx)
	if err != nil {
		return start(), errors.Wrap(err, "could not get latest history ledger")
	}

	// We should be ingesting the ledger which occurs after
	// lastHistoryLedger. Otherwise, some other horizon node has
	// already completed the ingest history range operation and
	// we should go back to the init state
	if lastHistoryLedger != h.fromLedger-1 {
		return start(), nil
	}

	ledgers := make([]xdr.LedgerCloseMeta, 0, s.maxLedgerPerFlush)
	for cur := h.fromLedger; cur <= h.toLedger; cur++ {
		var ledgerCloseMeta xdr.LedgerCloseMeta

		log.WithField("sequence", cur).Info("Waiting for ledger to be available in the backend...")
		startTime := time.Now()

		ledgerCloseMeta, err = s.ledgerBackend.GetLedger(s.ctx, cur)
		if err != nil {
			// Commit prior batches that have been flushed in case of ledger backend error.
			commitErr := s.historyQ.Commit()
			if commitErr != nil {
				log.WithError(commitErr).Error("Error committing partial range results")
			} else {
				log.Info("Committed partial range results")
			}
			return start(), errors.Wrap(err, "error getting ledger")
		}

		log.WithFields(logpkg.F{
			"sequence": cur,
			"duration": time.Since(startTime).Seconds(),
		}).Info("Ledger returned from the backend")
		ledgers = append(ledgers, ledgerCloseMeta)

		if len(ledgers) == cap(ledgers) {
			if err = s.runner.RunTransactionProcessorsOnLedgers(ledgers, false); err != nil {
				return start(), errors.Wrapf(err, "error processing ledger range %d - %d", ledgers[0].LedgerSequence(), ledgers[len(ledgers)-1].LedgerSequence())
			}
			ledgers = ledgers[0:0]
		}
	}

	if len(ledgers) > 0 {
		if err = s.runner.RunTransactionProcessorsOnLedgers(ledgers, false); err != nil {
			return start(), errors.Wrapf(err, "error processing ledger range %d - %d", ledgers[0].LedgerSequence(), ledgers[len(ledgers)-1].LedgerSequence())
		}
	}
	if err = s.historyQ.Commit(); err != nil {
		return start(), errors.Wrap(err, commitErrMsg)
	}

	return start(), nil
}

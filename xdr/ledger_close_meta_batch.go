package xdr

import (
	"fmt"
)

// GetLedger retrieves the LedgerCloseMeta for a given sequence number.
// It returns an error if LedgerCloseMeta for the sequence number is not found in the batch.
func (s *LedgerCloseMetaBatch) GetLedger(sequence uint32) (LedgerCloseMeta, error) {

	if sequence < uint32(s.StartSequence) || sequence > uint32(s.EndSequence) {
		return LedgerCloseMeta{}, fmt.Errorf("ledger sequence %d is outside the "+
			"valid range of ledger sequences [%d, %d] this batch holds",
			sequence, s.StartSequence, s.EndSequence)
	}

	ledgerIndex := sequence - uint32(s.StartSequence)
	if ledgerIndex >= uint32(len(s.LedgerCloseMetas)) {
		return LedgerCloseMeta{}, fmt.Errorf("LedgerCloseMeta for sequence %d not found in the batch", sequence)
	}
	return s.LedgerCloseMetas[ledgerIndex], nil
}

// AddLedger adds a LedgerCloseMeta to the batch.
func (s *LedgerCloseMetaBatch) AddLedger(ledgerCloseMeta LedgerCloseMeta) error {
	if ledgerCloseMeta.LedgerSequence() < uint32(s.StartSequence) ||
		ledgerCloseMeta.LedgerSequence() > uint32(s.EndSequence) {
		return fmt.Errorf("ledger sequence %d is outside valid range [%d, %d]",
			ledgerCloseMeta.LedgerSequence(), s.StartSequence, s.EndSequence)
	}

	if len(s.LedgerCloseMetas) > 0 {
		lastSequence := s.LedgerCloseMetas[len(s.LedgerCloseMetas)-1].LedgerSequence()
		if ledgerCloseMeta.LedgerSequence() != lastSequence+1 {
			return fmt.Errorf("ledgers must be added sequentially: expected sequence %d, got %d",
				lastSequence+1, ledgerCloseMeta.LedgerSequence())
		}
	}
	s.LedgerCloseMetas = append(s.LedgerCloseMetas, ledgerCloseMeta)
	return nil
}

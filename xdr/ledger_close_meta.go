package xdr

import (
	"fmt"
)

func (l LedgerCloseMeta) LedgerHeaderHistoryEntry() LedgerHeaderHistoryEntry {
	switch l.V {
	case 0:
		return l.MustV0().LedgerHeader
	case 1:
		return l.MustV1().LedgerHeader
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

func (l LedgerCloseMeta) LedgerSequence() uint32 {
	return uint32(l.LedgerHeaderHistoryEntry().Header.LedgerSeq)
}

func (l LedgerCloseMeta) LedgerCloseTime() int64 {
	return int64(l.LedgerHeaderHistoryEntry().Header.ScpValue.CloseTime)
}

func (l LedgerCloseMeta) LedgerHash() Hash {
	return l.LedgerHeaderHistoryEntry().Hash
}

func (l LedgerCloseMeta) PreviousLedgerHash() Hash {
	return l.LedgerHeaderHistoryEntry().Header.PreviousLedgerHash
}

func (l LedgerCloseMeta) ProtocolVersion() uint32 {
	return uint32(l.LedgerHeaderHistoryEntry().Header.LedgerVersion)
}

func (l LedgerCloseMeta) BucketListHash() Hash {
	return l.LedgerHeaderHistoryEntry().Header.BucketListHash
}

func (l LedgerCloseMeta) CountTransactions() int {
	switch l.V {
	case 0:
		return len(l.MustV0().TxProcessing)
	case 1:
		return len(l.MustV1().TxProcessing)

	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

func (l LedgerCloseMeta) TransactionEnvelopes() []TransactionEnvelope {
	switch l.V {
	case 0:
		return l.MustV0().TxSet.Txs
	case 1:
		var envelopes = make([]TransactionEnvelope, 0, l.CountTransactions())
		for _, phase := range l.MustV1().TxSet.V1TxSet.Phases {
			for _, component := range *phase.V0Components {
				envelopes = append(envelopes, component.TxsMaybeDiscountedFee.Txs...)
			}
		}
		return envelopes
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

// TransactionHash returns Hash for tx at index i in processing order..
func (l LedgerCloseMeta) TransactionHash(i int) Hash {
	switch l.V {
	case 0:
		return l.MustV0().TxProcessing[i].Result.TransactionHash
	case 1:
		return l.MustV1().TxProcessing[i].Result.TransactionHash
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

// TransactionResultPair returns TransactionResultPair for tx at index i in processing order.
func (l LedgerCloseMeta) TransactionResultPair(i int) TransactionResultPair {
	switch l.V {
	case 0:
		return l.MustV0().TxProcessing[i].Result
	case 1:
		return l.MustV1().TxProcessing[i].Result
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

// FeeProcessing returns FeeProcessing for tx at index i in processing order.
func (l LedgerCloseMeta) FeeProcessing(i int) LedgerEntryChanges {
	switch l.V {
	case 0:
		return l.MustV0().TxProcessing[i].FeeProcessing
	case 1:
		return l.MustV1().TxProcessing[i].FeeProcessing
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

// TxApplyProcessing returns TxApplyProcessing for tx at index i in processing order.
func (l LedgerCloseMeta) TxApplyProcessing(i int) TransactionMeta {
	switch l.V {
	case 0:
		return l.MustV0().TxProcessing[i].TxApplyProcessing
	case 1:
		if l.MustV1().TxProcessing[i].TxApplyProcessing.V != 3 {
			panic("TransactionResult unavailable because LedgerCloseMeta.V = 1 and TransactionMeta.V != 3")
		}
		return l.MustV1().TxProcessing[i].TxApplyProcessing
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

// UpgradesProcessing returns UpgradesProcessing for ledger.
func (l LedgerCloseMeta) UpgradesProcessing() []UpgradeEntryMeta {
	switch l.V {
	case 0:
		return l.MustV0().UpgradesProcessing
	case 1:
		return l.MustV1().UpgradesProcessing
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

// EvictedTemporaryLedgerKeys returns a slice of ledger keys for
// temporary ledger entries that have been evicted in this ledger.
func (l LedgerCloseMeta) EvictedTemporaryLedgerKeys() ([]LedgerKey, error) {
	switch l.V {
	case 0:
		return nil, nil
	case 1:
		return l.MustV1().EvictedTemporaryLedgerKeys, nil
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

// EvictedPersistentLedgerEntries returns the persistent ledger entries
// which have been evicted in this ledger.
func (l LedgerCloseMeta) EvictedPersistentLedgerEntries() ([]LedgerEntry, error) {
	switch l.V {
	case 0:
		return nil, nil
	case 1:
		return l.MustV1().EvictedPersistentLedgerEntries, nil
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

// TxProcessing returns the TransactionResultMeta in this ledger
func (l LedgerCloseMeta) TxProcessing() ([]TransactionResultMeta, error) {
	switch l.V {
	case 0:
		return l.MustV0().TxProcessing, nil
	case 1:
		return l.MustV1().TxProcessing, nil
	default:
		return []TransactionResultMeta{}, fmt.Errorf("Unsupported LedgerCloseMeta.V: %d", l.V)
	}
}

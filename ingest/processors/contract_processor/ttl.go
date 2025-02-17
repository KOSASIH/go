package contract

import (
	"fmt"
	"time"

	"github.com/stellar/go/ingest"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// TtlOutput is a representation of soroban ttl that aligns with the Bigquery table ttls
type TtlOutput struct {
	KeyHash            string    `json:"key_hash"` // key_hash is contract_code_hash or contract_id
	LiveUntilLedgerSeq uint32    `json:"live_until_ledger_seq"`
	LastModifiedLedger uint32    `json:"last_modified_ledger"`
	LedgerEntryChange  uint32    `json:"ledger_entry_change"`
	Deleted            bool      `json:"deleted"`
	ClosedAt           time.Time `json:"closed_at"`
	LedgerSequence     uint32    `json:"ledger_sequence"`
}

// TransformTtl converts an ttl ledger change entry into a form suitable for BigQuery
func TransformTtl(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) (TtlOutput, error) {
	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return TtlOutput{}, err
	}

	ttl, ok := ledgerEntry.Data.GetTtl()
	if !ok {
		return TtlOutput{}, fmt.Errorf("could not extract ttl from ledger entry; actual type is %s", ledgerEntry.Data.Type)
	}

	// LedgerEntryChange must contain a ttl change to be parsed, otherwise skip
	if ledgerEntry.Data.Type != xdr.LedgerEntryTypeTtl {
		return TtlOutput{}, nil
	}

	keyHash := ttl.KeyHash.HexString()
	liveUntilLedgerSeq := ttl.LiveUntilLedgerSeq

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return TtlOutput{}, err
	}

	ledgerSequence := header.Header.LedgerSeq

	transformedPool := TtlOutput{
		KeyHash:            keyHash,
		LiveUntilLedgerSeq: uint32(liveUntilLedgerSeq),
		LastModifiedLedger: uint32(ledgerEntry.LastModifiedLedgerSeq),
		LedgerEntryChange:  uint32(changeType),
		Deleted:            outputDeleted,
		ClosedAt:           closedAt,
		LedgerSequence:     uint32(ledgerSequence),
	}

	return transformedPool, nil
}

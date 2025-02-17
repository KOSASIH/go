package account

import (
	"fmt"
	"sort"
	"time"

	"github.com/guregu/null"
	"github.com/stellar/go/ingest"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// AccountSignerOutput is a representation of an account signer that aligns with the BigQuery table account_signers
type AccountSignerOutput struct {
	AccountID          string      `json:"account_id"`
	Signer             string      `json:"signer"`
	Weight             int32       `json:"weight"`
	Sponsor            null.String `json:"sponsor"`
	LastModifiedLedger uint32      `json:"last_modified_ledger"`
	LedgerEntryChange  uint32      `json:"ledger_entry_change"`
	Deleted            bool        `json:"deleted"`
	ClosedAt           time.Time   `json:"closed_at"`
	LedgerSequence     uint32      `json:"ledger_sequence"`
}

// TransformAccountSigners converts account signers from the history archive ingestion system into a form suitable for BigQuery
func TransformAccountSigners(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) ([]AccountSignerOutput, error) {
	var signers []AccountSignerOutput

	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return signers, err
	}
	outputLastModifiedLedger := uint32(ledgerEntry.LastModifiedLedgerSeq)
	accountEntry, accountFound := ledgerEntry.Data.GetAccount()
	if !accountFound {
		return signers, fmt.Errorf("could not extract signer data from ledger entry of type: %+v", ledgerEntry.Data.Type)
	}

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return signers, err
	}

	ledgerSequence := header.Header.LedgerSeq

	sponsors := accountEntry.SponsorPerSigner()
	for signer, weight := range accountEntry.SignerSummary() {
		var sponsor null.String
		if sponsorDesc, isSponsored := sponsors[signer]; isSponsored {
			sponsor = null.StringFrom(sponsorDesc.Address())
		}

		signers = append(signers, AccountSignerOutput{
			AccountID:          accountEntry.AccountId.Address(),
			Signer:             signer,
			Weight:             weight,
			Sponsor:            sponsor,
			LastModifiedLedger: outputLastModifiedLedger,
			LedgerEntryChange:  uint32(changeType),
			Deleted:            outputDeleted,
			ClosedAt:           closedAt,
			LedgerSequence:     uint32(ledgerSequence),
		})
	}
	sort.Slice(signers, func(a, b int) bool { return signers[a].Weight < signers[b].Weight })
	return signers, nil
}

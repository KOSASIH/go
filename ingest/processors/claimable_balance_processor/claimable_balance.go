package claimablebalance

import (
	"fmt"
	"time"

	"github.com/guregu/null"
	"github.com/stellar/go/ingest"
	asset "github.com/stellar/go/ingest/processors/asset_processor"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// ClaimableBalanceOutput is a representation of a claimable balances that aligns with the BigQuery table claimable_balances
type ClaimableBalanceOutput struct {
	BalanceID          string           `json:"balance_id"`
	Claimants          []utils.Claimant `json:"claimants"`
	AssetCode          string           `json:"asset_code"`
	AssetIssuer        string           `json:"asset_issuer"`
	AssetType          string           `json:"asset_type"`
	AssetID            int64            `json:"asset_id"`
	AssetAmount        float64          `json:"asset_amount"`
	Sponsor            null.String      `json:"sponsor"`
	Flags              uint32           `json:"flags"`
	LastModifiedLedger uint32           `json:"last_modified_ledger"`
	LedgerEntryChange  uint32           `json:"ledger_entry_change"`
	Deleted            bool             `json:"deleted"`
	ClosedAt           time.Time        `json:"closed_at"`
	LedgerSequence     uint32           `json:"ledger_sequence"`
}

func TransformClaimants(claimants []xdr.Claimant) []utils.Claimant {
	var transformed []utils.Claimant
	for _, c := range claimants {
		cv0 := c.MustV0()
		transformed = append(transformed, utils.Claimant{
			Destination: cv0.Destination.Address(),
			Predicate:   cv0.Predicate,
		})
	}
	return transformed
}

// TransformClaimableBalance converts a claimable balance from the history archive ingestion system into a form suitable for BigQuery
func TransformClaimableBalance(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) (ClaimableBalanceOutput, error) {
	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return ClaimableBalanceOutput{}, err
	}

	balanceEntry, balanceFound := ledgerEntry.Data.GetClaimableBalance()
	if !balanceFound {
		return ClaimableBalanceOutput{}, fmt.Errorf("could not extract claimable balance data from ledger entry; actual type is %s", ledgerEntry.Data.Type)
	}
	balanceID, err := xdr.MarshalHex(balanceEntry.BalanceId)
	if err != nil {
		return ClaimableBalanceOutput{}, fmt.Errorf("invalid balanceId in op: %d", uint32(ledgerEntry.LastModifiedLedgerSeq))
	}
	outputFlags := uint32(balanceEntry.Flags())
	outputAsset, err := asset.TransformSingleAsset(balanceEntry.Asset)
	if err != nil {
		return ClaimableBalanceOutput{}, err
	}
	outputClaimants := TransformClaimants(balanceEntry.Claimants)
	outputAmount := balanceEntry.Amount

	outputLastModifiedLedger := uint32(ledgerEntry.LastModifiedLedgerSeq)

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return ClaimableBalanceOutput{}, err
	}

	ledgerSequence := header.Header.LedgerSeq

	transformed := ClaimableBalanceOutput{
		BalanceID:          balanceID,
		AssetCode:          outputAsset.AssetCode,
		AssetIssuer:        outputAsset.AssetIssuer,
		AssetType:          outputAsset.AssetType,
		AssetID:            outputAsset.AssetID,
		Claimants:          outputClaimants,
		AssetAmount:        float64(outputAmount) / 1.0e7,
		Sponsor:            utils.LedgerEntrySponsorToNullString(ledgerEntry),
		LastModifiedLedger: outputLastModifiedLedger,
		LedgerEntryChange:  uint32(changeType),
		Flags:              outputFlags,
		Deleted:            outputDeleted,
		ClosedAt:           closedAt,
		LedgerSequence:     uint32(ledgerSequence),
	}
	return transformed, nil
}

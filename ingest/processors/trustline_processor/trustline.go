package trustline

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/guregu/null"
	"github.com/pkg/errors"

	"github.com/stellar/go/ingest"
	operations "github.com/stellar/go/ingest/processors/operation_processor"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// TrustlineOutput is a representation of a trustline that aligns with the BigQuery table trust_lines
type TrustlineOutput struct {
	LedgerKey          string      `json:"ledger_key"`
	AccountID          string      `json:"account_id"`
	AssetCode          string      `json:"asset_code"`
	AssetIssuer        string      `json:"asset_issuer"`
	AssetType          string      `json:"asset_type"`
	AssetID            int64       `json:"asset_id"`
	Balance            float64     `json:"balance"`
	TrustlineLimit     int64       `json:"trust_line_limit"`
	LiquidityPoolID    string      `json:"liquidity_pool_id"`
	BuyingLiabilities  float64     `json:"buying_liabilities"`
	SellingLiabilities float64     `json:"selling_liabilities"`
	Flags              uint32      `json:"flags"`
	LastModifiedLedger uint32      `json:"last_modified_ledger"`
	LedgerEntryChange  uint32      `json:"ledger_entry_change"`
	Sponsor            null.String `json:"sponsor"`
	Deleted            bool        `json:"deleted"`
	ClosedAt           time.Time   `json:"closed_at"`
	LedgerSequence     uint32      `json:"ledger_sequence"`
}

// TransformTrustline converts a trustline from the history archive ingestion system into a form suitable for BigQuery
func TransformTrustline(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) (TrustlineOutput, error) {
	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return TrustlineOutput{}, err
	}

	trustEntry, ok := ledgerEntry.Data.GetTrustLine()
	if !ok {
		return TrustlineOutput{}, fmt.Errorf("could not extract trustline data from ledger entry; actual type is %s", ledgerEntry.Data.Type)
	}

	outputAccountID, err := trustEntry.AccountId.GetAddress()
	if err != nil {
		return TrustlineOutput{}, err
	}

	var assetType, outputAssetCode, outputAssetIssuer, poolID string

	asset := trustEntry.Asset

	outputLedgerKey, err := trustLineEntryToLedgerKeyString(trustEntry)
	if err != nil {
		return TrustlineOutput{}, errors.Wrap(err, fmt.Sprintf("could not create ledger key string for trustline with account %s and asset %s", outputAccountID, asset.ToAsset().StringCanonical()))
	}

	if asset.Type == xdr.AssetTypeAssetTypePoolShare {
		poolID = operations.PoolIDToString(trustEntry.Asset.MustLiquidityPoolId())
		assetType = "pool_share"
	} else {
		if err = asset.Extract(&assetType, &outputAssetCode, &outputAssetIssuer); err != nil {
			return TrustlineOutput{}, errors.Wrap(err, fmt.Sprintf("could not parse asset for trustline with account %s", outputAccountID))
		}
	}

	outputAssetID := utils.FarmHashAsset(outputAssetCode, outputAssetIssuer, asset.Type.String())

	liabilities := trustEntry.Liabilities()

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return TrustlineOutput{}, err
	}

	ledgerSequence := header.Header.LedgerSeq

	transformedTrustline := TrustlineOutput{
		LedgerKey:          outputLedgerKey,
		AccountID:          outputAccountID,
		AssetType:          assetType,
		AssetCode:          outputAssetCode,
		AssetIssuer:        outputAssetIssuer,
		AssetID:            outputAssetID,
		Balance:            utils.ConvertStroopValueToReal(trustEntry.Balance),
		TrustlineLimit:     int64(trustEntry.Limit),
		LiquidityPoolID:    poolID,
		BuyingLiabilities:  utils.ConvertStroopValueToReal(liabilities.Buying),
		SellingLiabilities: utils.ConvertStroopValueToReal(liabilities.Selling),
		Flags:              uint32(trustEntry.Flags),
		LastModifiedLedger: uint32(ledgerEntry.LastModifiedLedgerSeq),
		LedgerEntryChange:  uint32(changeType),
		Sponsor:            utils.LedgerEntrySponsorToNullString(ledgerEntry),
		Deleted:            outputDeleted,
		ClosedAt:           closedAt,
		LedgerSequence:     uint32(ledgerSequence),
	}

	return transformedTrustline, nil
}

func trustLineEntryToLedgerKeyString(trustLine xdr.TrustLineEntry) (string, error) {
	ledgerKey := &xdr.LedgerKey{}
	err := ledgerKey.SetTrustline(trustLine.AccountId, trustLine.Asset)
	if err != nil {
		return "", fmt.Errorf("error running ledgerKey.SetTrustline when calculating ledger key")
	}

	key, err := ledgerKey.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("error running MarshalBinaryCompress when calculating ledger key")
	}

	return base64.StdEncoding.EncodeToString(key), nil

}

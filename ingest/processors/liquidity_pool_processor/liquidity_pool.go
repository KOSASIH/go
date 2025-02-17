package liquiditypool

import (
	"fmt"
	"time"

	"github.com/stellar/go/ingest"
	operations "github.com/stellar/go/ingest/processors/operation_processor"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// PoolOutput is a representation of a liquidity pool that aligns with the Bigquery table liquidity_pools
type PoolOutput struct {
	PoolID             string    `json:"liquidity_pool_id"`
	PoolType           string    `json:"type"`
	PoolFee            uint32    `json:"fee"`
	TrustlineCount     uint64    `json:"trustline_count"`
	PoolShareCount     float64   `json:"pool_share_count"`
	AssetAType         string    `json:"asset_a_type"`
	AssetACode         string    `json:"asset_a_code"`
	AssetAIssuer       string    `json:"asset_a_issuer"`
	AssetAReserve      float64   `json:"asset_a_amount"`
	AssetAID           int64     `json:"asset_a_id"`
	AssetBType         string    `json:"asset_b_type"`
	AssetBCode         string    `json:"asset_b_code"`
	AssetBIssuer       string    `json:"asset_b_issuer"`
	AssetBReserve      float64   `json:"asset_b_amount"`
	AssetBID           int64     `json:"asset_b_id"`
	LastModifiedLedger uint32    `json:"last_modified_ledger"`
	LedgerEntryChange  uint32    `json:"ledger_entry_change"`
	Deleted            bool      `json:"deleted"`
	ClosedAt           time.Time `json:"closed_at"`
	LedgerSequence     uint32    `json:"ledger_sequence"`
}

// TransformPool converts an liquidity pool ledger change entry into a form suitable for BigQuery
func TransformPool(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) (PoolOutput, error) {
	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return PoolOutput{}, err
	}

	// LedgerEntryChange must contain a liquidity pool state change to be parsed, otherwise skip
	if ledgerEntry.Data.Type != xdr.LedgerEntryTypeLiquidityPool {
		return PoolOutput{}, nil
	}

	lp, ok := ledgerEntry.Data.GetLiquidityPool()
	if !ok {
		return PoolOutput{}, fmt.Errorf("could not extract liquidity pool data from ledger entry; actual type is %s", ledgerEntry.Data.Type)
	}

	cp, ok := lp.Body.GetConstantProduct()
	if !ok {
		return PoolOutput{}, fmt.Errorf("could not extract constant product information for liquidity pool %s", xdr.Hash(lp.LiquidityPoolId).HexString())
	}

	poolType, ok := xdr.LiquidityPoolTypeToString[lp.Body.Type]
	if !ok {
		return PoolOutput{}, fmt.Errorf("unknown liquidity pool type: %d", lp.Body.Type)
	}

	var assetAType, assetACode, assetAIssuer string
	err = cp.Params.AssetA.Extract(&assetAType, &assetACode, &assetAIssuer)
	if err != nil {
		return PoolOutput{}, err
	}
	assetAID := utils.FarmHashAsset(assetACode, assetAIssuer, assetAType)

	var assetBType, assetBCode, assetBIssuer string
	err = cp.Params.AssetB.Extract(&assetBType, &assetBCode, &assetBIssuer)
	if err != nil {
		return PoolOutput{}, err
	}
	assetBID := utils.FarmHashAsset(assetBCode, assetBIssuer, assetBType)

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return PoolOutput{}, err
	}

	ledgerSequence := header.Header.LedgerSeq

	transformedPool := PoolOutput{
		PoolID:             operations.PoolIDToString(lp.LiquidityPoolId),
		PoolType:           poolType,
		PoolFee:            uint32(cp.Params.Fee),
		TrustlineCount:     uint64(cp.PoolSharesTrustLineCount),
		PoolShareCount:     utils.ConvertStroopValueToReal(cp.TotalPoolShares),
		AssetAType:         assetAType,
		AssetACode:         assetACode,
		AssetAIssuer:       assetAIssuer,
		AssetAID:           assetAID,
		AssetAReserve:      utils.ConvertStroopValueToReal(cp.ReserveA),
		AssetBType:         assetBType,
		AssetBCode:         assetBCode,
		AssetBIssuer:       assetBIssuer,
		AssetBID:           assetBID,
		AssetBReserve:      utils.ConvertStroopValueToReal(cp.ReserveB),
		LastModifiedLedger: uint32(ledgerEntry.LastModifiedLedgerSeq),
		LedgerEntryChange:  uint32(changeType),
		Deleted:            outputDeleted,
		ClosedAt:           closedAt,
		LedgerSequence:     uint32(ledgerSequence),
	}
	return transformedPool, nil
}

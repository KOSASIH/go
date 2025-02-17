package asset

import (
	"fmt"
	"time"

	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

// AssetOutput is a representation of an asset that aligns with the BigQuery table history_assets
type AssetOutput struct {
	AssetCode      string    `json:"asset_code"`
	AssetIssuer    string    `json:"asset_issuer"`
	AssetType      string    `json:"asset_type"`
	AssetID        int64     `json:"asset_id"`
	ClosedAt       time.Time `json:"closed_at"`
	LedgerSequence uint32    `json:"ledger_sequence"`
}

// TransformAsset converts an asset from a payment operation into a form suitable for BigQuery
func TransformAsset(operation xdr.Operation, operationIndex int32, transactionIndex int32, ledgerSeq int32, lcm xdr.LedgerCloseMeta) (AssetOutput, error) {
	operationID := toid.New(ledgerSeq, int32(transactionIndex), operationIndex).ToInt64()

	opType := operation.Body.Type
	if opType != xdr.OperationTypePayment && opType != xdr.OperationTypeManageSellOffer {
		return AssetOutput{}, fmt.Errorf("operation of type %d cannot issue an asset (id %d)", opType, operationID)
	}

	asset := xdr.Asset{}
	switch opType {
	case xdr.OperationTypeManageSellOffer:
		opSellOf, ok := operation.Body.GetManageSellOfferOp()
		if !ok {
			return AssetOutput{}, fmt.Errorf("operation of type ManageSellOfferOp cannot issue an asset (id %d)", operationID)
		}
		asset = opSellOf.Selling

	case xdr.OperationTypePayment:
		opPayment, ok := operation.Body.GetPaymentOp()
		if !ok {
			return AssetOutput{}, fmt.Errorf("could not access Payment info for this operation (id %d)", operationID)
		}
		asset = opPayment.Asset

	}

	outputAsset, err := TransformSingleAsset(asset)
	if err != nil {
		return AssetOutput{}, fmt.Errorf("%s (id %d)", err.Error(), operationID)
	}

	outputCloseTime, err := utils.GetCloseTime(lcm)
	if err != nil {
		return AssetOutput{}, err
	}
	outputAsset.ClosedAt = outputCloseTime
	outputAsset.LedgerSequence = utils.GetLedgerSequence(lcm)

	return outputAsset, nil
}

func TransformSingleAsset(asset xdr.Asset) (AssetOutput, error) {
	var outputAssetType, outputAssetCode, outputAssetIssuer string
	err := asset.Extract(&outputAssetType, &outputAssetCode, &outputAssetIssuer)
	if err != nil {
		return AssetOutput{}, fmt.Errorf("could not extract asset from this operation")
	}

	farmAssetID := utils.FarmHashAsset(outputAssetCode, outputAssetIssuer, outputAssetType)

	return AssetOutput{
		AssetCode:   outputAssetCode,
		AssetIssuer: outputAssetIssuer,
		AssetType:   outputAssetType,
		AssetID:     farmAssetID,
	}, nil
}

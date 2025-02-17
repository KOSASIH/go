package offer

import (
	"fmt"
	"time"

	"github.com/guregu/null"
	"github.com/stellar/go/ingest"
	asset "github.com/stellar/go/ingest/processors/asset_processor"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// OfferOutput is a representation of an offer that aligns with the BigQuery table offers
type OfferOutput struct {
	SellerID           string      `json:"seller_id"` // Account address of the seller
	OfferID            int64       `json:"offer_id"`
	SellingAssetType   string      `json:"selling_asset_type"`
	SellingAssetCode   string      `json:"selling_asset_code"`
	SellingAssetIssuer string      `json:"selling_asset_issuer"`
	SellingAssetID     int64       `json:"selling_asset_id"`
	BuyingAssetType    string      `json:"buying_asset_type"`
	BuyingAssetCode    string      `json:"buying_asset_code"`
	BuyingAssetIssuer  string      `json:"buying_asset_issuer"`
	BuyingAssetID      int64       `json:"buying_asset_id"`
	Amount             float64     `json:"amount"`
	PriceN             int32       `json:"pricen"`
	PriceD             int32       `json:"priced"`
	Price              float64     `json:"price"`
	Flags              uint32      `json:"flags"`
	LastModifiedLedger uint32      `json:"last_modified_ledger"`
	LedgerEntryChange  uint32      `json:"ledger_entry_change"`
	Deleted            bool        `json:"deleted"`
	Sponsor            null.String `json:"sponsor"`
	ClosedAt           time.Time   `json:"closed_at"`
	LedgerSequence     uint32      `json:"ledger_sequence"`
}

// TransformOffer converts an account from the history archive ingestion system into a form suitable for BigQuery
func TransformOffer(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) (OfferOutput, error) {
	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return OfferOutput{}, err
	}

	offerEntry, offerFound := ledgerEntry.Data.GetOffer()
	if !offerFound {
		return OfferOutput{}, fmt.Errorf("could not extract offer data from ledger entry; actual type is %s", ledgerEntry.Data.Type)
	}

	outputSellerID, err := offerEntry.SellerId.GetAddress()
	if err != nil {
		return OfferOutput{}, err
	}

	outputOfferID := int64(offerEntry.OfferId)
	if outputOfferID < 0 {
		return OfferOutput{}, fmt.Errorf("offerID is negative (%d) for offer from account: %s", outputOfferID, outputSellerID)
	}

	outputSellingAsset, err := asset.TransformSingleAsset(offerEntry.Selling)
	if err != nil {
		return OfferOutput{}, err
	}

	outputBuyingAsset, err := asset.TransformSingleAsset(offerEntry.Buying)
	if err != nil {
		return OfferOutput{}, err
	}

	outputAmount := offerEntry.Amount
	if outputAmount < 0 {
		return OfferOutput{}, fmt.Errorf("amount is negative (%d) for offer %d", outputAmount, outputOfferID)
	}

	outputPriceN := int32(offerEntry.Price.N)
	if outputPriceN < 0 {
		return OfferOutput{}, fmt.Errorf("price numerator is negative (%d) for offer %d", outputPriceN, outputOfferID)
	}

	outputPriceD := int32(offerEntry.Price.D)
	if outputPriceD == 0 {
		return OfferOutput{}, fmt.Errorf("price denominator is 0 for offer %d", outputOfferID)
	}

	if outputPriceD < 0 {
		return OfferOutput{}, fmt.Errorf("price denominator is negative (%d) for offer %d", outputPriceD, outputOfferID)
	}

	var outputPrice float64
	if outputPriceN > 0 {
		outputPrice = float64(outputPriceN) / float64(outputPriceD)
	}

	outputFlags := uint32(offerEntry.Flags)

	outputLastModifiedLedger := uint32(ledgerEntry.LastModifiedLedgerSeq)

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return OfferOutput{}, err
	}

	ledgerSequence := header.Header.LedgerSeq

	transformedOffer := OfferOutput{
		SellerID:           outputSellerID,
		OfferID:            outputOfferID,
		SellingAssetType:   outputSellingAsset.AssetType,
		SellingAssetCode:   outputSellingAsset.AssetCode,
		SellingAssetIssuer: outputSellingAsset.AssetIssuer,
		SellingAssetID:     outputSellingAsset.AssetID,
		BuyingAssetType:    outputBuyingAsset.AssetType,
		BuyingAssetCode:    outputBuyingAsset.AssetCode,
		BuyingAssetIssuer:  outputBuyingAsset.AssetIssuer,
		BuyingAssetID:      outputBuyingAsset.AssetID,
		Amount:             utils.ConvertStroopValueToReal(outputAmount),
		PriceN:             outputPriceN,
		PriceD:             outputPriceD,
		Price:              outputPrice,
		Flags:              outputFlags,
		LastModifiedLedger: outputLastModifiedLedger,
		LedgerEntryChange:  uint32(changeType),
		Deleted:            outputDeleted,
		Sponsor:            utils.LedgerEntrySponsorToNullString(ledgerEntry),
		ClosedAt:           closedAt,
		LedgerSequence:     uint32(ledgerSequence),
	}
	return transformedOffer, nil
}

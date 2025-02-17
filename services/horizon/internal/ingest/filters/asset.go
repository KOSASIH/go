package filters

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/support/collections/set"
	"github.com/stellar/go/support/log"
	"github.com/stellar/go/xdr"
)

var (
	logger = log.WithFields(log.F{
		"ingest filter": "asset",
	})
)

type assetFilter struct {
	canonicalAssetsLookup set.Set[string]
	lastModified          int64
	enabled               bool
}

type AssetFilter interface {
	processors.LedgerTransactionFilterer
	RefreshAssetFilter(filterConfig *history.AssetFilterConfig) error
}

func NewAssetFilter() AssetFilter {
	return &assetFilter{
		canonicalAssetsLookup: set.Set[string]{},
	}
}

func (f *assetFilter) Name() string {
	return "filters.assetFilter"
}

func (f *assetFilter) RefreshAssetFilter(filterConfig *history.AssetFilterConfig) error {
	// only need to re-initialize the filter config state(rules) if it's cached version(in  memory)
	// is older than the incoming config version based on lastModified epoch timestamp
	if filterConfig.LastModified > f.lastModified {
		logger.Infof("New Asset Filter config detected, reloading new config %v ", *filterConfig)
		f.enabled = filterConfig.Enabled
		f.canonicalAssetsLookup = listToSet(filterConfig.Whitelist)
		f.lastModified = filterConfig.LastModified
	}

	return nil
}

func (f *assetFilter) FilterTransaction(ctx context.Context, transaction ingest.LedgerTransaction) (bool, bool, error) {
	if !f.isEnabled() {
		return false, true, nil
	}

	var operations []xdr.Operation

	if txv1, v1Exists := transaction.Envelope.GetV1(); v1Exists {
		operations = txv1.Tx.Operations
	}

	if txv0, v0Exists := transaction.Envelope.GetV0(); v0Exists {
		operations = txv0.Tx.Operations
	}

	if f.filterOperationsMatchedOnRules(operations) {
		return true, true, nil
	}

	logger.Debugf("No match, dropped tx with seq %v ", transaction.Envelope.SeqNum())
	return true, false, nil
}

func (f assetFilter) filterOperationsMatchedOnRules(operations []xdr.Operation) bool {
	for _, operation := range operations {
		switch operation.Body.Type {
		case xdr.OperationTypeChangeTrust:
			if f.filterChangeTrustMatched(operation) {
				return true
			}
		case xdr.OperationTypeManageSellOffer:
			if f.assetMatchedFilter(&operation.Body.ManageSellOfferOp.Buying) || f.assetMatchedFilter(&operation.Body.ManageSellOfferOp.Selling) {
				return true
			}
		case xdr.OperationTypeManageBuyOffer:
			if f.assetMatchedFilter(&operation.Body.ManageBuyOfferOp.Buying) || f.assetMatchedFilter(&operation.Body.ManageBuyOfferOp.Selling) {
				return true
			}
		case xdr.OperationTypeCreateClaimableBalance:
			if f.assetMatchedFilter(&operation.Body.CreateClaimableBalanceOp.Asset) {
				return true
			}
		case xdr.OperationTypeCreatePassiveSellOffer:
			if f.assetMatchedFilter(&operation.Body.CreatePassiveSellOfferOp.Buying) || f.assetMatchedFilter(&operation.Body.CreatePassiveSellOfferOp.Selling) {
				return true
			}
		case xdr.OperationTypeClawback:
			if f.assetMatchedFilter(&operation.Body.ClawbackOp.Asset) {
				return true
			}
		case xdr.OperationTypePayment:
			if f.assetMatchedFilter(&operation.Body.PaymentOp.Asset) {
				return true
			}
		case xdr.OperationTypePathPaymentStrictReceive:
			if f.assetMatchedFilter(&operation.Body.PathPaymentStrictReceiveOp.DestAsset) || f.assetMatchedFilter(&operation.Body.PathPaymentStrictReceiveOp.SendAsset) {
				return true
			}
		case xdr.OperationTypePathPaymentStrictSend:
			if f.assetMatchedFilter(&operation.Body.PathPaymentStrictSendOp.DestAsset) || f.assetMatchedFilter(&operation.Body.PathPaymentStrictSendOp.SendAsset) {
				return true
			}
		}
	}
	return false
}

func (f assetFilter) filterChangeTrustMatched(operation xdr.Operation) bool {
	if pool, ok := operation.Body.ChangeTrustOp.Line.GetLiquidityPool(); ok {
		if f.assetMatchedFilter(&pool.ConstantProduct.AssetA) || f.assetMatchedFilter(&pool.ConstantProduct.AssetB) {
			return true
		}
	} else {
		asset := operation.Body.ChangeTrustOp.Line.ToAsset()
		if f.assetMatchedFilter(&asset) {
			return true
		}
	}
	return false
}

func (f *assetFilter) assetMatchedFilter(asset *xdr.Asset) bool {
	return f.canonicalAssetsLookup.Contains(asset.StringCanonical())
}

func listToSet(list []string) set.Set[string] {
	set := set.NewSet[string](len(list))
	for i := 0; i < len(list); i++ {
		set.Add(list[i])
	}
	return set
}

func (f assetFilter) isEnabled() bool {
	// filtering is disabled if the whitelist is empty for now as that is the only filter rule
	return len(f.canonicalAssetsLookup) >= 1 && f.enabled
}

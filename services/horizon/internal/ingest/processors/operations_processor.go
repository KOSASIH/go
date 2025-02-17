package processors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/guregu/null"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/ingest"
	"github.com/stellar/go/protocols/horizon/base"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/contractevents"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

// OperationProcessor operations processor
type OperationProcessor struct {
	batch   history.OperationBatchInsertBuilder
	network string
}

func NewOperationProcessor(batch history.OperationBatchInsertBuilder, network string) *OperationProcessor {
	return &OperationProcessor{
		batch:   batch,
		network: network,
	}
}

func (p *OperationProcessor) Name() string {
	return "processors.OperationProcessor"
}

// ProcessTransaction process the given transaction
func (p *OperationProcessor) ProcessTransaction(lcm xdr.LedgerCloseMeta, transaction ingest.LedgerTransaction) error {
	for i, op := range transaction.Envelope.Operations() {
		operation := transactionOperationWrapper{
			index:          uint32(i),
			transaction:    transaction,
			operation:      op,
			ledgerSequence: lcm.LedgerSequence(),
			network:        p.network,
		}
		details, err := operation.Details()
		if err != nil {
			return errors.Wrapf(err, "Error obtaining details for operation %v", operation.ID())
		}
		var detailsJSON []byte
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			return errors.Wrapf(err, "Error marshaling details for operation %v", operation.ID())
		}

		source := operation.SourceAccount()
		acID := source.ToAccountId()
		var sourceAccountMuxed null.String
		if source.Type == xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
			sourceAccountMuxed = null.StringFrom(source.Address())
		}
		if err := p.batch.Add(
			operation.ID(),
			operation.TransactionID(),
			operation.Order(),
			operation.OperationType(),
			detailsJSON,
			acID.Address(),
			sourceAccountMuxed,
			operation.IsPayment(),
		); err != nil {
			return errors.Wrap(err, "Error batch inserting operation rows")
		}
	}

	return nil
}

func (p *OperationProcessor) Flush(ctx context.Context, session db.SessionInterface) error {
	return p.batch.Exec(ctx, session)
}

// transactionOperationWrapper represents the data for a single operation within a transaction
type transactionOperationWrapper struct {
	index          uint32
	transaction    ingest.LedgerTransaction
	operation      xdr.Operation
	ledgerSequence uint32
	network        string
}

// ID returns the ID for the operation.
func (operation *transactionOperationWrapper) ID() int64 {
	return toid.New(
		int32(operation.ledgerSequence),
		int32(operation.transaction.Index),
		int32(operation.index+1),
	).ToInt64()
}

// Order returns the operation order.
func (operation *transactionOperationWrapper) Order() uint32 {
	return operation.index + 1
}

// TransactionID returns the id for the transaction related with this operation.
func (operation *transactionOperationWrapper) TransactionID() int64 {
	return toid.New(int32(operation.ledgerSequence), int32(operation.transaction.Index), 0).ToInt64()
}

// SourceAccount returns the operation's source account.
func (operation *transactionOperationWrapper) SourceAccount() *xdr.MuxedAccount {
	sourceAccount := operation.operation.SourceAccount
	if sourceAccount != nil {
		return sourceAccount
	} else {
		ret := operation.transaction.Envelope.SourceAccount()
		return &ret
	}
}

// OperationType returns the operation type.
func (operation *transactionOperationWrapper) OperationType() xdr.OperationType {
	return operation.operation.Body.Type
}

func (operation *transactionOperationWrapper) getSignerSponsorInChange(signerKey string, change ingest.Change) xdr.SponsorshipDescriptor {
	if change.Type != xdr.LedgerEntryTypeAccount || change.Post == nil {
		return nil
	}

	preSigners := map[string]xdr.AccountId{}
	if change.Pre != nil {
		account := change.Pre.Data.MustAccount()
		preSigners = account.SponsorPerSigner()
	}

	account := change.Post.Data.MustAccount()
	postSigners := account.SponsorPerSigner()

	pre, preFound := preSigners[signerKey]
	post, postFound := postSigners[signerKey]

	if !postFound {
		return nil
	}

	if preFound {
		formerSponsor := pre.Address()
		newSponsor := post.Address()
		if formerSponsor == newSponsor {
			return nil
		}
	}

	return &post
}

func (operation *transactionOperationWrapper) getSponsor() (*xdr.AccountId, error) {
	changes, err := operation.transaction.GetOperationChanges(operation.index)
	if err != nil {
		return nil, err
	}
	var signerKey string
	if setOps, ok := operation.operation.Body.GetSetOptionsOp(); ok && setOps.Signer != nil {
		signerKey = setOps.Signer.Key.Address()
	}

	for _, c := range changes {
		// Check Signer changes
		if signerKey != "" {
			if sponsorAccount := operation.getSignerSponsorInChange(signerKey, c); sponsorAccount != nil {
				return sponsorAccount, nil
			}
		}

		// Check Ledger key changes
		if c.Pre != nil || c.Post == nil {
			// We are only looking for entry creations denoting that a sponsor
			// is associated to the ledger entry of the operation.
			continue
		}
		if sponsorAccount := c.Post.SponsoringID(); sponsorAccount != nil {
			return sponsorAccount, nil
		}
	}

	return nil, nil
}

type liquidityPoolDelta struct {
	ReserveA        xdr.Int64
	ReserveB        xdr.Int64
	TotalPoolShares xdr.Int64
}

var errLiquidityPoolChangeNotFound = errors.New("liquidity pool change not found")

func (operation *transactionOperationWrapper) getLiquidityPoolAndProductDelta(lpID *xdr.PoolId) (*xdr.LiquidityPoolEntry, *liquidityPoolDelta, error) {
	changes, err := operation.transaction.GetOperationChanges(operation.index)
	if err != nil {
		return nil, nil, err
	}

	for _, c := range changes {
		if c.Type != xdr.LedgerEntryTypeLiquidityPool {
			continue
		}
		// The delta can be caused by a full removal or full creation of the liquidity pool
		var lp *xdr.LiquidityPoolEntry
		var preA, preB, preShares xdr.Int64
		if c.Pre != nil {
			if lpID != nil && c.Pre.Data.LiquidityPool.LiquidityPoolId != *lpID {
				// if we were looking for specific pool id, then check on it
				continue
			}
			lp = c.Pre.Data.LiquidityPool
			if c.Pre.Data.LiquidityPool.Body.Type != xdr.LiquidityPoolTypeLiquidityPoolConstantProduct {
				return nil, nil, fmt.Errorf("unexpected liquity pool body type %d", c.Pre.Data.LiquidityPool.Body.Type)
			}
			cpPre := c.Pre.Data.LiquidityPool.Body.ConstantProduct
			preA, preB, preShares = cpPre.ReserveA, cpPre.ReserveB, cpPre.TotalPoolShares
		}
		var postA, postB, postShares xdr.Int64
		if c.Post != nil {
			if lpID != nil && c.Post.Data.LiquidityPool.LiquidityPoolId != *lpID {
				// if we were looking for specific pool id, then check on it
				continue
			}
			lp = c.Post.Data.LiquidityPool
			if c.Post.Data.LiquidityPool.Body.Type != xdr.LiquidityPoolTypeLiquidityPoolConstantProduct {
				return nil, nil, fmt.Errorf("unexpected liquity pool body type %d", c.Post.Data.LiquidityPool.Body.Type)
			}
			cpPost := c.Post.Data.LiquidityPool.Body.ConstantProduct
			postA, postB, postShares = cpPost.ReserveA, cpPost.ReserveB, cpPost.TotalPoolShares
		}
		delta := &liquidityPoolDelta{
			ReserveA:        postA - preA,
			ReserveB:        postB - preB,
			TotalPoolShares: postShares - preShares,
		}
		return lp, delta, nil
	}

	return nil, nil, errLiquidityPoolChangeNotFound
}

// OperationResult returns the operation's result record
func (operation *transactionOperationWrapper) OperationResult() *xdr.OperationResultTr {
	results, _ := operation.transaction.Result.OperationResults()
	tr := results[operation.index].MustTr()
	return &tr
}

// Determines if an operation is qualified to represent a payment in horizon terms.
func (operation *transactionOperationWrapper) IsPayment() bool {
	switch operation.OperationType() {
	case xdr.OperationTypeCreateAccount:
		return true
	case xdr.OperationTypePayment:
		return true
	case xdr.OperationTypePathPaymentStrictReceive:
		return true
	case xdr.OperationTypePathPaymentStrictSend:
		return true
	case xdr.OperationTypeAccountMerge:
		return true
	case xdr.OperationTypeInvokeHostFunction:
		diagnosticEvents, err := operation.transaction.GetDiagnosticEvents()
		if err != nil {
			return false
		}
		// scan all the contract events for at least one SAC event, qualified to be a payment
		// in horizon
		for _, contractEvent := range filterEvents(diagnosticEvents) {
			if sacEvent, err := contractevents.NewStellarAssetContractEvent(&contractEvent, operation.network); err == nil {
				switch sacEvent.GetType() {
				case contractevents.EventTypeTransfer:
					return true
				case contractevents.EventTypeMint:
					return true
				case contractevents.EventTypeClawback:
					return true
				case contractevents.EventTypeBurn:
					return true
				}
			}
		}
	}

	return false
}

func (operation *transactionOperationWrapper) findInitatingBeginSponsoringOp() *transactionOperationWrapper {
	if !operation.transaction.Result.Successful() {
		// Failed transactions may not have a compliant sandwich structure
		// we can rely on (e.g. invalid nesting or a being operation with the wrong sponsoree ID)
		// and thus we bail out since we could return incorrect information.
		return nil
	}
	sponsoree := operation.SourceAccount().ToAccountId()
	operations := operation.transaction.Envelope.Operations()
	for i := int(operation.index) - 1; i >= 0; i-- {
		if beginOp, ok := operations[i].Body.GetBeginSponsoringFutureReservesOp(); ok &&
			beginOp.SponsoredId.Address() == sponsoree.Address() {
			result := *operation
			result.index = uint32(i)
			result.operation = operations[i]
			return &result
		}
	}
	return nil
}

func addAccountAndMuxedAccountDetails(result map[string]interface{}, a xdr.MuxedAccount, prefix string) {
	accid := a.ToAccountId()
	result[prefix] = accid.Address()
	if a.Type == xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
		result[prefix+"_muxed"] = a.Address()
		// _muxed_id fields should had ideally been stored in the DB as a string instead of uint64
		// due to Javascript not being able to handle them, see https://github.com/stellar/go/issues/3714
		// However, we released this code in the wild before correcting it. Thus, what we do is
		// work around it (by preprocessing it into a string) in Operation.UnmarshalDetails()
		result[prefix+"_muxed_id"] = uint64(a.Med25519.Id)
	}
}

// Details returns the operation details as a map which can be stored as JSON.
func (operation *transactionOperationWrapper) Details() (map[string]interface{}, error) {
	details := map[string]interface{}{}
	source := operation.SourceAccount()
	switch operation.OperationType() {
	case xdr.OperationTypeCreateAccount:
		op := operation.operation.Body.MustCreateAccountOp()
		addAccountAndMuxedAccountDetails(details, *source, "funder")
		details["account"] = op.Destination.Address()
		details["starting_balance"] = amount.String(op.StartingBalance)
	case xdr.OperationTypePayment:
		op := operation.operation.Body.MustPaymentOp()
		addAccountAndMuxedAccountDetails(details, *source, "from")
		addAccountAndMuxedAccountDetails(details, op.Destination, "to")
		details["amount"] = amount.String(op.Amount)
		if err := addAssetDetails(details, op.Asset, ""); err != nil {
			return nil, err
		}
	case xdr.OperationTypePathPaymentStrictReceive:
		op := operation.operation.Body.MustPathPaymentStrictReceiveOp()
		addAccountAndMuxedAccountDetails(details, *source, "from")
		addAccountAndMuxedAccountDetails(details, op.Destination, "to")

		details["amount"] = amount.String(op.DestAmount)
		details["source_amount"] = amount.String(0)
		details["source_max"] = amount.String(op.SendMax)
		if err := addAssetDetails(details, op.DestAsset, ""); err != nil {
			return nil, err
		}
		if err := addAssetDetails(details, op.SendAsset, "source_"); err != nil {
			return nil, err
		}

		if operation.transaction.Result.Successful() {
			result := operation.OperationResult().MustPathPaymentStrictReceiveResult()
			details["source_amount"] = amount.String(result.SendAmount())
		}

		var path = make([]map[string]interface{}, len(op.Path))
		for i := range op.Path {
			path[i] = make(map[string]interface{})
			if err := addAssetDetails(path[i], op.Path[i], ""); err != nil {
				return nil, err
			}
		}
		details["path"] = path

	case xdr.OperationTypePathPaymentStrictSend:
		op := operation.operation.Body.MustPathPaymentStrictSendOp()
		addAccountAndMuxedAccountDetails(details, *source, "from")
		addAccountAndMuxedAccountDetails(details, op.Destination, "to")

		details["amount"] = amount.String(0)
		details["source_amount"] = amount.String(op.SendAmount)
		details["destination_min"] = amount.String(op.DestMin)
		if err := addAssetDetails(details, op.DestAsset, ""); err != nil {
			return nil, err
		}
		if err := addAssetDetails(details, op.SendAsset, "source_"); err != nil {
			return nil, err
		}

		if operation.transaction.Result.Successful() {
			result := operation.OperationResult().MustPathPaymentStrictSendResult()
			details["amount"] = amount.String(result.DestAmount())
		}

		var path = make([]map[string]interface{}, len(op.Path))
		for i := range op.Path {
			path[i] = make(map[string]interface{})
			if err := addAssetDetails(path[i], op.Path[i], ""); err != nil {
				return nil, err
			}
		}
		details["path"] = path
	case xdr.OperationTypeManageBuyOffer:
		op := operation.operation.Body.MustManageBuyOfferOp()
		details["offer_id"] = op.OfferId
		details["amount"] = amount.String(op.BuyAmount)
		details["price"] = op.Price.String()
		details["price_r"] = map[string]interface{}{
			"n": op.Price.N,
			"d": op.Price.D,
		}
		if err := addAssetDetails(details, op.Buying, "buying_"); err != nil {
			return nil, err
		}
		if err := addAssetDetails(details, op.Selling, "selling_"); err != nil {
			return nil, err
		}
	case xdr.OperationTypeManageSellOffer:
		op := operation.operation.Body.MustManageSellOfferOp()
		details["offer_id"] = op.OfferId
		details["amount"] = amount.String(op.Amount)
		details["price"] = op.Price.String()
		details["price_r"] = map[string]interface{}{
			"n": op.Price.N,
			"d": op.Price.D,
		}
		if err := addAssetDetails(details, op.Buying, "buying_"); err != nil {
			return nil, err
		}
		if err := addAssetDetails(details, op.Selling, "selling_"); err != nil {
			return nil, err
		}
	case xdr.OperationTypeCreatePassiveSellOffer:
		op := operation.operation.Body.MustCreatePassiveSellOfferOp()
		details["amount"] = amount.String(op.Amount)
		details["price"] = op.Price.String()
		details["price_r"] = map[string]interface{}{
			"n": op.Price.N,
			"d": op.Price.D,
		}
		if err := addAssetDetails(details, op.Buying, "buying_"); err != nil {
			return nil, err
		}
		if err := addAssetDetails(details, op.Selling, "selling_"); err != nil {
			return nil, err
		}
	case xdr.OperationTypeSetOptions:
		op := operation.operation.Body.MustSetOptionsOp()

		if op.InflationDest != nil {
			details["inflation_dest"] = op.InflationDest.Address()
		}

		if op.SetFlags != nil && *op.SetFlags > 0 {
			addAuthFlagDetails(details, xdr.AccountFlags(*op.SetFlags), "set")
		}

		if op.ClearFlags != nil && *op.ClearFlags > 0 {
			addAuthFlagDetails(details, xdr.AccountFlags(*op.ClearFlags), "clear")
		}

		if op.MasterWeight != nil {
			details["master_key_weight"] = *op.MasterWeight
		}

		if op.LowThreshold != nil {
			details["low_threshold"] = *op.LowThreshold
		}

		if op.MedThreshold != nil {
			details["med_threshold"] = *op.MedThreshold
		}

		if op.HighThreshold != nil {
			details["high_threshold"] = *op.HighThreshold
		}

		if op.HomeDomain != nil {
			details["home_domain"] = *op.HomeDomain
		}

		if op.Signer != nil {
			details["signer_key"] = op.Signer.Key.Address()
			details["signer_weight"] = op.Signer.Weight
		}
	case xdr.OperationTypeChangeTrust:
		op := operation.operation.Body.MustChangeTrustOp()
		if op.Line.Type == xdr.AssetTypeAssetTypePoolShare {
			if err := addLiquidityPoolAssetDetails(details, *op.Line.LiquidityPool); err != nil {
				return nil, err
			}
		} else {
			if err := addAssetDetails(details, op.Line.ToAsset(), ""); err != nil {
				return nil, err
			}
			details["trustee"] = details["asset_issuer"]
		}
		addAccountAndMuxedAccountDetails(details, *source, "trustor")
		details["limit"] = amount.String(op.Limit)
	case xdr.OperationTypeAllowTrust:
		op := operation.operation.Body.MustAllowTrustOp()
		if err := addAssetDetails(details, op.Asset.ToAsset(source.ToAccountId()), ""); err != nil {
			return nil, err
		}
		addAccountAndMuxedAccountDetails(details, *source, "trustee")
		details["trustor"] = op.Trustor.Address()
		details["authorize"] = xdr.TrustLineFlags(op.Authorize).IsAuthorized()
		authLiabilities := xdr.TrustLineFlags(op.Authorize).IsAuthorizedToMaintainLiabilitiesFlag()
		if authLiabilities {
			details["authorize_to_maintain_liabilities"] = authLiabilities
		}
		clawbackEnabled := xdr.TrustLineFlags(op.Authorize).IsClawbackEnabledFlag()
		if clawbackEnabled {
			details["clawback_enabled"] = clawbackEnabled
		}
	case xdr.OperationTypeAccountMerge:
		addAccountAndMuxedAccountDetails(details, *source, "account")
		addAccountAndMuxedAccountDetails(details, operation.operation.Body.MustDestination(), "into")
	case xdr.OperationTypeInflation:
		// no inflation details, presently
	case xdr.OperationTypeManageData:
		op := operation.operation.Body.MustManageDataOp()
		details["name"] = string(op.DataName)
		if op.DataValue != nil {
			details["value"] = base64.StdEncoding.EncodeToString(*op.DataValue)
		} else {
			details["value"] = nil
		}
	case xdr.OperationTypeBumpSequence:
		op := operation.operation.Body.MustBumpSequenceOp()
		details["bump_to"] = fmt.Sprintf("%d", op.BumpTo)
	case xdr.OperationTypeCreateClaimableBalance:
		op := operation.operation.Body.MustCreateClaimableBalanceOp()
		details["asset"] = op.Asset.StringCanonical()
		details["amount"] = amount.String(op.Amount)
		var claimants history.Claimants
		for _, c := range op.Claimants {
			cv0 := c.MustV0()
			claimants = append(claimants, history.Claimant{
				Destination: cv0.Destination.Address(),
				Predicate:   cv0.Predicate,
			})
		}
		details["claimants"] = claimants
	case xdr.OperationTypeClaimClaimableBalance:
		op := operation.operation.Body.MustClaimClaimableBalanceOp()
		balanceID, err := xdr.MarshalHex(op.BalanceId)
		if err != nil {
			return nil, fmt.Errorf("Invalid balanceId in op: %d", operation.index)
		}
		details["balance_id"] = balanceID
		addAccountAndMuxedAccountDetails(details, *source, "claimant")
	case xdr.OperationTypeBeginSponsoringFutureReserves:
		op := operation.operation.Body.MustBeginSponsoringFutureReservesOp()
		details["sponsored_id"] = op.SponsoredId.Address()
	case xdr.OperationTypeEndSponsoringFutureReserves:
		beginSponsorshipOp := operation.findInitatingBeginSponsoringOp()
		if beginSponsorshipOp != nil {
			beginSponsorshipSource := beginSponsorshipOp.SourceAccount()
			addAccountAndMuxedAccountDetails(details, *beginSponsorshipSource, "begin_sponsor")
		}
	case xdr.OperationTypeRevokeSponsorship:
		op := operation.operation.Body.MustRevokeSponsorshipOp()
		switch op.Type {
		case xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry:
			if err := addLedgerKeyDetails(details, *op.LedgerKey); err != nil {
				return nil, err
			}
		case xdr.RevokeSponsorshipTypeRevokeSponsorshipSigner:
			details["signer_account_id"] = op.Signer.AccountId.Address()
			details["signer_key"] = op.Signer.SignerKey.Address()
		}
	case xdr.OperationTypeClawback:
		op := operation.operation.Body.MustClawbackOp()
		if err := addAssetDetails(details, op.Asset, ""); err != nil {
			return nil, err
		}
		addAccountAndMuxedAccountDetails(details, op.From, "from")
		details["amount"] = amount.String(op.Amount)
	case xdr.OperationTypeClawbackClaimableBalance:
		op := operation.operation.Body.MustClawbackClaimableBalanceOp()
		balanceID, err := xdr.MarshalHex(op.BalanceId)
		if err != nil {
			return nil, fmt.Errorf("Invalid balanceId in op: %d", operation.index)
		}
		details["balance_id"] = balanceID
	case xdr.OperationTypeSetTrustLineFlags:
		op := operation.operation.Body.MustSetTrustLineFlagsOp()
		details["trustor"] = op.Trustor.Address()
		if err := addAssetDetails(details, op.Asset, ""); err != nil {
			return nil, err
		}
		if op.SetFlags > 0 {
			addTrustLineFlagDetails(details, xdr.TrustLineFlags(op.SetFlags), "set")
		}

		if op.ClearFlags > 0 {
			addTrustLineFlagDetails(details, xdr.TrustLineFlags(op.ClearFlags), "clear")
		}
	case xdr.OperationTypeLiquidityPoolDeposit:
		op := operation.operation.Body.MustLiquidityPoolDepositOp()
		details["liquidity_pool_id"] = PoolIDToString(op.LiquidityPoolId)
		var (
			assetA, assetB         string
			depositedA, depositedB xdr.Int64
			sharesReceived         xdr.Int64
		)
		if operation.transaction.Result.Successful() {
			// we will use the defaults (omitted asset and 0 amounts) if the transaction failed
			lp, delta, err := operation.getLiquidityPoolAndProductDelta(&op.LiquidityPoolId)
			if err != nil {
				return nil, err
			}
			params := lp.Body.ConstantProduct.Params
			assetA, assetB = params.AssetA.StringCanonical(), params.AssetB.StringCanonical()
			depositedA, depositedB = delta.ReserveA, delta.ReserveB
			sharesReceived = delta.TotalPoolShares
		}
		details["reserves_max"] = []base.AssetAmount{
			{Asset: assetA, Amount: amount.String(op.MaxAmountA)},
			{Asset: assetB, Amount: amount.String(op.MaxAmountB)},
		}
		details["min_price"] = op.MinPrice.String()
		details["min_price_r"] = map[string]interface{}{
			"n": op.MinPrice.N,
			"d": op.MinPrice.D,
		}
		details["max_price"] = op.MaxPrice.String()
		details["max_price_r"] = map[string]interface{}{
			"n": op.MaxPrice.N,
			"d": op.MaxPrice.D,
		}
		details["reserves_deposited"] = []base.AssetAmount{
			{Asset: assetA, Amount: amount.String(depositedA)},
			{Asset: assetB, Amount: amount.String(depositedB)},
		}
		details["shares_received"] = amount.String(sharesReceived)
	case xdr.OperationTypeLiquidityPoolWithdraw:
		op := operation.operation.Body.MustLiquidityPoolWithdrawOp()
		details["liquidity_pool_id"] = PoolIDToString(op.LiquidityPoolId)
		var (
			assetA, assetB       string
			receivedA, receivedB xdr.Int64
		)
		if operation.transaction.Result.Successful() {
			// we will use the defaults (omitted asset and 0 amounts) if the transaction failed
			lp, delta, err := operation.getLiquidityPoolAndProductDelta(&op.LiquidityPoolId)
			if err != nil {
				return nil, err
			}
			params := lp.Body.ConstantProduct.Params
			assetA, assetB = params.AssetA.StringCanonical(), params.AssetB.StringCanonical()
			receivedA, receivedB = -delta.ReserveA, -delta.ReserveB
		}
		details["reserves_min"] = []base.AssetAmount{
			{Asset: assetA, Amount: amount.String(op.MinAmountA)},
			{Asset: assetB, Amount: amount.String(op.MinAmountB)},
		}
		details["shares"] = amount.String(op.Amount)
		details["reserves_received"] = []base.AssetAmount{
			{Asset: assetA, Amount: amount.String(receivedA)},
			{Asset: assetB, Amount: amount.String(receivedB)},
		}
	case xdr.OperationTypeInvokeHostFunction:
		op := operation.operation.Body.MustInvokeHostFunctionOp()
		details["function"] = op.HostFunction.Type.String()

		switch op.HostFunction.Type {
		case xdr.HostFunctionTypeHostFunctionTypeInvokeContract:
			invokeArgs := op.HostFunction.MustInvokeContract()
			args := make([]xdr.ScVal, 0, len(invokeArgs.Args)+2)
			args = append(args, xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &invokeArgs.ContractAddress})
			args = append(args, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &invokeArgs.FunctionName})
			args = append(args, invokeArgs.Args...)
			details["parameters"] = extractFunctionArgs(args)

			if balanceChanges, err := operation.parseAssetBalanceChangesFromContractEvents(); err != nil {
				return nil, err
			} else {
				details["asset_balance_changes"] = balanceChanges
			}

		case xdr.HostFunctionTypeHostFunctionTypeCreateContract:
			args := op.HostFunction.MustCreateContract()
			switch args.ContractIdPreimage.Type {
			case xdr.ContractIdPreimageTypeContractIdPreimageFromAddress:
				fromAddress := args.ContractIdPreimage.MustFromAddress()
				address, err := fromAddress.Address.String()
				if err != nil {
					return nil, fmt.Errorf("error obtaining address for: %s", args.ContractIdPreimage.Type)
				}
				details["from"] = "address"
				details["address"] = address
				details["salt"] = fromAddress.Salt.String()
			case xdr.ContractIdPreimageTypeContractIdPreimageFromAsset:
				details["from"] = "asset"
				details["asset"] = args.ContractIdPreimage.MustFromAsset().StringCanonical()
			default:
				return nil, fmt.Errorf("unknown contract id type: %s", args.ContractIdPreimage.Type)
			}
		case xdr.HostFunctionTypeHostFunctionTypeCreateContractV2:
			args := op.HostFunction.MustCreateContractV2()
			switch args.ContractIdPreimage.Type {
			case xdr.ContractIdPreimageTypeContractIdPreimageFromAddress:
				fromAddress := args.ContractIdPreimage.MustFromAddress()
				address, err := fromAddress.Address.String()
				if err != nil {
					return nil, fmt.Errorf("error obtaining address for: %s", args.ContractIdPreimage.Type)
				}
				details["from"] = "address"
				details["address"] = address
				details["salt"] = fromAddress.Salt.String()
			case xdr.ContractIdPreimageTypeContractIdPreimageFromAsset:
				details["from"] = "asset"
				details["asset"] = args.ContractIdPreimage.MustFromAsset().StringCanonical()
			default:
				return nil, fmt.Errorf("unknown contract id type: %s", args.ContractIdPreimage.Type)
			}

			details["parameters"] = extractFunctionArgs(args.ConstructorArgs)

			if balanceChanges, err := operation.parseAssetBalanceChangesFromContractEvents(); err != nil {
				return nil, err
			} else {
				details["asset_balance_changes"] = balanceChanges
			}
		case xdr.HostFunctionTypeHostFunctionTypeUploadContractWasm:
		default:
			return nil, fmt.Errorf("unknown host function type: %s", op.HostFunction.Type)
		}
	case xdr.OperationTypeExtendFootprintTtl:
		op := operation.operation.Body.MustExtendFootprintTtlOp()
		details["extend_to"] = op.ExtendTo
	case xdr.OperationTypeRestoreFootprint:
	default:
		return nil, fmt.Errorf("unknown operation type: %s", operation.OperationType())
	}

	sponsor, err := operation.getSponsor()
	if err != nil {
		return nil, err
	}
	if sponsor != nil {
		details["sponsor"] = sponsor.Address()
	}

	return details, nil
}

func extractFunctionArgs(args []xdr.ScVal) []map[string]string {
	params := make([]map[string]string, 0, len(args))
	for _, param := range args {
		serializedParam := map[string]string{}
		serializedParam["value"] = "n/a"
		serializedParam["type"] = "n/a"

		if scValTypeName, ok := param.ArmForSwitch(int32(param.Type)); ok {
			serializedParam["type"] = scValTypeName
			if raw, err := param.MarshalBinary(); err == nil {
				serializedParam["value"] = base64.StdEncoding.EncodeToString(raw)
			}
		}
		params = append(params, serializedParam)
	}
	return params
}

// Searches an operation for SAC events that are of a type which represent
// asset balances having changed.
//
// SAC events have a one-to-one association to SAC contract fn invocations.
// i.e. invoke the 'mint' function, will trigger one Mint Event to be emitted capturing the fn args.
//
// SAC events that involve asset balance changes follow some standard data formats.
// The 'amount' in the event is expressed as Int128Parts, which carries a sign, however it's expected
// that value will not be signed as it represents a absolute delta, the event type can provide the
// context of whether an amount was considered incremental or decremental, i.e. credit or debit to a balance.
func (operation *transactionOperationWrapper) parseAssetBalanceChangesFromContractEvents() ([]map[string]interface{}, error) {
	balanceChanges := []map[string]interface{}{}

	diagnosticEvents, err := operation.transaction.GetDiagnosticEvents()
	if err != nil {
		// this operation in this context must be an InvokeHostFunctionOp, therefore V3Meta should be present
		// as it's in same soroban model, so if any err, it's real,
		return nil, err
	}

	for _, contractEvent := range filterEvents(diagnosticEvents) {
		// Parse the xdr contract event to contractevents.StellarAssetContractEvent model

		// has some convenience like to/from attributes are expressed in strkey format for accounts(G...) and contracts(C...)
		if sacEvent, err := contractevents.NewStellarAssetContractEvent(&contractEvent, operation.network); err == nil {
			switch sacEvent.GetType() {
			case contractevents.EventTypeTransfer:
				transferEvt := sacEvent.(*contractevents.TransferEvent)
				balanceChanges = append(balanceChanges, createSACBalanceChangeEntry(transferEvt.From, transferEvt.To, transferEvt.Amount, transferEvt.Asset, "transfer"))
			case contractevents.EventTypeMint:
				mintEvt := sacEvent.(*contractevents.MintEvent)
				balanceChanges = append(balanceChanges, createSACBalanceChangeEntry("", mintEvt.To, mintEvt.Amount, mintEvt.Asset, "mint"))
			case contractevents.EventTypeClawback:
				clawbackEvt := sacEvent.(*contractevents.ClawbackEvent)
				balanceChanges = append(balanceChanges, createSACBalanceChangeEntry(clawbackEvt.From, "", clawbackEvt.Amount, clawbackEvt.Asset, "clawback"))
			case contractevents.EventTypeBurn:
				burnEvt := sacEvent.(*contractevents.BurnEvent)
				balanceChanges = append(balanceChanges, createSACBalanceChangeEntry(burnEvt.From, "", burnEvt.Amount, burnEvt.Asset, "burn"))
			}
		}
	}

	return balanceChanges, nil
}

// fromAccount   - strkey format of contract or address
// toAccount     - strkey format of contract or address, or nillable
// amountChanged - absolute value that asset balance changed
// asset         - the fully qualified issuer:code for asset that had balance change
// changeType    - the type of source sac event that triggered this change
//
// return        - a balance changed record expressed as map of key/value's
func createSACBalanceChangeEntry(fromAccount string, toAccount string, amountChanged xdr.Int128Parts, asset xdr.Asset, changeType string) map[string]interface{} {
	balanceChange := map[string]interface{}{}

	if fromAccount != "" {
		balanceChange["from"] = fromAccount
	}
	if toAccount != "" {
		balanceChange["to"] = toAccount
	}

	balanceChange["type"] = changeType
	balanceChange["amount"] = amount.String128(amountChanged)
	addAssetDetails(balanceChange, asset, "")
	return balanceChange
}

func addLiquidityPoolAssetDetails(result map[string]interface{}, lpp xdr.LiquidityPoolParameters) error {
	result["asset_type"] = "liquidity_pool_shares"
	if lpp.Type != xdr.LiquidityPoolTypeLiquidityPoolConstantProduct {
		return fmt.Errorf("unknown liquidity pool type %d", lpp.Type)
	}
	cp := lpp.ConstantProduct
	poolID, err := xdr.NewPoolId(cp.AssetA, cp.AssetB, cp.Fee)
	if err != nil {
		return err
	}
	result["liquidity_pool_id"] = PoolIDToString(poolID)
	return nil
}

// addAssetDetails sets the details for `a` on `result` using keys with `prefix`
func addAssetDetails(result map[string]interface{}, a xdr.Asset, prefix string) error {
	var (
		assetType string
		code      string
		issuer    string
	)
	err := a.Extract(&assetType, &code, &issuer)
	if err != nil {
		err = errors.Wrap(err, "xdr.Asset.Extract error")
		return err
	}
	result[prefix+"asset_type"] = assetType

	if a.Type == xdr.AssetTypeAssetTypeNative {
		return nil
	}

	result[prefix+"asset_code"] = code
	result[prefix+"asset_issuer"] = issuer
	return nil
}

// addAuthFlagDetails adds the account flag details for `f` on `result`.
func addAuthFlagDetails(result map[string]interface{}, f xdr.AccountFlags, prefix string) {
	var (
		n []int32
		s []string
	)

	if f.IsAuthRequired() {
		n = append(n, int32(xdr.AccountFlagsAuthRequiredFlag))
		s = append(s, "auth_required")
	}

	if f.IsAuthRevocable() {
		n = append(n, int32(xdr.AccountFlagsAuthRevocableFlag))
		s = append(s, "auth_revocable")
	}

	if f.IsAuthImmutable() {
		n = append(n, int32(xdr.AccountFlagsAuthImmutableFlag))
		s = append(s, "auth_immutable")
	}

	if f.IsAuthClawbackEnabled() {
		n = append(n, int32(xdr.AccountFlagsAuthClawbackEnabledFlag))
		s = append(s, "auth_clawback_enabled")
	}

	result[prefix+"_flags"] = n
	result[prefix+"_flags_s"] = s
}

// addTrustLineFlagDetails adds the trustline flag details for `f` on `result`.
func addTrustLineFlagDetails(result map[string]interface{}, f xdr.TrustLineFlags, prefix string) {
	var (
		n []int32
		s []string
	)

	if f.IsAuthorized() {
		n = append(n, int32(xdr.TrustLineFlagsAuthorizedFlag))
		s = append(s, "authorized")
	}

	if f.IsAuthorizedToMaintainLiabilitiesFlag() {
		n = append(n, int32(xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag))
		s = append(s, "authorized_to_maintain_liabilites")
	}

	if f.IsClawbackEnabledFlag() {
		n = append(n, int32(xdr.TrustLineFlagsTrustlineClawbackEnabledFlag))
		s = append(s, "clawback_enabled")
	}

	result[prefix+"_flags"] = n
	result[prefix+"_flags_s"] = s
}

func addLedgerKeyDetails(result map[string]interface{}, ledgerKey xdr.LedgerKey) error {
	switch ledgerKey.Type {
	case xdr.LedgerEntryTypeAccount:
		result["account_id"] = ledgerKey.Account.AccountId.Address()
	case xdr.LedgerEntryTypeClaimableBalance:
		marshalHex, err := xdr.MarshalHex(ledgerKey.ClaimableBalance.BalanceId)
		if err != nil {
			return errors.Wrapf(err, "in claimable balance")
		}
		result["claimable_balance_id"] = marshalHex
	case xdr.LedgerEntryTypeData:
		result["data_account_id"] = ledgerKey.Data.AccountId.Address()
		result["data_name"] = ledgerKey.Data.DataName
	case xdr.LedgerEntryTypeOffer:
		result["offer_id"] = fmt.Sprintf("%d", ledgerKey.Offer.OfferId)
	case xdr.LedgerEntryTypeTrustline:
		result["trustline_account_id"] = ledgerKey.TrustLine.AccountId.Address()
		if ledgerKey.TrustLine.Asset.Type == xdr.AssetTypeAssetTypePoolShare {
			result["trustline_liquidity_pool_id"] = PoolIDToString(*ledgerKey.TrustLine.Asset.LiquidityPoolId)
		} else {
			result["trustline_asset"] = ledgerKey.TrustLine.Asset.ToAsset().StringCanonical()
		}
	case xdr.LedgerEntryTypeLiquidityPool:
		result["liquidity_pool_id"] = PoolIDToString(ledgerKey.LiquidityPool.LiquidityPoolId)
	}
	return nil
}

func getLedgerKeyParticipants(ledgerKey xdr.LedgerKey) []xdr.AccountId {
	var result []xdr.AccountId
	switch ledgerKey.Type {
	case xdr.LedgerEntryTypeAccount:
		result = append(result, ledgerKey.Account.AccountId)
	case xdr.LedgerEntryTypeClaimableBalance:
		// nothing to do
	case xdr.LedgerEntryTypeData:
		result = append(result, ledgerKey.Data.AccountId)
	case xdr.LedgerEntryTypeOffer:
		result = append(result, ledgerKey.Offer.SellerId)
	case xdr.LedgerEntryTypeTrustline:
		result = append(result, ledgerKey.TrustLine.AccountId)
	}
	return result
}

// Participants returns the accounts taking part in the operation.
func (operation *transactionOperationWrapper) Participants() ([]xdr.AccountId, error) {
	participants := []xdr.AccountId{}
	participants = append(participants, operation.SourceAccount().ToAccountId())
	op := operation.operation

	switch operation.OperationType() {
	case xdr.OperationTypeCreateAccount:
		participants = append(participants, op.Body.MustCreateAccountOp().Destination)
	case xdr.OperationTypePayment:
		participants = append(participants, op.Body.MustPaymentOp().Destination.ToAccountId())
	case xdr.OperationTypePathPaymentStrictReceive:
		participants = append(participants, op.Body.MustPathPaymentStrictReceiveOp().Destination.ToAccountId())
	case xdr.OperationTypePathPaymentStrictSend:
		participants = append(participants, op.Body.MustPathPaymentStrictSendOp().Destination.ToAccountId())
	case xdr.OperationTypeManageBuyOffer:
		// the only direct participant is the source_account
	case xdr.OperationTypeManageSellOffer:
		// the only direct participant is the source_account
	case xdr.OperationTypeCreatePassiveSellOffer:
		// the only direct participant is the source_account
	case xdr.OperationTypeSetOptions:
		// the only direct participant is the source_account
	case xdr.OperationTypeChangeTrust:
		// the only direct participant is the source_account
	case xdr.OperationTypeAllowTrust:
		participants = append(participants, op.Body.MustAllowTrustOp().Trustor)
	case xdr.OperationTypeAccountMerge:
		participants = append(participants, op.Body.MustDestination().ToAccountId())
	case xdr.OperationTypeInflation:
		// the only direct participant is the source_account
	case xdr.OperationTypeManageData:
		// the only direct participant is the source_account
	case xdr.OperationTypeBumpSequence:
		// the only direct participant is the source_account
	case xdr.OperationTypeCreateClaimableBalance:
		for _, c := range op.Body.MustCreateClaimableBalanceOp().Claimants {
			participants = append(participants, c.MustV0().Destination)
		}
	case xdr.OperationTypeClaimClaimableBalance:
		// the only direct participant is the source_account
	case xdr.OperationTypeBeginSponsoringFutureReserves:
		participants = append(participants, op.Body.MustBeginSponsoringFutureReservesOp().SponsoredId)
	case xdr.OperationTypeEndSponsoringFutureReserves:
		beginSponsorshipOp := operation.findInitatingBeginSponsoringOp()
		if beginSponsorshipOp != nil {
			participants = append(participants, beginSponsorshipOp.SourceAccount().ToAccountId())
		}
	case xdr.OperationTypeRevokeSponsorship:
		op := operation.operation.Body.MustRevokeSponsorshipOp()
		switch op.Type {
		case xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry:
			participants = append(participants, getLedgerKeyParticipants(*op.LedgerKey)...)
		case xdr.RevokeSponsorshipTypeRevokeSponsorshipSigner:
			participants = append(participants, op.Signer.AccountId)
			// We don't add signer as a participant because a signer can be arbitrary account.
			// This can spam successful operations history of any account.
		}
	case xdr.OperationTypeClawback:
		op := operation.operation.Body.MustClawbackOp()
		participants = append(participants, op.From.ToAccountId())
	case xdr.OperationTypeClawbackClaimableBalance:
		// the only direct participant is the source_account
	case xdr.OperationTypeSetTrustLineFlags:
		op := operation.operation.Body.MustSetTrustLineFlagsOp()
		participants = append(participants, op.Trustor)
	case xdr.OperationTypeLiquidityPoolDeposit:
		// the only direct participant is the source_account
	case xdr.OperationTypeLiquidityPoolWithdraw:
		// the only direct participant is the source_account
	case xdr.OperationTypeInvokeHostFunction:
		if changes, err := operation.transaction.GetOperationChanges(operation.index); err != nil {
			return participants, err
		} else {
			for _, change := range changes {
				var data xdr.LedgerEntryData
				switch {
				case change.Post != nil:
					data = change.Post.Data
				case change.Pre != nil:
					data = change.Pre.Data
				default:
					log.Errorf("Change Type %s with no pre or post", change.Type.String())
					continue
				}
				if ledgerKey, err := data.LedgerKey(); err == nil {
					participants = append(participants, getLedgerKeyParticipants(ledgerKey)...)
				}
			}
		}
		if diagnosticEvents, err := operation.transaction.GetDiagnosticEvents(); err != nil {
			return participants, err
		} else {
			participants = append(participants, getParticipantsFromSACEvents(filterEvents(diagnosticEvents), operation.network)...)
		}

	case xdr.OperationTypeExtendFootprintTtl:
		// the only direct participant is the source_account
	case xdr.OperationTypeRestoreFootprint:
		// the only direct participant is the source_account
	default:
		return participants, fmt.Errorf("unknown operation type: %s", op.Body.Type)
	}

	sponsor, err := operation.getSponsor()
	if err != nil {
		return nil, err
	}
	if sponsor != nil {
		participants = append(participants, *sponsor)
	}

	return dedupeParticipants(participants), nil
}

func getParticipantsFromSACEvents(contractEvents []xdr.ContractEvent, network string) []xdr.AccountId {
	var participants []xdr.AccountId

	for _, contractEvent := range contractEvents {
		if sacEvent, err := contractevents.NewStellarAssetContractEvent(&contractEvent, network); err == nil {
			// 'to' and 'from' fields in the events can be either a Contract address or an Account address. We're
			// only interested in account addresses and will skip Contract addresses.
			switch sacEvent.GetType() {
			case contractevents.EventTypeTransfer:
				var transferEvt *contractevents.TransferEvent
				transferEvt = sacEvent.(*contractevents.TransferEvent)
				var from, to xdr.AccountId
				if from, err = xdr.AddressToAccountId(transferEvt.From); err == nil {
					participants = append(participants, from)
				}
				if to, err = xdr.AddressToAccountId(transferEvt.To); err == nil {
					participants = append(participants, to)
				}
			case contractevents.EventTypeMint:
				mintEvt := sacEvent.(*contractevents.MintEvent)
				var to xdr.AccountId
				if to, err = xdr.AddressToAccountId(mintEvt.To); err == nil {
					participants = append(participants, to)
				}
			case contractevents.EventTypeClawback:
				clawbackEvt := sacEvent.(*contractevents.ClawbackEvent)
				var from xdr.AccountId
				if from, err = xdr.AddressToAccountId(clawbackEvt.From); err == nil {
					participants = append(participants, from)
				}
			case contractevents.EventTypeBurn:
				burnEvt := sacEvent.(*contractevents.BurnEvent)
				var from xdr.AccountId
				if from, err = xdr.AddressToAccountId(burnEvt.From); err == nil {
					participants = append(participants, from)
				}
			}
		}
	}
	return participants
}

// dedupeParticipants remove any duplicate ids from `in`
func dedupeParticipants(in []xdr.AccountId) []xdr.AccountId {
	if len(in) <= 1 {
		return in
	}
	sort.Slice(in, func(i, j int) bool {
		return in[i].Address() < in[j].Address()
	})
	insert := 1
	for cur := 1; cur < len(in); cur++ {
		if in[cur].Equals(in[cur-1]) {
			continue
		}
		if insert != cur {
			in[insert] = in[cur]
		}
		insert++
	}
	return in[:insert]

}

// OperationsParticipants returns a map with all participants per operation
func operationsParticipants(transaction ingest.LedgerTransaction, sequence uint32, network string) (map[int64][]xdr.AccountId, error) {
	participants := map[int64][]xdr.AccountId{}

	for opi, op := range transaction.Envelope.Operations() {
		operation := transactionOperationWrapper{
			index:          uint32(opi),
			transaction:    transaction,
			operation:      op,
			ledgerSequence: sequence,
			network:        network,
		}

		p, err := operation.Participants()
		if err != nil {
			return participants, errors.Wrapf(err, "reading operation %v participants", operation.ID())
		}
		participants[operation.ID()] = p
	}

	return participants, nil
}

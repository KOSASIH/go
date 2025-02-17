package processors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/guregu/null"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/ingest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon/base"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/support/contractevents"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

// EffectProcessor process effects
type EffectProcessor struct {
	accountLoader *history.AccountLoader
	batch         history.EffectBatchInsertBuilder
	network       string
}

func NewEffectProcessor(
	accountLoader *history.AccountLoader,
	batch history.EffectBatchInsertBuilder,
	network string,
) *EffectProcessor {
	return &EffectProcessor{
		accountLoader: accountLoader,
		batch:         batch,
		network:       network,
	}
}

func (p *EffectProcessor) Name() string {
	return "processors.EffectProcessor"
}

func (p *EffectProcessor) ProcessTransaction(
	lcm xdr.LedgerCloseMeta, transaction ingest.LedgerTransaction,
) error {
	// Failed transactions don't have operation effects
	if !transaction.Result.Successful() {
		return nil
	}

	for opi, op := range transaction.Envelope.Operations() {
		operation := transactionOperationWrapper{
			index:          uint32(opi),
			transaction:    transaction,
			operation:      op,
			ledgerSequence: uint32(lcm.LedgerSequence()),
			network:        p.network,
		}
		if err := operation.ingestEffects(p.accountLoader, p.batch); err != nil {
			return errors.Wrapf(err, "reading operation %v effects", operation.ID())
		}
	}
	return nil
}

func (p *EffectProcessor) Flush(ctx context.Context, session db.SessionInterface) (err error) {
	return p.batch.Exec(ctx, session)
}

// ingestEffects adds effects from the operation to the given EffectBatchInsertBuilder
func (operation *transactionOperationWrapper) ingestEffects(accountLoader *history.AccountLoader, batch history.EffectBatchInsertBuilder) error {
	if !operation.transaction.Result.Successful() {
		return nil
	}
	var (
		op  = operation.operation
		err error
	)

	changes, err := operation.transaction.GetOperationChanges(operation.index)
	if err != nil {
		return err
	}

	wrapper := &effectsWrapper{
		accountLoader: accountLoader,
		batch:         batch,
		order:         1,
		operation:     operation,
	}

	switch operation.OperationType() {
	case xdr.OperationTypeCreateAccount:
		err = wrapper.addAccountCreatedEffects()
	case xdr.OperationTypePayment:
		err = wrapper.addPaymentEffects()
	case xdr.OperationTypePathPaymentStrictReceive:
		err = wrapper.pathPaymentStrictReceiveEffects()
	case xdr.OperationTypePathPaymentStrictSend:
		err = wrapper.addPathPaymentStrictSendEffects()
	case xdr.OperationTypeManageSellOffer:
		err = wrapper.addManageSellOfferEffects()
	case xdr.OperationTypeManageBuyOffer:
		err = wrapper.addManageBuyOfferEffects()
	case xdr.OperationTypeCreatePassiveSellOffer:
		err = wrapper.addCreatePassiveSellOfferEffect()
	case xdr.OperationTypeSetOptions:
		err = wrapper.addSetOptionsEffects()
	case xdr.OperationTypeChangeTrust:
		err = wrapper.addChangeTrustEffects()
	case xdr.OperationTypeAllowTrust:
		err = wrapper.addAllowTrustEffects()
	case xdr.OperationTypeAccountMerge:
		err = wrapper.addAccountMergeEffects()
	case xdr.OperationTypeInflation:
		err = wrapper.addInflationEffects()
	case xdr.OperationTypeManageData:
		err = wrapper.addManageDataEffects()
	case xdr.OperationTypeBumpSequence:
		err = wrapper.addBumpSequenceEffects()
	case xdr.OperationTypeCreateClaimableBalance:
		err = wrapper.addCreateClaimableBalanceEffects(changes)
	case xdr.OperationTypeClaimClaimableBalance:
		err = wrapper.addClaimClaimableBalanceEffects(changes)
	case xdr.OperationTypeBeginSponsoringFutureReserves,
		xdr.OperationTypeEndSponsoringFutureReserves,
		xdr.OperationTypeRevokeSponsorship:
		// The effects of these operations are obtained indirectly from the
		// ledger entries
	case xdr.OperationTypeClawback:
		err = wrapper.addClawbackEffects()
	case xdr.OperationTypeClawbackClaimableBalance:
		err = wrapper.addClawbackClaimableBalanceEffects(changes)
	case xdr.OperationTypeSetTrustLineFlags:
		err = wrapper.addSetTrustLineFlagsEffects()
	case xdr.OperationTypeLiquidityPoolDeposit:
		err = wrapper.addLiquidityPoolDepositEffect()
	case xdr.OperationTypeLiquidityPoolWithdraw:
		err = wrapper.addLiquidityPoolWithdrawEffect()
	case xdr.OperationTypeInvokeHostFunction:
		// If there's an invokeHostFunction operation, there's definitely V3
		// meta in the transaction, which means this error is real.
		diagnosticEvents, innerErr := operation.transaction.GetDiagnosticEvents()
		if innerErr != nil {
			return innerErr
		}

		// For now, the only effects are related to the events themselves.
		// Possible add'l work: https://github.com/stellar/go/issues/4585
		err = wrapper.addInvokeHostFunctionEffects(filterEvents(diagnosticEvents))
	case xdr.OperationTypeExtendFootprintTtl, xdr.OperationTypeRestoreFootprint:
		// do not produce effects for these operations as horizon only provides
		// limited visibility into soroban operations
	default:
		err = fmt.Errorf("Unknown operation type: %s", op.Body.Type)
	}
	if err != nil {
		return err
	}

	// Effects generated for multiple operations. Keep the effect categories
	// separated so they are "together" in case of different order or meta
	// changes generate by core (unordered_map).

	// Sponsorships
	for _, change := range changes {
		if err := wrapper.addLedgerEntrySponsorshipEffects(change); err != nil {
			return err
		}
		if err := wrapper.addSignerSponsorshipEffects(change); err != nil {
			return err
		}
	}

	// Liquidity pools
	for _, change := range changes {

		// Effects caused by ChangeTrust (creation), AllowTrust and SetTrustlineFlags (removal through revocation)
		if err := wrapper.addLedgerEntryLiquidityPoolEffects(change); err != nil {
			return err
		}
	}

	return nil
}

func filterEvents(diagnosticEvents []xdr.DiagnosticEvent) []xdr.ContractEvent {
	var filtered []xdr.ContractEvent
	for _, diagnosticEvent := range diagnosticEvents {
		if !diagnosticEvent.InSuccessfulContractCall || diagnosticEvent.Event.Type != xdr.ContractEventTypeContract {
			continue
		}
		filtered = append(filtered, diagnosticEvent.Event)
	}
	return filtered
}

type effectsWrapper struct {
	accountLoader *history.AccountLoader
	batch         history.EffectBatchInsertBuilder
	order         uint32
	operation     *transactionOperationWrapper
}

func (e *effectsWrapper) add(address string, addressMuxed null.String, effectType history.EffectType, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return errors.Wrapf(err, "Error marshaling details for operation effect %v", e.operation.ID())
	}

	if err := e.batch.Add(
		e.accountLoader.GetFuture(address),
		addressMuxed,
		e.operation.ID(),
		e.order,
		effectType,
		detailsJSON,
	); err != nil {
		return errors.Wrap(err, "could not insert operation effect in db")
	}
	e.order++
	return nil
}

func (e *effectsWrapper) addUnmuxed(address *xdr.AccountId, effectType history.EffectType, details map[string]interface{}) error {
	return e.add(address.Address(), null.String{}, effectType, details)
}

func (e *effectsWrapper) addMuxed(address *xdr.MuxedAccount, effectType history.EffectType, details map[string]interface{}) error {
	var addressMuxed null.String
	if address.Type == xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
		addressMuxed = null.StringFrom(address.Address())
	}
	accID := address.ToAccountId()
	return e.add(accID.Address(), addressMuxed, effectType, details)
}

var sponsoringEffectsTable = map[xdr.LedgerEntryType]struct {
	created, updated, removed history.EffectType
}{
	xdr.LedgerEntryTypeAccount: {
		created: history.EffectAccountSponsorshipCreated,
		updated: history.EffectAccountSponsorshipUpdated,
		removed: history.EffectAccountSponsorshipRemoved,
	},
	xdr.LedgerEntryTypeTrustline: {
		created: history.EffectTrustlineSponsorshipCreated,
		updated: history.EffectTrustlineSponsorshipUpdated,
		removed: history.EffectTrustlineSponsorshipRemoved,
	},
	xdr.LedgerEntryTypeData: {
		created: history.EffectDataSponsorshipCreated,
		updated: history.EffectDataSponsorshipUpdated,
		removed: history.EffectDataSponsorshipRemoved,
	},
	xdr.LedgerEntryTypeClaimableBalance: {
		created: history.EffectClaimableBalanceSponsorshipCreated,
		updated: history.EffectClaimableBalanceSponsorshipUpdated,
		removed: history.EffectClaimableBalanceSponsorshipRemoved,
	},

	// We intentionally don't have Sponsoring effects for Offer
	// entries because we don't generate creation effects for them.
}

func (e *effectsWrapper) addSignerSponsorshipEffects(change ingest.Change) error {
	if change.Type != xdr.LedgerEntryTypeAccount {
		return nil
	}

	preSigners := map[string]xdr.AccountId{}
	postSigners := map[string]xdr.AccountId{}
	if change.Pre != nil {
		account := change.Pre.Data.MustAccount()
		preSigners = account.SponsorPerSigner()
	}
	if change.Post != nil {
		account := change.Post.Data.MustAccount()
		postSigners = account.SponsorPerSigner()
	}

	var all []string
	for signer := range preSigners {
		all = append(all, signer)
	}
	for signer := range postSigners {
		if _, ok := preSigners[signer]; ok {
			continue
		}
		all = append(all, signer)
	}
	sort.Strings(all)

	for _, signer := range all {
		pre, foundPre := preSigners[signer]
		post, foundPost := postSigners[signer]
		details := map[string]interface{}{}

		switch {
		case !foundPre && !foundPost:
			continue
		case !foundPre && foundPost:
			details["sponsor"] = post.Address()
			details["signer"] = signer
			srcAccount := change.Post.Data.MustAccount().AccountId
			if err := e.addUnmuxed(&srcAccount, history.EffectSignerSponsorshipCreated, details); err != nil {
				return err
			}
		case !foundPost && foundPre:
			details["former_sponsor"] = pre.Address()
			details["signer"] = signer
			srcAccount := change.Pre.Data.MustAccount().AccountId
			if err := e.addUnmuxed(&srcAccount, history.EffectSignerSponsorshipRemoved, details); err != nil {
				return err
			}
		case foundPre && foundPost:
			formerSponsor := pre.Address()
			newSponsor := post.Address()
			if formerSponsor == newSponsor {
				continue
			}

			details["former_sponsor"] = formerSponsor
			details["new_sponsor"] = newSponsor
			details["signer"] = signer
			srcAccount := change.Post.Data.MustAccount().AccountId
			if err := e.addUnmuxed(&srcAccount, history.EffectSignerSponsorshipUpdated, details); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *effectsWrapper) addLedgerEntrySponsorshipEffects(change ingest.Change) error {
	effectsForEntryType, found := sponsoringEffectsTable[change.Type]
	if !found {
		return nil
	}

	details := map[string]interface{}{}
	var effectType history.EffectType

	switch {
	case (change.Pre == nil || change.Pre.SponsoringID() == nil) &&
		(change.Post != nil && change.Post.SponsoringID() != nil):
		effectType = effectsForEntryType.created
		details["sponsor"] = (*change.Post.SponsoringID()).Address()
	case (change.Pre != nil && change.Pre.SponsoringID() != nil) &&
		(change.Post == nil || change.Post.SponsoringID() == nil):
		effectType = effectsForEntryType.removed
		details["former_sponsor"] = (*change.Pre.SponsoringID()).Address()
	case (change.Pre != nil && change.Pre.SponsoringID() != nil) &&
		(change.Post != nil && change.Post.SponsoringID() != nil):
		preSponsor := (*change.Pre.SponsoringID()).Address()
		postSponsor := (*change.Post.SponsoringID()).Address()
		if preSponsor == postSponsor {
			return nil
		}
		effectType = effectsForEntryType.updated
		details["new_sponsor"] = postSponsor
		details["former_sponsor"] = preSponsor
	default:
		return nil
	}

	var (
		accountID    *xdr.AccountId
		muxedAccount *xdr.MuxedAccount
	)

	var data xdr.LedgerEntryData
	if change.Post != nil {
		data = change.Post.Data
	} else {
		data = change.Pre.Data
	}

	switch change.Type {
	case xdr.LedgerEntryTypeAccount:
		a := data.MustAccount().AccountId
		accountID = &a
	case xdr.LedgerEntryTypeTrustline:
		tl := data.MustTrustLine()
		accountID = &tl.AccountId
		if tl.Asset.Type == xdr.AssetTypeAssetTypePoolShare {
			details["asset_type"] = "liquidity_pool"
			details["liquidity_pool_id"] = PoolIDToString(*tl.Asset.LiquidityPoolId)
		} else {
			details["asset"] = tl.Asset.ToAsset().StringCanonical()
		}
	case xdr.LedgerEntryTypeData:
		muxedAccount = e.operation.SourceAccount()
		details["data_name"] = data.MustData().DataName
	case xdr.LedgerEntryTypeClaimableBalance:
		muxedAccount = e.operation.SourceAccount()
		var err error
		details["balance_id"], err = xdr.MarshalHex(data.MustClaimableBalance().BalanceId)
		if err != nil {
			return errors.Wrapf(err, "Invalid balanceId in change from op %d", e.operation.index)
		}
	case xdr.LedgerEntryTypeLiquidityPool:
		// liquidity pools cannot be sponsored
		fallthrough
	default:
		return errors.Errorf("invalid sponsorship ledger entry type %v", change.Type.String())
	}

	if accountID != nil {
		if err := e.addUnmuxed(accountID, effectType, details); err != nil {
			return err
		}
	} else {
		if err := e.addMuxed(muxedAccount, effectType, details); err != nil {
			return err
		}
	}

	return nil
}

func (e *effectsWrapper) addLedgerEntryLiquidityPoolEffects(change ingest.Change) error {
	if change.Type != xdr.LedgerEntryTypeLiquidityPool {
		return nil
	}
	var effectType history.EffectType

	var details map[string]interface{}
	switch {
	case change.Pre == nil && change.Post != nil:
		effectType = history.EffectLiquidityPoolCreated
		details = map[string]interface{}{
			"liquidity_pool": liquidityPoolDetails(change.Post.Data.LiquidityPool),
		}
	case change.Pre != nil && change.Post == nil:
		effectType = history.EffectLiquidityPoolRemoved
		poolID := change.Pre.Data.LiquidityPool.LiquidityPoolId
		details = map[string]interface{}{
			"liquidity_pool_id": PoolIDToString(poolID),
		}
	default:
		return nil
	}
	return e.addMuxed(
		e.operation.SourceAccount(),
		effectType,
		details,
	)
}

func (e *effectsWrapper) addAccountCreatedEffects() error {
	op := e.operation.operation.Body.MustCreateAccountOp()

	if err := e.addUnmuxed(
		&op.Destination,
		history.EffectAccountCreated,
		map[string]interface{}{
			"starting_balance": amount.String(op.StartingBalance),
		},
	); err != nil {
		return err
	}
	if err := e.addMuxed(
		e.operation.SourceAccount(),
		history.EffectAccountDebited,
		map[string]interface{}{
			"asset_type": "native",
			"amount":     amount.String(op.StartingBalance),
		},
	); err != nil {
		return err
	}
	if err := e.addUnmuxed(
		&op.Destination,
		history.EffectSignerCreated,
		map[string]interface{}{
			"public_key": op.Destination.Address(),
			"weight":     keypair.DefaultSignerWeight,
		},
	); err != nil {
		return err
	}
	return nil
}

func (e *effectsWrapper) addPaymentEffects() error {
	op := e.operation.operation.Body.MustPaymentOp()

	details := map[string]interface{}{"amount": amount.String(op.Amount)}
	if err := addAssetDetails(details, op.Asset, ""); err != nil {
		return err
	}

	if err := e.addMuxed(
		&op.Destination,
		history.EffectAccountCredited,
		details,
	); err != nil {
		return err
	}
	return e.addMuxed(
		e.operation.SourceAccount(),
		history.EffectAccountDebited,
		details,
	)
}

func (e *effectsWrapper) pathPaymentStrictReceiveEffects() error {
	op := e.operation.operation.Body.MustPathPaymentStrictReceiveOp()
	resultSuccess := e.operation.OperationResult().MustPathPaymentStrictReceiveResult().MustSuccess()
	source := e.operation.SourceAccount()

	details := map[string]interface{}{"amount": amount.String(op.DestAmount)}
	if err := addAssetDetails(details, op.DestAsset, ""); err != nil {
		return err
	}

	if err := e.addMuxed(
		&op.Destination,
		history.EffectAccountCredited,
		details,
	); err != nil {
		return err
	}

	result := e.operation.OperationResult().MustPathPaymentStrictReceiveResult()
	details = map[string]interface{}{"amount": amount.String(result.SendAmount())}
	if err := addAssetDetails(details, op.SendAsset, ""); err != nil {
		return err
	}

	if err := e.addMuxed(
		source,
		history.EffectAccountDebited,
		details,
	); err != nil {
		return err
	}

	return e.addIngestTradeEffects(*source, resultSuccess.Offers)
}

func (e *effectsWrapper) addPathPaymentStrictSendEffects() error {
	source := e.operation.SourceAccount()
	op := e.operation.operation.Body.MustPathPaymentStrictSendOp()
	resultSuccess := e.operation.OperationResult().MustPathPaymentStrictSendResult().MustSuccess()
	result := e.operation.OperationResult().MustPathPaymentStrictSendResult()

	details := map[string]interface{}{"amount": amount.String(result.DestAmount())}
	if err := addAssetDetails(details, op.DestAsset, ""); err != nil {
		return err
	}
	if err := e.addMuxed(&op.Destination, history.EffectAccountCredited, details); err != nil {
		return err
	}

	details = map[string]interface{}{"amount": amount.String(op.SendAmount)}
	if err := addAssetDetails(details, op.SendAsset, ""); err != nil {
		return err
	}
	if err := e.addMuxed(source, history.EffectAccountDebited, details); err != nil {
		return err
	}

	return e.addIngestTradeEffects(*source, resultSuccess.Offers)
}

func (e *effectsWrapper) addManageSellOfferEffects() error {
	source := e.operation.SourceAccount()
	result := e.operation.OperationResult().MustManageSellOfferResult().MustSuccess()
	return e.addIngestTradeEffects(*source, result.OffersClaimed)
}

func (e *effectsWrapper) addManageBuyOfferEffects() error {
	source := e.operation.SourceAccount()
	result := e.operation.OperationResult().MustManageBuyOfferResult().MustSuccess()
	return e.addIngestTradeEffects(*source, result.OffersClaimed)
}

func (e *effectsWrapper) addCreatePassiveSellOfferEffect() error {
	result := e.operation.OperationResult()
	source := e.operation.SourceAccount()

	var claims []xdr.ClaimAtom

	// KNOWN ISSUE:  stellar-core creates results for CreatePassiveOffer operations
	// with the wrong result arm set.
	if result.Type == xdr.OperationTypeManageSellOffer {
		claims = result.MustManageSellOfferResult().MustSuccess().OffersClaimed
	} else {
		claims = result.MustCreatePassiveSellOfferResult().MustSuccess().OffersClaimed
	}

	return e.addIngestTradeEffects(*source, claims)
}

func (e *effectsWrapper) addSetOptionsEffects() error {
	source := e.operation.SourceAccount()
	op := e.operation.operation.Body.MustSetOptionsOp()

	if op.HomeDomain != nil {
		if err := e.addMuxed(source, history.EffectAccountHomeDomainUpdated,
			map[string]interface{}{
				"home_domain": string(*op.HomeDomain),
			},
		); err != nil {
			return err
		}
	}

	thresholdDetails := map[string]interface{}{}

	if op.LowThreshold != nil {
		thresholdDetails["low_threshold"] = *op.LowThreshold
	}

	if op.MedThreshold != nil {
		thresholdDetails["med_threshold"] = *op.MedThreshold
	}

	if op.HighThreshold != nil {
		thresholdDetails["high_threshold"] = *op.HighThreshold
	}

	if len(thresholdDetails) > 0 {
		if err := e.addMuxed(source, history.EffectAccountThresholdsUpdated, thresholdDetails); err != nil {
			return err
		}
	}

	flagDetails := map[string]interface{}{}
	if op.SetFlags != nil {
		setAuthFlagDetails(flagDetails, xdr.AccountFlags(*op.SetFlags), true)
	}
	if op.ClearFlags != nil {
		setAuthFlagDetails(flagDetails, xdr.AccountFlags(*op.ClearFlags), false)
	}

	if len(flagDetails) > 0 {
		if err := e.addMuxed(source, history.EffectAccountFlagsUpdated, flagDetails); err != nil {
			return err
		}
	}

	if op.InflationDest != nil {
		if err := e.addMuxed(source, history.EffectAccountInflationDestinationUpdated,
			map[string]interface{}{
				"inflation_destination": op.InflationDest.Address(),
			},
		); err != nil {
			return err
		}
	}
	changes, err := e.operation.transaction.GetOperationChanges(e.operation.index)
	if err != nil {
		return err
	}

	for _, change := range changes {
		if change.Type != xdr.LedgerEntryTypeAccount {
			continue
		}

		beforeAccount := change.Pre.Data.MustAccount()
		afterAccount := change.Post.Data.MustAccount()

		before := beforeAccount.SignerSummary()
		after := afterAccount.SignerSummary()

		// if before and after are the same, the signers have not changed
		if reflect.DeepEqual(before, after) {
			continue
		}

		var beforeSortedSigners []string
		for signer := range before {
			beforeSortedSigners = append(beforeSortedSigners, signer)
		}
		sort.Strings(beforeSortedSigners)

		for _, addy := range beforeSortedSigners {
			weight, ok := after[addy]
			if !ok {
				if err := e.addMuxed(source, history.EffectSignerRemoved, map[string]interface{}{
					"public_key": addy,
				}); err != nil {
					return err
				}
				continue
			}

			if weight != before[addy] {
				if err := e.addMuxed(source, history.EffectSignerUpdated, map[string]interface{}{
					"public_key": addy,
					"weight":     weight,
				}); err != nil {
					return err
				}
			}
		}

		var afterSortedSigners []string
		for signer := range after {
			afterSortedSigners = append(afterSortedSigners, signer)
		}
		sort.Strings(afterSortedSigners)

		// Add the "created" effects
		for _, addy := range afterSortedSigners {
			weight := after[addy]
			// if `addy` is in before, the previous for loop should have recorded
			// the update, so skip this key
			if _, ok := before[addy]; ok {
				continue
			}

			if err := e.addMuxed(source, history.EffectSignerCreated, map[string]interface{}{
				"public_key": addy,
				"weight":     weight,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *effectsWrapper) addChangeTrustEffects() error {
	source := e.operation.SourceAccount()

	op := e.operation.operation.Body.MustChangeTrustOp()
	changes, err := e.operation.transaction.GetOperationChanges(e.operation.index)
	if err != nil {
		return err
	}

	// NOTE:  when an account trusts itself, the transaction is successful but
	// no ledger entries are actually modified.
	for _, change := range changes {
		if change.Type != xdr.LedgerEntryTypeTrustline {
			continue
		}

		var (
			effect    history.EffectType
			trustLine xdr.TrustLineEntry
		)

		switch {
		case change.Pre == nil && change.Post != nil:
			effect = history.EffectTrustlineCreated
			trustLine = *change.Post.Data.TrustLine
		case change.Pre != nil && change.Post == nil:
			effect = history.EffectTrustlineRemoved
			trustLine = *change.Pre.Data.TrustLine
		case change.Pre != nil && change.Post != nil:
			effect = history.EffectTrustlineUpdated
			trustLine = *change.Post.Data.TrustLine
		default:
			panic("Invalid change")
		}

		// We want to add a single effect for change_trust op. If it's modifying
		// credit_asset search for credit_asset trustline, otherwise search for
		// liquidity_pool.
		if op.Line.Type != trustLine.Asset.Type {
			continue
		}

		details := map[string]interface{}{"limit": amount.String(op.Limit)}
		if trustLine.Asset.Type == xdr.AssetTypeAssetTypePoolShare {
			// The only change_trust ops that can modify LP are those with
			// asset=liquidity_pool so *op.Line.LiquidityPool below is available.
			if err := addLiquidityPoolAssetDetails(details, *op.Line.LiquidityPool); err != nil {
				return err
			}
		} else {
			if err := addAssetDetails(details, op.Line.ToAsset(), ""); err != nil {
				return err
			}
		}

		if err := e.addMuxed(source, effect, details); err != nil {
			return err
		}
		break
	}

	return nil
}

func (e *effectsWrapper) addAllowTrustEffects() error {
	source := e.operation.SourceAccount()
	op := e.operation.operation.Body.MustAllowTrustOp()
	asset := op.Asset.ToAsset(source.ToAccountId())
	details := map[string]interface{}{
		"trustor": op.Trustor.Address(),
	}
	if err := addAssetDetails(details, asset, ""); err != nil {
		return err
	}

	switch {
	case xdr.TrustLineFlags(op.Authorize).IsAuthorized():
		if err := e.addMuxed(source, history.EffectTrustlineAuthorized, details); err != nil {
			return err
		}
		// Forward compatibility
		setFlags := xdr.Uint32(xdr.TrustLineFlagsAuthorizedFlag)
		if err := e.addTrustLineFlagsEffect(source, &op.Trustor, asset, &setFlags, nil); err != nil {
			return err
		}
	case xdr.TrustLineFlags(op.Authorize).IsAuthorizedToMaintainLiabilitiesFlag():
		if err := e.addMuxed(
			source,
			history.EffectTrustlineAuthorizedToMaintainLiabilities,
			details,
		); err != nil {
			return err
		}
		// Forward compatibility
		setFlags := xdr.Uint32(xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag)
		if err := e.addTrustLineFlagsEffect(source, &op.Trustor, asset, &setFlags, nil); err != nil {
			return err
		}
	default:
		if err := e.addMuxed(source, history.EffectTrustlineDeauthorized, details); err != nil {
			return err
		}
		// Forward compatibility, show both as cleared
		clearFlags := xdr.Uint32(xdr.TrustLineFlagsAuthorizedFlag | xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag)
		if err := e.addTrustLineFlagsEffect(source, &op.Trustor, asset, nil, &clearFlags); err != nil {
			return err
		}
	}
	return e.addLiquidityPoolRevokedEffect()
}

func (e *effectsWrapper) addAccountMergeEffects() error {
	source := e.operation.SourceAccount()

	dest := e.operation.operation.Body.MustDestination()
	result := e.operation.OperationResult().MustAccountMergeResult()
	details := map[string]interface{}{
		"amount":     amount.String(result.MustSourceAccountBalance()),
		"asset_type": "native",
	}

	if err := e.addMuxed(source, history.EffectAccountDebited, details); err != nil {
		return err
	}
	if err := e.addMuxed(&dest, history.EffectAccountCredited, details); err != nil {
		return err
	}
	if err := e.addMuxed(source, history.EffectAccountRemoved, map[string]interface{}{}); err != nil {
		return err
	}
	return nil
}

func (e *effectsWrapper) addInflationEffects() error {
	payouts := e.operation.OperationResult().MustInflationResult().MustPayouts()
	for _, payout := range payouts {
		if err := e.addUnmuxed(&payout.Destination, history.EffectAccountCredited,
			map[string]interface{}{
				"amount":     amount.String(payout.Amount),
				"asset_type": "native",
			},
		); err != nil {
			return err
		}
	}
	return nil
}

func (e *effectsWrapper) addManageDataEffects() error {
	source := e.operation.SourceAccount()
	op := e.operation.operation.Body.MustManageDataOp()
	details := map[string]interface{}{"name": op.DataName}
	effect := history.EffectType(0)
	changes, err := e.operation.transaction.GetOperationChanges(e.operation.index)
	if err != nil {
		return err
	}

	for _, change := range changes {
		if change.Type != xdr.LedgerEntryTypeData {
			continue
		}

		before := change.Pre
		after := change.Post

		if after != nil {
			raw := after.Data.MustData().DataValue
			details["value"] = base64.StdEncoding.EncodeToString(raw)
		}

		switch {
		case before == nil && after != nil:
			effect = history.EffectDataCreated
		case before != nil && after == nil:
			effect = history.EffectDataRemoved
		case before != nil && after != nil:
			effect = history.EffectDataUpdated
		default:
			panic("Invalid before-and-after state")
		}

		break
	}

	return e.addMuxed(source, effect, details)
}

func (e *effectsWrapper) addBumpSequenceEffects() error {
	source := e.operation.SourceAccount()
	changes, err := e.operation.transaction.GetOperationChanges(e.operation.index)
	if err != nil {
		return err
	}

	for _, change := range changes {
		if change.Type != xdr.LedgerEntryTypeAccount {
			continue
		}

		before := change.Pre
		after := change.Post

		beforeAccount := before.Data.MustAccount()
		afterAccount := after.Data.MustAccount()

		if beforeAccount.SeqNum != afterAccount.SeqNum {
			details := map[string]interface{}{"new_seq": afterAccount.SeqNum}
			if err := e.addMuxed(source, history.EffectSequenceBumped, details); err != nil {
				return err
			}
		}
		break
	}

	return nil
}

func setClaimableBalanceFlagDetails(details map[string]interface{}, flags xdr.ClaimableBalanceFlags) {
	if flags.IsClawbackEnabled() {
		details["claimable_balance_clawback_enabled_flag"] = true
		return
	}
}

func (e *effectsWrapper) addCreateClaimableBalanceEffects(changes []ingest.Change) error {
	source := e.operation.SourceAccount()
	var cb *xdr.ClaimableBalanceEntry
	for _, change := range changes {
		if change.Type != xdr.LedgerEntryTypeClaimableBalance || change.Post == nil {
			continue
		}
		cb = change.Post.Data.ClaimableBalance
		if err := e.addClaimableBalanceEntryCreatedEffects(source, cb); err != nil {
			return err
		}
		break
	}
	if cb == nil {
		return errors.New("claimable balance entry not found")
	}

	details := map[string]interface{}{
		"amount": amount.String(cb.Amount),
	}
	if err := addAssetDetails(details, cb.Asset, ""); err != nil {
		return err
	}
	return e.addMuxed(
		source,
		history.EffectAccountDebited,
		details,
	)
}

func (e *effectsWrapper) addClaimableBalanceEntryCreatedEffects(source *xdr.MuxedAccount, cb *xdr.ClaimableBalanceEntry) error {
	id, err := xdr.MarshalHex(cb.BalanceId)
	if err != nil {
		return err
	}
	details := map[string]interface{}{
		"balance_id": id,
		"amount":     amount.String(cb.Amount),
		"asset":      cb.Asset.StringCanonical(),
	}
	setClaimableBalanceFlagDetails(details, cb.Flags())
	if err := e.addMuxed(
		source,
		history.EffectClaimableBalanceCreated,
		details,
	); err != nil {
		return err
	}
	// EffectClaimableBalanceClaimantCreated can be generated by
	// `create_claimable_balance` operation but also by `liquidity_pool_withdraw`
	// operation causing a revocation.
	// In case of `create_claimable_balance` we use `op.Claimants` to make
	// effects backward compatible. The reason for this is that Stellar-Core
	// changes all `rel_before` predicated to `abs_before` when tx is included
	// in the ledger.
	var claimants []xdr.Claimant
	if op, ok := e.operation.operation.Body.GetCreateClaimableBalanceOp(); ok {
		claimants = op.Claimants
	} else {
		claimants = cb.Claimants
	}
	for _, c := range claimants {
		cv0 := c.MustV0()
		if err := e.addUnmuxed(
			&cv0.Destination,
			history.EffectClaimableBalanceClaimantCreated,
			map[string]interface{}{
				"balance_id": id,
				"amount":     amount.String(cb.Amount),
				"predicate":  cv0.Predicate,
				"asset":      cb.Asset.StringCanonical(),
			},
		); err != nil {
			return err
		}
	}
	return nil
}

func (e *effectsWrapper) addClaimClaimableBalanceEffects(changes []ingest.Change) error {
	op := e.operation.operation.Body.MustClaimClaimableBalanceOp()

	balanceID, err := xdr.MarshalHex(op.BalanceId)
	if err != nil {
		return fmt.Errorf("Invalid balanceId in op: %d", e.operation.index)
	}

	var cBalance xdr.ClaimableBalanceEntry
	found := false
	for _, change := range changes {
		if change.Type != xdr.LedgerEntryTypeClaimableBalance {
			continue
		}

		if change.Pre != nil && change.Post == nil {
			cBalance = change.Pre.Data.MustClaimableBalance()
			preBalanceID, err := xdr.MarshalHex(cBalance.BalanceId)
			if err != nil {
				return fmt.Errorf("Invalid balanceId in meta changes for op: %d", e.operation.index)
			}

			if preBalanceID == balanceID {
				found = true
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("Change not found for balanceId : %s", balanceID)
	}

	details := map[string]interface{}{
		"amount":     amount.String(cBalance.Amount),
		"balance_id": balanceID,
		"asset":      cBalance.Asset.StringCanonical(),
	}
	setClaimableBalanceFlagDetails(details, cBalance.Flags())
	source := e.operation.SourceAccount()
	if err := e.addMuxed(
		source,
		history.EffectClaimableBalanceClaimed,
		details,
	); err != nil {
		return err
	}

	details = map[string]interface{}{
		"amount": amount.String(cBalance.Amount),
	}
	if err := addAssetDetails(details, cBalance.Asset, ""); err != nil {
		return err
	}
	return e.addMuxed(
		source,
		history.EffectAccountCredited,
		details,
	)
}

func (e *effectsWrapper) addIngestTradeEffects(buyer xdr.MuxedAccount, claims []xdr.ClaimAtom) error {
	for _, claim := range claims {
		if claim.AmountSold() == 0 && claim.AmountBought() == 0 {
			continue
		}
		switch claim.Type {
		case xdr.ClaimAtomTypeClaimAtomTypeLiquidityPool:
			if err := e.addClaimLiquidityPoolTradeEffect(claim); err != nil {
				return err
			}
		default:
			if err := e.addClaimTradeEffects(buyer, claim); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *effectsWrapper) addClaimTradeEffects(buyer xdr.MuxedAccount, claim xdr.ClaimAtom) error {
	seller := claim.SellerId()
	bd, sd, err := tradeDetails(buyer, seller, claim)
	if err != nil {
		return err
	}

	if err := e.addMuxed(
		&buyer,
		history.EffectTrade,
		bd,
	); err != nil {
		return err
	}

	return e.addUnmuxed(
		&seller,
		history.EffectTrade,
		sd,
	)
}

func (e *effectsWrapper) addClaimLiquidityPoolTradeEffect(claim xdr.ClaimAtom) error {
	lp, _, err := e.operation.getLiquidityPoolAndProductDelta(&claim.LiquidityPool.LiquidityPoolId)
	if err != nil {
		return err
	}
	details := map[string]interface{}{
		"liquidity_pool": liquidityPoolDetails(lp),
		"sold": map[string]string{
			"asset":  claim.LiquidityPool.AssetSold.StringCanonical(),
			"amount": amount.String(claim.LiquidityPool.AmountSold),
		},
		"bought": map[string]string{
			"asset":  claim.LiquidityPool.AssetBought.StringCanonical(),
			"amount": amount.String(claim.LiquidityPool.AmountBought),
		},
	}
	return e.addMuxed(e.operation.SourceAccount(), history.EffectLiquidityPoolTrade, details)
}

func (e *effectsWrapper) addClawbackEffects() error {
	op := e.operation.operation.Body.MustClawbackOp()
	details := map[string]interface{}{
		"amount": amount.String(op.Amount),
	}
	source := e.operation.SourceAccount()
	if err := addAssetDetails(details, op.Asset, ""); err != nil {
		return err
	}

	// The funds will be burned, but even with that, we generated an account credited effect
	if err := e.addMuxed(
		source,
		history.EffectAccountCredited,
		details,
	); err != nil {
		return err
	}

	if err := e.addMuxed(
		&op.From,
		history.EffectAccountDebited,
		details,
	); err != nil {
		return err
	}

	return nil
}

func (e *effectsWrapper) addClawbackClaimableBalanceEffects(changes []ingest.Change) error {
	op := e.operation.operation.Body.MustClawbackClaimableBalanceOp()
	balanceId, err := xdr.MarshalHex(op.BalanceId)
	if err != nil {
		return errors.Wrapf(err, "Invalid balanceId in op %d", e.operation.index)
	}
	details := map[string]interface{}{
		"balance_id": balanceId,
	}
	source := e.operation.SourceAccount()
	if err := e.addMuxed(
		source,
		history.EffectClaimableBalanceClawedBack,
		details,
	); err != nil {
		return err
	}

	// Generate the account credited effect (although the funds will be burned) for the asset issuer
	for _, c := range changes {
		if c.Type == xdr.LedgerEntryTypeClaimableBalance && c.Post == nil && c.Pre != nil {
			cb := c.Pre.Data.ClaimableBalance
			details = map[string]interface{}{"amount": amount.String(cb.Amount)}
			if err := addAssetDetails(details, cb.Asset, ""); err != nil {
				return err
			}
			if err := e.addMuxed(
				source,
				history.EffectAccountCredited,
				details,
			); err != nil {
				return err
			}
			break
		}
	}

	return nil
}

func (e *effectsWrapper) addSetTrustLineFlagsEffects() error {
	source := e.operation.SourceAccount()
	op := e.operation.operation.Body.MustSetTrustLineFlagsOp()
	if err := e.addTrustLineFlagsEffect(source, &op.Trustor, op.Asset, &op.SetFlags, &op.ClearFlags); err != nil {
		return err
	}
	return e.addLiquidityPoolRevokedEffect()
}

func (e *effectsWrapper) addTrustLineFlagsEffect(
	account *xdr.MuxedAccount,
	trustor *xdr.AccountId,
	asset xdr.Asset,
	setFlags *xdr.Uint32,
	clearFlags *xdr.Uint32) error {
	details := map[string]interface{}{
		"trustor": trustor.Address(),
	}
	if err := addAssetDetails(details, asset, ""); err != nil {
		return err
	}

	var flagDetailsAdded bool
	if setFlags != nil {
		setTrustLineFlagDetails(details, xdr.TrustLineFlags(*setFlags), true)
		flagDetailsAdded = true
	}
	if clearFlags != nil {
		setTrustLineFlagDetails(details, xdr.TrustLineFlags(*clearFlags), false)
		flagDetailsAdded = true
	}

	if flagDetailsAdded {
		if err := e.addMuxed(account, history.EffectTrustlineFlagsUpdated, details); err != nil {
			return err
		}
	}
	return nil
}

func setTrustLineFlagDetails(flagDetails map[string]interface{}, flags xdr.TrustLineFlags, setValue bool) {
	if flags.IsAuthorized() {
		flagDetails["authorized_flag"] = setValue
	}
	if flags.IsAuthorizedToMaintainLiabilitiesFlag() {
		flagDetails["authorized_to_maintain_liabilites"] = setValue
	}
	if flags.IsClawbackEnabledFlag() {
		flagDetails["clawback_enabled_flag"] = setValue
	}
}

func (e *effectsWrapper) addLiquidityPoolRevokedEffect() error {
	source := e.operation.SourceAccount()
	lp, delta, err := e.operation.getLiquidityPoolAndProductDelta(nil)
	if err != nil {
		if err == errLiquidityPoolChangeNotFound {
			// no revocation happened
			return nil
		}
		return err
	}
	changes, err := e.operation.transaction.GetOperationChanges(e.operation.index)
	if err != nil {
		return err
	}
	assetToCBID := map[string]string{}
	for _, change := range changes {
		if change.Type == xdr.LedgerEntryTypeClaimableBalance && change.Pre == nil && change.Post != nil {
			cb := change.Post.Data.ClaimableBalance
			id, err := xdr.MarshalHex(cb.BalanceId)
			if err != nil {
				return err
			}
			assetToCBID[cb.Asset.StringCanonical()] = id
			if err := e.addClaimableBalanceEntryCreatedEffects(source, cb); err != nil {
				return err
			}
		}
	}
	if len(assetToCBID) == 0 {
		// no claimable balances were created, and thus, no revocation happened
		return nil
	}

	reservesRevoked := make([]map[string]string, 0, 2)
	for _, aa := range []base.AssetAmount{
		{
			Asset:  lp.Body.ConstantProduct.Params.AssetA.StringCanonical(),
			Amount: amount.String(-delta.ReserveA),
		},
		{
			Asset:  lp.Body.ConstantProduct.Params.AssetB.StringCanonical(),
			Amount: amount.String(-delta.ReserveB),
		},
	} {
		if cbID, ok := assetToCBID[aa.Asset]; ok {
			assetAmountDetail := map[string]string{
				"asset":                aa.Asset,
				"amount":               aa.Amount,
				"claimable_balance_id": cbID,
			}
			reservesRevoked = append(reservesRevoked, assetAmountDetail)
		}
	}
	details := map[string]interface{}{
		"liquidity_pool":   liquidityPoolDetails(lp),
		"reserves_revoked": reservesRevoked,
		"shares_revoked":   amount.String(-delta.TotalPoolShares),
	}

	return e.addMuxed(source, history.EffectLiquidityPoolRevoked, details)
}

func setAuthFlagDetails(flagDetails map[string]interface{}, flags xdr.AccountFlags, setValue bool) {
	if flags.IsAuthRequired() {
		flagDetails["auth_required_flag"] = setValue
	}
	if flags.IsAuthRevocable() {
		flagDetails["auth_revocable_flag"] = setValue
	}
	if flags.IsAuthImmutable() {
		flagDetails["auth_immutable_flag"] = setValue
	}
	if flags.IsAuthClawbackEnabled() {
		flagDetails["auth_clawback_enabled_flag"] = setValue
	}
}

func tradeDetails(buyer xdr.MuxedAccount, seller xdr.AccountId, claim xdr.ClaimAtom) (bd map[string]interface{}, sd map[string]interface{}, err error) {
	bd = map[string]interface{}{
		"offer_id":      claim.OfferId(),
		"seller":        seller.Address(),
		"bought_amount": amount.String(claim.AmountSold()),
		"sold_amount":   amount.String(claim.AmountBought()),
	}
	if err = addAssetDetails(bd, claim.AssetSold(), "bought_"); err != nil {
		return
	}
	if err = addAssetDetails(bd, claim.AssetBought(), "sold_"); err != nil {
		return
	}

	sd = map[string]interface{}{
		"offer_id":      claim.OfferId(),
		"bought_amount": amount.String(claim.AmountBought()),
		"sold_amount":   amount.String(claim.AmountSold()),
	}
	addAccountAndMuxedAccountDetails(sd, buyer, "seller")
	if err = addAssetDetails(sd, claim.AssetBought(), "bought_"); err != nil {
		return
	}
	if err = addAssetDetails(sd, claim.AssetSold(), "sold_"); err != nil {
		return
	}
	return
}

func liquidityPoolDetails(lp *xdr.LiquidityPoolEntry) map[string]interface{} {
	return map[string]interface{}{
		"id":               PoolIDToString(lp.LiquidityPoolId),
		"fee_bp":           uint32(lp.Body.ConstantProduct.Params.Fee),
		"type":             "constant_product",
		"total_trustlines": strconv.FormatInt(int64(lp.Body.ConstantProduct.PoolSharesTrustLineCount), 10),
		"total_shares":     amount.String(lp.Body.ConstantProduct.TotalPoolShares),
		"reserves": []base.AssetAmount{
			{
				Asset:  lp.Body.ConstantProduct.Params.AssetA.StringCanonical(),
				Amount: amount.String(lp.Body.ConstantProduct.ReserveA),
			},
			{
				Asset:  lp.Body.ConstantProduct.Params.AssetB.StringCanonical(),
				Amount: amount.String(lp.Body.ConstantProduct.ReserveB),
			},
		},
	}
}

func (e *effectsWrapper) addLiquidityPoolDepositEffect() error {
	op := e.operation.operation.Body.MustLiquidityPoolDepositOp()
	lp, delta, err := e.operation.getLiquidityPoolAndProductDelta(&op.LiquidityPoolId)
	if err != nil {
		return err
	}
	details := map[string]interface{}{
		"liquidity_pool": liquidityPoolDetails(lp),
		"reserves_deposited": []base.AssetAmount{
			{
				Asset:  lp.Body.ConstantProduct.Params.AssetA.StringCanonical(),
				Amount: amount.String(delta.ReserveA),
			},
			{
				Asset:  lp.Body.ConstantProduct.Params.AssetB.StringCanonical(),
				Amount: amount.String(delta.ReserveB),
			},
		},
		"shares_received": amount.String(delta.TotalPoolShares),
	}

	return e.addMuxed(e.operation.SourceAccount(), history.EffectLiquidityPoolDeposited, details)
}

func (e *effectsWrapper) addLiquidityPoolWithdrawEffect() error {
	op := e.operation.operation.Body.MustLiquidityPoolWithdrawOp()
	lp, delta, err := e.operation.getLiquidityPoolAndProductDelta(&op.LiquidityPoolId)
	if err != nil {
		return err
	}
	details := map[string]interface{}{
		"liquidity_pool": liquidityPoolDetails(lp),
		"reserves_received": []base.AssetAmount{
			{
				Asset:  lp.Body.ConstantProduct.Params.AssetA.StringCanonical(),
				Amount: amount.String(-delta.ReserveA),
			},
			{
				Asset:  lp.Body.ConstantProduct.Params.AssetB.StringCanonical(),
				Amount: amount.String(-delta.ReserveB),
			},
		},
		"shares_redeemed": amount.String(-delta.TotalPoolShares),
	}

	return e.addMuxed(e.operation.SourceAccount(), history.EffectLiquidityPoolWithdrew, details)
}

// addInvokeHostFunctionEffects iterates through the events and generates
// account_credited and account_debited effects when it sees events related to
// the Stellar Asset Contract corresponding to those effects.
func (e *effectsWrapper) addInvokeHostFunctionEffects(events []contractevents.Event) error {
	if e.operation.network == "" {
		return errors.New("invokeHostFunction effects cannot be determined unless network passphrase is set")
	}

	source := e.operation.SourceAccount()
	for _, event := range events {
		evt, err := contractevents.NewStellarAssetContractEvent(&event, e.operation.network)
		if err != nil {
			continue // irrelevant or unsupported event
		}

		details := make(map[string]interface{}, 4)
		if err := addAssetDetails(details, evt.GetAsset(), ""); err != nil {
			return errors.Wrapf(err, "invokeHostFunction asset details had an error")
		}

		//
		// Note: We ignore effects that involve contracts (until the day we have
		// contract_debited/credited effects, may it never come :pray:)
		//

		switch evt.GetType() {
		// Transfer events generate an `account_debited` effect for the `from`
		// (sender) and an `account_credited` effect for the `to` (recipient).
		case contractevents.EventTypeTransfer:
			transferEvent := evt.(*contractevents.TransferEvent)
			details["amount"] = amount.String128(transferEvent.Amount)
			toDetails := map[string]interface{}{}
			for key, val := range details {
				toDetails[key] = val
			}

			if strkey.IsValidEd25519PublicKey(transferEvent.From) {
				if err := e.add(
					transferEvent.From,
					null.String{},
					history.EffectAccountDebited,
					details,
				); err != nil {
					return errors.Wrapf(err, "invokeHostFunction asset details from contract xfr-from had an error")
				}
			} else {
				details["contract"] = transferEvent.From
				e.addMuxed(source, history.EffectContractDebited, details)
			}

			if strkey.IsValidEd25519PublicKey(transferEvent.To) {
				if err := e.add(
					transferEvent.To,
					null.String{},
					history.EffectAccountCredited,
					toDetails,
				); err != nil {
					return errors.Wrapf(err, "invokeHostFunction asset details from contract xfr-to had an error")
				}
			} else {
				toDetails["contract"] = transferEvent.To
				e.addMuxed(source, history.EffectContractCredited, toDetails)
			}

		// Mint events imply a non-native asset, and it results in a credit to
		// the `to` recipient.
		case contractevents.EventTypeMint:
			mintEvent := evt.(*contractevents.MintEvent)
			details["amount"] = amount.String128(mintEvent.Amount)
			if strkey.IsValidEd25519PublicKey(mintEvent.To) {
				if err := e.add(
					mintEvent.To,
					null.String{},
					history.EffectAccountCredited,
					details,
				); err != nil {
					return errors.Wrapf(err, "invokeHostFunction asset details from contract mint had an error")
				}
			} else {
				details["contract"] = mintEvent.To
				e.addMuxed(source, history.EffectContractCredited, details)
			}

		// Clawback events result in a debit to the `from` address, but acts
		// like a burn to the recipient, so these are functionally equivalent
		case contractevents.EventTypeClawback:
			cbEvent := evt.(*contractevents.ClawbackEvent)
			details["amount"] = amount.String128(cbEvent.Amount)
			if strkey.IsValidEd25519PublicKey(cbEvent.From) {
				if err := e.add(
					cbEvent.From,
					null.String{},
					history.EffectAccountDebited,
					details,
				); err != nil {
					return errors.Wrapf(err, "invokeHostFunction asset details from contract clawback had an error")
				}
			} else {
				details["contract"] = cbEvent.From
				e.addMuxed(source, history.EffectContractDebited, details)
			}

		case contractevents.EventTypeBurn:
			burnEvent := evt.(*contractevents.BurnEvent)
			details["amount"] = amount.String128(burnEvent.Amount)
			if strkey.IsValidEd25519PublicKey(burnEvent.From) {
				if err := e.add(
					burnEvent.From,
					null.String{},
					history.EffectAccountDebited,
					details,
				); err != nil {
					return errors.Wrapf(err, "invokeHostFunction asset details from contract burn had an error")
				}
			} else {
				details["contract"] = burnEvent.From
				e.addMuxed(source, history.EffectContractDebited, details)
			}
		}
	}

	return nil
}

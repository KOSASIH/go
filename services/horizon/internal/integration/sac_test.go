package integration

import (
	"context"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/protocols/horizon/operations"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

const sac_contract = "soroban_sac_test.wasm"

// LongTermTTL is used to extend the lifetime of ledger entries by 10000 ledgers.
// This will ensure that the ledger entries never expire during the execution
// of the integration tests.
const LongTermTTL = 10000

// Tests use precompiled wasm bin files that are added to the testdata directory.
// Refer to ./services/horizon/internal/integration/contracts/README.md on how to recompile
// contract code if needed to new wasm.

func TestContractMintToAccount(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		HorizonEnvironment: map[string]string{"INGEST_DISABLE_STATE_VERIFICATION": "true", "CONNECTION_TIMEOUT": "360000"},
		EnableStellarRPC:   true,
		QuickExpiration:    true,
	})

	issuer := itest.Master().Address()
	code := "USD"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	recipientKp, recipient := itest.CreateAccount("100")
	itest.MustEstablishTrustline(recipientKp, recipient, txnbuild.MustAssetFromXDR(asset))

	_, mintTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		mint(itest, issuer, asset, "20", accountAddressParam(recipient.GetAccountID())),
	)
	assertAccountInvokeHostFunctionOperation(itest, recipientKp.Address(), "", recipientKp.Address(), "20.0000000")
	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("20"))
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          amount.MustParse("20"),
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             0,
		balanceContracts:         big.NewInt(0),
		contractID:               stellarAssetContractID(itest, asset),
	})

	fx := getTxEffects(itest, mintTx, asset)
	require.Len(t, fx, 1)
	creditEffect := assertContainsEffect(t, fx,
		effects.EffectAccountCredited)[0].(effects.AccountCredited)
	assert.Equal(t, recipientKp.Address(), creditEffect.Account)
	assert.Equal(t, issuer, creditEffect.Asset.Issuer)
	assert.Equal(t, code, creditEffect.Asset.Code)
	assert.Equal(t, "20.0000000", creditEffect.Amount)
	assertEventPayments(itest, mintTx, asset, "", recipient.GetAccountID(), "mint", "20.0000000")

	otherRecipientKp, otherRecipient := itest.CreateAccount("100")
	itest.MustEstablishTrustline(otherRecipientKp, otherRecipient, txnbuild.MustAssetFromXDR(asset))

	// calling transfer from the issuer account will also mint the asset
	_, transferTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		transfer(itest, issuer, asset, "30", accountAddressParam(otherRecipient.GetAccountID())),
	)
	assertAccountInvokeHostFunctionOperation(itest, otherRecipientKp.Address(), issuer, otherRecipientKp.Address(), "30.0000000")
	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("20"))
	assertContainsBalance(itest, otherRecipientKp, issuer, code, amount.MustParse("30"))

	fx = getTxEffects(itest, transferTx, asset)
	assert.Len(t, fx, 2)
	assertContainsEffect(t, fx,
		effects.EffectAccountCredited,
		effects.EffectAccountDebited)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              2,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		balanceAccounts:          amount.MustParse("50"),
		numContracts:             0,
		balanceContracts:         big.NewInt(0),
		contractID:               stellarAssetContractID(itest, asset),
	})
}

func createSAC(itest *integration.Test, asset xdr.Asset) {
	invokeHostFunction := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeCreateContract,
			CreateContract: &xdr.CreateContractArgs{
				ContractIdPreimage: xdr.ContractIdPreimage{
					Type:      xdr.ContractIdPreimageTypeContractIdPreimageFromAsset,
					FromAsset: &asset,
				},
				Executable: xdr.ContractExecutable{
					Type:     xdr.ContractExecutableTypeContractExecutableStellarAsset,
					WasmHash: nil,
				},
			},
		},
		SourceAccount: itest.Master().Address(),
	}
	_, _, preFlightOp := assertInvokeHostFnSucceeds(itest, itest.Master(), invokeHostFunction)
	sourceAccount, extendTTLOp, minFee := itest.PreflightExtendExpiration(
		itest.Master().Address(),
		preFlightOp.Ext.SorobanData.Resources.Footprint.ReadWrite,
		LongTermTTL,
	)
	itest.MustSubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &extendTTLOp)
}

func TestContractMintToContract(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		QuickExpiration:  true,
	})

	issuer := itest.Master().Address()
	code := "USD"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	// Create recipient contract
	recipientContractID, _ := mustCreateAndInstallContract(itest, itest.Master(), "a1", add_u64_contract)
	strkeyRecipientContractID, err := strkey.Encode(strkey.VersionByteContract, recipientContractID[:])
	assert.NoError(t, err)

	mintAmount := xdr.Int128Parts{Lo: math.MaxUint64 - 3, Hi: math.MaxInt64}
	_, mintTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		mintWithAmt(
			itest,
			issuer, asset,
			i128Param(int64(mintAmount.Hi), uint64(mintAmount.Lo)),
			contractAddressParam(recipientContractID)),
	)
	assertContainsEffect(t, getTxEffects(itest, mintTx, asset),
		effects.EffectContractCredited)

	balanceAmount, _, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		contractBalance(itest, issuer, asset, recipientContractID),
	)
	assert.Equal(itest.CurrentTest(), xdr.ScValTypeScvI128, balanceAmount.Type)
	assert.Equal(itest.CurrentTest(), xdr.Uint64(math.MaxUint64-3), (*balanceAmount.I128).Lo)
	assert.Equal(itest.CurrentTest(), xdr.Int64(math.MaxInt64), (*balanceAmount.I128).Hi)
	assertEventPayments(itest, mintTx, asset, "", strkeyRecipientContractID, "mint", amount.String128(mintAmount))

	// calling transfer from the issuer account will also mint the asset
	_, transferTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		transferWithAmount(itest, issuer, asset, i128Param(0, 3), contractAddressParam(recipientContractID)),
	)

	assertContainsEffect(t, getTxEffects(itest, transferTx, asset),
		effects.EffectAccountDebited,
		effects.EffectContractCredited)

	balanceAmount, _, _ = assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		contractBalance(itest, issuer, asset, recipientContractID),
	)
	assert.Equal(itest.CurrentTest(), xdr.ScValTypeScvI128, balanceAmount.Type)
	assert.Equal(itest.CurrentTest(), xdr.Uint64(math.MaxUint64), (*balanceAmount.I128).Lo)
	assert.Equal(itest.CurrentTest(), xdr.Int64(math.MaxInt64), (*balanceAmount.I128).Hi)

	// 2^127 - 1
	balanceContracts := new(big.Int).Lsh(big.NewInt(1), 127)
	balanceContracts.Sub(balanceContracts, big.NewInt(1))
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             1,
		balanceContracts:         balanceContracts,
		contractID:               stellarAssetContractID(itest, asset),
	})
}

func TestExpirationAndRestoration(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		HorizonIngestParameters: map[string]string{
			// disable state verification because we will insert
			// a fake asset contract in the horizon db and we don't
			// want state verification to detect this
			"ingest-disable-state-verification": "true",
		},
		QuickExpiration: true,
	})

	issuer := itest.Master().Address()
	code := "USD"

	// Create contract to store synthetic asset balances
	storeContractID, _ := mustCreateAndInstallContract(
		itest,
		itest.Master(),
		"a1",
		"soroban_store.wasm",
	)
	syntheticAssetStat := history.ExpAssetStat{
		AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
		AssetCode:   code,
		AssetIssuer: issuer,
		Accounts: history.ExpAssetStatAccounts{
			Authorized:                      0,
			AuthorizedToMaintainLiabilities: 0,
			ClaimableBalances:               0,
			LiquidityPools:                  0,
			Unauthorized:                    0,
		},
		Balances: history.ExpAssetStatBalances{
			Authorized:                      "0",
			AuthorizedToMaintainLiabilities: "0",
			ClaimableBalances:               "0",
			LiquidityPools:                  "0",
			Unauthorized:                    "0",
		},
		ContractID: nil,
	}
	syntheticAssetStat.SetContractID(storeContractID)
	_, err := itest.HorizonIngest().HistoryQ().InsertAssetStat(
		context.Background(),
		syntheticAssetStat,
	)
	assert.NoError(t, err)

	// create active balance
	_, _, setOp := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		invokeStoreSet(
			itest,
			storeContractID,
			processors.BalanceToContractData(
				storeContractID,
				[32]byte{1},
				23,
			),
		),
	)
	sourceAccount, extendTTLOp, minFee := itest.PreflightExtendExpiration(
		itest.Master().Address(),
		setOp.Ext.SorobanData.Resources.Footprint.ReadWrite,
		LongTermTTL,
	)
	itest.MustSubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &extendTTLOp)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             1,
		balanceContracts:         big.NewInt(23),
		contractID:               storeContractID,
	})

	// create balance which we will expire
	balanceToExpire := processors.BalanceToContractData(
		storeContractID,
		[32]byte{2},
		37,
	)
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		invokeStoreSet(
			itest,
			storeContractID,
			balanceToExpire,
		),
	)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             2,
		balanceContracts:         big.NewInt(60),
		contractID:               storeContractID,
	})

	balanceToExpireLedgerKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract:   balanceToExpire.ContractData.Contract,
			Key:        balanceToExpire.ContractData.Key,
			Durability: balanceToExpire.ContractData.Durability,
		},
	}
	// The TESTING_MINIMUM_PERSISTENT_ENTRY_LIFETIME=10 configuration in stellar-core
	// will ensure that the ledger entry expires after 10 ledgers.
	// Because ARTIFICIALLY_ACCELERATE_TIME_FOR_TESTING is set to true, 10 ledgers
	// should elapse in 10 seconds
	itest.WaitUntilLedgerEntryTTL(balanceToExpireLedgerKey)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(37),
		numArchivedContracts:     1,
		numContracts:             1,
		balanceContracts:         big.NewInt(23),
		contractID:               storeContractID,
	})

	// increase active balance from 23 to 50
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		invokeStoreSet(
			itest,
			storeContractID,
			processors.BalanceToContractData(
				storeContractID,
				[32]byte{1},
				50,
			),
		),
	)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(37),
		numArchivedContracts:     1,
		numContracts:             1,
		balanceContracts:         big.NewInt(50),
		contractID:               storeContractID,
	})

	// restore expired balance
	sourceAccount, restoreFootprint, minFee := itest.RestoreFootprint(
		itest.Master().Address(),
		balanceToExpireLedgerKey,
	)
	itest.MustSubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &restoreFootprint)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             2,
		balanceContracts:         big.NewInt(87),
		contractID:               storeContractID,
	})

	// expire the balance again
	itest.WaitUntilLedgerEntryTTL(balanceToExpireLedgerKey)

	// decrease active balance from 50 to 3
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		invokeStoreSet(
			itest,
			storeContractID,
			processors.BalanceToContractData(
				storeContractID,
				[32]byte{1},
				3,
			),
		),
	)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(37),
		numArchivedContracts:     1,
		numContracts:             1,
		balanceContracts:         big.NewInt(3),
		contractID:               storeContractID,
	})

	// remove active balance
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		invokeStoreRemove(
			itest,
			storeContractID,
			processors.ContractBalanceLedgerKey(
				storeContractID,
				[32]byte{1},
			),
		),
	)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(37),
		numArchivedContracts:     1,
		numContracts:             0,
		balanceContracts:         big.NewInt(0),
		contractID:               storeContractID,
	})
}

func invokeStoreSet(
	itest *integration.Test,
	storeContractID xdr.Hash,
	ledgerEntryData xdr.LedgerEntryData,
) *txnbuild.InvokeHostFunction {
	key := ledgerEntryData.MustContractData().Key
	val := ledgerEntryData.MustContractData().Val
	return &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(storeContractID),
				FunctionName:    "set",
				Args: xdr.ScVec{
					key,
					val,
				},
			},
		},
		SourceAccount: itest.Master().Address(),
	}
}

func invokeStoreRemove(
	itest *integration.Test,
	storeContractID xdr.Hash,
	ledgerKey xdr.LedgerKey,
) *txnbuild.InvokeHostFunction {
	return &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(storeContractID),
				FunctionName:    "remove",
				Args: xdr.ScVec{
					ledgerKey.MustContractData().Key,
				},
			},
		},
		SourceAccount: itest.Master().Address(),
	}
}

func TestContractTransferBetweenAccounts(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		QuickExpiration:  true,
	})

	issuer := itest.Master().Address()
	code := "USD"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	recipientKp, recipient := itest.CreateAccount("100")
	itest.MustEstablishTrustline(recipientKp, recipient, txnbuild.MustAssetFromXDR(asset))

	itest.MustSubmitOperations(
		itest.MasterAccount(),
		itest.Master(),
		&txnbuild.Payment{
			SourceAccount: issuer,
			Destination:   recipient.GetAccountID(),
			Asset: txnbuild.CreditAsset{
				Code:   code,
				Issuer: issuer,
			},
			Amount: "1000",
		},
	)

	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("1000"))
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          amount.MustParse("1000"),
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             0,
		balanceContracts:         big.NewInt(0),
		contractID:               stellarAssetContractID(itest, asset),
	})

	otherRecipientKp, otherRecipient := itest.CreateAccount("100")
	itest.MustEstablishTrustline(otherRecipientKp, otherRecipient, txnbuild.MustAssetFromXDR(asset))

	_, transferTx, _ := assertInvokeHostFnSucceeds(
		itest,
		recipientKp,
		transfer(itest, recipientKp.Address(), asset, "30", accountAddressParam(otherRecipient.GetAccountID())),
	)
	assertAccountInvokeHostFunctionOperation(itest, recipientKp.Address(), recipientKp.Address(), otherRecipientKp.Address(), "30.0000000")
	assertAccountInvokeHostFunctionOperation(itest, otherRecipientKp.Address(), recipientKp.Address(), otherRecipientKp.Address(), "30.0000000")
	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("970"))
	assertContainsBalance(itest, otherRecipientKp, issuer, code, amount.MustParse("30"))

	fx := getTxEffects(itest, transferTx, asset)
	assert.NotEmpty(t, fx)
	assertContainsEffect(t, fx, effects.EffectAccountCredited, effects.EffectAccountDebited)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              2,
		balanceAccounts:          amount.MustParse("1000"),
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             0,
		balanceContracts:         big.NewInt(0),
		contractID:               stellarAssetContractID(itest, asset),
	})
	assertEventPayments(itest, transferTx, asset, recipientKp.Address(), otherRecipient.GetAccountID(), "transfer", "30.0000000")
}

func TestContractTransferBetweenAccountAndContract(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		QuickExpiration:  true,
	})

	issuer := itest.Master().Address()
	code := "USDLONG"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	recipientKp, recipient := itest.CreateAccount("100")
	itest.MustEstablishTrustline(recipientKp, recipient, txnbuild.MustAssetFromXDR(asset))

	itest.MustSubmitOperations(
		itest.MasterAccount(),
		itest.Master(),
		&txnbuild.Payment{
			SourceAccount: issuer,
			Destination:   recipient.GetAccountID(),
			Asset: txnbuild.CreditAsset{
				Code:   code,
				Issuer: issuer,
			},
			Amount: "1000",
		},
	)

	// Create recipient contract
	recipientContractID, recipientContractHash := mustCreateAndInstallContract(itest, itest.Master(), "a1", sac_contract)
	strkeyRecipientContractID, err := strkey.Encode(strkey.VersionByteContract, recipientContractID[:])
	assert.NoError(t, err)

	// init recipient contract with the asset contract id
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		initAssetContract(itest, issuer, asset, recipientContractID, recipientContractHash),
	)

	// Add funds to recipient contract
	_, mintTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		mint(itest, issuer, asset, "1000", contractAddressParam(recipientContractID)),
	)
	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("1000"))
	assertContainsEffect(t, getTxEffects(itest, mintTx, asset),
		effects.EffectContractCredited)

	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          amount.MustParse("1000"),
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             1,
		balanceContracts:         big.NewInt(int64(amount.MustParse("1000"))),
		contractID:               stellarAssetContractID(itest, asset),
	})

	// transfer from account to contract
	_, transferTx, _ := assertInvokeHostFnSucceeds(
		itest,
		recipientKp,
		transfer(itest, recipientKp.Address(), asset, "30", contractAddressParam(recipientContractID)),
	)
	assertAccountInvokeHostFunctionOperation(itest, recipientKp.Address(), recipientKp.Address(), strkeyRecipientContractID, "30.0000000")
	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("970"))
	assertContainsEffect(t, getTxEffects(itest, transferTx, asset),
		effects.EffectAccountDebited, effects.EffectContractCredited)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          amount.MustParse("970"),
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             1,
		balanceContracts:         big.NewInt(int64(amount.MustParse("1030"))),
		contractID:               stellarAssetContractID(itest, asset),
	})
	assertEventPayments(itest, transferTx, asset, recipientKp.Address(), strkeyRecipientContractID, "transfer", "30.0000000")

	// transfer from contract to account
	_, transferTx, _ = assertInvokeHostFnSucceeds(
		itest,
		recipientKp,
		transferFromContract(itest, recipientKp.Address(), asset, recipientContractID, recipientContractHash, "500", accountAddressParam(recipient.GetAccountID())),
	)
	assertAccountInvokeHostFunctionOperation(itest, recipientKp.Address(), strkeyRecipientContractID, recipientKp.Address(), "500.0000000")
	assertContainsEffect(t, getTxEffects(itest, transferTx, asset),
		effects.EffectContractDebited, effects.EffectAccountCredited)
	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("1470"))
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          amount.MustParse("1470"),
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             1,
		balanceContracts:         big.NewInt(int64(amount.MustParse("530"))),
		contractID:               stellarAssetContractID(itest, asset),
	})
	assertEventPayments(itest, transferTx, asset, strkeyRecipientContractID, recipientKp.Address(), "transfer", "500.0000000")

	balanceAmount, _, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		contractBalance(itest, issuer, asset, recipientContractID),
	)
	assert.Equal(itest.CurrentTest(), xdr.ScValTypeScvI128, balanceAmount.Type)
	assert.Equal(itest.CurrentTest(), xdr.Uint64(5300000000), (*balanceAmount.I128).Lo)
	assert.Equal(itest.CurrentTest(), xdr.Int64(0), (*balanceAmount.I128).Hi)
}

func TestContractTransferBetweenContracts(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		QuickExpiration:  true,
	})

	issuer := itest.Master().Address()
	code := "USD"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	// Create recipient contract
	recipientContractID, _ := mustCreateAndInstallContract(itest, itest.Master(), "a1", sac_contract)
	strkeyRecipientContractID, err := strkey.Encode(strkey.VersionByteContract, recipientContractID[:])
	assert.NoError(t, err)

	// Create emitter contract
	emitterContractID, emitterContractHash := mustCreateAndInstallContract(itest, itest.Master(), "a2", sac_contract)
	strkeyEmitterContractID, err := strkey.Encode(strkey.VersionByteContract, emitterContractID[:])
	assert.NoError(t, err)

	// init emitter contract with the asset contract id
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		initAssetContract(itest, issuer, asset, emitterContractID, emitterContractHash),
	)

	// Add funds to emitter contract
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		mint(itest, issuer, asset, "1000", contractAddressParam(emitterContractID)),
	)

	// Transfer funds from emitter to recipient
	_, transferTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		transferFromContract(itest, issuer, asset, emitterContractID, emitterContractHash, "10", contractAddressParam(recipientContractID)),
	)
	assertContainsEffect(t, getTxEffects(itest, transferTx, asset),
		effects.EffectContractCredited, effects.EffectContractDebited)

	// Check balances of emitter and recipient
	emitterBalanceAmount, _, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		contractBalance(itest, issuer, asset, emitterContractID),
	)
	assert.Equal(itest.CurrentTest(), xdr.ScValTypeScvI128, emitterBalanceAmount.Type)
	assert.Equal(itest.CurrentTest(), xdr.Uint64(9900000000), (*emitterBalanceAmount.I128).Lo)
	assert.Equal(itest.CurrentTest(), xdr.Int64(0), (*emitterBalanceAmount.I128).Hi)

	recipientBalanceAmount, _, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		contractBalance(itest, issuer, asset, recipientContractID),
	)
	assert.Equal(itest.CurrentTest(), xdr.ScValTypeScvI128, recipientBalanceAmount.Type)
	assert.Equal(itest.CurrentTest(), xdr.Uint64(100000000), (*recipientBalanceAmount.I128).Lo)
	assert.Equal(itest.CurrentTest(), xdr.Int64(0), (*recipientBalanceAmount.I128).Hi)

	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             2,
		balanceContracts:         big.NewInt(int64(amount.MustParse("1000"))),
		contractID:               stellarAssetContractID(itest, asset),
	})
	assertEventPayments(itest, transferTx, asset, strkeyEmitterContractID, strkeyRecipientContractID, "transfer", "10.0000000")
}

func TestContractBurnFromAccount(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		QuickExpiration:  true,
	})

	issuer := itest.Master().Address()
	code := "USD"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	recipientKp, recipient := itest.CreateAccount("100")
	itest.MustEstablishTrustline(recipientKp, recipient, txnbuild.MustAssetFromXDR(asset))

	itest.MustSubmitOperations(
		itest.MasterAccount(),
		itest.Master(),
		&txnbuild.Payment{
			SourceAccount: issuer,
			Destination:   recipient.GetAccountID(),
			Asset: txnbuild.CreditAsset{
				Code:   code,
				Issuer: issuer,
			},
			Amount: "1000",
		},
	)

	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("1000"))
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          amount.MustParse("1000"),
		numContracts:             0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		balanceContracts:         big.NewInt(0),
		contractID:               stellarAssetContractID(itest, asset),
	})

	_, burnTx, _ := assertInvokeHostFnSucceeds(
		itest,
		recipientKp,
		burn(itest, recipientKp.Address(), asset, "500"),
	)
	assertAccountInvokeHostFunctionOperation(itest, recipientKp.Address(), recipientKp.Address(), "", "500.0000000")

	fx := getTxEffects(itest, burnTx, asset)
	require.Len(t, fx, 1)
	assetEffects := assertContainsEffect(t, fx, effects.EffectAccountDebited)
	require.GreaterOrEqual(t, len(assetEffects), 1)
	burnEffect := assetEffects[0].(effects.AccountDebited)

	assert.Equal(t, issuer, burnEffect.Asset.Issuer)
	assert.Equal(t, code, burnEffect.Asset.Code)
	assert.Equal(t, "500.0000000", burnEffect.Amount)
	assert.Equal(t, recipientKp.Address(), burnEffect.Account)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          amount.MustParse("500"),
		numContracts:             0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		balanceContracts:         big.NewInt(0),
		contractID:               stellarAssetContractID(itest, asset),
	})
	assertEventPayments(itest, burnTx, asset, recipientKp.Address(), "", "burn", "500.0000000")
}

func TestContractBurnFromContract(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		QuickExpiration:  true,
	})

	issuer := itest.Master().Address()
	code := "USD"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	// Create recipient contract
	recipientContractID, recipientContractHash := mustCreateAndInstallContract(itest, itest.Master(), "a1", sac_contract)
	strkeyRecipientContractID, err := strkey.Encode(strkey.VersionByteContract, recipientContractID[:])
	assert.NoError(t, err)
	// init contract with asset contract id
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		initAssetContract(itest, issuer, asset, recipientContractID, recipientContractHash),
	)

	// Add funds to recipient contract
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		mint(itest, issuer, asset, "1000", contractAddressParam(recipientContractID)),
	)

	// Burn funds
	_, burnTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		burnSelf(itest, issuer, asset, recipientContractID, recipientContractHash, "10"),
	)

	balanceAmount, _, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		contractBalance(itest, issuer, asset, recipientContractID),
	)

	assert.Equal(itest.CurrentTest(), xdr.ScValTypeScvI128, balanceAmount.Type)
	assert.Equal(itest.CurrentTest(), xdr.Uint64(9900000000), (*balanceAmount.I128).Lo)
	assert.Equal(itest.CurrentTest(), xdr.Int64(0), (*balanceAmount.I128).Hi)

	assertContainsEffect(t, getTxEffects(itest, burnTx, asset),
		effects.EffectContractDebited)

	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             1,
		balanceContracts:         big.NewInt(int64(amount.MustParse("990"))),
		contractID:               stellarAssetContractID(itest, asset),
	})
	assertEventPayments(itest, burnTx, asset, strkeyRecipientContractID, "", "burn", "10.0000000")
}

func TestContractClawbackFromAccount(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		QuickExpiration:  true,
	})

	// Give the master account the revocable flag (needed to set the clawback flag)
	// and the clawback flag
	setRevocableFlag := txnbuild.SetOptions{
		SetFlags: []txnbuild.AccountFlag{
			txnbuild.AuthRevocable,
			txnbuild.AuthClawbackEnabled,
		},
	}
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &setRevocableFlag)

	issuer := itest.Master().Address()
	code := "USD"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	recipientKp, recipient := itest.CreateAccount("100")
	itest.MustEstablishTrustline(recipientKp, recipient, txnbuild.MustAssetFromXDR(asset))

	itest.MustSubmitOperations(
		itest.MasterAccount(),
		itest.Master(),
		&txnbuild.Payment{
			SourceAccount: issuer,
			Destination:   recipient.GetAccountID(),
			Asset: txnbuild.CreditAsset{
				Code:   code,
				Issuer: issuer,
			},
			Amount: "1000",
		},
	)

	assertContainsBalance(itest, recipientKp, issuer, code, amount.MustParse("1000"))
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          amount.MustParse("1000"),
		numContracts:             0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		balanceContracts:         big.NewInt(0),
		contractID:               stellarAssetContractID(itest, asset),
	})

	_, clawTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		clawback(itest, issuer, asset, "1000", accountAddressParam(recipientKp.Address())),
	)
	assertAccountInvokeHostFunctionOperation(itest, recipientKp.Address(), recipientKp.Address(), "", "1000.0000000")

	assertContainsEffect(t, getTxEffects(itest, clawTx, asset), effects.EffectAccountDebited)
	assertContainsBalance(itest, recipientKp, issuer, code, 0)
	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              1,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             0,
		balanceContracts:         big.NewInt(0),
		contractID:               stellarAssetContractID(itest, asset),
	})
	assertEventPayments(itest, clawTx, asset, recipientKp.Address(), "", "clawback", "1000.0000000")
}

func TestContractClawbackFromContract(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		QuickExpiration:  true,
	})

	// Give the master account the revocable flag (needed to set the clawback flag)
	// and the clawback flag
	setRevocableFlag := txnbuild.SetOptions{
		SetFlags: []txnbuild.AccountFlag{
			txnbuild.AuthRevocable,
			txnbuild.AuthClawbackEnabled,
		},
	}
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &setRevocableFlag)

	issuer := itest.Master().Address()
	code := "USD"
	asset := xdr.MustNewCreditAsset(code, issuer)

	createSAC(itest, asset)

	// Create recipient contract
	recipientContractID, _ := mustCreateAndInstallContract(itest, itest.Master(), "a2", sac_contract)
	strkeyRecipientContractID, err := strkey.Encode(strkey.VersionByteContract, recipientContractID[:])
	assert.NoError(itest.CurrentTest(), err)

	// Add funds to recipient contract
	assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		mint(itest, issuer, asset, "1000", contractAddressParam(recipientContractID)),
	)

	// Clawback funds
	_, clawTx, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		clawback(itest, issuer, asset, "10", contractAddressParam(recipientContractID)),
	)

	balanceAmount, _, _ := assertInvokeHostFnSucceeds(
		itest,
		itest.Master(),
		contractBalance(itest, issuer, asset, recipientContractID),
	)
	assert.Equal(itest.CurrentTest(), xdr.ScValTypeScvI128, balanceAmount.Type)
	assert.Equal(itest.CurrentTest(), xdr.Uint64(9900000000), (*balanceAmount.I128).Lo)
	assert.Equal(itest.CurrentTest(), xdr.Int64(0), (*balanceAmount.I128).Hi)

	assertContainsEffect(t, getTxEffects(itest, clawTx, asset),
		effects.EffectContractDebited)

	assertAssetStats(itest, assetStats{
		code:                     code,
		issuer:                   issuer,
		numAccounts:              0,
		balanceAccounts:          0,
		balanceArchivedContracts: big.NewInt(0),
		numArchivedContracts:     0,
		numContracts:             1,
		balanceContracts:         big.NewInt(int64(amount.MustParse("990"))),
		contractID:               stellarAssetContractID(itest, asset),
	})
	assertEventPayments(itest, clawTx, asset, strkeyRecipientContractID, "", "clawback", "10.0000000")
}

func assertContainsBalance(itest *integration.Test, acct *keypair.Full, issuer, code string, amt xdr.Int64) {
	accountResponse := itest.MustGetAccount(acct)
	if issuer == "" && code == "" {
		xlmBalance, err := accountResponse.GetNativeBalance()
		assert.NoError(itest.CurrentTest(), err)
		assert.Equal(itest.CurrentTest(), amt, amount.MustParse(xlmBalance))
	} else {
		assetBalance := accountResponse.GetCreditBalance(code, issuer)
		assert.Equal(itest.CurrentTest(), amt, amount.MustParse(assetBalance))
	}
}

type assetStats struct {
	code                     string
	issuer                   string
	numAccounts              int32
	balanceAccounts          xdr.Int64
	numContracts             int32
	numArchivedContracts     int32
	balanceContracts         *big.Int
	balanceArchivedContracts *big.Int
	contractID               [32]byte
}

func assertAssetStats(itest *integration.Test, expected assetStats) {
	assets, err := itest.Client().Assets(horizonclient.AssetRequest{
		ForAssetCode:   expected.code,
		ForAssetIssuer: expected.issuer,
		Limit:          1,
	})
	assert.NoError(itest.CurrentTest(), err)

	if expected.numContracts == 0 && expected.numAccounts == 0 && expected.numArchivedContracts == 0 &&
		expected.balanceArchivedContracts.Cmp(big.NewInt(0)) == 0 &&
		expected.balanceContracts.Cmp(big.NewInt(0)) == 0 && expected.balanceAccounts == 0 {
		assert.Empty(itest.CurrentTest(), assets)
		return
	}

	assert.Len(itest.CurrentTest(), assets.Embedded.Records, 1)
	asset := assets.Embedded.Records[0]
	assert.Equal(itest.CurrentTest(), expected.code, asset.Code)
	assert.Equal(itest.CurrentTest(), expected.issuer, asset.Issuer)
	assert.Equal(itest.CurrentTest(), expected.numAccounts, asset.Accounts.Authorized)
	assert.Equal(itest.CurrentTest(), expected.numContracts, asset.NumContracts)
	assert.Equal(itest.CurrentTest(), expected.numArchivedContracts, asset.NumArchivedContracts)
	assert.Equal(itest.CurrentTest(), expected.balanceContracts.String(), parseBalance(itest, asset.ContractsAmount).String())
	assert.Equal(itest.CurrentTest(), expected.balanceArchivedContracts.String(), parseBalance(itest, asset.ArchivedContractsAmount).String())
	assert.Equal(itest.CurrentTest(), strkey.MustEncode(strkey.VersionByteContract, expected.contractID[:]), asset.ContractID)
}

func parseBalance(itest *integration.Test, balance string) *big.Int {
	parts := strings.Split(balance, ".")
	assert.Len(itest.CurrentTest(), parts, 2)
	contractsAmount, ok := new(big.Int).SetString(parts[0]+parts[1], 10)
	assert.True(itest.CurrentTest(), ok)
	return contractsAmount
}

// assertContainsEffect checks that the list of json effects contains the given
// effect type(s) by name (no other details are checked). It returns the last
// effect matching each given type.
func assertContainsEffect(t *testing.T, fx []effects.Effect, effectTypes ...effects.EffectType) []effects.Effect {
	found := map[string]int{}
	for idx, effect := range fx {
		found[effect.GetType()] = idx
	}

	for _, type_ := range effectTypes {
		assert.Containsf(t, found, effects.EffectTypeNames[type_], "effects: %v", fx)
	}

	var rv []effects.Effect
	for _, i := range found {
		rv = append(rv, fx[i])
	}

	return rv
}

// getTxEffects returns a transaction's effects, limited to 2 because it's to be
// used for checking SAC effects.
func getTxEffects(itest *integration.Test, txHash string, asset xdr.Asset) []effects.Effect {
	t := itest.CurrentTest()
	effects, err := itest.Client().Effects(horizonclient.EffectRequest{
		ForTransaction: txHash,
		Order:          horizonclient.OrderDesc,
	})
	assert.NoError(t, err)
	result := effects.Embedded.Records

	assert.LessOrEqualf(t, len(result), 2, "txhash: %s", txHash)
	return result
}

func assertAccountInvokeHostFunctionOperation(itest *integration.Test, account string, from string, to string, amount string) {
	ops, err := itest.Client().Operations(horizonclient.OperationRequest{
		ForAccount: account,
		Limit:      1,
		Order:      "desc",
	})

	assert.NoError(itest.CurrentTest(), err)
	result := ops.Embedded.Records[0]
	assert.Equal(itest.CurrentTest(), result.GetType(), operations.TypeNames[xdr.OperationTypeInvokeHostFunction])
	invokeHostFn := result.(operations.InvokeHostFunction)
	assert.Equal(itest.CurrentTest(), invokeHostFn.Function, "HostFunctionTypeHostFunctionTypeInvokeContract")
	assert.Equal(itest.CurrentTest(), to, invokeHostFn.AssetBalanceChanges[0].To)
	assert.Equal(itest.CurrentTest(), from, invokeHostFn.AssetBalanceChanges[0].From)
	assert.Equal(itest.CurrentTest(), amount, invokeHostFn.AssetBalanceChanges[0].Amount)
}

func assertEventPayments(itest *integration.Test, txHash string, asset xdr.Asset, from string, to string, evtType string, amount string) {
	ops, err := itest.Client().Operations(horizonclient.OperationRequest{
		ForTransaction: txHash,
		Limit:          1,
	})
	assert.NoError(itest.CurrentTest(), err)
	assert.Equal(itest.CurrentTest(), 1, len(ops.Embedded.Records))
	assert.Equal(itest.CurrentTest(), ops.Embedded.Records[0].GetType(), operations.TypeNames[xdr.OperationTypeInvokeHostFunction])

	invokeHostFn := ops.Embedded.Records[0].(operations.InvokeHostFunction)
	assert.Equal(itest.CurrentTest(), invokeHostFn.Function, "HostFunctionTypeHostFunctionTypeInvokeContract")
	require.Equal(itest.CurrentTest(), 1, len(invokeHostFn.AssetBalanceChanges))
	assetBalanceChange := invokeHostFn.AssetBalanceChanges[0]
	assert.Equal(itest.CurrentTest(), assetBalanceChange.Amount, amount)
	assert.Equal(itest.CurrentTest(), assetBalanceChange.From, from)
	assert.Equal(itest.CurrentTest(), assetBalanceChange.To, to)
	assert.Equal(itest.CurrentTest(), assetBalanceChange.Type, evtType)
	assert.Equal(itest.CurrentTest(), assetBalanceChange.Asset.Code, strings.TrimRight(asset.GetCode(), "\x00"))
	assert.Equal(itest.CurrentTest(), assetBalanceChange.Asset.Issuer, asset.GetIssuer())
}

func contractIDParam(contractID xdr.Hash) xdr.ScAddress {
	return xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}
}

func accountAddressParam(accountID string) xdr.ScVal {
	address := xdr.ScAddress{
		Type:      xdr.ScAddressTypeScAddressTypeAccount,
		AccountId: xdr.MustAddressPtr(accountID),
	}
	return xdr.ScVal{
		Type:    xdr.ScValTypeScvAddress,
		Address: &address,
	}
}

func contractAddressParam(contractID xdr.Hash) xdr.ScVal {
	address := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}
	return xdr.ScVal{
		Type:    xdr.ScValTypeScvAddress,
		Address: &address,
	}
}

func i128Param(hi int64, lo uint64) xdr.ScVal {
	i128 := &xdr.Int128Parts{
		Hi: xdr.Int64(hi),
		Lo: xdr.Uint64(lo),
	}
	return xdr.ScVal{
		Type: xdr.ScValTypeScvI128,
		I128: i128,
	}
}

func mint(itest *integration.Test, sourceAccount string, asset xdr.Asset, assetAmount string, recipient xdr.ScVal) *txnbuild.InvokeHostFunction {
	return mintWithAmt(itest, sourceAccount, asset, i128Param(0, uint64(amount.MustParse(assetAmount))), recipient)
}

func mintWithAmt(itest *integration.Test, sourceAccount string, asset xdr.Asset, assetAmount xdr.ScVal, recipient xdr.ScVal) *txnbuild.InvokeHostFunction {
	invokeHostFn := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(stellarAssetContractID(itest, asset)),
				FunctionName:    "mint",
				Args: xdr.ScVec{
					recipient,
					assetAmount,
				},
			},
		},
		SourceAccount: sourceAccount,
	}

	return invokeHostFn
}

func initAssetContract(itest *integration.Test, sourceAccount string, asset xdr.Asset, sacTestcontractID, sacTestcontractHash xdr.Hash) *txnbuild.InvokeHostFunction {
	targetContract := contractIDParam(stellarAssetContractID(itest, asset))
	invokeHostFn := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(sacTestcontractID),
				FunctionName:    "init",
				Args: xdr.ScVec{
					{
						Type:    xdr.ScValTypeScvAddress,
						Address: &targetContract,
					},
				},
			},
		},
		SourceAccount: sourceAccount,
	}

	return invokeHostFn
}

func clawback(itest *integration.Test, sourceAccount string, asset xdr.Asset, assetAmount string, recipient xdr.ScVal) *txnbuild.InvokeHostFunction {
	invokeHostFn := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(stellarAssetContractID(itest, asset)),
				FunctionName:    "clawback",
				Args: xdr.ScVec{
					recipient,
					i128Param(0, uint64(amount.MustParse(assetAmount))),
				},
			},
		},
		SourceAccount: sourceAccount,
	}

	return invokeHostFn
}

func contractBalance(itest *integration.Test, sourceAccount string, asset xdr.Asset, sacTestcontractID xdr.Hash) *txnbuild.InvokeHostFunction {
	invokeHostFn := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(stellarAssetContractID(itest, asset)),
				FunctionName:    "balance",
				Args:            xdr.ScVec{contractAddressParam(sacTestcontractID)},
			},
		},
		SourceAccount: sourceAccount,
	}

	return invokeHostFn
}

func transfer(itest *integration.Test, sourceAccount string, asset xdr.Asset, assetAmount string, recipient xdr.ScVal) *txnbuild.InvokeHostFunction {
	return transferWithAmount(itest, sourceAccount, asset, i128Param(0, uint64(amount.MustParse(assetAmount))), recipient)
}

func transferWithAmount(itest *integration.Test, sourceAccount string, asset xdr.Asset, assetAmount xdr.ScVal, recipient xdr.ScVal) *txnbuild.InvokeHostFunction {
	invokeHostFn := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(stellarAssetContractID(itest, asset)),
				FunctionName:    "transfer",
				Args: xdr.ScVec{
					accountAddressParam(sourceAccount),
					recipient,
					assetAmount,
				},
			},
		},
		SourceAccount: sourceAccount,
	}

	return invokeHostFn
}

func transferFromContract(itest *integration.Test, sourceAccount string, asset xdr.Asset, sacTestcontractID xdr.Hash, sacTestContractHash xdr.Hash, assetAmount string, recipient xdr.ScVal) *txnbuild.InvokeHostFunction {
	invokeHostFn := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(sacTestcontractID),
				FunctionName:    "transfer",
				Args: xdr.ScVec{
					recipient,
					i128Param(0, uint64(amount.MustParse(assetAmount))),
				},
			},
		},
		SourceAccount: sourceAccount,
	}

	return invokeHostFn
}

// Invokes burn_self from the sac_test contract (which just burns assets from itself)
func burnSelf(itest *integration.Test, sourceAccount string, asset xdr.Asset, sacTestcontractID xdr.Hash, sacTestContractHash xdr.Hash, assetAmount string) *txnbuild.InvokeHostFunction {
	invokeHostFn := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(sacTestcontractID),
				FunctionName:    "burn_self",
				Args: xdr.ScVec{
					i128Param(0, uint64(amount.MustParse(assetAmount))),
				},
			},
		},
		SourceAccount: sourceAccount,
	}

	return invokeHostFn
}

func burn(itest *integration.Test, sourceAccount string, asset xdr.Asset, assetAmount string) *txnbuild.InvokeHostFunction {
	invokeHostFn := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(stellarAssetContractID(itest, asset)),
				FunctionName:    "burn",
				Args: xdr.ScVec{
					accountAddressParam(sourceAccount),
					i128Param(0, uint64(amount.MustParse(assetAmount))),
				},
			},
		},
		SourceAccount: sourceAccount,
	}

	return invokeHostFn
}

func assertInvokeHostFnSucceeds(itest *integration.Test, signer *keypair.Full, op *txnbuild.InvokeHostFunction) (*xdr.ScVal, string, *txnbuild.InvokeHostFunction) {
	acc := itest.MustGetAccount(signer)
	preFlightOp, minFee := itest.PreflightHostFunctions(&acc, *op)
	clientTx, err := itest.SubmitOperationsWithFee(&acc, signer, minFee+txnbuild.MinBaseFee, &preFlightOp)
	require.NoError(itest.CurrentTest(), err)

	var txResult xdr.TransactionResult
	err = xdr.SafeUnmarshalBase64(clientTx.ResultXdr, &txResult)
	require.NoError(itest.CurrentTest(), err)

	var txMetaResult xdr.TransactionMeta
	err = xdr.SafeUnmarshalBase64(clientTx.ResultMetaXdr, &txMetaResult)
	require.NoError(itest.CurrentTest(), err)

	opResults, ok := txResult.OperationResults()
	assert.True(itest.CurrentTest(), ok)
	assert.Equal(itest.CurrentTest(), len(opResults), 1)
	invokeHostFunctionResult, ok := opResults[0].MustTr().GetInvokeHostFunctionResult()
	assert.True(itest.CurrentTest(), ok)
	assert.Equal(itest.CurrentTest(), invokeHostFunctionResult.Code, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess)

	returnValue := txMetaResult.MustV3().SorobanMeta.ReturnValue

	return &returnValue, clientTx.Hash, &preFlightOp
}

func stellarAssetContractID(itest *integration.Test, asset xdr.Asset) xdr.Hash {
	contractID, err := asset.ContractID(itest.GetPassPhrase())
	require.NoError(itest.CurrentTest(), err)
	return contractID
}

func mustCreateAndInstallContract(itest *integration.Test, signer *keypair.Full, contractSalt string, wasmFileName string) (xdr.Hash, xdr.Hash) {
	_, _, installContractOp := assertInvokeHostFnSucceeds(
		itest,
		signer,
		assembleInstallContractCodeOp(
			itest.CurrentTest(),
			itest.Master().Address(),
			wasmFileName,
		),
	)
	_, _, createContractOp := assertInvokeHostFnSucceeds(
		itest,
		signer,
		assembleCreateContractOp(itest.CurrentTest(), itest.Master().Address(), wasmFileName, contractSalt),
	)

	keys := append(
		installContractOp.Ext.SorobanData.Resources.Footprint.ReadWrite,
		createContractOp.Ext.SorobanData.Resources.Footprint.ReadWrite...,
	)

	sourceAccount, extendTTLOp, minFee := itest.PreflightExtendExpiration(
		itest.Master().Address(),
		keys,
		LongTermTTL,
	)
	itest.MustSubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &extendTTLOp)

	contractHash := createContractOp.Ext.SorobanData.Resources.Footprint.ReadOnly[0].MustContractCode().Hash
	contractID := createContractOp.Ext.SorobanData.Resources.Footprint.ReadWrite[0].MustContractData().Contract.ContractId
	return *contractID, contractHash
}

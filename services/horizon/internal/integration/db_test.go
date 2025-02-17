package integration

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	hProtocol "github.com/stellar/go/protocols/horizon"
	horizoncmd "github.com/stellar/go/services/horizon/cmd"
	horizon "github.com/stellar/go/services/horizon/internal"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/db2/schema"
	"github.com/stellar/go/services/horizon/internal/ingest/filters"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/support/collections/set"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/db/dbtest"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

func submitLiquidityPoolOps(itest *integration.Test, tt *assert.Assertions) (submittedOperations []txnbuild.Operation, lastLedger int32) {
	master := itest.Master()
	keys, accounts := itest.CreateAccounts(2, "1000")
	shareKeys, shareAccount := keys[0], accounts[0]
	tradeKeys, tradeAccount := keys[1], accounts[1]

	allOps := []txnbuild.Operation{
		&txnbuild.ChangeTrust{
			Line: txnbuild.ChangeTrustAssetWrapper{
				Asset: txnbuild.CreditAsset{
					Code:   "USD",
					Issuer: master.Address(),
				},
			},
			Limit: txnbuild.MaxTrustlineLimit,
		},
		&txnbuild.ChangeTrust{
			Line: txnbuild.LiquidityPoolShareChangeTrustAsset{
				LiquidityPoolParameters: txnbuild.LiquidityPoolParameters{
					AssetA: txnbuild.NativeAsset{},
					AssetB: txnbuild.CreditAsset{
						Code:   "USD",
						Issuer: master.Address(),
					},
					Fee: 30,
				},
			},
			Limit: txnbuild.MaxTrustlineLimit,
		},
		&txnbuild.Payment{
			SourceAccount: master.Address(),
			Destination:   shareAccount.GetAccountID(),
			Asset: txnbuild.CreditAsset{
				Code:   "USD",
				Issuer: master.Address(),
			},
			Amount: "1000",
		},
	}
	itest.MustSubmitMultiSigOperations(shareAccount, []*keypair.Full{shareKeys, master}, allOps...)

	poolID, err := xdr.NewPoolId(
		xdr.MustNewNativeAsset(),
		xdr.MustNewCreditAsset("USD", master.Address()),
		30,
	)
	tt.NoError(err)
	poolIDHexString := xdr.Hash(poolID).HexString()

	var op txnbuild.Operation = &txnbuild.LiquidityPoolDeposit{
		LiquidityPoolID: [32]byte(poolID),
		MaxAmountA:      "400",
		MaxAmountB:      "777",
		MinPrice:        xdr.Price{N: 1, D: 2},
		MaxPrice:        xdr.Price{N: 2, D: 1},
	}
	allOps = append(allOps, op)
	itest.MustSubmitOperations(shareAccount, shareKeys, op)

	ops := []txnbuild.Operation{
		&txnbuild.ChangeTrust{
			Line: txnbuild.ChangeTrustAssetWrapper{
				Asset: txnbuild.CreditAsset{
					Code:   "USD",
					Issuer: master.Address(),
				},
			},
			Limit: txnbuild.MaxTrustlineLimit,
		},
		&txnbuild.PathPaymentStrictReceive{
			SendAsset: txnbuild.NativeAsset{},
			DestAsset: txnbuild.CreditAsset{
				Code:   "USD",
				Issuer: master.Address(),
			},
			SendMax:     "1000",
			DestAmount:  "2",
			Destination: tradeKeys.Address(),
		},
	}
	itest.MustSubmitOperations(tradeAccount, tradeKeys, ops...)

	pool, err := itest.Client().LiquidityPoolDetail(horizonclient.LiquidityPoolRequest{
		LiquidityPoolID: poolIDHexString,
	})
	tt.NoError(err)

	op = &txnbuild.LiquidityPoolWithdraw{
		LiquidityPoolID: [32]byte(poolID),
		Amount:          pool.TotalShares,
		MinAmountA:      "10",
		MinAmountB:      "20",
	}
	allOps = append(allOps, op)
	txResp := itest.MustSubmitOperations(shareAccount, shareKeys, op)

	return allOps, txResp.Ledger
}

func submitPaymentOps(itest *integration.Test, tt *assert.Assertions) (submittedOperations []txnbuild.Operation, lastLedger int32) {
	ops := []txnbuild.Operation{
		&txnbuild.Payment{
			Destination: itest.Master().Address(),
			Amount:      "10",
			Asset:       txnbuild.NativeAsset{},
		},
		&txnbuild.PathPaymentStrictSend{
			SendAsset:   txnbuild.NativeAsset{},
			SendAmount:  "10",
			Destination: itest.Master().Address(),
			DestAsset:   txnbuild.NativeAsset{},
			DestMin:     "10",
			Path:        []txnbuild.Asset{txnbuild.NativeAsset{}},
		},
		&txnbuild.PathPaymentStrictReceive{
			SendAsset:   txnbuild.NativeAsset{},
			SendMax:     "10",
			Destination: itest.Master().Address(),
			DestAsset:   txnbuild.NativeAsset{},
			DestAmount:  "10",
			Path:        []txnbuild.Asset{txnbuild.NativeAsset{}},
		},
		&txnbuild.PathPayment{
			SendAsset:   txnbuild.NativeAsset{},
			SendMax:     "10",
			Destination: itest.Master().Address(),
			DestAsset:   txnbuild.NativeAsset{},
			DestAmount:  "10",
			Path:        []txnbuild.Asset{txnbuild.NativeAsset{}},
		},
	}
	txResp := itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), ops...)

	return ops, txResp.Ledger
}

//lint:ignore U1000 Ignore unused function temporarily until fees/preflight are working in test
func submitSorobanOps(itest *integration.Test, tt *assert.Assertions) (submittedOperations []txnbuild.Operation, lastLedger int32) {
	installContractOp := assembleInstallContractCodeOp(itest.CurrentTest(), itest.Master().Address(), add_u64_contract)
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), installContractOp)

	extendFootprintTtlOp := &txnbuild.ExtendFootprintTtl{
		ExtendTo:      100,
		SourceAccount: itest.Master().Address(),
	}
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), extendFootprintTtlOp)

	restoreFootprintOp := &txnbuild.RestoreFootprint{
		SourceAccount: itest.Master().Address(),
	}
	txResp := itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), restoreFootprintOp)

	return []txnbuild.Operation{installContractOp, extendFootprintTtlOp, restoreFootprintOp}, txResp.Ledger
}

func submitSponsorshipOps(itest *integration.Test, tt *assert.Assertions) (submittedOperations []txnbuild.Operation, lastLedger int32) {
	keys, accounts := itest.CreateAccounts(1, "1000")
	sponsor, sponsorPair := accounts[0], keys[0]
	newAccountKeys := keypair.MustRandom()
	newAccountID := newAccountKeys.Address()

	ops := sponsorOperations(newAccountID,
		&txnbuild.CreateAccount{
			Destination: newAccountID,
			Amount:      "100",
		})

	signers := []*keypair.Full{sponsorPair, newAccountKeys}
	allOps := ops
	itest.MustSubmitMultiSigOperations(sponsor, signers, ops...)

	// Revoke sponsorship
	op := &txnbuild.RevokeSponsorship{
		SponsorshipType: txnbuild.RevokeSponsorshipTypeAccount,
		Account:         &newAccountID,
	}
	allOps = append(allOps, op)
	txResp := itest.MustSubmitOperations(sponsor, sponsorPair, op)

	return allOps, txResp.Ledger
}

func submitClawbackOps(itest *integration.Test, tt *assert.Assertions) (submittedOperations []txnbuild.Operation, lastLedger int32) {
	// Give the master account the revocable flag (needed to set the clawback flag)
	setRevocableFlag := txnbuild.SetOptions{
		SetFlags: []txnbuild.AccountFlag{
			txnbuild.AuthRevocable,
		},
	}
	allOps := []txnbuild.Operation{&setRevocableFlag}

	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &setRevocableFlag)

	// Give the master account the clawback flag
	setClawBackFlag := txnbuild.SetOptions{
		SetFlags: []txnbuild.AccountFlag{
			txnbuild.AuthClawbackEnabled,
		},
	}
	allOps = append(allOps, &setClawBackFlag)
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &setClawBackFlag)

	// Create another account from which to claw an asset back
	keyPairs, accounts := itest.CreateAccounts(1, "100")
	accountKeyPair := keyPairs[0]
	account := accounts[0]

	// Add some assets to the account with asset which allows clawback

	// Time machine to Spain before Euros were a thing
	pesetasAsset := txnbuild.CreditAsset{Code: "PTS", Issuer: itest.Master().Address()}
	itest.MustEstablishTrustline(accountKeyPair, account, pesetasAsset)
	pesetasPayment := txnbuild.Payment{
		Destination: accountKeyPair.Address(),
		Amount:      "10",
		Asset:       pesetasAsset,
	}
	allOps = append(allOps, &pesetasPayment)
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &pesetasPayment)

	clawback := txnbuild.Clawback{
		From:   account.GetAccountID(),
		Amount: "10",
		Asset:  pesetasAsset,
	}
	allOps = append(allOps, &clawback)
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &clawback)

	// Make a claimable balance from the master account (and asset issuer) to the account with an asset which allows clawback
	pesetasCreateCB := txnbuild.CreateClaimableBalance{
		Amount: "10",
		Asset:  pesetasAsset,
		Destinations: []txnbuild.Claimant{
			txnbuild.NewClaimant(accountKeyPair.Address(), nil),
		},
	}
	allOps = append(allOps, &pesetasCreateCB)
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &pesetasCreateCB)

	listCBResp, err := itest.Client().ClaimableBalances(horizonclient.ClaimableBalanceRequest{
		Claimant: accountKeyPair.Address(),
	})
	tt.NoError(err)
	cbID := listCBResp.Embedded.Records[0].BalanceID

	// Clawback the claimable balance
	pesetasClawbackCB := txnbuild.ClawbackClaimableBalance{
		BalanceID: cbID,
	}
	allOps = append(allOps, &pesetasClawbackCB)
	txResp := itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &pesetasClawbackCB)

	return allOps, txResp.Ledger
}

func submitClaimableBalanceOps(itest *integration.Test, tt *assert.Assertions) (submittedOperations []txnbuild.Operation, lastLedger int32) {

	// Create another account from which to claim an asset back
	keyPairs, accounts := itest.CreateAccounts(1, "100")
	accountKeyPair := keyPairs[0]
	account := accounts[0]

	// Make a claimable balance from the master account (and asset issuer) to the account with an asset which allows clawback
	createCB := txnbuild.CreateClaimableBalance{
		Amount: "10",
		Asset:  txnbuild.NativeAsset{},
		Destinations: []txnbuild.Claimant{
			txnbuild.NewClaimant(accountKeyPair.Address(), nil),
		},
	}
	allOps := []txnbuild.Operation{&createCB}
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), &createCB)

	listCBResp, err := itest.Client().ClaimableBalances(horizonclient.ClaimableBalanceRequest{
		Claimant: accountKeyPair.Address(),
	})
	tt.NoError(err)
	cbID := listCBResp.Embedded.Records[0].BalanceID

	// Claim the claimable balance
	claimCB := txnbuild.ClaimClaimableBalance{
		BalanceID: cbID,
	}
	allOps = append(allOps, &claimCB)
	txResp := itest.MustSubmitOperations(account, accountKeyPair, &claimCB)

	return allOps, txResp.Ledger
}

func submitOfferAndTrustlineOps(itest *integration.Test, tt *assert.Assertions) (submittedOperations []txnbuild.Operation, lastLedger int32) {
	// Create another account from which to claw an asset back
	keyPairs, accounts := itest.CreateAccounts(1, "100")
	accountKeyPair := keyPairs[0]
	account := accounts[0]

	// Add some assets to the account with asset which allows clawback

	// Time machine to Spain before Euros were a thing
	pesetasAsset := txnbuild.CreditAsset{Code: "PTS", Issuer: itest.Master().Address()}
	itest.MustEstablishTrustline(accountKeyPair, account, pesetasAsset)

	ops := []txnbuild.Operation{
		&txnbuild.ManageSellOffer{
			Selling: txnbuild.NativeAsset{},
			Buying:  pesetasAsset,
			Amount:  "10",
			Price:   xdr.Price{N: 1, D: 1},
			OfferID: 0,
		},
		&txnbuild.ManageBuyOffer{
			Selling: txnbuild.NativeAsset{},
			Buying:  pesetasAsset,
			Amount:  "10",
			Price:   xdr.Price{N: 1, D: 1},
			OfferID: 0,
		},
		&txnbuild.CreatePassiveSellOffer{
			Selling: txnbuild.NativeAsset{},
			Buying:  pesetasAsset,
			Amount:  "10",
			Price:   xdr.Price{N: 1, D: 1},
		},
	}
	allOps := ops
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), ops...)

	ops = []txnbuild.Operation{
		&txnbuild.AllowTrust{
			Trustor:   account.GetAccountID(),
			Type:      pesetasAsset,
			Authorize: true,
		},
		&txnbuild.SetTrustLineFlags{
			Trustor:  account.GetAccountID(),
			Asset:    pesetasAsset,
			SetFlags: []txnbuild.TrustLineFlag{txnbuild.TrustLineAuthorized},
		},
	}
	allOps = append(allOps, ops...)
	txResp := itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), ops...)

	return allOps, txResp.Ledger
}

func submitAccountOps(itest *integration.Test, tt *assert.Assertions) (submittedOperations []txnbuild.Operation, lastLedger int32) {
	accountPair, _ := keypair.Random()

	ops := []txnbuild.Operation{
		&txnbuild.CreateAccount{
			Destination: accountPair.Address(),
			Amount:      "100",
		},
	}
	allOps := ops
	itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(), ops...)
	account := itest.MustGetAccount(accountPair)
	domain := "www.example.com"
	ops = []txnbuild.Operation{
		&txnbuild.BumpSequence{
			BumpTo: account.Sequence + 1000,
		},
		&txnbuild.SetOptions{
			HomeDomain: &domain,
		},
		&txnbuild.ManageData{
			Name:  "foo",
			Value: []byte("bar"),
		},
	}
	allOps = append(allOps, ops...)
	itest.MustSubmitOperations(&account, accountPair, ops...)
	// Resync bump sequence
	account = itest.MustGetAccount(accountPair)

	ops = []txnbuild.Operation{
		&txnbuild.ManageData{
			Name:  "foo",
			Value: nil,
		},
		&txnbuild.AccountMerge{
			Destination: itest.Master().Address(),
		},
	}
	allOps = append(allOps, ops...)
	txResp := itest.MustSubmitOperations(&account, accountPair, ops...)

	return allOps, txResp.Ledger
}

func initializeDBIntegrationTest(t *testing.T) (*integration.Test, int32) {
	itest := integration.NewTest(t, integration.Config{
		HorizonIngestParameters: map[string]string{
			"admin-port": strconv.Itoa(6000),
		}})
	tt := assert.New(t)

	// Make sure all possible operations are covered by reingestion
	allOpTypes := set.Set[xdr.OperationType]{}
	for typ := range xdr.OperationTypeToStringMap {
		allOpTypes.Add(xdr.OperationType(typ))
	}

	submitters := []func(*integration.Test, *assert.Assertions) ([]txnbuild.Operation, int32){
		submitAccountOps,
		submitPaymentOps,
		submitOfferAndTrustlineOps,
		submitSponsorshipOps,
		submitClaimableBalanceOps,
		submitClawbackOps,
		submitLiquidityPoolOps,
	}

	// TODO - re-enable invoke host function 'submitSorobanOps' test
	// once fees/footprint from preflight are working in test
	if false && integration.GetCoreMaxSupportedProtocol() > 19 {
		submitters = append(submitters, submitSorobanOps)
	} else {
		delete(allOpTypes, xdr.OperationTypeInvokeHostFunction)
		delete(allOpTypes, xdr.OperationTypeExtendFootprintTtl)
		delete(allOpTypes, xdr.OperationTypeRestoreFootprint)
	}

	// Inflation is not supported
	delete(allOpTypes, xdr.OperationTypeInflation)

	var submittedOps []txnbuild.Operation
	var ledgerOfLastSubmittedTx int32
	// submit all possible operations
	for i, f := range submitters {
		var ops []txnbuild.Operation
		ops, ledgerOfLastSubmittedTx = f(itest, tt)
		t.Logf("%v ledgerOfLastSubmittedTx %v", i, ledgerOfLastSubmittedTx)
		submittedOps = append(submittedOps, ops...)
	}

	for _, op := range submittedOps {
		opXDR, err := op.BuildXDR()
		tt.NoError(err)
		delete(allOpTypes, opXDR.Body.Type)
	}
	tt.Empty(allOpTypes)

	reachedLedger := func() bool {
		root, err := itest.Client().Root()
		tt.NoError(err)
		return root.HorizonSequence >= ledgerOfLastSubmittedTx
	}
	tt.Eventually(reachedLedger, 15*time.Second, 5*time.Second)

	return itest, ledgerOfLastSubmittedTx
}

func TestReingestDB(t *testing.T) {
	itest, reachedLedger := initializeDBIntegrationTest(t)
	tt := assert.New(t)

	horizonConfig := itest.GetHorizonIngestConfig()
	t.Run("validate parallel range", func(t *testing.T) {
		var rootCmd = horizoncmd.NewRootCmd()
		rootCmd.SetArgs(command(t, horizonConfig,
			"db",
			"reingest",
			"range",
			"--parallel-workers=2",
			"10",
			"2",
		))

		assert.EqualError(t, rootCmd.Execute(), "Invalid range: {10 2} from > to")
	})

	t.Logf("reached ledger is %v", reachedLedger)
	// cap reachedLedger to the nearest checkpoint ledger because reingest range
	// cannot ingest past the most recent checkpoint ledger when using captive
	// core
	toLedger := uint32(reachedLedger)
	archive, err := itest.GetHistoryArchive()
	tt.NoError(err)

	// make sure a full checkpoint has elapsed otherwise there will be nothing to reingest
	var latestCheckpoint uint32
	publishedFirstCheckpoint := func() bool {
		has, requestErr := archive.GetRootHAS()
		if requestErr != nil {
			t.Logf("request to fetch checkpoint failed: %v", requestErr)
			return false
		}
		latestCheckpoint = has.CurrentLedger
		return latestCheckpoint > 1
	}
	tt.Eventually(publishedFirstCheckpoint, 10*time.Second, time.Second)

	if toLedger > latestCheckpoint {
		toLedger = latestCheckpoint
	}

	// We just want to test reingestion, so there's no reason for a background
	// Horizon to run. Keeping it running will actually cause the Captive Core
	// subprocesses to conflict.
	itest.StopHorizon()

	var rootCmd = horizoncmd.NewRootCmd()
	rootCmd.SetArgs(command(t, horizonConfig, "db",
		"reingest",
		"range",
		"--parallel-workers=1",
		"1",
		fmt.Sprintf("%d", toLedger),
	))

	tt.NoError(rootCmd.Execute())
	tt.NoError(rootCmd.Execute(), "Repeat the same reingest range against db, should not have errors.")
}

func TestReingestDatastore(t *testing.T) {
	test := integration.NewTest(t, integration.Config{
		SkipHorizonStart:          true,
		SkipCoreContainerCreation: true,
	})
	err := test.StartHorizon(false)
	assert.NoError(t, err)
	test.WaitForHorizonWeb()

	testTempDir := t.TempDir()
	fakeBucketFilesSource := "testdata/testbucket"
	fakeBucketFiles := []fakestorage.Object{}

	if err = filepath.WalkDir(fakeBucketFilesSource, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.Type().IsRegular() {
			contents, err := os.ReadFile(fmt.Sprintf("%s/%s", fakeBucketFilesSource, entry.Name()))
			if err != nil {
				return err
			}

			fakeBucketFiles = append(fakeBucketFiles, fakestorage.Object{
				ObjectAttrs: fakestorage.ObjectAttrs{
					BucketName: "path",
					Name:       fmt.Sprintf("to/my/bucket/FFFFFFFF--0-63999/%s", entry.Name()),
				},
				Content: contents,
			})
		}
		return nil
	}); err != nil {
		t.Fatalf("unable to setup fake bucket files: %v", err)
	}

	testWriter := &testWriter{test: t}
	opts := fakestorage.Options{
		Scheme:         "http",
		Host:           "127.0.0.1",
		Port:           uint16(0),
		Writer:         testWriter,
		StorageRoot:    filepath.Join(testTempDir, "bucket"),
		PublicHost:     "127.0.0.1",
		InitialObjects: fakeBucketFiles,
	}

	gcsServer, err := fakestorage.NewServerWithOptions(opts)

	if err != nil {
		t.Fatalf("couldn't start the fake gcs http server %v", err)
	}

	defer gcsServer.Stop()
	t.Logf("fake gcs server started at %v", gcsServer.URL())
	t.Setenv("STORAGE_EMULATOR_HOST", gcsServer.URL())

	rootCmd := horizoncmd.NewRootCmd()
	rootCmd.SetArgs([]string{"db",
		"reingest",
		"range",
		"--db-url", test.GetTestDB().DSN,
		"--network", "testnet",
		"--parallel-workers", "1",
		"--ledgerbackend", "datastore",
		"--datastore-config", "../ingest/testdata/config.storagebackend.toml",
		"997",
		"999"})

	require.NoError(t, rootCmd.Execute())

	_, err = test.Client().LedgerDetail(998)
	require.NoError(t, err)
}

func TestReingestDBWithFilterRules(t *testing.T) {
	itest, _ := initializeDBIntegrationTest(t)
	tt := assert.New(t)

	archive, err := itest.GetHistoryArchive()
	tt.NoError(err)

	// make sure one full checkpoint has elapsed before making ledger entries
	// as test can't reap before first checkpoint in general later in test
	publishedFirstCheckpoint := func() bool {
		has, requestErr := archive.GetRootHAS()
		if requestErr != nil {
			t.Logf("request to fetch checkpoint failed: %v", requestErr)
			return false
		}
		return has.CurrentLedger > 1
	}
	tt.Eventually(publishedFirstCheckpoint, 10*time.Second, time.Second)

	fullKeys, accounts := itest.CreateAccounts(2, "10000")
	whitelistedAccount := accounts[0]
	whitelistedAccountKey := fullKeys[0]
	nonWhitelistedAccount := accounts[1]
	nonWhitelistedAccountKey := fullKeys[1]
	enabled := true

	// all assets are allowed by default because the asset filter config is empty.
	defaultAllowedAsset := txnbuild.CreditAsset{Code: "PTS", Issuer: itest.Master().Address()}
	itest.MustEstablishTrustline(whitelistedAccountKey, whitelistedAccount, defaultAllowedAsset)
	itest.MustEstablishTrustline(nonWhitelistedAccountKey, nonWhitelistedAccount, defaultAllowedAsset)

	// Setup a whitelisted account rule, force refresh of filter configs to be quick
	filters.SetFilterConfigCheckIntervalSeconds(1)

	expectedAccountFilter := hProtocol.AccountFilterConfig{
		Whitelist: []string{whitelistedAccount.GetAccountID()},
		Enabled:   &enabled,
	}
	err = itest.AdminClient().SetIngestionAccountFilter(expectedAccountFilter)
	tt.NoError(err)

	accountFilter, err := itest.AdminClient().GetIngestionAccountFilter()
	tt.NoError(err)

	tt.ElementsMatch(expectedAccountFilter.Whitelist, accountFilter.Whitelist)
	tt.Equal(expectedAccountFilter.Enabled, accountFilter.Enabled)

	// Ensure the latest filter configs are reloaded by the ingestion state machine processor
	time.Sleep(time.Duration(filters.GetFilterConfigCheckIntervalSeconds()) * time.Second)

	// Make sure that when using a non-whitelisted account, the transaction is not stored
	nonWhiteListTxResp := itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(),
		&txnbuild.Payment{
			Destination: nonWhitelistedAccount.GetAccountID(),
			Amount:      "10",
			Asset:       defaultAllowedAsset,
		},
	)
	_, err = itest.Client().TransactionDetail(nonWhiteListTxResp.Hash)
	tt.True(horizonclient.IsNotFoundError(err))

	// Make sure that when using a whitelisted account, the transaction is stored
	whiteListTxResp := itest.MustSubmitOperations(itest.MasterAccount(), itest.Master(),
		&txnbuild.Payment{
			Destination: whitelistedAccount.GetAccountID(),
			Amount:      "10",
			Asset:       defaultAllowedAsset,
		},
	)
	lastTx, err := itest.Client().TransactionDetail(whiteListTxResp.Hash)
	tt.NoError(err)

	reachedLedger := uint32(lastTx.Ledger)

	t.Logf("reached ledger is %v", reachedLedger)

	// make sure a checkpoint has elapsed to lock in the chagnes made on network for reingest later
	var latestCheckpoint uint32
	publishedNextCheckpoint := func() bool {
		has, requestErr := archive.GetRootHAS()
		if requestErr != nil {
			t.Logf("request to fetch checkpoint failed: %v", requestErr)
			return false
		}
		latestCheckpoint = has.CurrentLedger
		return latestCheckpoint > reachedLedger
	}
	tt.Eventually(publishedNextCheckpoint, 10*time.Second, time.Second)

	// to test reingestion, stop horizon web and captive core,
	// it was used to create ledger entries for test.
	itest.StopHorizon()

	// clear the db with reaping all ledgers
	var rootCmd = horizoncmd.NewRootCmd()
	rootCmd.SetArgs(command(t, itest.GetHorizonIngestConfig(), "db",
		"reap",
		"--history-retention-count=1",
	))
	tt.NoError(rootCmd.Execute())

	// repopulate the db with reingestion which should catchup using core reapply filter rules
	// correctly on reingestion ranged
	rootCmd = horizoncmd.NewRootCmd()
	rootCmd.SetArgs(command(t, itest.GetHorizonIngestConfig(), "db",
		"reingest",
		"range",
		"1",
		fmt.Sprintf("%d", reachedLedger),
	))

	tt.NoError(rootCmd.Execute())

	// bring up horizon, just the api server no ingestion, to query
	// for tx's that should have been repopulated on db from reingestion per
	// filter rule expectations
	webApp, err := horizon.NewApp(itest.GetHorizonWebConfig())
	tt.NoError(err)

	webAppDone := make(chan struct{})
	go func() {
		webApp.Serve()
		close(webAppDone)
	}()

	// wait until the web server is up before continuing to test requests
	itest.WaitForHorizonIngest()

	// Make sure that a tx from non-whitelisted account is not stored after reingestion
	_, err = itest.Client().TransactionDetail(nonWhiteListTxResp.Hash)
	tt.True(horizonclient.IsNotFoundError(err))

	// Make sure that a tx from whitelisted account is stored after reingestion
	_, err = itest.Client().TransactionDetail(whiteListTxResp.Hash)
	tt.NoError(err)

	// tell the horizon web server to shutdown
	webApp.Close()

	// wait for horizon to finish shutdown
	tt.Eventually(func() bool {
		select {
		case <-webAppDone:
			return true
		default:
			return false
		}
	}, 30*time.Second, time.Second)
}

func command(t *testing.T, horizonConfig horizon.Config, args ...string) []string {
	return append([]string{
		"--stellar-core-url",
		horizonConfig.StellarCoreURL,
		"--history-archive-urls",
		horizonConfig.HistoryArchiveURLs[0],
		"--db-url",
		horizonConfig.DatabaseURL,
		"--stellar-core-binary-path",
		horizonConfig.CaptiveCoreBinaryPath,
		"--captive-core-config-path",
		horizonConfig.CaptiveCoreConfigPath,
		"--captive-core-use-db=" +
			strconv.FormatBool(horizonConfig.CaptiveCoreConfigUseDB),
		"--network-passphrase",
		horizonConfig.NetworkPassphrase,
		// due to ARTIFICIALLY_ACCELERATE_TIME_FOR_TESTING
		"--checkpoint-frequency",
		"8",
		// Create the storage directory outside of the source repo,
		// otherwise it will break Golang test caching.
		"--captive-core-storage-path=" + t.TempDir(),
	}, args...)
}

func TestMigrateIngestIsTrueByDefault(t *testing.T) {
	tt := assert.New(t)
	// Create a fresh Horizon database
	newDB := dbtest.Postgres(t)
	freshHorizonPostgresURL := newDB.DSN

	rootCmd := horizoncmd.NewRootCmd()
	rootCmd.SetArgs([]string{
		// ingest is set to true by default
		"--db-url", freshHorizonPostgresURL,
		"db", "migrate", "up",
	})
	tt.NoError(rootCmd.Execute())

	dbConn, err := db.Open("postgres", freshHorizonPostgresURL)
	tt.NoError(err)

	status, err := schema.Status(dbConn.DB.DB)
	tt.NoError(err)
	tt.NotContains(status, "1_initial_schema.sql\t\t\t\t\t\tno")
}

func TestMigrateChecksIngestFlag(t *testing.T) {
	tt := assert.New(t)
	// Create a fresh Horizon database
	newDB := dbtest.Postgres(t)
	freshHorizonPostgresURL := newDB.DSN

	rootCmd := horizoncmd.NewRootCmd()
	rootCmd.SetArgs([]string{
		"--ingest=false",
		"--db-url", freshHorizonPostgresURL,
		"db", "migrate", "up",
	})
	tt.NoError(rootCmd.Execute())

	dbConn, err := db.Open("postgres", freshHorizonPostgresURL)
	tt.NoError(err)

	status, err := schema.Status(dbConn.DB.DB)
	tt.NoError(err)
	tt.Contains(status, "1_initial_schema.sql\t\t\t\t\t\tno")
}

func TestFillGaps(t *testing.T) {
	itest, reachedLedger := initializeDBIntegrationTest(t)
	tt := assert.New(t)

	// Create a fresh Horizon database
	newDB := dbtest.Postgres(t)
	freshHorizonPostgresURL := newDB.DSN
	horizonConfig := itest.GetHorizonIngestConfig()
	horizonConfig.DatabaseURL = freshHorizonPostgresURL
	// Initialize the DB schema
	dbConn, err := db.Open("postgres", freshHorizonPostgresURL)
	tt.NoError(err)
	historyQ := history.Q{SessionInterface: dbConn}
	defer func() {
		historyQ.Close()
		newDB.Close()
	}()

	_, err = schema.Migrate(dbConn.DB.DB, schema.MigrateUp, 0)
	tt.NoError(err)

	// cap reachedLedger to the nearest checkpoint ledger because reingest range cannot ingest past the most
	// recent checkpoint ledger when using captive core
	toLedger := uint32(reachedLedger)
	archive, err := itest.GetHistoryArchive()
	tt.NoError(err)

	t.Run("validate parallel range", func(t *testing.T) {
		var rootCmd = horizoncmd.NewRootCmd()
		rootCmd.SetArgs(command(t, horizonConfig,
			"db",
			"fill-gaps",
			"--parallel-workers=2",
			"10",
			"2",
		))

		assert.EqualError(t, rootCmd.Execute(), "Invalid range: {10 2} from > to")
	})

	// make sure a full checkpoint has elapsed otherwise there will be nothing to reingest
	var latestCheckpoint uint32
	publishedFirstCheckpoint := func() bool {
		has, requestErr := archive.GetRootHAS()
		tt.NoError(requestErr)
		latestCheckpoint = has.CurrentLedger
		return latestCheckpoint > 1
	}
	tt.Eventually(publishedFirstCheckpoint, 10*time.Second, time.Second)

	if toLedger > latestCheckpoint {
		toLedger = latestCheckpoint
	}

	// We just want to test reingestion, so there's no reason for a background
	// Horizon to run. Keeping it running will actually cause the Captive Core
	// subprocesses to conflict.
	itest.StopHorizon()

	var oldestLedger, latestLedger int64
	tt.NoError(historyQ.ElderLedger(context.Background(), &oldestLedger))
	tt.NoError(historyQ.LatestLedger(context.Background(), &latestLedger))
	_, err = historyQ.DeleteRangeAll(context.Background(), oldestLedger, latestLedger)
	tt.NoError(err)

	rootCmd := horizoncmd.NewRootCmd()
	rootCmd.SetArgs(command(t, horizonConfig, "db", "fill-gaps", "--parallel-workers=1"))
	tt.NoError(rootCmd.Execute())

	tt.NoError(historyQ.LatestLedger(context.Background(), &latestLedger))
	tt.Equal(int64(0), latestLedger)

	rootCmd = horizoncmd.NewRootCmd()
	rootCmd.SetArgs(command(t, horizonConfig, "db", "fill-gaps", "3", "4"))
	tt.NoError(rootCmd.Execute())
	tt.NoError(historyQ.LatestLedger(context.Background(), &latestLedger))
	tt.NoError(historyQ.ElderLedger(context.Background(), &oldestLedger))
	tt.Equal(int64(3), oldestLedger)
	tt.Equal(int64(4), latestLedger)

	rootCmd = horizoncmd.NewRootCmd()
	rootCmd.SetArgs(command(t, horizonConfig, "db", "fill-gaps", "6", "7"))
	tt.NoError(rootCmd.Execute())
	tt.NoError(historyQ.LatestLedger(context.Background(), &latestLedger))
	tt.NoError(historyQ.ElderLedger(context.Background(), &oldestLedger))
	tt.Equal(int64(3), oldestLedger)
	tt.Equal(int64(7), latestLedger)
	var gaps []history.LedgerRange
	gaps, err = historyQ.GetLedgerGaps(context.Background())
	tt.NoError(err)
	tt.Equal([]history.LedgerRange{{StartSequence: 5, EndSequence: 5}}, gaps)

	rootCmd = horizoncmd.NewRootCmd()
	rootCmd.SetArgs(command(t, horizonConfig, "db", "fill-gaps"))
	tt.NoError(rootCmd.Execute())
	tt.NoError(historyQ.LatestLedger(context.Background(), &latestLedger))
	tt.NoError(historyQ.ElderLedger(context.Background(), &oldestLedger))
	tt.Equal(int64(3), oldestLedger)
	tt.Equal(int64(7), latestLedger)
	gaps, err = historyQ.GetLedgerGaps(context.Background())
	tt.NoError(err)
	tt.Empty(gaps)

	rootCmd = horizoncmd.NewRootCmd()
	rootCmd.SetArgs(command(t, horizonConfig, "db", "fill-gaps", "2", "8"))
	tt.NoError(rootCmd.Execute())
	tt.NoError(historyQ.LatestLedger(context.Background(), &latestLedger))
	tt.NoError(historyQ.ElderLedger(context.Background(), &oldestLedger))
	tt.Equal(int64(2), oldestLedger)
	tt.Equal(int64(8), latestLedger)
	gaps, err = historyQ.GetLedgerGaps(context.Background())
	tt.NoError(err)
	tt.Empty(gaps)
}

func TestResumeFromInitializedDB(t *testing.T) {
	itest, reachedLedger := initializeDBIntegrationTest(t)
	tt := assert.New(t)

	// Stop the integration test, and restart it with the same database
	err := itest.RestartHorizon(true)
	tt.NoError(err)

	successfullyResumed := func() bool {
		root, err := itest.Client().Root()
		tt.NoError(err)
		// It must be able to reach the ledger and surpass it
		const ledgersPastStopPoint = 4
		return root.HorizonSequence > (reachedLedger + ledgersPastStopPoint)
	}

	tt.Eventually(successfullyResumed, 1*time.Minute, 1*time.Second)
}

type testWriter struct {
	test *testing.T
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.test.Log(string(p))
	return len(p), nil
}

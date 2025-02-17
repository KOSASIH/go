package integration

import (
	"context"
	"encoding"
	"flag"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/clients/stellarcore"
	"github.com/stellar/go/ingest"
	"github.com/stellar/go/ingest/loadtest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	proto "github.com/stellar/go/protocols/stellarcore"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

const loadTestNetworkPassphrase = "load test network"

type sorobanTransaction struct {
	op             *txnbuild.InvokeHostFunction
	fee            int64
	signer         *keypair.Full
	sequenceNumber int64
}

func TestGenerateLedgers(t *testing.T) {
	var transactionsPerLedger, ledgers, transfersPerTx int
	var output bool
	var networkPassphrase string
	flag.IntVar(&transactionsPerLedger, "transactions-per-ledger", 100, "number of transactions per ledger")
	flag.IntVar(&transfersPerTx, "transfers-per-tx", 10, "number of asset transfers for each transaction")
	flag.IntVar(&ledgers, "ledgers", 2, "number of ledgers to generate")
	flag.BoolVar(&output, "output", false, "overwrite the generated output files")
	flag.StringVar(&networkPassphrase, "network-passphrase", loadTestNetworkPassphrase, "network passphrase")
	flag.Parse()

	if integration.GetCoreMaxSupportedProtocol() < 22 {
		t.Skip("This test run does not support less than Protocol 22")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC:  true,
		NetworkPassphrase: networkPassphrase,
	})

	maxAccountsPerTransaction := 100
	// transactionsPerLedger should be a multiple of maxAccountsPerTransaction
	require.Zero(t, transactionsPerLedger%maxAccountsPerTransaction)

	txErr := itest.CoreClient().UpgradeTxSetSize(context.Background(), uint32(transactionsPerLedger*100), time.Unix(0, 0))
	require.NoError(t, txErr)

	txErr = itest.CoreClient().UpgradeSorobanTxSetSize(context.Background(), uint32(transactionsPerLedger*100), time.Unix(0, 0))
	require.NoError(t, txErr)

	contents, err := os.ReadFile(filepath.Join("testdata", "unlimited-config.xdr"))
	require.NoError(t, err)
	var configSet xdr.ConfigUpgradeSet
	err = xdr.SafeUnmarshalBase64(string(contents), &configSet)
	require.NoError(t, err)

	upgradeTransactions, upgradeKey, err := stellarcore.GenSorobanConfigUpgradeTxAndKey(stellarcore.GenSorobanConfig{
		BaseSeqNum:        0,
		NetworkPassphrase: itest.Config().NetworkPassphrase,
		SigningKey:        itest.Master(),
		StellarCorePath:   itest.CoreBinaryPath(),
	}, configSet)
	require.NoError(t, err)

	for _, transaction := range upgradeTransactions {
		var b64 string
		b64, err = xdr.MarshalBase64(transaction)
		require.NoError(t, err)
		var response horizon.Transaction
		response, err = itest.Client().SubmitTransactionXDR(b64)
		require.NoError(t, err)
		require.True(t, response.Successful)
	}

	require.NoError(t,
		itest.CoreClient().UpgradeSorobanConfig(context.Background(), upgradeKey, time.Unix(0, 0)),
	)

	xlm := xdr.MustNewNativeAsset()
	createSAC(itest, xlm)

	bulkContractID, _ := mustCreateAndInstallContract(
		itest,
		itest.Master(),
		"a1",
		"soroban_bulk.wasm",
	)

	var signers []*keypair.Full
	var accounts []txnbuild.Account
	var accountLedgers []uint32
	for i := 0; i < 2*transactionsPerLedger; i += maxAccountsPerTransaction {
		keys, curAccounts := itest.CreateAccounts(maxAccountsPerTransaction, "10000000")
		var account horizon.Account
		account, err = itest.Client().AccountDetail(horizonclient.AccountRequest{AccountID: curAccounts[0].GetAccountID()})
		require.NoError(t, err)
		accountLedgers = append(accountLedgers, account.LastModifiedLedger)

		signers = append(signers, keys...)
		accounts = append(accounts, curAccounts...)
	}
	recipients := signers[transactionsPerLedger:]
	signers = signers[:transactionsPerLedger]
	accounts = accounts[:transactionsPerLedger]
	var transactions []sorobanTransaction
	var bulkAmounts xdr.ScVec
	for i := 0; i < transfersPerTx; i++ {
		bulkAmounts = append(bulkAmounts, i128Param(0, uint64(amount.MustParse("1"))))
	}

	for i := range signers {
		var op *txnbuild.InvokeHostFunction
		sender := accounts[i].GetAccountID()

		var bulkRecipients xdr.ScVec
		if i%2 == 0 {
			for j := i; j < i+transfersPerTx; j++ {
				recipient := accountAddressParam(recipients[j%len(recipients)].Address())
				bulkRecipients = append(bulkRecipients, recipient)
			}
		} else if i%2 == 1 {
			for j := 0; j < transfersPerTx; j++ {
				var contractID xdr.Hash
				_, err = rand.Read(contractID[:])
				require.NoError(t, err)
				bulkRecipients = append(bulkRecipients, contractAddressParam(contractID))
			}
		}

		op = bulkTransfer(itest, bulkContractID, sender, xlm, &bulkRecipients, &bulkAmounts)
		preFlightOp, minFee := itest.PreflightHostFunctions(accounts[i], *op)
		preFlightOp.Ext.SorobanData.Resources.ReadBytes *= 10
		preFlightOp.Ext.SorobanData.Resources.WriteBytes *= 10
		preFlightOp.Ext.SorobanData.Resources.Instructions *= 10
		preFlightOp.Ext.SorobanData.ResourceFee *= 10
		minFee *= 10
		var sequenceNumber int64
		sequenceNumber, err = accounts[i].GetSequenceNumber()
		require.NoError(t, err)
		transactions = append(transactions, sorobanTransaction{
			op:             &preFlightOp,
			fee:            minFee + txnbuild.MinBaseFee,
			signer:         signers[i],
			sequenceNumber: sequenceNumber,
		})
	}

	lock := &sync.Mutex{}
	ledgerMap := map[int32]int{}
	wg := &sync.WaitGroup{}
	transactionsPerWorker := 100
	// transactions should be a multiple of transactionsPerWorker
	require.Zero(t, len(transactions)%transactionsPerWorker)
	for repetitions := 0; repetitions < ledgers; repetitions++ {
		for i := 0; i < len(transactions); i += transactionsPerWorker {
			subset := transactions[i : i+transactionsPerWorker]
			wg.Add(1)
			go func() {
				defer wg.Done()
				txSubWorker(
					itest,
					subset,
					itest.Client(),
					itest.CoreClient(),
					lock,
					ledgerMap,
					int64(repetitions),
				)
			}()
		}
		wg.Wait()
	}

	start, end := int32(-1), int32(-1)
	for ledgerSeq := range ledgerMap {
		if start < 0 || start > ledgerSeq {
			start = ledgerSeq
		}
		if end < 0 || ledgerSeq > end {
			end = ledgerSeq
		}
	}
	t.Logf("waiting for ledgers [%v, %v] to be in history archive", start, end)
	itest.WaitForLedgerInArchive(6*time.Minute, uint32(end))
	allLedgers := getLedgers(itest, uint32(start), uint32(end))

	var sortedLegers []xdr.LedgerCloseMeta
	for ledgerSeq := range ledgerMap {
		lcm, ok := allLedgers[uint32(ledgerSeq)]
		require.True(t, ok)
		sortedLegers = append(sortedLegers, lcm)
	}
	sort.Slice(sortedLegers, func(i, j int) bool {
		return sortedLegers[i].LedgerSequence() < sortedLegers[j].LedgerSequence()
	})

	ledgersForAccounts := getLedgers(itest, accountLedgers[0], accountLedgers[len(accountLedgers)-1])
	var accountLedgerEntries []xdr.LedgerEntry
	accountSet := map[string]bool{}
	for _, seq := range accountLedgers {
		for _, change := range extractChanges(
			t, itest.Config().NetworkPassphrase, []xdr.LedgerCloseMeta{ledgersForAccounts[seq]},
		) {
			if change.Type == xdr.LedgerEntryTypeAccount && change.Post != nil && change.Pre == nil {
				account := *change.Post
				accountSet[account.Data.MustAccount().AccountId.Address()] = true
				accountLedgerEntries = append(accountLedgerEntries, *change.Post)
			}
		}
	}
	require.Len(t, accountLedgerEntries, 2*transactionsPerLedger)

	if output {
		writeFile(t, filepath.Join("testdata", "load-test-accounts.xdr.zstd"), accountLedgerEntries)
	}

	merged := merge(t, itest.Config().NetworkPassphrase, sortedLegers, transactionsPerLedger)
	changes := extractChanges(t, itest.Config().NetworkPassphrase, sortedLegers)
	for _, change := range changes {
		if change.Type != xdr.LedgerEntryTypeAccount {
			continue
		}
		var ledgerKey xdr.LedgerKey
		ledgerKey, err = change.LedgerKey()
		require.NoError(t, err)
		require.True(t, accountSet[ledgerKey.MustAccount().AccountId.Address()])
	}
	requireChangesAreEqual(t, changes, extractChanges(t, itest.Config().NetworkPassphrase, merged))

	orignalTransactions := extractTransactions(t, itest.Config().NetworkPassphrase, sortedLegers)
	mergedTransactions := extractTransactions(t, itest.Config().NetworkPassphrase, merged)
	require.Equal(t, len(orignalTransactions), len(mergedTransactions))
	for i, original := range orignalTransactions {
		requireTransactionsMatch(t, original, mergedTransactions[i])
	}

	if output {
		writeFile(t, filepath.Join("testdata", "load-test-ledgers.xdr.zstd"), merged)
	}
}

func writeFile[T any](t *testing.T, path string, data []T) {
	file, err := os.Create(path)
	require.NoError(t, err)
	writer, err := zstd.NewWriter(file)
	require.NoError(t, err)
	for _, entry := range data {
		require.NoError(t, xdr.MarshalFramed(writer, entry))
	}
	require.NoError(t, writer.Close())
	require.NoError(t, file.Close())
}

func readFile[T xdr.DecoderFrom](t *testing.T, path string, constructor func() T, consume func(T)) {
	file, err := os.Open(path)
	require.NoError(t, err)
	stream, err := xdr.NewZstdStream(file)
	require.NoError(t, err)
	for {
		entry := constructor()
		if err = stream.ReadOne(entry); err == io.EOF {
			break
		}
		require.NoError(t, err)
		consume(entry)
	}
	require.NoError(t, stream.Close())
}

func bulkTransfer(
	itest *integration.Test,
	bulkContractID xdr.Hash,
	sender string,
	asset xdr.Asset,
	recipients *xdr.ScVec,
	amounts *xdr.ScVec,
) *txnbuild.InvokeHostFunction {
	return &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contractIDParam(bulkContractID),
				FunctionName:    "bulk_transfer",
				Args: xdr.ScVec{
					accountAddressParam(sender),
					contractAddressParam(stellarAssetContractID(itest, asset)),
					xdr.ScVal{Type: xdr.ScValTypeScvVec, Vec: &recipients},
					xdr.ScVal{Type: xdr.ScValTypeScvVec, Vec: &amounts},
				},
			},
		},
		SourceAccount: sender,
	}
}

func extractChanges(t *testing.T, networkPassphrase string, ledgers []xdr.LedgerCloseMeta) []ingest.Change {
	var changes []ingest.Change
	for _, ledger := range ledgers {
		reader, err := ingest.NewLedgerChangeReaderFromLedgerCloseMeta(networkPassphrase, ledger)
		require.NoError(t, err)
		for {
			var change ingest.Change
			change, err = reader.Read()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			changes = append(changes, change)
		}
	}
	return changes
}

func extractTransactions(t *testing.T, networkPassphrase string, ledgers []xdr.LedgerCloseMeta) []ingest.LedgerTransaction {
	var transactions []ingest.LedgerTransaction
	for _, ledger := range ledgers {
		txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(networkPassphrase, ledger)
		require.NoError(t, err)
		for {
			var tx ingest.LedgerTransaction
			tx, err = txReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			transactions = append(transactions, tx)
		}
	}
	return transactions
}

func groupChangesByLedgerKey(t *testing.T, changes []ingest.Change) map[string][]ingest.Change {
	byLedgerKey := map[string][]ingest.Change{}
	for _, change := range changes {
		key, err := change.LedgerKey()
		require.NoError(t, err)
		keyB64, err := key.MarshalBinaryBase64()
		require.NoError(t, err)
		byLedgerKey[keyB64] = append(byLedgerKey[keyB64], change)
	}
	return byLedgerKey
}

func requireChangesAreEqual(t *testing.T, a, b []ingest.Change) {
	aByLedgerKey := groupChangesByLedgerKey(t, a)
	bByLedgerKey := groupChangesByLedgerKey(t, b)

	require.Equal(t, len(aByLedgerKey), len(bByLedgerKey))
	for key, aChanges := range aByLedgerKey {
		bChanges := bByLedgerKey[key]
		require.Equal(t, len(aChanges), len(bChanges))
		for i, aChange := range aChanges {
			bChange := bChanges[i]
			require.Equal(t, aChange.Reason, bChange.Reason)
			require.Equal(t, aChange.Type, bChange.Type)
			if aChange.Pre == nil {
				require.Nil(t, bChange.Pre)
			} else {
				requireXDREquals(t, aChange.Pre, bChange.Pre)
			}
			if aChange.Post == nil {
				require.Nil(t, bChange.Post)
			} else {
				requireXDREquals(t, aChange.Post, bChange.Post)
			}
		}
	}
}

func requireTransactionsMatch(t *testing.T, a, b ingest.LedgerTransaction) {
	requireXDREquals(t, a.Hash, b.Hash)
	requireXDREquals(t, a.UnsafeMeta, b.UnsafeMeta)
	requireXDREquals(t, a.Result, b.Result)
	requireXDREquals(t, a.Envelope, b.Envelope)
	requireXDREquals(t, a.FeeChanges, b.FeeChanges)
	require.Equal(t, a.LedgerVersion, b.LedgerVersion)
}

func requireXDREquals(t *testing.T, a, b encoding.BinaryMarshaler) {
	ok, err := xdr.Equals(a, b)
	require.NoError(t, err)
	require.True(t, ok)
}

func txSubWorker(
	itest *integration.Test,
	subset []sorobanTransaction,
	horizonClient *horizonclient.Client,
	coreClient *stellarcore.Client,
	ledgerLock *sync.Mutex,
	ledgerMap map[int32]int,
	sequenceOffset int64,
) {
	var total time.Duration
	pending := map[string]bool{}
	for _, tx := range subset {
		account := txnbuild.NewSimpleAccount(tx.signer.Address(), tx.sequenceNumber+sequenceOffset)
		tx, err := itest.CreateSignedTransactionFromOpsWithFee(&account, []*keypair.Full{tx.signer}, tx.fee, tx.op)
		require.NoError(itest.CurrentTest(), err)

		hash, err := tx.HashHex(itest.Config().NetworkPassphrase)
		require.NoError(itest.CurrentTest(), err)
		b64Tx, err := tx.Base64()
		require.NoError(itest.CurrentTest(), err)
		start := time.Now()
		resp, err := coreClient.SubmitTransaction(context.Background(), b64Tx)
		elapsed := time.Since(start)
		require.NoError(itest.CurrentTest(), err)
		require.Empty(itest.CurrentTest(), resp.Exception)
		require.False(itest.CurrentTest(), resp.IsException())
		require.Equal(itest.CurrentTest(), proto.TXStatusPending, resp.Status)
		pending[hash] = true
		total += elapsed
	}
	avg := total / time.Duration(len(subset))
	itest.CurrentTest().Logf("avg %v total %v", avg, total)

	start := time.Now()
	waitForTransactions(itest.CurrentTest(), horizonClient, pending, ledgerLock, ledgerMap)
	itest.CurrentTest().Logf("wait duration %v", time.Since(start))

}

func waitForTransactions(
	t *testing.T,
	client *horizonclient.Client,
	pending map[string]bool,
	ledgerLock *sync.Mutex,
	ledgerMap map[int32]int,
) {
	require.Eventually(t, func() bool {
		for hash := range pending {
			resp, err := client.TransactionDetail(hash)
			if err == nil {
				delete(pending, hash)
				require.True(t, resp.Successful)
				ledgerLock.Lock()
				ledgerMap[resp.Ledger]++
				ledgerLock.Unlock()
				continue
			}
			if horizonclient.IsNotFoundError(err) {
				continue
			} else {
				require.NoError(t, err)
			}
		}
		return len(pending) == 0
	}, time.Second*90, time.Millisecond*100)
}

func merge(t *testing.T, networkPassphrase string, ledgers []xdr.LedgerCloseMeta, transactionsPerLedger int) []xdr.LedgerCloseMeta {
	var merged []xdr.LedgerCloseMeta
	if len(ledgers) == 0 {
		return merged
	}
	var cur xdr.LedgerCloseMeta
	var curCount int
	for _, ledger := range ledgers {
		transactionCount := ledger.CountTransactions()
		require.Empty(t, ledger.V1.EvictedTemporaryLedgerKeys)
		require.Empty(t, ledger.V1.EvictedPersistentLedgerEntries)
		require.Empty(t, ledger.V1.UpgradesProcessing)
		if transactionCount == 0 {
			continue
		}

		if curCount == 0 {
			cur = copyLedger(t, ledger)
		} else {
			require.NoError(t, loadtest.MergeLedgers(networkPassphrase, &cur, ledger))
		}

		require.LessOrEqual(t, curCount+transactionCount, transactionsPerLedger)
		curCount += transactionCount
		if curCount == transactionsPerLedger {
			merged = append(merged, cur)
			curCount = 0
		}
	}
	require.Zero(t, curCount)
	return merged
}

func copyLedger(t *testing.T, src xdr.LedgerCloseMeta) xdr.LedgerCloseMeta {
	var dst xdr.LedgerCloseMeta
	serialized, err := src.MarshalBinary()
	require.NoError(t, err)
	require.NoError(t, dst.UnmarshalBinary(serialized))
	return dst
}

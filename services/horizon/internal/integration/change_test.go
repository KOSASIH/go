package integration

import (
	"context"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

func TestProtocolUpgradeChanges(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{SkipHorizonStart: true})

	upgradedLedgerAppx, _ := itest.GetUpgradedLedgerSeqAppx()
	itest.WaitForLedgerInArchive(6*time.Minute, upgradedLedgerAppx)

	ledgerSeqToLedgers := getLedgers(itest, 2, upgradedLedgerAppx)

	// It is important to find the "exact" ledger which is representative of protocol upgrade
	// and the one before it, to check for upgrade related changes
	upgradedLedgerSeq := getExactUpgradedLedgerSeq(ledgerSeqToLedgers, itest.Config().ProtocolVersion)
	prevLedgerToUpgrade := upgradedLedgerSeq - 1

	prevLedgerChanges := getChangesFromLedger(itest, ledgerSeqToLedgers[prevLedgerToUpgrade])
	prevLedgerChangeMap := changeReasonCountMap(prevLedgerChanges)
	upgradedLedgerChanges := getChangesFromLedger(itest, ledgerSeqToLedgers[upgradedLedgerSeq])
	upgradedLedgerChangeMap := changeReasonCountMap(upgradedLedgerChanges)

	tt.Zero(prevLedgerChangeMap[ingest.LedgerEntryChangeReasonUpgrade])
	tt.NotZero(upgradedLedgerChangeMap[ingest.LedgerEntryChangeReasonUpgrade])
	for _, change := range upgradedLedgerChanges {
		tt.Equal(change.Ledger.LedgerSequence(), upgradedLedgerSeq)
		tt.Empty(change.Transaction)
		tt.NotEmpty(change.LedgerUpgrade)
	}
}

func TestOneTxOneOperationChanges(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})

	master := itest.Master()
	keys, _ := itest.CreateAccounts(2, "1000")
	srcAcc, destAcc := keys[0], keys[1]

	operation := txnbuild.Payment{
		SourceAccount: srcAcc.Address(),
		Destination:   destAcc.Address(),
		Asset:         txnbuild.NativeAsset{},
		Amount:        "900",
	}
	txResp, err := itest.SubmitMultiSigOperations(itest.MasterAccount(), []*keypair.Full{master, srcAcc}, &operation)
	tt.NoError(err)

	ledgerSeq := uint32(txResp.Ledger)
	itest.WaitForLedgerInArchive(6*time.Minute, ledgerSeq)

	ledger := getLedgers(itest, ledgerSeq, ledgerSeq)[ledgerSeq]
	changes := getChangesFromLedger(itest, ledger)

	reasonCntMap := changeReasonCountMap(changes)
	tt.Equal(2, reasonCntMap[ingest.LedgerEntryChangeReasonOperation])
	tt.Equal(1, reasonCntMap[ingest.LedgerEntryChangeReasonTransaction])
	tt.Equal(1, reasonCntMap[ingest.LedgerEntryChangeReasonFee])

	reasonToChangeMap := changeReasonToChangeMap(changes)
	// Assert Transaction Hash and Ledger Sequence within Transaction are accurate in all changes
	for _, change := range changes {
		tt.Equal(change.Transaction.Hash.HexString(), txResp.Hash)
		tt.Equal(change.Transaction.Ledger.LedgerSequence(), ledgerSeq)
		tt.Empty(change.Ledger)
		tt.Empty(change.LedgerUpgrade)
	}

	feeRelatedChange := reasonToChangeMap[ingest.LedgerEntryChangeReasonFee][0]
	txRelatedChange := reasonToChangeMap[ingest.LedgerEntryChangeReasonTransaction][0]
	operationChanges := reasonToChangeMap[ingest.LedgerEntryChangeReasonOperation]

	accountFromEntry := func(e *xdr.LedgerEntry) xdr.AccountEntry {
		return e.Data.MustAccount()
	}

	changeForAccount := func(changes []ingest.Change, target string) ingest.Change {
		for _, change := range changes {
			acc := change.Pre.Data.MustAccount()
			if acc.AccountId.Address() == target {
				return change
			}
		}
		return ingest.Change{}
	}

	containsAccount := func(changes []ingest.Change, target string) bool {
		for _, change := range changes {
			addr := change.Pre.Data.MustAccount().AccountId.Address()
			if addr == target {
				return true
			}
		}
		return false
	}

	tt.Equal(accountFromEntry(feeRelatedChange.Pre).AccountId.Address(), master.Address())
	tt.Equal(accountFromEntry(txRelatedChange.Pre).AccountId.Address(), master.Address())
	tt.True(containsAccount(operationChanges, srcAcc.Address()))
	tt.True(containsAccount(operationChanges, destAcc.Address()))
	// MasterAccount shouldn't show up in operation level changes
	tt.False(containsAccount(operationChanges, master.Address()))
	tt.True(accountFromEntry(feeRelatedChange.Pre).Balance > accountFromEntry(feeRelatedChange.Post).Balance)
	tt.True(accountFromEntry(txRelatedChange.Post).SeqNum == accountFromEntry(txRelatedChange.Pre).SeqNum+1)

	srcAccChange := changeForAccount(operationChanges, srcAcc.Address())
	destAccChange := changeForAccount(operationChanges, destAcc.Address())
	tt.True(accountFromEntry(srcAccChange.Pre).Balance > accountFromEntry(srcAccChange.Post).Balance)
	tt.True(accountFromEntry(destAccChange.Pre).Balance < accountFromEntry(destAccChange.Post).Balance)
}

func getChangesFromLedger(itest *integration.Test, ledger xdr.LedgerCloseMeta) []ingest.Change {
	t := itest.CurrentTest()
	changeReader, err := ingest.NewLedgerChangeReaderFromLedgerCloseMeta(itest.GetPassPhrase(), ledger)
	changes := make([]ingest.Change, 0)
	defer changeReader.Close()
	if err != nil {
		t.Fatalf("unable to create ledger change reader: %v", err)
	}
	for {
		change, err := changeReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unable to read ledger change: %v", err)
		}
		changes = append(changes, change)
	}
	return changes
}

func getLedgers(itest *integration.Test, startingLedger uint32, endLedger uint32) map[uint32]xdr.LedgerCloseMeta {
	t := itest.CurrentTest()

	ccConfig, err := itest.CreateCaptiveCoreConfig()
	require.NoError(t, err)

	captiveCore, err := ledgerbackend.NewCaptive(ccConfig)
	require.NoError(t, err)

	ctx := context.Background()
	err = captiveCore.PrepareRange(ctx, ledgerbackend.BoundedRange(startingLedger, endLedger))
	if err != nil {
		t.Fatalf("failed to prepare range: %v", err)
	}

	var seqToLedgersMap = make(map[uint32]xdr.LedgerCloseMeta)
	for ledgerSeq := startingLedger; ledgerSeq <= endLedger; ledgerSeq++ {
		ledger, err := captiveCore.GetLedger(ctx, ledgerSeq)
		if err != nil {
			t.Fatalf("failed to get ledgerNum: %v, error: %v", ledgerSeq, err)
		}
		seqToLedgersMap[ledgerSeq] = ledger
	}

	require.NoError(t, captiveCore.Close())
	return seqToLedgersMap
}

func changeReasonCountMap(changes []ingest.Change) map[ingest.LedgerEntryChangeReason]int {
	changeMap := make(map[ingest.LedgerEntryChangeReason]int)
	for _, change := range changes {
		changeMap[change.Reason]++
	}
	return changeMap
}

func changeReasonToChangeMap(changes []ingest.Change) map[ingest.LedgerEntryChangeReason][]ingest.Change {
	changeMap := make(map[ingest.LedgerEntryChangeReason][]ingest.Change)
	for _, change := range changes {
		changeMap[change.Reason] = append(changeMap[change.Reason], change)
	}
	return changeMap
}

func getExactUpgradedLedgerSeq(ledgerMap map[uint32]xdr.LedgerCloseMeta, version uint32) uint32 {
	keys := make([]int, 0, len(ledgerMap))
	for key := range ledgerMap {
		keys = append(keys, int(key))
	}
	sort.Ints(keys)

	for _, key := range keys {
		if ledgerMap[uint32(key)].ProtocolVersion() == version {
			return uint32(key)
		}
	}
	return 0
}

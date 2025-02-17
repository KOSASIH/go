package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/ingest/loadtest"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

func TestLoadTestLedgerBackend(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{
		NetworkPassphrase: loadTestNetworkPassphrase,
	})
	senderKP, senderAccount := itest.CreateAccount("10000000")
	recipientKP, _ := itest.CreateAccount("10000000")

	tx := itest.MustSubmitOperations(
		senderAccount,
		senderKP,
		&txnbuild.Payment{
			SourceAccount: senderKP.Address(),
			Destination:   recipientKP.Address(),
			Asset:         txnbuild.NativeAsset{},
			Amount:        "1000",
		},
	)
	require.True(t, tx.Successful)

	ccConfig, err := itest.CreateCaptiveCoreConfig()
	require.NoError(t, err)

	captiveCore, err := ledgerbackend.NewCaptive(ccConfig)
	require.NoError(t, err)

	replayConfig := loadtest.LedgerBackendConfig{
		NetworkPassphrase:     itest.Config().NetworkPassphrase,
		LedgersFilePath:       filepath.Join("testdata", "load-test-ledgers.xdr.zstd"),
		LedgerEntriesFilePath: filepath.Join("testdata", "load-test-accounts.xdr.zstd"),
		LedgerCloseDuration:   3 * time.Second / 2,
		LedgerBackend:         captiveCore,
	}
	loadTestBackend := loadtest.NewLedgerBackend(replayConfig)

	var generatedLedgers []xdr.LedgerCloseMeta
	var generatedLedgerEntries []xdr.LedgerEntry

	readFile(t, replayConfig.LedgersFilePath,
		func() *xdr.LedgerCloseMeta { return &xdr.LedgerCloseMeta{} },
		func(ledger *xdr.LedgerCloseMeta) {
			generatedLedgers = append(generatedLedgers, *ledger)
		},
	)
	readFile(t, replayConfig.LedgerEntriesFilePath,
		func() *xdr.LedgerEntry { return &xdr.LedgerEntry{} },
		func(ledgerEntry *xdr.LedgerEntry) {
			generatedLedgerEntries = append(generatedLedgerEntries, *ledgerEntry)
		},
	)

	startLedger := uint32(tx.Ledger - 1)
	endLedger := startLedger + uint32(len(generatedLedgers))

	_, err = loadTestBackend.GetLatestLedgerSequence(context.Background())
	require.EqualError(t, err, "PrepareRange() must be called before GetLatestLedgerSequence()")

	prepared, err := loadTestBackend.IsPrepared(context.Background(), ledgerbackend.BoundedRange(startLedger, endLedger))
	require.NoError(t, err)
	require.False(t, prepared)

	itest.WaitForLedgerInArchive(6*time.Minute, endLedger)
	require.NoError(t, loadTestBackend.PrepareRange(context.Background(), ledgerbackend.BoundedRange(startLedger, endLedger)))

	latest, err := loadTestBackend.GetLatestLedgerSequence(context.Background())
	require.NoError(t, err)
	require.Equal(t, endLedger, latest)

	prepared, err = loadTestBackend.IsPrepared(context.Background(), ledgerbackend.BoundedRange(startLedger, endLedger))
	require.NoError(t, err)
	require.True(t, prepared)

	prepared, err = loadTestBackend.IsPrepared(context.Background(), ledgerbackend.BoundedRange(startLedger+1, endLedger))
	require.NoError(t, err)
	require.True(t, prepared)

	prepared, err = loadTestBackend.IsPrepared(context.Background(), ledgerbackend.BoundedRange(startLedger-1, endLedger))
	require.NoError(t, err)
	require.False(t, prepared)

	prepared, err = loadTestBackend.IsPrepared(context.Background(), ledgerbackend.BoundedRange(endLedger+1, endLedger+2))
	require.NoError(t, err)
	require.False(t, prepared)

	prepared, err = loadTestBackend.IsPrepared(context.Background(), ledgerbackend.UnboundedRange(startLedger))
	require.NoError(t, err)
	require.False(t, prepared)

	require.NoError(
		t,
		loadTestBackend.PrepareRange(context.Background(), ledgerbackend.BoundedRange(startLedger, endLedger)),
	)
	require.NoError(
		t,
		loadTestBackend.PrepareRange(context.Background(), ledgerbackend.BoundedRange(startLedger+1, endLedger)),
	)
	require.EqualError(
		t,
		loadTestBackend.PrepareRange(context.Background(), ledgerbackend.UnboundedRange(startLedger)),
		"PrepareRange() already called",
	)

	_, err = loadTestBackend.GetLedger(context.Background(), startLedger-1)
	require.EqualError(t, err,
		fmt.Sprintf(
			"sequence number %v is behind the ledger stream sequence %v",
			startLedger-1,
			startLedger,
		),
	)

	var ledgers []xdr.LedgerCloseMeta
	for cur := startLedger; cur <= endLedger; cur++ {
		startTime := time.Now()
		var ledger xdr.LedgerCloseMeta
		ledger, err = loadTestBackend.GetLedger(context.Background(), cur)
		duration := time.Since(startTime)
		require.NoError(t, err)
		ledgers = append(ledgers, ledger)
		require.WithinDuration(t, startTime.Add(replayConfig.LedgerCloseDuration), startTime.Add(duration), time.Millisecond*100)
	}

	prepared, err = loadTestBackend.IsPrepared(context.Background(), ledgerbackend.BoundedRange(startLedger, endLedger))
	require.NoError(t, err)
	require.False(t, prepared)

	_, err = loadTestBackend.GetLedger(context.Background(), endLedger+1)
	require.EqualError(t, err,
		fmt.Sprintf("sequence number %v is greater than the latest ledger available", endLedger+1),
	)

	require.NoError(t, loadTestBackend.Close())

	originalLedgers := getLedgers(itest, startLedger, endLedger)

	changes := extractChanges(t, itest.Config().NetworkPassphrase, ledgers[0:1])
	expectedChanges := extractChanges(
		t, itest.Config().NetworkPassphrase, []xdr.LedgerCloseMeta{originalLedgers[startLedger]},
	)
	for i := range generatedLedgerEntries {
		expectedChanges = append(expectedChanges, ingest.Change{
			Type:   generatedLedgerEntries[i].Data.Type,
			Post:   &generatedLedgerEntries[i],
			Reason: ingest.LedgerEntryChangeReasonUpgrade,
		})
	}
	requireChangesAreEqual(t, expectedChanges, changes)

	for cur := startLedger + 1; cur <= endLedger; cur++ {
		i := int(cur - startLedger)
		changes = extractChanges(t, itest.Config().NetworkPassphrase, ledgers[i:i+1])
		expectedChanges = extractChanges(
			t, itest.Config().NetworkPassphrase, []xdr.LedgerCloseMeta{originalLedgers[cur], generatedLedgers[i-1]},
		)
		requireChangesAreEqual(t, expectedChanges, changes)
	}
}

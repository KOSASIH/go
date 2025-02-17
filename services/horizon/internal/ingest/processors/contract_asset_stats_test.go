package processors

import (
	"context"
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/xdr"
)

func getKeyHashForBalance(t *testing.T, assetContractId, holderID [32]byte) xdr.Hash {
	ledgerKey := ContractBalanceLedgerKey(assetContractId, holderID)
	bin, err := ledgerKey.MarshalBinary()
	assert.NoError(t, err)
	return sha256.Sum256(bin)
}

func TestAddContractData(t *testing.T) {
	xlmID, err := xdr.MustNewNativeAsset().ContractID("passphrase")
	assert.NoError(t, err)
	usdcIssuer := keypair.MustRandom().Address()
	usdcAsset := xdr.MustNewCreditAsset("USDC", usdcIssuer)
	usdcID, err := usdcAsset.ContractID("passphrase")
	assert.NoError(t, err)
	etherIssuer := keypair.MustRandom().Address()
	etherAsset := xdr.MustNewCreditAsset("ETHER", etherIssuer)
	etherID, err := etherAsset.ContractID("passphrase")
	assert.NoError(t, err)
	uniAsset := xdr.MustNewCreditAsset("UNI", etherIssuer)
	uniID, err := uniAsset.ContractID("passphrase")
	assert.NoError(t, err)

	set := NewContractAssetStatSet(
		&history.MockQAssetStats{},
		"passphrase",
		map[xdr.Hash]uint32{},
		map[xdr.Hash]uint32{},
		map[xdr.Hash][2]uint32{},
		150,
	)

	xlmContractData, err := AssetToContractData(true, "", "", xlmID)
	assert.NoError(t, err)
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: xlmContractData,
		},
	})
	assert.NoError(t, err)

	xlmBalanceKeyHash := getKeyHashForBalance(t, xlmID, [32]byte{})
	assert.NoError(t, err)
	set.createdExpirationEntries[xlmBalanceKeyHash] = 150
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(xlmID, [32]byte{}, 100),
		},
	})
	assert.NoError(t, err)

	uniBalanceKeyHash := getKeyHashForBalance(t, uniID, [32]byte{})
	set.createdExpirationEntries[uniBalanceKeyHash] = 150
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(uniID, [32]byte{}, 0),
		},
	})
	assert.NoError(t, err)

	usdcContractData, err := AssetToContractData(false, "USDC", usdcIssuer, usdcID)
	assert.NoError(t, err)
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: usdcContractData,
		},
	})
	assert.NoError(t, err)

	etherContractData, err := AssetToContractData(false, "ETHER", etherIssuer, etherID)
	assert.NoError(t, err)
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: etherContractData,
		},
	})
	assert.NoError(t, err)

	etherBalanceKeyHash := getKeyHashForBalance(t, etherID, [32]byte{})
	set.createdExpirationEntries[etherBalanceKeyHash] = 100
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(etherID, [32]byte{}, 50),
		},
	})
	assert.NoError(t, err)

	otherEtherBalanceKeyHash := getKeyHashForBalance(t, etherID, [32]byte{1})
	set.createdExpirationEntries[otherEtherBalanceKeyHash] = 150
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(etherID, [32]byte{1}, 150),
		},
	})
	assert.NoError(t, err)

	// negative balances will be ignored
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: balanceToContractData(etherID, [32]byte{1}, xdr.Int128Parts{Hi: -1, Lo: 0}),
		},
	})
	assert.NoError(t, err)

	btcAsset := xdr.MustNewCreditAsset("BTC", etherIssuer)
	btcID, err := btcAsset.ContractID("passphrase")
	assert.NoError(t, err)

	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(btcID, [32]byte{2}, 300),
		},
	})
	assert.NoError(t, err)

	assert.Empty(t, set.updatedBalances)
	assert.Empty(t, set.removedBalances)
	assert.Len(t, set.contractToAsset, 2)
	assert.True(t, set.contractToAsset[usdcID].Equals(usdcAsset))
	assert.True(t, set.contractToAsset[etherID].Equals(etherAsset))
	assert.Equal(t, []history.ContractAssetBalance{
		{
			KeyHash:          uniBalanceKeyHash[:],
			ContractID:       uniID[:],
			Amount:           "0",
			ExpirationLedger: 150,
		},
		{
			KeyHash:          etherBalanceKeyHash[:],
			ContractID:       etherID[:],
			Amount:           "50",
			ExpirationLedger: 100,
		},
		{
			KeyHash:          otherEtherBalanceKeyHash[:],
			ContractID:       etherID[:],
			Amount:           "150",
			ExpirationLedger: 150,
		},
	}, set.createdBalances)
	assert.ElementsMatch(t, set.GetContractStats(), []history.ContractAssetStatRow{
		{
			ContractID: uniID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "0",
				ArchivedBalance: "0",
				ActiveHolders:   1,
				ArchivedHolders: 0,
			},
		},
		{
			ContractID: etherID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "150",
				ArchivedBalance: "50",
				ActiveHolders:   1,
				ArchivedHolders: 1,
			},
		},
	})
}

func TestUpdateContractBalance(t *testing.T) {
	usdcIssuer := keypair.MustRandom().Address()
	usdcAsset := xdr.MustNewCreditAsset("USDC", usdcIssuer)
	usdcID, err := usdcAsset.ContractID("passphrase")
	assert.NoError(t, err)
	etherIssuer := keypair.MustRandom().Address()
	etherAsset := xdr.MustNewCreditAsset("ETHER", etherIssuer)
	etherID, err := etherAsset.ContractID("passphrase")
	assert.NoError(t, err)
	btcAsset := xdr.MustNewCreditAsset("BTC", etherIssuer)
	btcID, err := btcAsset.ContractID("passphrase")
	assert.NoError(t, err)
	uniAsset := xdr.MustNewCreditAsset("UNI", etherIssuer)
	uniID, err := uniAsset.ContractID("passphrase")
	assert.NoError(t, err)

	mockQ := &history.MockQAssetStats{}
	set := NewContractAssetStatSet(
		mockQ,
		"passphrase",
		map[xdr.Hash]uint32{},
		map[xdr.Hash]uint32{},
		map[xdr.Hash][2]uint32{},
		150,
	)
	expectedBalances := map[xdr.Hash]string{}

	keyHash := getKeyHashForBalance(t, usdcID, [32]byte{})
	set.updatedExpirationEntries[keyHash] = [2]uint32{160, 170}
	expectedBalances[keyHash] = "100"
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{}, 50),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{}, 100),
		},
	})
	assert.NoError(t, err)

	keyHash = getKeyHashForBalance(t, usdcID, [32]byte{2})
	ctx := context.Background()
	mockQ.On("GetContractAssetBalances", ctx, []xdr.Hash{keyHash}).
		Return([]history.ContractAssetBalance{
			{
				KeyHash:          keyHash[:],
				ContractID:       usdcID[:],
				Amount:           "30",
				ExpirationLedger: 180,
			},
		}, nil).Once()
	expectedBalances[keyHash] = "100"
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{2}, 30),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{2}, 100),
		},
	})
	assert.NoError(t, err)

	keyHash = getKeyHashForBalance(t, usdcID, [32]byte{4})
	// balances which don't exist in the db will be ignored
	mockQ.On("GetContractAssetBalances", ctx, []xdr.Hash{keyHash}).
		Return([]history.ContractAssetBalance{}, nil).Once()
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{4}, 0),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{4}, 100),
		},
	})
	assert.NoError(t, err)

	keyHash = getKeyHashForBalance(t, etherID, [32]byte{})
	mockQ.On("GetContractAssetBalances", ctx, []xdr.Hash{keyHash}).
		Return([]history.ContractAssetBalance{
			{
				KeyHash:          keyHash[:],
				ContractID:       etherID[:],
				Amount:           "200",
				ExpirationLedger: 200,
			},
		}, nil).Once()
	expectedBalances[keyHash] = "50"
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(etherID, [32]byte{}, 200),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(etherID, [32]byte{}, 50),
		},
	})
	assert.NoError(t, err)

	// negative balances will be ignored
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(etherID, [32]byte{}, 200),
		},
		Post: &xdr.LedgerEntry{
			Data: balanceToContractData(etherID, [32]byte{1}, xdr.Int128Parts{Hi: -1, Lo: 0}),
		},
	})
	assert.NoError(t, err)

	// negative balances will be ignored
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: balanceToContractData(etherID, [32]byte{1}, xdr.Int128Parts{Hi: -1, Lo: 0}),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(etherID, [32]byte{}, 200),
		},
	})
	assert.NoError(t, err)

	// balances where the amount doesn't change will be ignored
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(btcID, [32]byte{2}, 300),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(btcID, [32]byte{2}, 300),
		},
	})
	assert.NoError(t, err)

	keyHash = getKeyHashForBalance(t, btcID, [32]byte{5})
	mockQ.On("GetContractAssetBalances", ctx, []xdr.Hash{keyHash}).
		Return([]history.ContractAssetBalance{
			{
				KeyHash:          keyHash[:],
				ContractID:       btcID[:],
				Amount:           "10",
				ExpirationLedger: 20,
			},
		}, nil).Once()
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(btcID, [32]byte{5}, 10),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(btcID, [32]byte{5}, 15),
		},
	})
	assert.ErrorContains(t, err, "contract balance has invalid expiration ledger keyhash")

	keyHash = getKeyHashForBalance(t, btcID, [32]byte{6})
	set.updatedExpirationEntries[keyHash] = [2]uint32{100, 110}
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(btcID, [32]byte{6}, 120),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(btcID, [32]byte{6}, 135),
		},
	})
	assert.ErrorContains(t, err, "contract balance has invalid expiration ledger keyhash")

	keyHash = getKeyHashForBalance(t, uniID, [32]byte{4})
	set.updatedExpirationEntries[keyHash] = [2]uint32{100, 170}
	expectedBalances[keyHash] = "75"
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(uniID, [32]byte{4}, 50),
		},
		Post: &xdr.LedgerEntry{
			Data: BalanceToContractData(uniID, [32]byte{4}, 75),
		},
	})
	assert.NoError(t, err)

	assert.Empty(t, set.contractToAsset)
	assert.Empty(t, set.removedBalances)
	assert.Empty(t, set.createdExpirationEntries)
	for key, amt := range set.updatedBalances {
		assert.Equal(t, expectedBalances[key], amt.String())
		delete(expectedBalances, key)
	}
	assert.Empty(t, expectedBalances)

	assert.ElementsMatch(t, set.GetContractStats(), []history.ContractAssetStatRow{
		{
			ContractID: usdcID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "120",
				ActiveHolders:   0,
				ArchivedBalance: "0",
				ArchivedHolders: 0,
			},
		},
		{
			ContractID: etherID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "-150",
				ActiveHolders:   0,
				ArchivedBalance: "0",
				ArchivedHolders: 0,
			},
		},
		{
			ContractID: uniID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "75",
				ActiveHolders:   1,
				ArchivedBalance: "-50",
				ArchivedHolders: -1,
			},
		},
	})

	mockQ.AssertExpectations(t)
}

func TestRemoveContractData(t *testing.T) {
	usdcIssuer := keypair.MustRandom().Address()
	usdcAsset := xdr.MustNewCreditAsset("USDC", usdcIssuer)
	usdcID, err := usdcAsset.ContractID("passphrase")
	assert.NoError(t, err)

	set := NewContractAssetStatSet(
		&history.MockQAssetStats{},
		"passphrase",
		map[xdr.Hash]uint32{},
		map[xdr.Hash]uint32{},
		map[xdr.Hash][2]uint32{},
		150,
	)

	usdcContractData, err := AssetToContractData(false, "USDC", usdcIssuer, usdcID)
	assert.NoError(t, err)
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: usdcContractData,
		},
	})
	assert.NoError(t, err)

	keyHash := getKeyHashForBalance(t, usdcID, [32]byte{})
	set.removedExpirationEntries[keyHash] = 170
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{}, 50),
		},
	})
	assert.NoError(t, err)

	keyHash1 := getKeyHashForBalance(t, usdcID, [32]byte{1})
	set.removedExpirationEntries[keyHash1] = 100
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{1}, 20),
		},
	})
	assert.NoError(t, err)

	keyHash2 := getKeyHashForBalance(t, usdcID, [32]byte{2})
	err = set.AddContractData(context.Background(), ingest.Change{
		Type: xdr.LedgerEntryTypeContractData,
		Pre: &xdr.LedgerEntry{
			Data: BalanceToContractData(usdcID, [32]byte{2}, 34),
		},
	})
	assert.NoError(t, err)

	assert.Equal(t, []xdr.Hash{keyHash, keyHash1, keyHash2}, set.removedBalances)
	assert.Len(t, set.contractToAsset, 1)
	asset, ok := set.contractToAsset[usdcID]
	assert.Nil(t, asset)
	assert.True(t, ok)

	assert.ElementsMatch(t, set.GetContractStats(), []history.ContractAssetStatRow{
		{
			ContractID: usdcID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "-50",
				ActiveHolders:   -1,
				ArchivedBalance: "-20",
				ArchivedHolders: -1,
			},
		},
	})
}

func TestIngestRestoredBalances(t *testing.T) {
	usdcIssuer := keypair.MustRandom().Address()
	usdcAsset := xdr.MustNewCreditAsset("USDC", usdcIssuer)
	usdcID, err := usdcAsset.ContractID("passphrase")
	assert.NoError(t, err)

	mockQ := &history.MockQAssetStats{}
	set := NewContractAssetStatSet(
		mockQ,
		"passphrase",
		map[xdr.Hash]uint32{},
		map[xdr.Hash]uint32{},
		map[xdr.Hash][2]uint32{},
		150,
	)

	usdcKeyHash := getKeyHashForBalance(t, usdcID, [32]byte{})
	set.updatedBalances[usdcKeyHash] = big.NewInt(190)
	set.updatedExpirationEntries[usdcKeyHash] = [2]uint32{120, 170}

	usdcKeyHash1 := getKeyHashForBalance(t, usdcID, [32]byte{1})
	set.updatedExpirationEntries[usdcKeyHash1] = [2]uint32{149, 190}

	usdcKeyHash2 := getKeyHashForBalance(t, usdcID, [32]byte{2})
	set.updatedExpirationEntries[usdcKeyHash2] = [2]uint32{100, 200}

	usdcKeyHash3 := getKeyHashForBalance(t, usdcID, [32]byte{3})
	set.updatedExpirationEntries[usdcKeyHash3] = [2]uint32{150, 210}

	usdcKeyHash4 := getKeyHashForBalance(t, usdcID, [32]byte{4})
	set.updatedExpirationEntries[usdcKeyHash4] = [2]uint32{170, 900}

	usdcKeyHash5 := getKeyHashForBalance(t, usdcID, [32]byte{5})
	set.updatedExpirationEntries[usdcKeyHash5] = [2]uint32{120, 600}

	ctx := context.Background()

	mockQ.On("GetContractAssetBalances", ctx, mock.MatchedBy(func(keys []xdr.Hash) bool {
		return assert.ElementsMatch(t, []xdr.Hash{usdcKeyHash2, usdcKeyHash5}, keys)
	})).
		Return([]history.ContractAssetBalance{
			{
				KeyHash:          usdcKeyHash2[:],
				ContractID:       usdcID[:],
				Amount:           "67",
				ExpirationLedger: 100,
			},
			{
				KeyHash:          usdcKeyHash5[:],
				ContractID:       usdcID[:],
				Amount:           "200",
				ExpirationLedger: 120,
			},
		}, nil).Once()

	assert.NoError(t, set.ingestRestoredBalances(ctx))
	assert.ElementsMatch(t, set.GetContractStats(), []history.ContractAssetStatRow{
		{
			ContractID: usdcID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "267",
				ActiveHolders:   2,
				ArchivedBalance: "-267",
				ArchivedHolders: -2,
			},
		},
	})

	mockQ.AssertExpectations(t)
}

func TestIngestExpiredBalances(t *testing.T) {
	usdcIssuer := keypair.MustRandom().Address()
	usdcAsset := xdr.MustNewCreditAsset("USDC", usdcIssuer)
	usdcID, err := usdcAsset.ContractID("passphrase")
	assert.NoError(t, err)

	etherIssuer := keypair.MustRandom().Address()
	etherAsset := xdr.MustNewCreditAsset("ETHER", etherIssuer)
	etherID, err := etherAsset.ContractID("passphrase")
	assert.NoError(t, err)

	mockQ := &history.MockQAssetStats{}
	set := NewContractAssetStatSet(
		mockQ,
		"passphrase",
		map[xdr.Hash]uint32{},
		map[xdr.Hash]uint32{},
		map[xdr.Hash][2]uint32{},
		150,
	)

	usdcKeyHash := getKeyHashForBalance(t, usdcID, [32]byte{})
	usdcKeyHash1 := getKeyHashForBalance(t, usdcID, [32]byte{1})
	ethKeyHash := getKeyHashForBalance(t, etherID, [32]byte{})
	ethKeyHash1 := getKeyHashForBalance(t, etherID, [32]byte{1})
	set.updatedExpirationEntries[ethKeyHash1] = [2]uint32{149, 180}
	ctx := context.Background()
	mockQ.On("GetContractAssetBalancesExpiringAt", ctx, set.currentLedger-1).
		Return([]history.ContractAssetBalance{
			{
				KeyHash:          usdcKeyHash[:],
				ContractID:       usdcID[:],
				Amount:           "67",
				ExpirationLedger: 149,
			},
			{
				KeyHash:          usdcKeyHash1[:],
				ContractID:       usdcID[:],
				Amount:           "200",
				ExpirationLedger: 149,
			},
			{
				KeyHash:          ethKeyHash[:],
				ContractID:       etherID[:],
				Amount:           "8",
				ExpirationLedger: 149,
			},
			{
				KeyHash:          ethKeyHash1[:],
				ContractID:       etherID[:],
				Amount:           "67",
				ExpirationLedger: 149,
			},
		}, nil).Once()

	assert.NoError(t, set.ingestExpiredBalances(ctx))
	assert.ElementsMatch(t, set.GetContractStats(), []history.ContractAssetStatRow{
		{
			ContractID: usdcID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "-267",
				ActiveHolders:   -2,
				ArchivedBalance: "267",
				ArchivedHolders: 2,
			},
		},
		{
			ContractID: etherID[:],
			Stat: history.ContractStat{
				ActiveBalance:   "-8",
				ActiveHolders:   -1,
				ArchivedBalance: "8",
				ArchivedHolders: 1,
			},
		},
	})
	mockQ.AssertExpectations(t)
}

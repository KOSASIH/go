package history

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/xdr"
)

// MockQAssetStats is a mock implementation of the QAssetStats interface
type MockQAssetStats struct {
	mock.Mock
}

func (m *MockQAssetStats) InsertContractAssetBalances(ctx context.Context, rows []ContractAssetBalance) error {
	a := m.Called(ctx, rows)
	return a.Error(0)
}

func (m *MockQAssetStats) RemoveContractAssetBalances(ctx context.Context, keys []xdr.Hash) error {
	a := m.Called(ctx, keys)
	return a.Error(0)
}

func (m *MockQAssetStats) UpdateContractAssetBalanceAmounts(ctx context.Context, keys []xdr.Hash, amounts []string) error {
	a := m.Called(ctx, keys, amounts)
	return a.Error(0)
}

func (m *MockQAssetStats) UpdateContractAssetBalanceExpirations(ctx context.Context, keys []xdr.Hash, expirationLedgers []uint32) error {
	a := m.Called(ctx, keys, expirationLedgers)
	return a.Error(0)
}

func (m *MockQAssetStats) GetContractAssetBalances(ctx context.Context, keys []xdr.Hash) ([]ContractAssetBalance, error) {
	a := m.Called(ctx, keys)
	return a.Get(0).([]ContractAssetBalance), a.Error(1)
}

func (m *MockQAssetStats) GetContractAssetBalancesExpiringAt(ctx context.Context, ledger uint32) ([]ContractAssetBalance, error) {
	a := m.Called(ctx, ledger)
	return a.Get(0).([]ContractAssetBalance), a.Error(1)
}

func (m *MockQAssetStats) InsertContractAssetStats(ctx context.Context, rows []ContractAssetStatRow) error {
	a := m.Called(ctx, rows)
	return a.Error(0)
}

func (m *MockQAssetStats) InsertContractAssetStat(ctx context.Context, row ContractAssetStatRow) (int64, error) {
	a := m.Called(ctx, row)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockQAssetStats) UpdateContractAssetStat(ctx context.Context, row ContractAssetStatRow) (int64, error) {
	a := m.Called(ctx, row)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockQAssetStats) GetContractAssetStat(ctx context.Context, contractID []byte) (ContractAssetStatRow, error) {
	a := m.Called(ctx, contractID)
	return a.Get(0).(ContractAssetStatRow), a.Error(1)
}

func (m *MockQAssetStats) RemoveAssetContractStat(ctx context.Context, contractID []byte) (int64, error) {
	a := m.Called(ctx, contractID)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockQAssetStats) InsertAssetStats(ctx context.Context, assetStats []ExpAssetStat) error {
	a := m.Called(ctx, assetStats)
	return a.Error(0)
}

func (m *MockQAssetStats) InsertAssetStat(ctx context.Context, assetStat ExpAssetStat) (int64, error) {
	a := m.Called(ctx, assetStat)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockQAssetStats) UpdateAssetStat(ctx context.Context, assetStat ExpAssetStat) (int64, error) {
	a := m.Called(ctx, assetStat)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockQAssetStats) GetAssetStat(ctx context.Context, assetType xdr.AssetType, assetCode, assetIssuer string) (ExpAssetStat, error) {
	a := m.Called(ctx, assetType, assetCode, assetIssuer)
	return a.Get(0).(ExpAssetStat), a.Error(1)
}

func (m *MockQAssetStats) GetAssetStatByContract(ctx context.Context, contractID xdr.Hash) (ExpAssetStat, error) {
	a := m.Called(ctx, contractID)
	return a.Get(0).(ExpAssetStat), a.Error(1)
}

func (m *MockQAssetStats) RemoveAssetStat(ctx context.Context, assetType xdr.AssetType, assetCode, assetIssuer string) (int64, error) {
	a := m.Called(ctx, assetType, assetCode, assetIssuer)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockQAssetStats) GetAssetStats(ctx context.Context, assetCode, assetIssuer string, page db2.PageQuery) ([]AssetAndContractStat, error) {
	a := m.Called(ctx, assetCode, assetIssuer, page)
	return a.Get(0).([]AssetAndContractStat), a.Error(1)
}

func (m *MockQAssetStats) GetAssetStatByContracts(ctx context.Context, contractIDs []xdr.Hash) ([]ExpAssetStat, error) {
	a := m.Called(ctx, contractIDs)
	return a.Get(0).([]ExpAssetStat), a.Error(1)
}

func (m *MockQAssetStats) CountTrustLines(ctx context.Context) (int, error) {
	a := m.Called(ctx)
	return a.Get(0).(int), a.Error(1)
}

func (m *MockQAssetStats) CountContractIDs(ctx context.Context) (int, error) {
	a := m.Called(ctx)
	return a.Get(0).(int), a.Error(1)
}

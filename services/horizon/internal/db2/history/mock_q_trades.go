package history

import (
	"context"

	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"

	"github.com/stretchr/testify/mock"
)

type MockQTrades struct {
	mock.Mock
}

func (m *MockQTrades) CreateAccounts(ctx context.Context, addresses []string, maxBatchSize int) (map[string]int64, error) {
	a := m.Called(ctx, addresses, maxBatchSize)
	return a.Get(0).(map[string]int64), a.Error(1)
}

func (m *MockQTrades) CreateAssets(ctx context.Context, assets []xdr.Asset, maxBatchSize int) (map[string]Asset, error) {
	a := m.Called(ctx, assets, maxBatchSize)
	return a.Get(0).(map[string]Asset), a.Error(1)
}

func (m *MockQTrades) CreateHistoryLiquidityPools(ctx context.Context, poolIDs []string, maxBatchSize int) (map[string]int64, error) {
	a := m.Called(ctx, poolIDs, maxBatchSize)
	return a.Get(0).(map[string]int64), a.Error(1)
}

func (m *MockQTrades) NewTradeBatchInsertBuilder() TradeBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(TradeBatchInsertBuilder)
}

func (m *MockQTrades) RebuildTradeAggregationBuckets(ctx context.Context, fromLedger, toLedger uint32, roundingSlippageFilter int) error {
	a := m.Called(ctx, fromLedger, toLedger, roundingSlippageFilter)
	return a.Error(0)
}

type MockTradeBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockTradeBatchInsertBuilder) Add(entries ...InsertTrade) error {
	a := m.Called(entries)
	return a.Error(0)
}

func (m *MockTradeBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	a := m.Called(ctx, session)
	return a.Error(0)
}

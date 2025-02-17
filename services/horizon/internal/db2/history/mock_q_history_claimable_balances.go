package history

import (
	"context"

	"github.com/stellar/go/support/db"

	"github.com/stretchr/testify/mock"
)

// MockQHistoryClaimableBalances is a mock implementation of the QClaimableBalances interface
type MockQHistoryClaimableBalances struct {
	mock.Mock
}

func (m *MockQHistoryClaimableBalances) CreateHistoryClaimableBalances(ctx context.Context, ids []string, maxBatchSize int) (map[string]int64, error) {
	a := m.Called(ctx, ids, maxBatchSize)
	return a.Get(0).(map[string]int64), a.Error(1)
}

func (m *MockQHistoryClaimableBalances) NewTransactionClaimableBalanceBatchInsertBuilder() TransactionClaimableBalanceBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(TransactionClaimableBalanceBatchInsertBuilder)
}

// MockTransactionClaimableBalanceBatchInsertBuilder is a mock implementation of the
// TransactionClaimableBalanceBatchInsertBuilder interface
type MockTransactionClaimableBalanceBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockTransactionClaimableBalanceBatchInsertBuilder) Add(transactionID int64, claimableBalance FutureClaimableBalanceID) error {
	a := m.Called(transactionID, claimableBalance)
	return a.Error(0)
}

func (m *MockTransactionClaimableBalanceBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	a := m.Called(ctx, session)
	return a.Error(0)
}

// NewOperationClaimableBalanceBatchInsertBuilder mock
func (m *MockQHistoryClaimableBalances) NewOperationClaimableBalanceBatchInsertBuilder() OperationClaimableBalanceBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(OperationClaimableBalanceBatchInsertBuilder)
}

// MockOperationClaimableBalanceBatchInsertBuilder is a mock implementation of the
// OperationClaimableBalanceBatchInsertBuilder interface
type MockOperationClaimableBalanceBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockOperationClaimableBalanceBatchInsertBuilder) Add(operationID int64, claimableBalance FutureClaimableBalanceID) error {
	a := m.Called(operationID, claimableBalance)
	return a.Error(0)
}

func (m *MockOperationClaimableBalanceBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	a := m.Called(ctx, session)
	return a.Error(0)
}

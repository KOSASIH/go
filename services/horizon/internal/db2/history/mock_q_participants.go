package history

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/stellar/go/support/db"
)

// MockQParticipants is a mock implementation of the QParticipants interface
type MockQParticipants struct {
	mock.Mock
}

func (m *MockQParticipants) CreateAccounts(ctx context.Context, addresses []string, maxBatchSize int) (map[string]int64, error) {
	a := m.Called(ctx, addresses, maxBatchSize)
	return a.Get(0).(map[string]int64), a.Error(1)
}

func (m *MockQParticipants) NewTransactionParticipantsBatchInsertBuilder() TransactionParticipantsBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(TransactionParticipantsBatchInsertBuilder)
}

// MockTransactionParticipantsBatchInsertBuilder is a mock implementation of the
// TransactionParticipantsBatchInsertBuilder interface
type MockTransactionParticipantsBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockTransactionParticipantsBatchInsertBuilder) Add(transactionID int64, accountID FutureAccountID) error {
	a := m.Called(transactionID, accountID)
	return a.Error(0)
}

func (m *MockTransactionParticipantsBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	a := m.Called(ctx, session)
	return a.Error(0)
}

// NewOperationParticipantBatchInsertBuilder mock
func (m *MockQParticipants) NewOperationParticipantBatchInsertBuilder() OperationParticipantBatchInsertBuilder {
	a := m.Called()
	v := a.Get(0)
	return v.(OperationParticipantBatchInsertBuilder)
}

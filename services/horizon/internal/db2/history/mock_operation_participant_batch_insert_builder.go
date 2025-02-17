package history

import (
	"context"

	"github.com/stellar/go/support/db"

	"github.com/stretchr/testify/mock"
)

// MockOperationParticipantBatchInsertBuilder OperationParticipantBatchInsertBuilder mock
type MockOperationParticipantBatchInsertBuilder struct {
	mock.Mock
}

// Add mock
func (m *MockOperationParticipantBatchInsertBuilder) Add(operationID int64, account FutureAccountID) error {
	a := m.Called(operationID, account)
	return a.Error(0)
}

// Exec mock
func (m *MockOperationParticipantBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	a := m.Called(ctx, session)
	return a.Error(0)
}

package history

import (
	"context"

	"github.com/guregu/null"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/mock"
)

// MockOperationsBatchInsertBuilder OperationsBatchInsertBuilder mock
type MockOperationsBatchInsertBuilder struct {
	mock.Mock
}

// Add mock
func (m *MockOperationsBatchInsertBuilder) Add(
	id int64,
	transactionID int64,
	applicationOrder uint32,
	operationType xdr.OperationType,
	details []byte,
	sourceAccount string,
	sourceAccountMuxed null.String,
	isPayment bool,
) error {
	a := m.Called(
		id,
		transactionID,
		applicationOrder,
		operationType,
		details,
		sourceAccount,
		sourceAccountMuxed,
		isPayment,
	)
	return a.Error(0)
}

// Exec mock
func (m *MockOperationsBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	a := m.Called(ctx, session)
	return a.Error(0)
}

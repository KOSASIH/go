package history

import (
	"context"

	"github.com/stellar/go/support/db"

	"github.com/guregu/null"
	"github.com/stretchr/testify/mock"
)

// MockEffectBatchInsertBuilder mock EffectBatchInsertBuilder
type MockEffectBatchInsertBuilder struct {
	mock.Mock
}

// Add mock
func (m *MockEffectBatchInsertBuilder) Add(
	accountID FutureAccountID,
	muxedAccount null.String,
	operationID int64,
	order uint32,
	effectType EffectType,
	details []byte,
) error {
	a := m.Called(
		accountID,
		muxedAccount,
		operationID,
		order,
		effectType,
		details,
	)
	return a.Error(0)
}

// Exec mock
func (m *MockEffectBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	a := m.Called(ctx, session)
	return a.Error(0)
}

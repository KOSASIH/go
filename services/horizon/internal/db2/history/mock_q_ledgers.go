package history

import (
	"context"

	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"

	"github.com/stretchr/testify/mock"
)

type MockQLedgers struct {
	mock.Mock
}

func (m *MockQLedgers) NewLedgerBatchInsertBuilder() LedgerBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(LedgerBatchInsertBuilder)
}

type MockLedgersBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockLedgersBatchInsertBuilder) Add(
	ledger xdr.LedgerHeaderHistoryEntry,
	successTxsCount int,
	failedTxsCount int,
	opCount int,
	txSetOpCount int,
	ingestVersion int,
) error {
	a := m.Called(ledger, successTxsCount, failedTxsCount, opCount, txSetOpCount, ingestVersion)
	return a.Error(0)
}

func (m *MockLedgersBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	a := m.Called(ctx, session)
	return a.Error(0)
}

package history

import "github.com/stretchr/testify/mock"

// MockQTransactions is a mock implementation of the QTransactions interface
type MockQTransactions struct {
	mock.Mock
}

func (m *MockQTransactions) NewTransactionBatchInsertBuilder() TransactionBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(TransactionBatchInsertBuilder)
}

func (m *MockQTransactions) NewTransactionFilteredTmpBatchInsertBuilder() TransactionBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(TransactionBatchInsertBuilder)
}

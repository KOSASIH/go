package history

import "github.com/stretchr/testify/mock"

// MockQOperations is a mock implementation of the QOperations interface
type MockQOperations struct {
	mock.Mock
}

// NewOperationBatchInsertBuilder mock
func (m *MockQOperations) NewOperationBatchInsertBuilder() OperationBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(OperationBatchInsertBuilder)
}

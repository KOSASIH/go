package history

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockAccountDataBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockAccountDataBatchInsertBuilder) Add(data Data) error {
	a := m.Called(data)
	return a.Error(0)
}

func (m *MockAccountDataBatchInsertBuilder) Exec(ctx context.Context) error {
	a := m.Called(ctx)
	return a.Error(0)
}

func (m *MockAccountDataBatchInsertBuilder) Len() int {
	a := m.Called()
	return a.Int(0)
}

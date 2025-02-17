package history

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockAccountsBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockAccountsBatchInsertBuilder) Add(account AccountEntry) error {
	a := m.Called(account)
	return a.Error(0)
}

func (m *MockAccountsBatchInsertBuilder) Exec(ctx context.Context) error {
	a := m.Called(ctx)
	return a.Error(0)
}

func (m *MockAccountsBatchInsertBuilder) Len() int {
	a := m.Called()
	return a.Int(0)
}

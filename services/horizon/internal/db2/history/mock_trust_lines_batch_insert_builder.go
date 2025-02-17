package history

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockTrustLinesBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockTrustLinesBatchInsertBuilder) Add(line TrustLine) error {
	a := m.Called(line)
	return a.Error(0)
}

func (m *MockTrustLinesBatchInsertBuilder) Exec(ctx context.Context) error {
	a := m.Called(ctx)
	return a.Error(0)
}

func (m *MockTrustLinesBatchInsertBuilder) Len() int {
	a := m.Called()
	return a.Int(0)
}

package history

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockOffersBatchInsertBuilder struct {
	mock.Mock
}

func (m *MockOffersBatchInsertBuilder) Add(offer Offer) error {
	a := m.Called(offer)
	return a.Error(0)
}

func (m *MockOffersBatchInsertBuilder) Exec(ctx context.Context) error {
	a := m.Called(ctx)
	return a.Error(0)
}

func (m *MockOffersBatchInsertBuilder) Len() int {
	a := m.Called()
	return a.Int(0)
}

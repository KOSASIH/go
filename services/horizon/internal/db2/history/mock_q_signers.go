package history

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/stellar/go/services/horizon/internal/db2"
)

type MockQSigners struct {
	mock.Mock
}

func (m *MockQSigners) GetLastLedgerIngestNonBlocking(ctx context.Context) (uint32, error) {
	a := m.Called(ctx)
	return a.Get(0).(uint32), a.Error(1)
}

func (m *MockQSigners) GetLastLedgerIngest(ctx context.Context) (uint32, error) {
	a := m.Called(ctx)
	return a.Get(0).(uint32), a.Error(1)
}

func (m *MockQSigners) UpdateLastLedgerIngest(ctx context.Context, ledgerSequence uint32) error {
	a := m.Called(ctx, ledgerSequence)
	return a.Error(0)
}

func (m *MockQSigners) AccountsForSigner(ctx context.Context, signer string, page db2.PageQuery) ([]AccountSigner, error) {
	a := m.Called(ctx, signer, page)
	return a.Get(0).([]AccountSigner), a.Error(1)
}

func (m *MockQSigners) NewAccountSignersBatchInsertBuilder() AccountSignersBatchInsertBuilder {
	a := m.Called()
	return a.Get(0).(AccountSignersBatchInsertBuilder)
}

func (m *MockQSigners) CreateAccountSigner(ctx context.Context, account, signer string, weight int32, sponsor *string) (int64, error) {
	a := m.Called(ctx, account, signer, weight, sponsor)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockQSigners) RemoveAccountSigners(ctx context.Context, account string, signers []string) (int64, error) {
	a := m.Called(ctx, account, signers)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockQSigners) SignersForAccounts(ctx context.Context, accounts []string) ([]AccountSigner, error) {
	a := m.Called(ctx, accounts)
	return a.Get(0).([]AccountSigner), a.Error(1)
}

func (m *MockQSigners) CountAccounts(ctx context.Context) (int, error) {
	a := m.Called(ctx)
	return a.Get(0).(int), a.Error(1)
}

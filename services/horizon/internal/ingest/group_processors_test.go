//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
)

var _ horizonChangeProcessor = (*mockHorizonChangeProcessor)(nil)

type mockHorizonChangeProcessor struct {
	mock.Mock
}

func (m *mockHorizonChangeProcessor) Name() string {
	return "mockHorizonChangeProcessor"
}

func (m *mockHorizonChangeProcessor) ProcessChange(ctx context.Context, change ingest.Change) error {
	args := m.Called(ctx, change)
	return args.Error(0)
}

func (m *mockHorizonChangeProcessor) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

var _ horizonTransactionProcessor = (*mockHorizonTransactionProcessor)(nil)

type mockHorizonTransactionProcessor struct {
	mock.Mock
}

func (m *mockHorizonTransactionProcessor) Name() string {
	return "mockHorizonTransactionProcessor"
}

func (m *mockHorizonTransactionProcessor) ProcessTransaction(lcm xdr.LedgerCloseMeta, transaction ingest.LedgerTransaction) error {
	args := m.Called(lcm, transaction)
	return args.Error(0)
}

func (m *mockHorizonTransactionProcessor) Flush(ctx context.Context, session db.SessionInterface) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

type GroupChangeProcessorsTestSuiteLedger struct {
	suite.Suite
	ctx        context.Context
	processors *groupChangeProcessors
	processorA *mockHorizonChangeProcessor
	processorB *mockHorizonChangeProcessor
}

func TestGroupChangeProcessorsTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(GroupChangeProcessorsTestSuiteLedger))
}

func (s *GroupChangeProcessorsTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.processorA = &mockHorizonChangeProcessor{}
	s.processorB = &mockHorizonChangeProcessor{}
	s.processors = newGroupChangeProcessors([]horizonChangeProcessor{
		s.processorA,
		s.processorB,
	})
}

func (s *GroupChangeProcessorsTestSuiteLedger) TearDownTest() {
	s.processorA.AssertExpectations(s.T())
	s.processorB.AssertExpectations(s.T())
}

func (s *GroupChangeProcessorsTestSuiteLedger) TestProcessChangeFails() {
	change := ingest.Change{}
	s.processorA.
		On("ProcessChange", s.ctx, change).
		Return(errors.New("transient error")).Once()

	err := s.processors.ProcessChange(s.ctx, change)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error in *ingest.mockHorizonChangeProcessor.ProcessChange: transient error")
}

func (s *GroupChangeProcessorsTestSuiteLedger) TestProcessChangeSucceeds() {
	change := ingest.Change{}
	s.processorA.
		On("ProcessChange", s.ctx, change).
		Return(nil).Once()
	s.processorB.
		On("ProcessChange", s.ctx, change).
		Return(nil).Once()

	err := s.processors.ProcessChange(s.ctx, change)
	s.Assert().NoError(err)
}

func (s *GroupChangeProcessorsTestSuiteLedger) TestCommitFails() {
	s.processorA.
		On("Commit", s.ctx).
		Return(errors.New("transient error")).Once()

	err := s.processors.Commit(s.ctx)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error in *ingest.mockHorizonChangeProcessor.Commit: transient error")
}

func (s *GroupChangeProcessorsTestSuiteLedger) TestCommitSucceeds() {
	s.processorA.
		On("Commit", s.ctx).
		Return(nil).Once()
	s.processorB.
		On("Commit", s.ctx).
		Return(nil).Once()

	err := s.processors.Commit(s.ctx)
	s.Assert().NoError(err)
}

type GroupTransactionProcessorsTestSuiteLedger struct {
	suite.Suite
	ctx        context.Context
	processors *groupTransactionProcessors
	processorA *mockHorizonTransactionProcessor
	processorB *mockHorizonTransactionProcessor
	session    db.SessionInterface
}

func TestGroupTransactionProcessorsTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(GroupTransactionProcessorsTestSuiteLedger))
}

func (s *GroupTransactionProcessorsTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	statsProcessor := processors.NewStatsLedgerTransactionProcessor()

	tradesProcessor := processors.NewTradeProcessor(history.NewAccountLoaderStub().Loader,
		history.NewLiquidityPoolLoaderStub().Loader,
		history.NewAssetLoaderStub().Loader,
		&history.MockTradeBatchInsertBuilder{})

	s.processorA = &mockHorizonTransactionProcessor{}
	s.processorB = &mockHorizonTransactionProcessor{}
	s.processors = newGroupTransactionProcessors([]horizonTransactionProcessor{
		s.processorA,
		s.processorB,
	}, statsProcessor, tradesProcessor)
	s.session = &db.MockSession{}
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TearDownTest() {
	s.processorA.AssertExpectations(s.T())
	s.processorB.AssertExpectations(s.T())
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TestProcessTransactionFails() {
	transaction := ingest.LedgerTransaction{}
	closeMeta := xdr.LedgerCloseMeta{}
	s.processorA.
		On("ProcessTransaction", closeMeta, transaction).
		Return(errors.New("transient error")).Once()

	err := s.processors.ProcessTransaction(closeMeta, transaction)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error in *ingest.mockHorizonTransactionProcessor.ProcessTransaction: transient error")
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TestProcessTransactionSucceeds() {
	transaction := ingest.LedgerTransaction{}
	closeMeta := xdr.LedgerCloseMeta{}
	s.processorA.
		On("ProcessTransaction", closeMeta, transaction).
		Return(nil).Once()
	s.processorB.
		On("ProcessTransaction", closeMeta, transaction).
		Return(nil).Once()

	err := s.processors.ProcessTransaction(closeMeta, transaction)
	s.Assert().NoError(err)
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TestFlushFails() {
	s.processorA.
		On("Flush", s.ctx, s.session).
		Return(errors.New("transient error")).Once()

	err := s.processors.Flush(s.ctx, s.session)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error in *ingest.mockHorizonTransactionProcessor.Flush: transient error")
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TestFlushSucceeds() {
	s.processorA.
		On("Flush", s.ctx, s.session).
		Return(nil).Once()
	s.processorB.
		On("Flush", s.ctx, s.session).
		Return(nil).Once()

	err := s.processors.Flush(s.ctx, s.session)
	s.Assert().NoError(err)
}

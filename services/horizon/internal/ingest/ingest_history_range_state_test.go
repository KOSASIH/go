//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package ingest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

func TestIngestHistoryRangeStateTestSuite(t *testing.T) {
	suite.Run(t, new(IngestHistoryRangeStateTestSuite))
}

type IngestHistoryRangeStateTestSuite struct {
	suite.Suite
	ctx            context.Context
	historyQ       *mockDBQ
	historyAdapter *mockHistoryArchiveAdapter
	ledgerBackend  *ledgerbackend.MockDatabaseBackend
	runner         *mockProcessorsRunner
	system         *system
}

func (s *IngestHistoryRangeStateTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.historyQ = &mockDBQ{}
	s.ledgerBackend = &ledgerbackend.MockDatabaseBackend{}
	s.historyAdapter = &mockHistoryArchiveAdapter{}
	s.runner = &mockProcessorsRunner{}
	s.system = &system{
		ctx:               s.ctx,
		historyQ:          s.historyQ,
		historyAdapter:    s.historyAdapter,
		ledgerBackend:     s.ledgerBackend,
		runner:            s.runner,
		maxLedgerPerFlush: 1,
	}
	s.system.initMetrics()
	s.ledgerBackend.On("IsPrepared", s.ctx, ledgerbackend.UnboundedRange(100)).Return(false, nil).Once()
	s.ledgerBackend.On("PrepareRange", s.ctx, ledgerbackend.UnboundedRange(100)).Return(nil).Once()
}

func (s *IngestHistoryRangeStateTestSuite) TearDownTest() {
	t := s.T()
	s.historyQ.AssertExpectations(t)
	s.ledgerBackend.AssertExpectations(t)
	s.historyAdapter.AssertExpectations(t)
	s.runner.AssertExpectations(t)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeInvalidRange() {
	*s.ledgerBackend = ledgerbackend.MockDatabaseBackend{}

	next, err := historyRangeState{fromLedger: 0, toLedger: 0}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "invalid range: [0, 0]")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)

	next, err = historyRangeState{fromLedger: 0, toLedger: 100}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "invalid range: [0, 100]")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)

	next, err = historyRangeState{fromLedger: 100, toLedger: 0}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "invalid range: [100, 0]")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)

	next, err = historyRangeState{fromLedger: 100, toLedger: 99}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "invalid range: [100, 99]")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeInvalidMaxFlush() {
	*s.ledgerBackend = ledgerbackend.MockDatabaseBackend{}

	s.system.maxLedgerPerFlush = 0
	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "invalid maxLedgerPerFlush, must be greater than 0")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeFailPrepare() {
	*s.ledgerBackend = ledgerbackend.MockDatabaseBackend{}
	s.ledgerBackend.On("IsPrepared", s.ctx, ledgerbackend.UnboundedRange(100)).Return(false, nil).Once()
	s.ledgerBackend.On("PrepareRange", s.ctx, ledgerbackend.UnboundedRange(100)).Return(errors.New("my error")).Once()

	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error preparing range: my error")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeBeginReturnsError() {
	s.historyQ.On("Begin", s.ctx).Return(errors.New("my error")).Once()

	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "Error starting a transaction: my error")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeGetLastLedgerIngestReturnsError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), errors.New("my error")).Once()

	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "Error getting last ingested ledger: my error")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeGetLatestLedgerReturnsError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(0), errors.New("my error")).Once()

	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "could not get latest history ledger: my error")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

// TestAnotherNodeIngested tests the case when another node has ingested the range.
// In such case we go back to `init` state without processing.
func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeAnotherNodeIngested() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(200), nil).Once()

	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeRunTransactionProcessorsReturnsError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(99), nil).Once()

	meta := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(100),
				},
			},
		},
	}

	s.ledgerBackend.On("GetLedger", s.ctx, uint32(100)).Return(meta, nil).Once()
	s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, false).Return(errors.New("my error")).Once()

	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error processing ledger range 100 - 100: my error")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeSuccess() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(99), nil).Once()

	for i := 100; i <= 200; i++ {
		meta := xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq: xdr.Uint32(i),
					},
				},
			},
		}
		s.ledgerBackend.On("GetLedger", s.ctx, uint32(i)).Return(meta, nil).Once()
		s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, false).Return(nil).Once()
	}

	s.historyQ.On("Commit").Return(nil).Once()

	// system.maxLedgerPerFlush has been set by default to 1 in test suite setup
	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeSuccessWithFlushMax() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(99), nil).Once()
	s.historyQ.On("Commit").Return(nil).Once()

	firstLedgersBatch := []xdr.LedgerCloseMeta{}
	secondLedgersBatch := []xdr.LedgerCloseMeta{}
	for i := 100; i <= 200; i++ {
		meta := xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq: xdr.Uint32(i),
					},
				},
			},
		}
		if i-60 < 100 {
			firstLedgersBatch = append(firstLedgersBatch, meta)
		} else {
			secondLedgersBatch = append(secondLedgersBatch, meta)
		}
		s.ledgerBackend.On("GetLedger", s.ctx, uint32(i)).Return(meta, nil).Once()
	}
	s.runner.On("RunTransactionProcessorsOnLedgers", firstLedgersBatch, false).Return(nil).Once()
	s.runner.On("RunTransactionProcessorsOnLedgers", secondLedgersBatch, false).Return(nil).Once()

	s.system.maxLedgerPerFlush = 60
	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *IngestHistoryRangeStateTestSuite) TestHistoryRangeCommitsWorkOnLedgerBackendFailure() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(99), nil).Once()
	s.historyQ.On("Commit").Return(nil).Once()

	meta := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(100),
				},
			},
		},
	}
	s.ledgerBackend.On("GetLedger", s.ctx, uint32(100)).Return(meta, nil).Once()
	s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, false).Return(nil).Once()
	s.ledgerBackend.On("GetLedger", s.ctx, uint32(101)).
		Return(xdr.LedgerCloseMeta{}, errors.New("my error")).Once()

	next, err := historyRangeState{fromLedger: 100, toLedger: 200}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error getting ledger: my error")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func TestReingestHistoryRangeStateTestSuite(t *testing.T) {
	suite.Run(t, new(ReingestHistoryRangeStateTestSuite))
}

type ReingestHistoryRangeStateTestSuite struct {
	suite.Suite
	ctx            context.Context
	historyQ       *mockDBQ
	historyAdapter *mockHistoryArchiveAdapter
	ledgerBackend  *mockLedgerBackend
	runner         *mockProcessorsRunner
	system         *system
}

func (s *ReingestHistoryRangeStateTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.historyQ = &mockDBQ{}
	s.historyAdapter = &mockHistoryArchiveAdapter{}
	s.ledgerBackend = &mockLedgerBackend{}
	s.runner = &mockProcessorsRunner{}
	s.system = &system{
		ctx:               s.ctx,
		historyQ:          s.historyQ,
		historyAdapter:    s.historyAdapter,
		ledgerBackend:     s.ledgerBackend,
		runner:            s.runner,
		maxLedgerPerFlush: 1,
	}
	s.historyQ.On("GetTx").Return(nil).Once()
	s.ledgerBackend.On("PrepareRange", s.ctx, ledgerbackend.BoundedRange(100, 200)).Return(nil).Once()
	s.system.initMetrics()
}

func (s *ReingestHistoryRangeStateTestSuite) TearDownTest() {
	t := s.T()
	s.historyQ.AssertExpectations(t)
	s.historyAdapter.AssertExpectations(t)
	s.runner.AssertExpectations(t)
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateInvalidRange() {
	// Recreate mock in this single test to remove Rollback assertion.
	s.historyQ = &mockDBQ{}
	err := s.system.ReingestRange([]history.LedgerRange{{0, 0}}, false, true)
	s.Assert().EqualError(err, "Invalid range: {0 0} genesis ledger starts at 1")

	err = s.system.ReingestRange([]history.LedgerRange{{0, 100}}, false, true)
	s.Assert().EqualError(err, "Invalid range: {0 100} genesis ledger starts at 1")

	err = s.system.ReingestRange([]history.LedgerRange{{100, 0}}, false, true)
	s.Assert().EqualError(err, "Invalid range: {100 0} from > to")

	err = s.system.ReingestRange([]history.LedgerRange{{100, 99}}, false, true)
	s.Assert().EqualError(err, "Invalid range: {100 99} from > to")
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateInvalidMaxFlush() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(0), nil).Once()
	s.system.maxLedgerPerFlush = 0
	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, false, true)
	s.Assert().EqualError(err, "invalid maxLedgerPerFlush, must be greater than 0")
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateGetLastLedgerIngestNonBlockingError() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(0), errors.New("my error")).Once()

	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, false, true)
	s.Assert().EqualError(err, "Error getting last ingested ledger: my error")
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateRangeOverlaps() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(190), nil).Once()

	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, false, true)
	s.Assert().Equal(ErrReingestRangeConflict{190}, err)
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStatRangeOverlapsAtEnd() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(200), nil).Once()

	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, false, true)
	s.Assert().Equal(ErrReingestRangeConflict{200}, err)
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateClearHistoryFails() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(0), nil).Once()
	toidFrom := toid.New(100, 0, 0)
	// the state deletes range once, calc'd by toid.LedgerRangeInclusive(), which adjusts to = to + 1
	toidTo := toid.New(201, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(0), errors.New("my error")).Once()

	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, false, true)
	s.Assert().EqualError(err, "error in DeleteRangeAll: my error")
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateRunTransactionProcessorsReturnsError() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(0), nil).Once()
	toidFrom := toid.New(100, 0, 0)
	toidTo := toid.New(201, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(100), nil).Once()

	meta := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(100),
				},
			},
		},
	}

	s.ledgerBackend.On("GetLedger", s.ctx, uint32(100)).Return(meta, nil).Once()
	s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, true).Return(errors.New("my error")).Once()

	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, false, true)
	s.Assert().EqualError(err, "error processing ledger range 100 - 100: my error")
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateSuccess() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(0), nil).Once()
	toidFrom := toid.New(100, 0, 0)
	toidTo := toid.New(201, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(100), nil).Once()
	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(100), uint32(200), 0).Return(nil).Once()

	for i := uint32(100); i <= uint32(200); i++ {
		meta := xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq: xdr.Uint32(i),
					},
				},
			},
		}
		s.ledgerBackend.On("GetLedger", s.ctx, uint32(i)).Return(meta, nil).Once()
		s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, true).Return(nil).Once()
	}

	// system.maxLedgerPerFlush has been set by default to 1 in test suite setup
	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, false, true)
	s.Assert().NoError(err)
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateSuccessWithFlushMax() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(0), nil).Once()
	toidFrom := toid.New(100, 0, 0)
	toidTo := toid.New(201, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(100), nil).Once()
	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(100), uint32(200), 0).Return(nil).Once()

	firstLedgersBatch := []xdr.LedgerCloseMeta{}
	secondLedgersBatch := []xdr.LedgerCloseMeta{}
	for i := uint32(100); i <= uint32(200); i++ {
		meta := xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq: xdr.Uint32(i),
					},
				},
			},
		}
		// triggers two flushes, one for max during loop and one for remainder
		if i-60 < 100 {
			firstLedgersBatch = append(firstLedgersBatch, meta)
		} else {
			secondLedgersBatch = append(secondLedgersBatch, meta)
		}
		s.ledgerBackend.On("GetLedger", s.ctx, uint32(i)).Return(meta, nil).Once()
	}
	s.runner.On("RunTransactionProcessorsOnLedgers", firstLedgersBatch, true).Return(nil).Once()
	s.runner.On("RunTransactionProcessorsOnLedgers", secondLedgersBatch, true).Return(nil).Once()
	s.system.maxLedgerPerFlush = 60
	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, false, true)
	s.Assert().NoError(err)
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateSuccessOneLedger() {
	s.historyQ.On("GetLastLedgerIngestNonBlocking", s.ctx).Return(uint32(0), nil).Once()
	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(100), uint32(100), 0).Return(nil).Once()
	// Recreate mock in this single ledger test to remove setup assertion on ledger range.
	*s.ledgerBackend = mockLedgerBackend{}
	s.ledgerBackend.On("PrepareRange", s.ctx, ledgerbackend.BoundedRange(100, 100)).Return(nil).Once()

	toidFrom := toid.New(100, 0, 0)
	toidTo := toid.New(101, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(100), nil).Once()

	meta := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(100),
				},
			},
		},
	}

	s.ledgerBackend.On("GetLedger", s.ctx, uint32(100)).Return(meta, nil).Once()
	s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, true).Return(nil).Once()

	err := s.system.ReingestRange([]history.LedgerRange{{100, 100}}, false, true)
	s.Assert().NoError(err)
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateForceGetLastLedgerIngestError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), errors.New("my error")).Once()

	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, true, true)
	s.Assert().EqualError(err, "Error getting last ingested ledger: my error")
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateForce() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(190), nil).Once()
	s.historyQ.On("Commit").Return(nil).Once()
	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(100), uint32(200), 0).Return(nil).Once()

	toidFrom := toid.New(100, 0, 0)
	toidTo := toid.New(201, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(100), nil).Once()

	for i := 100; i <= 200; i++ {
		meta := xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq: xdr.Uint32(i),
					},
				},
			},
		}
		s.ledgerBackend.On("GetLedger", s.ctx, uint32(i)).Return(meta, nil).Once()
		s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, false).Return(nil).Once()
	}

	// system.maxLedgerPerFlush has been set by default to 1 in test suite setup
	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, true, true)
	s.Assert().NoError(err)
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateForceLedgerRetrievalError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(190), nil).Once()
	s.historyQ.On("Commit").Return(nil).Once()

	toidFrom := toid.New(100, 0, 0)
	toidTo := toid.New(201, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(100), nil).Once()

	for i := 100; i <= 105; i++ {
		meta := xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq: xdr.Uint32(i),
					},
				},
			},
		}
		s.ledgerBackend.On("GetLedger", s.ctx, uint32(i)).Return(meta, nil).Once()
		s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, false).Return(nil).Once()
	}

	s.ledgerBackend.On("GetLedger", s.ctx, uint32(106)).Return(xdr.LedgerCloseMeta{}, errors.New("my error")).Once()

	// system.maxLedgerPerFlush has been set by default to 1 in test suite setup
	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, true, true)
	s.Assert().EqualError(err, "error getting ledger: my error")
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateForceLedgerRetrievalAndCommitError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(190), nil).Once()
	s.historyQ.On("Commit").Return(errors.New("commit error")).Once()

	toidFrom := toid.New(100, 0, 0)
	toidTo := toid.New(201, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(100), nil).Once()

	for i := 100; i <= 105; i++ {
		meta := xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq: xdr.Uint32(i),
					},
				},
			},
		}
		s.ledgerBackend.On("GetLedger", s.ctx, uint32(i)).Return(meta, nil).Once()
		s.runner.On("RunTransactionProcessorsOnLedgers", []xdr.LedgerCloseMeta{meta}, false).Return(nil).Once()
	}

	s.ledgerBackend.On("GetLedger", s.ctx, uint32(106)).Return(xdr.LedgerCloseMeta{}, errors.New("my error")).Once()

	// system.maxLedgerPerFlush has been set by default to 1 in test suite setup
	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, true, true)
	s.Assert().EqualError(err, "Error committing db transaction: error getting ledger: my error")
}

func (s *ReingestHistoryRangeStateTestSuite) TestReingestHistoryRangeStateForceWithFlushMax() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("Rollback").Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(190), nil).Once()
	s.historyQ.On("Commit").Return(nil).Once()
	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(100), uint32(200), 0).Return(nil).Once()

	toidFrom := toid.New(100, 0, 0)
	toidTo := toid.New(201, 0, 0)
	s.historyQ.On(
		"DeleteRangeAll", s.ctx, toidFrom.ToInt64(), toidTo.ToInt64(),
	).Return(int64(100), nil).Once()

	firstLedgersBatch := []xdr.LedgerCloseMeta{}
	secondLedgersBatch := []xdr.LedgerCloseMeta{}
	for i := 100; i <= 200; i++ {
		meta := xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq: xdr.Uint32(i),
					},
				},
			},
		}
		// triggers two flushes, one for max during loop and one for remainder
		if i-60 < 100 {
			firstLedgersBatch = append(firstLedgersBatch, meta)
		} else {
			secondLedgersBatch = append(secondLedgersBatch, meta)
		}
		s.ledgerBackend.On("GetLedger", s.ctx, uint32(i)).Return(meta, nil).Once()
	}
	s.runner.On("RunTransactionProcessorsOnLedgers", firstLedgersBatch, false).Return(nil).Once()
	s.runner.On("RunTransactionProcessorsOnLedgers", secondLedgersBatch, false).Return(nil).Once()

	s.system.maxLedgerPerFlush = 60
	err := s.system.ReingestRange([]history.LedgerRange{{100, 200}}, true, true)
	s.Assert().NoError(err)
}

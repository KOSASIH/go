//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package ingest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/historyarchive"
	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

func TestResumeTestTestSuite(t *testing.T) {
	suite.Run(t, new(ResumeTestTestSuite))
}

type ResumeTestTestSuite struct {
	suite.Suite
	ctx               context.Context
	ledgerBackend     *ledgerbackend.MockDatabaseBackend
	historyQ          *mockDBQ
	historyAdapter    *mockHistoryArchiveAdapter
	runner            *mockProcessorsRunner
	stellarCoreClient *mockStellarCoreClient
	system            *system
}

func (s *ResumeTestTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.ledgerBackend = &ledgerbackend.MockDatabaseBackend{}
	s.historyQ = &mockDBQ{}
	s.historyAdapter = &mockHistoryArchiveAdapter{}
	s.runner = &mockProcessorsRunner{}
	s.stellarCoreClient = &mockStellarCoreClient{}
	s.system = &system{
		ctx:                          s.ctx,
		historyQ:                     s.historyQ,
		historyAdapter:               s.historyAdapter,
		runner:                       s.runner,
		ledgerBackend:                s.ledgerBackend,
		stellarCoreClient:            s.stellarCoreClient,
		runStateVerificationOnLedger: ledgerEligibleForStateVerification(64, 1),
		reaper:                       &Reaper{},
	}
	s.system.initMetrics()

	s.historyQ.On("Rollback").Return(nil).Once()

	s.ledgerBackend.On("IsPrepared", s.ctx, ledgerbackend.UnboundedRange(101)).Return(false, nil).Once()
	s.ledgerBackend.On("PrepareRange", s.ctx, ledgerbackend.UnboundedRange(101)).Return(nil).Once()
	s.ledgerBackend.On("GetLedger", s.ctx, uint32(101)).Return(xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq:      101,
					LedgerVersion:  xdr.Uint32(MaxSupportedProtocolVersion),
					BucketListHash: xdr.Hash{1, 2, 3},
				},
			},
		},
	}, nil).Once()
}

func (s *ResumeTestTestSuite) TearDownTest() {
	t := s.T()
	s.historyQ.AssertExpectations(t)
	s.runner.AssertExpectations(t)
	s.historyAdapter.AssertExpectations(t)
	s.ledgerBackend.AssertExpectations(t)
	s.stellarCoreClient.AssertExpectations(t)
}

func (s *ResumeTestTestSuite) TestInvalidParam() {
	// Recreate mock in this single test to remove Rollback assertion.
	*s.historyQ = mockDBQ{}
	*s.ledgerBackend = ledgerbackend.MockDatabaseBackend{}

	next, err := resumeState{latestSuccessfullyProcessedLedger: 0}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "unexpected latestSuccessfullyProcessedLedger value")
	s.Assert().Equal(
		transition{node: startState{}, sleepDuration: defaultSleep},
		next,
	)
}

func (s *ResumeTestTestSuite) TestRangeNotPreparedFailPrepare() {
	// Recreate mock in this single test to remove Rollback assertion.
	*s.historyQ = mockDBQ{}
	*s.ledgerBackend = ledgerbackend.MockDatabaseBackend{}

	s.ledgerBackend.On("IsPrepared", s.ctx, ledgerbackend.UnboundedRange(101)).Return(false, nil).Once()
	s.ledgerBackend.On("PrepareRange", s.ctx, ledgerbackend.UnboundedRange(101)).Return(errors.New("my error")).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error preparing range: my error")
	s.Assert().Equal(
		transition{node: startState{}, sleepDuration: defaultSleep},
		next,
	)
}

func (s *ResumeTestTestSuite) TestRangeNotPreparedSuccessPrepareGetLedgerFail() {
	// Recreate mock in this single test to remove Rollback assertion.
	*s.historyQ = mockDBQ{}
	*s.ledgerBackend = ledgerbackend.MockDatabaseBackend{}

	s.ledgerBackend.On("IsPrepared", s.ctx, ledgerbackend.UnboundedRange(101)).Return(false, nil).Once()
	s.ledgerBackend.On("PrepareRange", s.ctx, ledgerbackend.UnboundedRange(101)).Return(nil).Once()
	s.ledgerBackend.On("GetLedger", s.ctx, uint32(101)).Return(xdr.LedgerCloseMeta{}, errors.New("my error")).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error getting ledger blocking: my error")
	s.Assert().Equal(transition{node: startState{}, sleepDuration: defaultSleep}, next)
}

func (s *ResumeTestTestSuite) TestBeginReturnsError() {
	// Recreate mock in this single test to remove Rollback assertion.
	*s.historyQ = mockDBQ{}

	s.historyQ.On("Begin", s.ctx).Return(errors.New("my error")).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "Error starting a transaction: my error")
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 100},
			sleepDuration: defaultSleep,
		},
		next,
	)
}

func (s *ResumeTestTestSuite) TestGetLastLedgerIngestReturnsError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(0), errors.New("my error")).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "Error getting last ingested ledger: my error")
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 100},
			sleepDuration: defaultSleep,
		},
		next,
	)
}

func (s *ResumeTestTestSuite) TestGetLatestLedgerLessThanCurrent() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(99), nil).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "expected ingest ledger to be at most one greater than last ingested ledger in db")
	s.Assert().Equal(
		transition{node: startState{}, sleepDuration: defaultSleep},
		next,
	)
}

func (s *ResumeTestTestSuite) TestGetIngestionVersionError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(0, errors.New("my error")).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "Error getting ingestion version: my error")
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 100},
			sleepDuration: defaultSleep,
		},
		next,
	)
}

func (s *ResumeTestTestSuite) TestIngestionVersionLessThanCurrentVersion() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion-1, nil).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(
		transition{node: startState{}, sleepDuration: defaultSleep},
		next,
	)
}

func (s *ResumeTestTestSuite) TestIngestionVersionGreaterThanCurrentVersion() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion+1, nil).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(
		transition{node: startState{}, sleepDuration: defaultSleep},
		next,
	)
}

func (s *ResumeTestTestSuite) TestGetLatestLedgerError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion, nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(0), errors.New("my error"))

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "could not get latest history ledger: my error")
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 100},
			sleepDuration: defaultSleep,
		},
		next,
	)
}

func (s *ResumeTestTestSuite) TestLatestHistoryLedgerLessThanIngestLedger() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion, nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(99), nil)

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(
		transition{node: startState{}, sleepDuration: defaultSleep},
		next,
	)
}

func (s *ResumeTestTestSuite) TestLatestHistoryLedgerGreaterThanIngestLedger() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion, nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(101), nil)

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(
		transition{node: startState{}, sleepDuration: defaultSleep},
		next,
	)
}

func (s *ResumeTestTestSuite) mockSuccessfulIngestion() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion, nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(100), nil)
	mockStats := &historyarchive.MockArchiveStats{}
	mockStats.On("GetBackendName").Return("name")
	mockStats.On("GetDownloads").Return(uint32(0))
	mockStats.On("GetRequests").Return(uint32(0))
	mockStats.On("GetUploads").Return(uint32(0))
	mockStats.On("GetCacheHits").Return(uint32(0))
	mockStats.On("GetCacheBandwidth").Return(uint64(0))
	s.historyAdapter.On("GetStats").Return([]historyarchive.ArchiveStats{mockStats}).Once()

	s.runner.On("RunAllProcessorsOnLedger", mock.AnythingOfType("xdr.LedgerCloseMeta")).
		Run(func(args mock.Arguments) {
			meta := args.Get(0).(xdr.LedgerCloseMeta)
			s.Assert().Equal(uint32(101), meta.LedgerSequence())
		}).
		Return(
			ledgerStats{},
			nil,
		).Once()
	s.historyQ.On("UpdateLastLedgerIngest", s.ctx, uint32(101)).Return(nil).Once()
	s.historyQ.On("Commit").Return(nil).Once()
	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(101), uint32(101), 0).Return(nil).Once()
	s.historyQ.On("GetExpStateInvalid", s.ctx).Return(false, nil).Once()
}
func (s *ResumeTestTestSuite) TestBumpIngestLedger() {
	*s.ledgerBackend = ledgerbackend.MockDatabaseBackend{}

	s.ledgerBackend.On("IsPrepared", s.ctx, ledgerbackend.UnboundedRange(100)).Return(false, nil).Once()
	s.ledgerBackend.On("PrepareRange", s.ctx, ledgerbackend.UnboundedRange(100)).Return(nil).Once()
	s.ledgerBackend.On("GetLedger", s.ctx, uint32(100)).Return(xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq:      100,
					LedgerVersion:  xdr.Uint32(MaxSupportedProtocolVersion),
					BucketListHash: xdr.Hash{1, 2, 3},
				},
			},
		},
	}, nil).Once()

	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(101), nil).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 99}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 101},
			sleepDuration: 0,
		},
		next,
	)
}

func (s *ResumeTestTestSuite) TestIngestAllMasterNode() {
	s.mockSuccessfulIngestion()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 101},
			sleepDuration: 0,
		},
		next,
	)
}

func (s *ResumeTestTestSuite) TestRebuildTradeAggregationBucketsError() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion, nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(100), nil)

	s.runner.On("RunAllProcessorsOnLedger", mock.AnythingOfType("xdr.LedgerCloseMeta")).
		Run(func(args mock.Arguments) {
			meta := args.Get(0).(xdr.LedgerCloseMeta)
			s.Assert().Equal(uint32(101), meta.LedgerSequence())
		}).
		Return(
			ledgerStats{},
			nil,
		).Once()

	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(101), uint32(101), 0).
		Return(errors.New("transient error")).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().EqualError(err, "error rebuilding trade aggregations: transient error")
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 100},
			sleepDuration: defaultSleep,
		},
		next,
	)
}

func (s *ResumeTestTestSuite) TestReapingObjectsDisabled() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion, nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(100), nil)

	s.runner.On("RunAllProcessorsOnLedger", mock.AnythingOfType("xdr.LedgerCloseMeta")).
		Run(func(args mock.Arguments) {
			meta := args.Get(0).(xdr.LedgerCloseMeta)
			s.Assert().Equal(uint32(101), meta.LedgerSequence())
		}).
		Return(
			ledgerStats{},
			nil,
		).Once()
	s.historyQ.On("UpdateLastLedgerIngest", s.ctx, uint32(101)).Return(nil).Once()
	s.historyQ.On("Commit").Return(nil).Once()

	s.historyQ.On("GetExpStateInvalid", s.ctx).Return(false, nil).Once()
	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(101), uint32(101), 0).Return(nil).Once()
	mockStats := &historyarchive.MockArchiveStats{}
	mockStats.On("GetBackendName").Return("name")
	mockStats.On("GetDownloads").Return(uint32(0))
	mockStats.On("GetRequests").Return(uint32(0))
	mockStats.On("GetUploads").Return(uint32(0))
	mockStats.On("GetCacheHits").Return(uint32(0))
	mockStats.On("GetCacheBandwidth").Return(uint64(0))
	s.historyAdapter.On("GetStats").Return([]historyarchive.ArchiveStats{mockStats}).Once()
	// Reap lookup tables not executed

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 101},
			sleepDuration: 0,
		},
		next,
	)
}

func (s *ResumeTestTestSuite) TestErrorReapingObjectsIgnored() {
	s.historyQ.On("Begin", s.ctx).Return(nil).Once()
	s.historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(100), nil).Once()
	s.historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion, nil).Once()
	s.historyQ.On("GetLatestHistoryLedger", s.ctx).Return(uint32(100), nil)

	s.runner.On("RunAllProcessorsOnLedger", mock.AnythingOfType("xdr.LedgerCloseMeta")).
		Run(func(args mock.Arguments) {
			meta := args.Get(0).(xdr.LedgerCloseMeta)
			s.Assert().Equal(uint32(101), meta.LedgerSequence())
		}).
		Return(
			ledgerStats{},
			nil,
		).Once()
	s.historyQ.On("UpdateLastLedgerIngest", s.ctx, uint32(101)).Return(nil).Once()
	s.historyQ.On("Commit").Return(nil).Once()

	s.historyQ.On("GetExpStateInvalid", s.ctx).Return(false, nil).Once()
	s.historyQ.On("RebuildTradeAggregationBuckets", s.ctx, uint32(101), uint32(101), 0).Return(nil).Once()
	mockStats := &historyarchive.MockArchiveStats{}
	mockStats.On("GetBackendName").Return("name")
	mockStats.On("GetDownloads").Return(uint32(0))
	mockStats.On("GetRequests").Return(uint32(0))
	mockStats.On("GetUploads").Return(uint32(0))
	mockStats.On("GetCacheHits").Return(uint32(0))
	mockStats.On("GetCacheBandwidth").Return(uint64(0))
	s.historyAdapter.On("GetStats").Return([]historyarchive.ArchiveStats{mockStats}).Once()

	next, err := resumeState{latestSuccessfullyProcessedLedger: 100}.run(s.system)
	s.Assert().NoError(err)
	s.Assert().Equal(
		transition{
			node:          resumeState{latestSuccessfullyProcessedLedger: 101},
			sleepDuration: 0,
		},
		next,
	)
}

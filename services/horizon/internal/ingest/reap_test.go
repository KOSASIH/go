package ingest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/toid"
)

func TestDeleteUnretainedHistory(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	tt.Scenario("kahuna")

	db := tt.HorizonSession()

	reaper := NewReaper(ReapConfig{
		RetentionCount: 0,
		BatchSize:      50,
	}, db)

	// Disable sleeps for this.
	prevSleep := sleep
	sleep = 0
	t.Cleanup(func() {
		sleep = prevSleep
	})

	var (
		prev int
		cur  int
	)
	err := db.GetRaw(tt.Ctx, &prev, `SELECT COUNT(*) FROM history_ledgers`)
	tt.Require.NoError(err)

	err = reaper.DeleteUnretainedHistory(tt.Ctx)
	if tt.Assert.NoError(err) {
		err = db.GetRaw(tt.Ctx, &cur, `SELECT COUNT(*) FROM history_ledgers`)
		tt.Require.NoError(err)
		tt.Assert.Equal(prev, cur, "Ledgers deleted when RetentionCount == 0")
	}

	reaper.config.RetentionCount = 10
	err = reaper.DeleteUnretainedHistory(tt.Ctx)
	if tt.Assert.NoError(err) {
		err = db.GetRaw(tt.Ctx, &cur, `SELECT COUNT(*) FROM history_ledgers`)
		tt.Require.NoError(err)
		tt.Assert.Equal(10, cur)
	}

	reaper.config.RetentionCount = 1
	err = reaper.DeleteUnretainedHistory(tt.Ctx)
	if tt.Assert.NoError(err) {
		err = db.GetRaw(tt.Ctx, &cur, `SELECT COUNT(*) FROM history_ledgers`)
		tt.Require.NoError(err)
		tt.Assert.Equal(1, cur)
	}
}

type ReaperTestSuite struct {
	suite.Suite
	ctx       context.Context
	historyQ  *mockDBQ
	reapLockQ *mockDBQ
	reaper    *Reaper
	prevSleep time.Duration
}

func TestReaper(t *testing.T) {
	suite.Run(t, new(ReaperTestSuite))
}

func (t *ReaperTestSuite) SetupTest() {
	t.ctx = context.Background()
	t.historyQ = &mockDBQ{}
	t.reapLockQ = &mockDBQ{}
	t.reaper = newReaper(ReapConfig{
		RetentionCount: 30,
		BatchSize:      10,
		Frequency:      7,
	}, t.historyQ, t.reapLockQ)
	t.prevSleep = sleep
	sleep = 0
}

func (t *ReaperTestSuite) TearDownTest() {
	t.historyQ.AssertExpectations(t.T())
	t.reapLockQ.AssertExpectations(t.T())
	sleep = t.prevSleep
}

func (t *ReaperTestSuite) TestDisabled() {
	t.reaper.config.RetentionCount = 0
	t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
}

func assertMocksInOrder(calls ...*mock.Call) {
	for i := len(calls) - 1; i > 0; i-- {
		calls[i].NotBefore(calls[i-1])
	}
}

func (t *ReaperTestSuite) TestInProgressOnOtherNode() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(false, nil).Once(),
		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
}

func (t *ReaperTestSuite) TestInProgress() {
	t.reapLockQ.On("Begin", t.ctx).Return(fmt.Errorf("transient error")).Once().Run(
		func(args mock.Arguments) {
			t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
		},
	)
	t.Assert().EqualError(
		t.reaper.DeleteUnretainedHistory(t.ctx),
		"error while starting reaper lock transaction: transient error",
	)
}

func (t *ReaperTestSuite) TestReaperInvokedOnMatchingLedger() {
	s := &system{
		ctx:    t.ctx,
		reaper: t.reaper,
	}
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(false, nil).Once(),
		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	s.maybeReapHistory(49)
	s.wg.Wait()
}

func (t *ReaperTestSuite) TestReaperIgnoredOnMismatchingLedger() {
	s := &system{
		ctx:    t.ctx,
		reaper: t.reaper,
	}
	s.maybeReapHistory(48)
	s.wg.Wait()
}

func (t *ReaperTestSuite) TestLatestLedgerTooSmall() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(true, nil).Once(),
		t.historyQ.On("GetLatestHistoryLedger", t.ctx).Return(uint32(30), nil).Once(),
		t.historyQ.On("ElderLedger", t.ctx, mock.AnythingOfType("*uint32")).
			Return(nil).Once().Run(
			func(args mock.Arguments) {
				ledger := args.Get(1).(*uint32)
				*ledger = 1
			}),
		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
}

func (t *ReaperTestSuite) TestNotEnoughHistory() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(true, nil).Once(),
		t.historyQ.On("GetLatestHistoryLedger", t.ctx).Return(uint32(90), nil).Once(),
		t.historyQ.On("ElderLedger", t.ctx, mock.AnythingOfType("*uint32")).
			Return(nil).Once().Run(
			func(args mock.Arguments) {
				ledger := args.Get(1).(*uint32)
				*ledger = 85
			}),
		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
}

func (t *ReaperTestSuite) TestSucceeds() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(true, nil).Once(),
		t.historyQ.On("GetLatestHistoryLedger", t.ctx).Return(uint32(90), nil).Once(),
		t.historyQ.On("ElderLedger", t.ctx, mock.AnythingOfType("*uint32")).
			Return(nil).Once().Run(
			func(args mock.Arguments) {
				ledger := args.Get(1).(*uint32)
				*ledger = 55
			}),
		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(55, 0, 0).ToInt64(), toid.New(61, 0, 0).ToInt64(),
		).Return(int64(400), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),
		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
}

func (t *ReaperTestSuite) TestFails() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(true, nil).Once(),
		t.historyQ.On("GetLatestHistoryLedger", t.ctx).Return(uint32(90), nil).Once(),
		t.historyQ.On("ElderLedger", t.ctx, mock.AnythingOfType("*uint32")).
			Return(nil).Once().Run(
			func(args mock.Arguments) {
				ledger := args.Get(1).(*uint32)
				*ledger = 2
			}),
		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(2, 0, 0).ToInt64(), toid.New(13, 0, 0).ToInt64(),
		).Return(int64(0), fmt.Errorf("transient error")).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),
		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().EqualError(t.reaper.DeleteUnretainedHistory(t.ctx), "Error in DeleteRangeAll: transient error")
}

func (t *ReaperTestSuite) TestPartiallySucceeds() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(true, nil).Once(),
		t.historyQ.On("GetLatestHistoryLedger", t.ctx).Return(uint32(90), nil).Once(),
		t.historyQ.On("ElderLedger", t.ctx, mock.AnythingOfType("*uint32")).
			Return(nil).Once().Run(
			func(args mock.Arguments) {
				ledger := args.Get(1).(*uint32)
				*ledger = 30
			}),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(30, 0, 0).ToInt64(), toid.New(41, 0, 0).ToInt64(),
		).Return(int64(200), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(41, 0, 0).ToInt64(), toid.New(52, 0, 0).ToInt64(),
		).Return(int64(0), fmt.Errorf("transient error")).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),

		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().EqualError(t.reaper.DeleteUnretainedHistory(t.ctx), "Error in DeleteRangeAll: transient error")
}

func (t *ReaperTestSuite) TestSucceedsOnMultipleBatches() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(true, nil).Once(),
		t.historyQ.On("GetLatestHistoryLedger", t.ctx).Return(uint32(90), nil).Once(),
		t.historyQ.On("ElderLedger", t.ctx, mock.AnythingOfType("*uint32")).
			Return(nil).Once().Run(
			func(args mock.Arguments) {
				ledger := args.Get(1).(*uint32)
				*ledger = 35
			}),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(35, 0, 0).ToInt64(), toid.New(46, 0, 0).ToInt64(),
		).Return(int64(200), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(46, 0, 0).ToInt64(), toid.New(57, 0, 0).ToInt64(),
		).Return(int64(150), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(57, 0, 0).ToInt64(), toid.New(61, 0, 0).ToInt64(),
		).Return(int64(80), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),

		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
}

func (t *ReaperTestSuite) TestSkipGap() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(true, nil).Once(),
		t.historyQ.On("GetLatestHistoryLedger", t.ctx).Return(uint32(90), nil).Once(),
		t.historyQ.On("ElderLedger", t.ctx, mock.AnythingOfType("*uint32")).
			Return(nil).Once().Run(
			func(args mock.Arguments) {
				ledger := args.Get(1).(*uint32)
				*ledger = 2
			}),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(2, 0, 0).ToInt64(), toid.New(13, 0, 0).ToInt64(),
		).Return(int64(200), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(13, 0, 0).ToInt64(), toid.New(24, 0, 0).ToInt64(),
		).Return(int64(0), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),
		t.historyQ.On("GetNextLedgerSequence", t.ctx, uint32(13)).Return(uint32(55), true, nil).Once(),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(55, 0, 0).ToInt64(), toid.New(61, 0, 0).ToInt64(),
		).Return(int64(20), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),

		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
}

func (t *ReaperTestSuite) TestSkipGapTerminatesEarly() {
	assertMocksInOrder(
		t.reapLockQ.On("Begin", t.ctx).Return(nil).Once(),
		t.reapLockQ.On("TryReaperLock", t.ctx).Return(true, nil).Once(),
		t.historyQ.On("GetLatestHistoryLedger", t.ctx).Return(uint32(90), nil).Once(),
		t.historyQ.On("ElderLedger", t.ctx, mock.AnythingOfType("*uint32")).
			Return(nil).Once().Run(
			func(args mock.Arguments) {
				ledger := args.Get(1).(*uint32)
				*ledger = 2
			}),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(2, 0, 0).ToInt64(), toid.New(13, 0, 0).ToInt64(),
		).Return(int64(200), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),

		t.historyQ.On("Begin", t.ctx).Return(nil).Once(),
		t.historyQ.On("DeleteRangeAll", t.ctx,
			toid.New(13, 0, 0).ToInt64(), toid.New(24, 0, 0).ToInt64(),
		).Return(int64(0), nil).Once(),
		t.historyQ.On("Commit").Return(nil).Once(),
		t.historyQ.On("Rollback").Return(nil).Once(),
		t.historyQ.On("GetNextLedgerSequence", t.ctx, uint32(13)).Return(uint32(65), true, nil).Once(),

		t.reapLockQ.On("Rollback").Return(nil).Once(),
	)
	t.Assert().NoError(t.reaper.DeleteUnretainedHistory(t.ctx))
}

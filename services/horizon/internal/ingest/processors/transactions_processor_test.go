//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"testing"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type TransactionsProcessorTestSuiteLedger struct {
	suite.Suite
	ctx                    context.Context
	processor              *TransactionProcessor
	mockSession            *db.MockSession
	mockBatchInsertBuilder *history.MockTransactionsBatchInsertBuilder
}

func TestTransactionsProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(TransactionsProcessorTestSuiteLedger))
}

func (s *TransactionsProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockBatchInsertBuilder = &history.MockTransactionsBatchInsertBuilder{}
	s.processor = NewTransactionProcessor(s.mockBatchInsertBuilder, false)
}

func (s *TransactionsProcessorTestSuiteLedger) TearDownTest() {
	s.mockBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *TransactionsProcessorTestSuiteLedger) TestAddTransactionsWithMetaSucceeds() {
	sequence := uint32(20)
	lcm := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}
	creator := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	ledgerEntryChange := xdr.LedgerEntryChange{
		Type: 3,
		State: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 0x39,
			Data: xdr.LedgerEntryData{
				Type: 0,
				Account: &xdr.AccountEntry{
					AccountId:     creator,
					Balance:       800152377009533292,
					SeqNum:        25,
					InflationDest: &creator,
					Thresholds:    xdr.Thresholds{0x1, 0x0, 0x0, 0x0},
				},
			},
		},
	}

	firstTx := createTransaction(true, 1, 1)
	firstTx.UnsafeMeta.V1.TxChanges = xdr.LedgerEntryChanges{ledgerEntryChange}
	secondTx := createTransaction(false, 3, 2)
	secondTx.UnsafeMeta.V2.TxChangesBefore = xdr.LedgerEntryChanges{ledgerEntryChange}
	secondTx.UnsafeMeta.V2.TxChangesAfter = xdr.LedgerEntryChanges{ledgerEntryChange}
	thirdTx := createTransaction(true, 4, 3)
	thirdTx.UnsafeMeta.V3.TxChangesBefore = xdr.LedgerEntryChanges{ledgerEntryChange}
	thirdTx.UnsafeMeta.V3.TxChangesAfter = xdr.LedgerEntryChanges{ledgerEntryChange}
	thirdTx.UnsafeMeta.V3.SorobanMeta = &xdr.SorobanTransactionMeta{}

	s.mockBatchInsertBuilder.On("Add", firstTx, sequence).Run(func(args mock.Arguments) {
		tx := args.Get(0).(ingest.LedgerTransaction)
		s.Assert().Len(tx.UnsafeMeta.V1.TxChanges, 1)
		s.Assert().Len(tx.UnsafeMeta.V1.Operations, 1)
	}).Return(nil).Once()

	s.mockBatchInsertBuilder.On("Add", secondTx, sequence).Run(func(args mock.Arguments) {
		tx := args.Get(0).(ingest.LedgerTransaction)
		s.Assert().Len(tx.UnsafeMeta.V2.TxChangesAfter, 1)
		s.Assert().Len(tx.UnsafeMeta.V2.TxChangesBefore, 1)
		s.Assert().Len(tx.UnsafeMeta.V2.Operations, 3)
	}).Return(nil).Once()

	s.mockBatchInsertBuilder.On("Add", thirdTx, sequence+1).Run(func(args mock.Arguments) {
		tx := args.Get(0).(ingest.LedgerTransaction)
		s.Assert().Len(tx.UnsafeMeta.V3.TxChangesAfter, 1)
		s.Assert().Len(tx.UnsafeMeta.V3.TxChangesBefore, 1)
		s.Assert().NotNil(tx.UnsafeMeta.V3.SorobanMeta)
		s.Assert().Len(tx.UnsafeMeta.V3.Operations, 4)
	}).Return(nil).Once()

	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	s.Assert().NoError(s.processor.ProcessTransaction(lcm, firstTx))
	s.Assert().NoError(s.processor.ProcessTransaction(lcm, secondTx))
	lcm.V0.LedgerHeader.Header.LedgerSeq++
	s.Assert().NoError(s.processor.ProcessTransaction(lcm, thirdTx))

	s.Assert().NoError(s.processor.Flush(s.ctx, s.mockSession))
}

func (s *TransactionsProcessorTestSuiteLedger) TestAddTransactionsWithSkippedMetaSucceeds() {
	elidingTxProcessor := NewTransactionProcessor(s.mockBatchInsertBuilder, true)

	sequence := uint32(20)
	lcm := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}
	creator := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	ledgerEntryChange := xdr.LedgerEntryChange{
		Type: 3,
		State: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 0x39,
			Data: xdr.LedgerEntryData{
				Type: 0,
				Account: &xdr.AccountEntry{
					AccountId:     creator,
					Balance:       800152377009533292,
					SeqNum:        25,
					InflationDest: &creator,
					Thresholds:    xdr.Thresholds{0x1, 0x0, 0x0, 0x0},
				},
			},
		},
	}

	firstTx := createTransaction(true, 1, 1)
	firstTx.UnsafeMeta.V1.TxChanges = xdr.LedgerEntryChanges{ledgerEntryChange}
	secondTx := createTransaction(false, 3, 2)
	secondTx.UnsafeMeta.V2.TxChangesBefore = xdr.LedgerEntryChanges{ledgerEntryChange}
	secondTx.UnsafeMeta.V2.TxChangesAfter = xdr.LedgerEntryChanges{ledgerEntryChange}
	thirdTx := createTransaction(true, 4, 3)
	thirdTx.UnsafeMeta.V3.TxChangesBefore = xdr.LedgerEntryChanges{ledgerEntryChange}
	thirdTx.UnsafeMeta.V3.TxChangesAfter = xdr.LedgerEntryChanges{ledgerEntryChange}
	thirdTx.UnsafeMeta.V3.SorobanMeta = &xdr.SorobanTransactionMeta{}

	s.mockBatchInsertBuilder.On("Add", mock.AnythingOfType("ingest.LedgerTransaction"), sequence).Run(func(args mock.Arguments) {
		tx := args.Get(0).(ingest.LedgerTransaction)
		s.Assert().Len(tx.UnsafeMeta.V1.TxChanges, 0)
		s.Assert().Len(tx.UnsafeMeta.V1.Operations, 0)
	}).Return(nil).Once()

	sequence++
	s.mockBatchInsertBuilder.On("Add", mock.AnythingOfType("ingest.LedgerTransaction"), sequence).Run(func(args mock.Arguments) {
		tx := args.Get(0).(ingest.LedgerTransaction)
		s.Assert().Len(tx.UnsafeMeta.V2.TxChangesAfter, 0)
		s.Assert().Len(tx.UnsafeMeta.V2.TxChangesBefore, 0)
		s.Assert().Len(tx.UnsafeMeta.V2.Operations, 0)
	}).Return(nil).Once()

	sequence++
	s.mockBatchInsertBuilder.On("Add", mock.AnythingOfType("ingest.LedgerTransaction"), sequence).Run(func(args mock.Arguments) {
		tx := args.Get(0).(ingest.LedgerTransaction)
		s.Assert().Len(tx.UnsafeMeta.V3.TxChangesAfter, 0)
		s.Assert().Len(tx.UnsafeMeta.V3.TxChangesBefore, 0)
		s.Assert().Nil(tx.UnsafeMeta.V3.SorobanMeta)
		s.Assert().Len(tx.UnsafeMeta.V3.Operations, 0)
	}).Return(nil).Once()

	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	s.Assert().NoError(elidingTxProcessor.ProcessTransaction(lcm, firstTx))
	lcm.V0.LedgerHeader.Header.LedgerSeq++
	s.Assert().NoError(elidingTxProcessor.ProcessTransaction(lcm, secondTx))
	lcm.V0.LedgerHeader.Header.LedgerSeq++
	s.Assert().NoError(elidingTxProcessor.ProcessTransaction(lcm, thirdTx))

	s.Assert().NoError(elidingTxProcessor.Flush(s.ctx, s.mockSession))
}

func (s *TransactionsProcessorTestSuiteLedger) TestAddTransactionsWithSkippedMetaFails() {
	elidingTxProcessor := NewTransactionProcessor(s.mockBatchInsertBuilder, true)

	sequence := uint32(20)
	lcm := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}

	firstTx := createTransaction(true, 1, 1)
	// intentionally mangle the transaction to have an invalid tx meta version
	firstTx.UnsafeMeta.V = 8

	s.Assert().ErrorContains(elidingTxProcessor.ProcessTransaction(lcm, firstTx), "received an un-supported tx-meta version 8")
}

func (s *TransactionsProcessorTestSuiteLedger) TestAddTransactionsFails() {
	sequence := uint32(20)
	lcm := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}
	firstTx := createTransaction(true, 1, 2)
	s.mockBatchInsertBuilder.On("Add", firstTx, sequence).
		Return(errors.New("transient error")).Once()

	err := s.processor.ProcessTransaction(lcm, firstTx)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "Error batch inserting transaction rows: transient error")
}

func (s *TransactionsProcessorTestSuiteLedger) TestExecFails() {
	sequence := uint32(20)
	lcm := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}
	firstTx := createTransaction(true, 1, 2)

	s.mockBatchInsertBuilder.On("Add", firstTx, sequence).Return(nil).Once()
	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(errors.New("transient error")).Once()

	s.Assert().NoError(s.processor.ProcessTransaction(lcm, firstTx))

	err := s.processor.Flush(s.ctx, s.mockSession)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "transient error")
}

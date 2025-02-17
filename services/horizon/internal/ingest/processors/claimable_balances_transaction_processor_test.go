//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

type ClaimableBalancesTransactionProcessorTestSuiteLedger struct {
	suite.Suite
	ctx                               context.Context
	processor                         *ClaimableBalancesTransactionProcessor
	mockSession                       *db.MockSession
	mockTransactionBatchInsertBuilder *history.MockTransactionClaimableBalanceBatchInsertBuilder
	mockOperationBatchInsertBuilder   *history.MockOperationClaimableBalanceBatchInsertBuilder
	cbLoader                          *history.ClaimableBalanceLoader

	lcm xdr.LedgerCloseMeta
}

func TestClaimableBalancesTransactionProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(ClaimableBalancesTransactionProcessorTestSuiteLedger))
}

func (s *ClaimableBalancesTransactionProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockTransactionBatchInsertBuilder = &history.MockTransactionClaimableBalanceBatchInsertBuilder{}
	s.mockOperationBatchInsertBuilder = &history.MockOperationClaimableBalanceBatchInsertBuilder{}
	sequence := uint32(20)
	s.lcm = xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}
	s.cbLoader = history.NewClaimableBalanceLoader(history.ConcurrentInserts)

	s.processor = NewClaimableBalancesTransactionProcessor(
		s.cbLoader,
		s.mockTransactionBatchInsertBuilder,
		s.mockOperationBatchInsertBuilder,
	)
}

func (s *ClaimableBalancesTransactionProcessorTestSuiteLedger) TearDownTest() {
	s.mockTransactionBatchInsertBuilder.AssertExpectations(s.T())
	s.mockOperationBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *ClaimableBalancesTransactionProcessorTestSuiteLedger) TestEmptyClaimableBalances() {
	s.mockTransactionBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()
	s.mockOperationBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	s.Assert().NoError(s.processor.Flush(s.ctx, s.mockSession))
}

func (s *ClaimableBalancesTransactionProcessorTestSuiteLedger) testOperationInserts(balanceID xdr.ClaimableBalanceId, body xdr.OperationBody, change xdr.LedgerEntryChange) {
	// Setup the transaction
	txn := createTransaction(true, 1, 2)
	txn.Envelope.Operations()[0].Body = body
	txn.UnsafeMeta.V = 2
	txn.UnsafeMeta.V2.Operations = []xdr.OperationMeta{
		{Changes: xdr.LedgerEntryChanges{
			{
				Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
				State: &xdr.LedgerEntry{
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeClaimableBalance,
						ClaimableBalance: &xdr.ClaimableBalanceEntry{
							BalanceId: balanceID,
						},
					},
				},
			},
			change,
			// add a duplicate change to test that the processor
			// does not insert duplicate rows
			{
				Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
				State: &xdr.LedgerEntry{
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeClaimableBalance,
						ClaimableBalance: &xdr.ClaimableBalanceEntry{
							BalanceId: balanceID,
						},
					},
				},
			},
			change,
		}},
	}

	if body.Type == xdr.OperationTypeCreateClaimableBalance {
		// For insert test
		txn.Result.Result.Result.Results =
			&[]xdr.OperationResult{
				{
					Code: xdr.OperationResultCodeOpInner,
					Tr: &xdr.OperationResultTr{
						Type: xdr.OperationTypeCreateClaimableBalance,
						CreateClaimableBalanceResult: &xdr.CreateClaimableBalanceResult{
							Code:      xdr.CreateClaimableBalanceResultCodeCreateClaimableBalanceSuccess,
							BalanceId: &balanceID,
						},
					},
				},
			}
	}
	txnID := toid.New(int32(s.lcm.LedgerSequence()), int32(txn.Index), 0).ToInt64()
	opID := (&transactionOperationWrapper{
		index:          uint32(0),
		transaction:    txn,
		operation:      txn.Envelope.Operations()[0],
		ledgerSequence: s.lcm.LedgerSequence(),
	}).ID()

	hexID, err := xdr.MarshalHex(balanceID)
	s.Assert().NoError(err)

	s.mockTransactionBatchInsertBuilder.On("Add", txnID, s.cbLoader.GetFuture(hexID)).Return(nil).Once()
	s.mockTransactionBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	// Prepare to process operations successfully
	s.mockOperationBatchInsertBuilder.On("Add", opID, s.cbLoader.GetFuture(hexID)).Return(nil).Once()
	s.mockOperationBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	// Process the transaction
	err = s.processor.ProcessTransaction(s.lcm, txn)
	s.Assert().NoError(err)
	err = s.processor.Flush(s.ctx, s.mockSession)
	s.Assert().NoError(err)
}

func (s *ClaimableBalancesTransactionProcessorTestSuiteLedger) TestIngestClaimableBalancesInsertsClaimClaimableBalance() {
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	s.testOperationInserts(balanceID, xdr.OperationBody{
		Type: xdr.OperationTypeClaimClaimableBalance,
		ClaimClaimableBalanceOp: &xdr.ClaimClaimableBalanceOp{
			BalanceId: balanceID,
		},
	},
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved,
			Removed: &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeClaimableBalance,
				ClaimableBalance: &xdr.LedgerKeyClaimableBalance{
					BalanceId: balanceID,
				},
			},
		})
}

func (s *ClaimableBalancesTransactionProcessorTestSuiteLedger) TestIngestClaimableBalancesInsertsClawbackClaimableBalance() {
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	s.testOperationInserts(balanceID, xdr.OperationBody{
		Type: xdr.OperationTypeClawbackClaimableBalance,
		ClawbackClaimableBalanceOp: &xdr.ClawbackClaimableBalanceOp{
			BalanceId: balanceID,
		},
	}, xdr.LedgerEntryChange{
		Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
		Updated: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeClaimableBalance,
				ClaimableBalance: &xdr.ClaimableBalanceEntry{
					BalanceId: balanceID,
				},
			},
		},
	})
}

func (s *ClaimableBalancesTransactionProcessorTestSuiteLedger) TestIngestClaimableBalancesInsertsCreateClaimableBalance() {
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	s.testOperationInserts(balanceID, xdr.OperationBody{
		Type:                     xdr.OperationTypeCreateClaimableBalance,
		CreateClaimableBalanceOp: &xdr.CreateClaimableBalanceOp{},
	}, xdr.LedgerEntryChange{
		Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated,
		Created: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeClaimableBalance,
				ClaimableBalance: &xdr.ClaimableBalanceEntry{
					BalanceId: balanceID,
				},
			},
		},
	})
}

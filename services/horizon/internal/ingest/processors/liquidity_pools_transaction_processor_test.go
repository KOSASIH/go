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

type LiquidityPoolsTransactionProcessorTestSuiteLedger struct {
	suite.Suite
	ctx                               context.Context
	processor                         *LiquidityPoolsTransactionProcessor
	mockSession                       *db.MockSession
	lpLoader                          *history.LiquidityPoolLoader
	mockTransactionBatchInsertBuilder *history.MockTransactionLiquidityPoolBatchInsertBuilder
	mockOperationBatchInsertBuilder   *history.MockOperationLiquidityPoolBatchInsertBuilder

	lcm xdr.LedgerCloseMeta
}

func TestLiquidityPoolsTransactionProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(LiquidityPoolsTransactionProcessorTestSuiteLedger))
}

func (s *LiquidityPoolsTransactionProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockTransactionBatchInsertBuilder = &history.MockTransactionLiquidityPoolBatchInsertBuilder{}
	s.mockOperationBatchInsertBuilder = &history.MockOperationLiquidityPoolBatchInsertBuilder{}
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
	s.lpLoader = history.NewLiquidityPoolLoader(history.ConcurrentInserts)

	s.processor = NewLiquidityPoolsTransactionProcessor(
		s.lpLoader,
		s.mockTransactionBatchInsertBuilder,
		s.mockOperationBatchInsertBuilder,
	)
}

func (s *LiquidityPoolsTransactionProcessorTestSuiteLedger) TearDownTest() {
	s.mockTransactionBatchInsertBuilder.AssertExpectations(s.T())
	s.mockOperationBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *LiquidityPoolsTransactionProcessorTestSuiteLedger) TestEmptyLiquidityPools() {
	s.mockTransactionBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()
	s.mockOperationBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	err := s.processor.Flush(context.Background(), s.mockSession)
	s.Assert().NoError(err)
}

func (s *LiquidityPoolsTransactionProcessorTestSuiteLedger) testOperationInserts(poolID xdr.PoolId, body xdr.OperationBody, change xdr.LedgerEntryChange) {
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
						Type: xdr.LedgerEntryTypeLiquidityPool,
						LiquidityPool: &xdr.LiquidityPoolEntry{
							LiquidityPoolId: poolID,
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
						Type: xdr.LedgerEntryTypeLiquidityPool,
						LiquidityPool: &xdr.LiquidityPoolEntry{
							LiquidityPoolId: poolID,
						},
					},
				},
			},
			change,
		}},
	}

	if body.Type == xdr.OperationTypeChangeTrust {
		// For insert test
		txn.Result.Result.Result.Results =
			&[]xdr.OperationResult{
				{
					Code: xdr.OperationResultCodeOpInner,
					Tr: &xdr.OperationResultTr{
						Type: xdr.OperationTypeChangeTrust,
						ChangeTrustResult: &xdr.ChangeTrustResult{
							Code: xdr.ChangeTrustResultCodeChangeTrustSuccess,
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

	hexID := PoolIDToString(poolID)

	// Prepare to process transactions successfully
	s.mockTransactionBatchInsertBuilder.On("Add", txnID, s.lpLoader.GetFuture(hexID)).Return(nil).Once()
	s.mockTransactionBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	// Prepare to process operations successfully
	s.mockOperationBatchInsertBuilder.On("Add", opID, s.lpLoader.GetFuture(hexID)).Return(nil).Once()
	s.mockOperationBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	// Process the transaction
	err := s.processor.ProcessTransaction(s.lcm, txn)
	s.Assert().NoError(err)
	err = s.processor.Flush(s.ctx, s.mockSession)
	s.Assert().NoError(err)
}

func (s *LiquidityPoolsTransactionProcessorTestSuiteLedger) TestIngestLiquidityPoolsRemoval() {
	poolID := xdr.PoolId{0xca, 0xfe, 0xba, 0xbe}
	s.testOperationInserts(poolID,
		xdr.OperationBody{
			Type: xdr.OperationTypeLiquidityPoolDeposit,
			ChangeTrustOp: &xdr.ChangeTrustOp{
				Line: xdr.ChangeTrustAsset{
					Type: xdr.AssetTypeAssetTypePoolShare,
					LiquidityPool: &xdr.LiquidityPoolParameters{
						Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
						ConstantProduct: &xdr.LiquidityPoolConstantProductParameters{
							AssetA: xdr.MustNewCreditAsset("EUR", "GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB"),
							AssetB: xdr.MustNewNativeAsset(),
							Fee:    30,
						},
					},
				},
				Limit: 0,
			},
		},
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved,
			Removed: &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeLiquidityPool,
				LiquidityPool: &xdr.LedgerKeyLiquidityPool{
					LiquidityPoolId: poolID,
				},
			},
		},
	)
}

func (s *LiquidityPoolsTransactionProcessorTestSuiteLedger) TestIngestLiquidityPoolsUpdate() {
	poolID := xdr.PoolId{0xca, 0xfe, 0xba, 0xbe}
	s.testOperationInserts(poolID,
		xdr.OperationBody{
			Type: xdr.OperationTypeLiquidityPoolDeposit,
			ChangeTrustOp: &xdr.ChangeTrustOp{
				Line: xdr.ChangeTrustAsset{
					Type: xdr.AssetTypeAssetTypePoolShare,
					LiquidityPool: &xdr.LiquidityPoolParameters{
						Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
						ConstantProduct: &xdr.LiquidityPoolConstantProductParameters{
							AssetA: xdr.MustNewCreditAsset("EUR", "GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB"),
							AssetB: xdr.MustNewNativeAsset(),
							Fee:    30,
						},
					},
				},
				Limit: 10,
			},
		},
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
			Updated: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type: xdr.LedgerEntryTypeLiquidityPool,
					LiquidityPool: &xdr.LiquidityPoolEntry{
						LiquidityPoolId: poolID,
						Body:            xdr.LiquidityPoolEntryBody{},
					},
				},
			},
		},
	)
}

func (s *LiquidityPoolsTransactionProcessorTestSuiteLedger) TestIngestLiquidityPoolsCreate() {
	poolID := xdr.PoolId{0xca, 0xfe, 0xba, 0xbe}
	s.testOperationInserts(poolID,
		xdr.OperationBody{
			Type: xdr.OperationTypeLiquidityPoolDeposit,
			ChangeTrustOp: &xdr.ChangeTrustOp{
				Line: xdr.ChangeTrustAsset{
					Type: xdr.AssetTypeAssetTypePoolShare,
					LiquidityPool: &xdr.LiquidityPoolParameters{
						Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
						ConstantProduct: &xdr.LiquidityPoolConstantProductParameters{
							AssetA: xdr.MustNewCreditAsset("EUR", "GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB"),
							AssetB: xdr.MustNewNativeAsset(),
							Fee:    30,
						},
					},
				},
				Limit: 10,
			},
		},
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated,
			Created: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type: xdr.LedgerEntryTypeLiquidityPool,
					LiquidityPool: &xdr.LiquidityPoolEntry{
						LiquidityPoolId: poolID,
						Body:            xdr.LiquidityPoolEntryBody{},
					},
				},
			},
		},
	)
}

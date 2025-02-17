package ingest

import (
	"context"
	"fmt"
	"github.com/stellar/go/amount"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/network"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
)

func TestProcessorRunnerRunHistoryArchiveIngestionHistoryArchive(t *testing.T) {
	ctx := context.Background()

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	mockSession := &db.MockSession{}
	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)
	historyAdapter := &mockHistoryArchiveAdapter{}
	defer mock.AssertExpectationsForObjects(t, historyAdapter)

	m := &ingest.MockChangeReader{}
	m.On("Close").Return(nil).Once()
	bucketListHash := xdr.Hash([32]byte{0, 1, 2})
	m.On("VerifyBucketList", bucketListHash).Return(nil).Once()

	changeEntry := ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 1,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					// Master account address from Ledger 1
					AccountId: xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7"),
					// 100B
					Balance:    amount.MustParse("100000000000"),
					SeqNum:     0,
					Thresholds: xdr.Thresholds{1, 0, 0, 0},
				},
			},
		},
	}
	m.On("Read").Return(changeEntry, nil).Once()
	m.On("Read").Return(ingest.Change{}, io.EOF).Once()

	historyAdapter.
		On("GetState", ctx, uint32(63)).
		Return(
			m,
			nil,
		).Once()

	batchBuilders := mockChangeProcessorBatchBuilders(q, ctx, true)
	defer mock.AssertExpectationsForObjects(t, batchBuilders...)

	assert.IsType(t, &history.MockAccountSignersBatchInsertBuilder{}, batchBuilders[0])
	batchBuilders[0].(*history.MockAccountSignersBatchInsertBuilder).On("Add", history.AccountSigner{
		Account: "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7",
		Signer:  "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7",
		Weight:  1,
	}).Return(nil).Once()

	assert.IsType(t, &history.MockAccountsBatchInsertBuilder{}, batchBuilders[1])
	batchBuilders[1].(*history.MockAccountsBatchInsertBuilder).On("Add", history.AccountEntry{
		LastModifiedLedger: 1,
		AccountID:          "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7",
		Balance:            int64(1000000000000000000),
		SequenceNumber:     0,
		MasterWeight:       1,
	}).Return(nil).Once()

	q.MockQAssetStats.On("InsertAssetStats", ctx, []history.ExpAssetStat{}, 100000).
		Return(nil)

	runner := ProcessorRunner{
		ctx:            ctx,
		config:         config,
		historyQ:       q,
		historyAdapter: historyAdapter,
		filters:        &MockFilters{},
		session:        mockSession,
	}

	_, err := runner.RunHistoryArchiveIngestion(63, false, MaxSupportedProtocolVersion, bucketListHash)
	assert.NoError(t, err)
}

func TestProcessorRunnerRunHistoryArchiveIngestionProtocolVersionNotSupported(t *testing.T) {
	ctx := context.Background()

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	mockSession := &db.MockSession{}
	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)
	historyAdapter := &mockHistoryArchiveAdapter{}
	defer mock.AssertExpectationsForObjects(t, historyAdapter)

	// Batches
	defer mock.AssertExpectationsForObjects(t, mockChangeProcessorBatchBuilders(q, ctx, false)...)

	q.MockQAssetStats.On("InsertAssetStats", ctx, []history.ExpAssetStat{}, 100000).
		Return(nil)

	runner := ProcessorRunner{
		ctx:            ctx,
		config:         config,
		historyQ:       q,
		historyAdapter: historyAdapter,
		filters:        &MockFilters{},
		session:        mockSession,
	}

	_, err := runner.RunHistoryArchiveIngestion(100, false, 200, xdr.Hash{})
	assert.EqualError(t, err,
		fmt.Sprintf(
			"Error while checking for supported protocol version: This Horizon version does not support protocol version 200. The latest supported protocol version is %d. Please upgrade to the latest Horizon version.",
			MaxSupportedProtocolVersion,
		),
	)
}

func TestProcessorRunnerBuildChangeProcessor(t *testing.T) {
	ctx := context.Background()

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	defer mock.AssertExpectationsForObjects(t, mockChangeProcessorBatchBuilders(q, ctx, false)...)

	runner := ProcessorRunner{
		ctx:      ctx,
		historyQ: q,
		filters:  &MockFilters{},
	}

	stats := &ingest.StatsChangeProcessor{}
	processor := buildChangeProcessor(runner.historyQ, stats, ledgerSource, 123, "")
	assert.IsType(t, &groupChangeProcessors{}, processor)

	assert.IsType(t, &statsChangeProcessor{}, processor.processors[0])
	assert.IsType(t, &processors.AccountDataProcessor{}, processor.processors[1])
	assert.IsType(t, &processors.AccountsProcessor{}, processor.processors[2])
	assert.IsType(t, &processors.OffersProcessor{}, processor.processors[3])
	assert.IsType(t, &processors.AssetStatsProcessor{}, processor.processors[4])
	assert.False(t, reflect.ValueOf(processor.processors[4]).
		Elem().FieldByName("ingestFromHistoryArchive").Bool())
	assert.IsType(t, &processors.SignersProcessor{}, processor.processors[5])
	assert.IsType(t, &processors.TrustLinesProcessor{}, processor.processors[6])

	runner = ProcessorRunner{
		ctx:      ctx,
		historyQ: q,
		filters:  &MockFilters{},
	}

	processor = buildChangeProcessor(runner.historyQ, stats, historyArchiveSource, 456, "")
	assert.IsType(t, &groupChangeProcessors{}, processor)

	assert.IsType(t, &statsChangeProcessor{}, processor.processors[0])
	assert.IsType(t, &processors.AccountDataProcessor{}, processor.processors[1])
	assert.IsType(t, &processors.AccountsProcessor{}, processor.processors[2])
	assert.IsType(t, &processors.OffersProcessor{}, processor.processors[3])
	assert.IsType(t, &processors.AssetStatsProcessor{}, processor.processors[4])
	assert.True(t, reflect.ValueOf(processor.processors[4]).
		Elem().FieldByName("ingestFromHistoryArchive").Bool())
	assert.IsType(t, &processors.SignersProcessor{}, processor.processors[5])
	assert.IsType(t, &processors.TrustLinesProcessor{}, processor.processors[6])
}

func TestProcessorRunnerBuildTransactionProcessor(t *testing.T) {
	ctx := context.Background()

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	q.MockQTransactions.On("NewTransactionBatchInsertBuilder").
		Return(&history.MockTransactionsBatchInsertBuilder{})
	q.On("NewTradeBatchInsertBuilder").Return(&history.MockTradeBatchInsertBuilder{})
	q.MockQLedgers.On("NewLedgerBatchInsertBuilder").
		Return(&history.MockLedgersBatchInsertBuilder{})
	q.MockQEffects.On("NewEffectBatchInsertBuilder").
		Return(&history.MockEffectBatchInsertBuilder{})
	q.MockQOperations.On("NewOperationBatchInsertBuilder").
		Return(&history.MockOperationsBatchInsertBuilder{})
	q.On("NewTransactionParticipantsBatchInsertBuilder").
		Return(&history.MockTransactionParticipantsBatchInsertBuilder{})
	q.On("NewOperationParticipantBatchInsertBuilder").
		Return(&history.MockOperationParticipantBatchInsertBuilder{})
	q.MockQHistoryClaimableBalances.On("NewTransactionClaimableBalanceBatchInsertBuilder").
		Return(&history.MockTransactionClaimableBalanceBatchInsertBuilder{})
	q.MockQHistoryClaimableBalances.On("NewOperationClaimableBalanceBatchInsertBuilder").
		Return(&history.MockOperationClaimableBalanceBatchInsertBuilder{})
	q.MockQHistoryLiquidityPools.On("NewTransactionLiquidityPoolBatchInsertBuilder").
		Return(&history.MockTransactionLiquidityPoolBatchInsertBuilder{})
	q.MockQHistoryLiquidityPools.On("NewOperationLiquidityPoolBatchInsertBuilder").
		Return(&history.MockOperationLiquidityPoolBatchInsertBuilder{})

	runner := ProcessorRunner{
		ctx:      ctx,
		config:   Config{},
		historyQ: q,
	}

	ledgersProcessor := &processors.LedgersProcessor{}

	_, processor := runner.buildTransactionProcessor(ledgersProcessor, history.ConcurrentInserts)
	assert.IsType(t, &groupTransactionProcessors{}, processor)
	assert.IsType(t, &processors.StatsLedgerTransactionProcessor{}, processor.processors[0])
	assert.IsType(t, &processors.EffectProcessor{}, processor.processors[1])
	assert.IsType(t, &processors.LedgersProcessor{}, processor.processors[2])
	assert.IsType(t, &processors.OperationProcessor{}, processor.processors[3])
	assert.IsType(t, &processors.TradeProcessor{}, processor.processors[4])
	assert.IsType(t, &processors.ParticipantsProcessor{}, processor.processors[5])
	assert.IsType(t, &processors.ClaimableBalancesTransactionProcessor{}, processor.processors[7])
	assert.IsType(t, &processors.LiquidityPoolsTransactionProcessor{}, processor.processors[8])
}

func TestProcessorRunnerRunAllProcessorsOnLedger(t *testing.T) {
	ctx := context.Background()

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	mockSession := &db.MockSession{}
	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	ledger := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					BucketListHash: xdr.Hash([32]byte{0, 1, 2}),
					LedgerSeq:      23,
				},
			},
		},
	}

	// Batches
	defer mock.AssertExpectationsForObjects(t, mockTxProcessorBatchBuilders(q, mockSession, ctx)...)
	defer mock.AssertExpectationsForObjects(t, mockChangeProcessorBatchBuilders(q, ctx, true)...)
	defer mock.AssertExpectationsForObjects(t, mockFilteredOutProcessorsForNoRules(q, mockSession, ctx)...)

	mockBatchInsertBuilder := &history.MockLedgersBatchInsertBuilder{}
	q.MockQLedgers.On("NewLedgerBatchInsertBuilder").Return(mockBatchInsertBuilder)
	mockBatchInsertBuilder.On(
		"Add",
		ledger.V0.LedgerHeader, 0, 0, 0, 0, CurrentVersion).Return(nil).Once()
	mockBatchInsertBuilder.On(
		"Exec",
		ctx,
		mockSession,
	).Return(nil).Once()

	defer mock.AssertExpectationsForObjects(t, mockBatchInsertBuilder)

	q.MockQAssetStats.On("RemoveContractAssetBalances", ctx, []xdr.Hash(nil)).
		Return(nil).Once()
	q.MockQAssetStats.On("UpdateContractAssetBalanceAmounts", ctx, []xdr.Hash{}, []string{}).
		Return(nil).Once()
	q.MockQAssetStats.On("InsertContractAssetBalances", ctx, []history.ContractAssetBalance(nil)).
		Return(nil).Once()
	q.MockQAssetStats.On("UpdateContractAssetBalanceExpirations", ctx, []xdr.Hash{}, []uint32{}).
		Return(nil).Once()
	q.MockQAssetStats.On("GetContractAssetBalancesExpiringAt", ctx, uint32(22)).
		Return([]history.ContractAssetBalance{}, nil).Once()

	runner := ProcessorRunner{
		ctx:      ctx,
		config:   config,
		historyQ: q,
		session:  mockSession,
		filters:  &MockFilters{},
	}

	_, err := runner.RunAllProcessorsOnLedger(ledger)
	assert.NoError(t, err)
}

func TestProcessorRunnerRunTransactionsProcessorsOnLedgers(t *testing.T) {
	ctx := context.Background()

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	mockSession := &db.MockSession{}
	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	ledgers := []xdr.LedgerCloseMeta{{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					BucketListHash: xdr.Hash([32]byte{0, 1, 2}),
					LedgerSeq:      xdr.Uint32(1),
				},
			},
		},
	},
		{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						BucketListHash: xdr.Hash([32]byte{3, 4, 5}),
						LedgerSeq:      xdr.Uint32(2),
					},
				},
			},
		},
		{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						BucketListHash: xdr.Hash([32]byte{6, 7, 8}),
						LedgerSeq:      xdr.Uint32(3),
					},
				},
			},
		},
	}

	// filtered out processor should not be created
	q.MockQTransactions.AssertNotCalled(t, "NewTransactionFilteredTmpBatchInsertBuilder")
	q.AssertNotCalled(t, "DeleteTransactionsFilteredTmpOlderThan", ctx, mock.AnythingOfType("uint64"))

	// Batches
	defer mock.AssertExpectationsForObjects(t, mockTxProcessorBatchBuilders(q, mockSession, ctx)...)

	mockBatchInsertBuilder := &history.MockLedgersBatchInsertBuilder{}
	q.MockQLedgers.On("NewLedgerBatchInsertBuilder").Return(mockBatchInsertBuilder)
	mockBatchInsertBuilder.On(
		"Add",
		ledgers[0].V0.LedgerHeader, 0, 0, 0, 0, CurrentVersion).Return(nil).Once()
	mockBatchInsertBuilder.On(
		"Add",
		ledgers[1].V0.LedgerHeader, 0, 0, 0, 0, CurrentVersion).Return(nil).Once()
	mockBatchInsertBuilder.On(
		"Add",
		ledgers[2].V0.LedgerHeader, 0, 0, 0, 0, CurrentVersion).Return(nil).Once()

	mockBatchInsertBuilder.On(
		"Exec",
		ctx,
		mockSession,
	).Return(nil).Once()

	defer mock.AssertExpectationsForObjects(t, mockBatchInsertBuilder)

	q.MockQAssetStats.On("RemoveContractAssetBalances", ctx, []xdr.Hash(nil)).
		Return(nil).Once()
	q.MockQAssetStats.On("UpdateContractAssetBalanceAmounts", ctx, []xdr.Hash{}, []string{}).
		Return(nil).Once()
	q.MockQAssetStats.On("InsertContractAssetBalances", ctx, []history.ContractAssetBalance(nil)).
		Return(nil).Once()
	q.MockQAssetStats.On("UpdateContractAssetBalanceExpirations", ctx, []xdr.Hash{}, []uint32{}).
		Return(nil).Once()
	q.MockQAssetStats.On("GetContractAssetBalancesExpiringAt", ctx, uint32(22)).
		Return([]history.ContractAssetBalance{}, nil).Once()

	runner := ProcessorRunner{
		ctx:      ctx,
		config:   config,
		historyQ: q,
		session:  mockSession,
		filters:  &MockFilters{},
	}

	err := runner.RunTransactionProcessorsOnLedgers(ledgers, false)
	assert.NoError(t, err)
}

func TestProcessorRunnerRunAllProcessorsOnLedgerProtocolVersionNotSupported(t *testing.T) {
	ctx := context.Background()
	maxBatchSize := 100000

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	ledger := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerVersion: 200,
				},
			},
		},
	}

	// Batches
	mockTransactionsBatchInsertBuilder := &history.MockTransactionsBatchInsertBuilder{}
	q.MockQTransactions.On("NewTransactionBatchInsertBuilder", maxBatchSize).
		Return(mockTransactionsBatchInsertBuilder).Twice()

	mockAccountSignersBatchInsertBuilder := &history.MockAccountSignersBatchInsertBuilder{}
	q.MockQSigners.On("NewAccountSignersBatchInsertBuilder").
		Return(mockAccountSignersBatchInsertBuilder).Twice()

	mockOperationsBatchInsertBuilder := &history.MockOperationsBatchInsertBuilder{}
	q.MockQOperations.On("NewOperationBatchInsertBuilder").
		Return(mockOperationsBatchInsertBuilder).Twice()

	defer mock.AssertExpectationsForObjects(t, mockTransactionsBatchInsertBuilder,
		mockAccountSignersBatchInsertBuilder,
		mockOperationsBatchInsertBuilder)

	runner := ProcessorRunner{
		ctx:      ctx,
		config:   config,
		historyQ: q,
		filters:  &MockFilters{},
	}

	_, err := runner.RunAllProcessorsOnLedger(ledger)
	assert.EqualError(t, err,
		fmt.Sprintf(
			"Error while checking for supported protocol version: This Horizon version does not support protocol version 200. The latest supported protocol version is %d. Please upgrade to the latest Horizon version.",
			MaxSupportedProtocolVersion,
		),
	)
}

func mockTxProcessorBatchBuilders(q *mockDBQ, mockSession *db.MockSession, ctx context.Context) []interface{} {
	// no mocking of builder Add methods needed, the fake ledgers used in tests don't have any operations
	// that would trigger the respective processors to invoke Add, each test locally decides to use
	// MockLedgersBatchInsertBuilder with asserts on Add invocations, as those are fired once per ledger.
	mockTradeBatchInsertBuilder := &history.MockTradeBatchInsertBuilder{}
	q.On("NewTradeBatchInsertBuilder").Return(mockTradeBatchInsertBuilder).Once()

	mockTransactionsBatchInsertBuilder := &history.MockTransactionsBatchInsertBuilder{}
	mockTransactionsBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.MockQTransactions.On("NewTransactionBatchInsertBuilder").
		Return(mockTransactionsBatchInsertBuilder).Once()

	mockOperationsBatchInsertBuilder := &history.MockOperationsBatchInsertBuilder{}
	mockOperationsBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.MockQOperations.On("NewOperationBatchInsertBuilder").
		Return(mockOperationsBatchInsertBuilder).Once()

	mockEffectBatchInsertBuilder := &history.MockEffectBatchInsertBuilder{}
	mockEffectBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.MockQEffects.On("NewEffectBatchInsertBuilder").
		Return(mockEffectBatchInsertBuilder).Once()

	mockTransactionsParticipantsBatchInsertBuilder := &history.MockTransactionParticipantsBatchInsertBuilder{}
	mockTransactionsParticipantsBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.On("NewTransactionParticipantsBatchInsertBuilder").
		Return(mockTransactionsParticipantsBatchInsertBuilder).Once()

	mockOperationParticipantBatchInsertBuilder := &history.MockOperationParticipantBatchInsertBuilder{}
	mockOperationParticipantBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.On("NewOperationParticipantBatchInsertBuilder").
		Return(mockOperationParticipantBatchInsertBuilder).Once()

	mockTransactionClaimableBalanceBatchInsertBuilder := &history.MockTransactionClaimableBalanceBatchInsertBuilder{}
	mockTransactionClaimableBalanceBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.MockQHistoryClaimableBalances.On("NewTransactionClaimableBalanceBatchInsertBuilder").
		Return(mockTransactionClaimableBalanceBatchInsertBuilder).Once()

	mockOperationClaimableBalanceBatchInsertBuilder := &history.MockOperationClaimableBalanceBatchInsertBuilder{}
	mockOperationClaimableBalanceBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.MockQHistoryClaimableBalances.On("NewOperationClaimableBalanceBatchInsertBuilder").
		Return(mockOperationClaimableBalanceBatchInsertBuilder).Once()

	mockTransactionLiquidityPoolBatchInsertBuilder := &history.MockTransactionLiquidityPoolBatchInsertBuilder{}
	mockTransactionLiquidityPoolBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.MockQHistoryLiquidityPools.On("NewTransactionLiquidityPoolBatchInsertBuilder").
		Return(mockTransactionLiquidityPoolBatchInsertBuilder).Once()

	mockOperationLiquidityPoolBatchInsertBuilder := &history.MockOperationLiquidityPoolBatchInsertBuilder{}
	mockOperationLiquidityPoolBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.MockQHistoryLiquidityPools.On("NewOperationLiquidityPoolBatchInsertBuilder").
		Return(mockOperationLiquidityPoolBatchInsertBuilder).Once()

	return []interface{}{mockTradeBatchInsertBuilder,
		mockTransactionsBatchInsertBuilder,
		mockOperationsBatchInsertBuilder,
		mockEffectBatchInsertBuilder,
		mockTransactionsParticipantsBatchInsertBuilder,
		mockOperationParticipantBatchInsertBuilder,
		mockTransactionClaimableBalanceBatchInsertBuilder,
		mockOperationClaimableBalanceBatchInsertBuilder,
		mockTransactionLiquidityPoolBatchInsertBuilder,
		mockOperationLiquidityPoolBatchInsertBuilder}
}

func mockChangeProcessorBatchBuilders(q *mockDBQ, ctx context.Context, mockExec bool) []interface{} {
	mockAccountSignersBatchInsertBuilder := &history.MockAccountSignersBatchInsertBuilder{}
	mockAccountSignersBatchInsertBuilder.On("Len").Return(1).Maybe()
	if mockExec {
		mockAccountSignersBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	}
	q.MockQSigners.On("NewAccountSignersBatchInsertBuilder").
		Return(mockAccountSignersBatchInsertBuilder).Twice()

	mockAccountsBatchInsertBuilder := &history.MockAccountsBatchInsertBuilder{}
	mockAccountsBatchInsertBuilder.On("Len").Return(1).Maybe()
	if mockExec {
		mockAccountsBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	}
	q.MockQAccounts.On("NewAccountsBatchInsertBuilder").
		Return(mockAccountsBatchInsertBuilder).Twice()

	mockClaimableBalanceClaimantBatchInsertBuilder := &history.MockClaimableBalanceClaimantBatchInsertBuilder{}
	if mockExec {
		mockClaimableBalanceClaimantBatchInsertBuilder.On("Exec", ctx).
			Return(nil).Once()
	}
	q.MockQClaimableBalances.On("NewClaimableBalanceClaimantBatchInsertBuilder").
		Return(mockClaimableBalanceClaimantBatchInsertBuilder).Twice()

	mockClaimableBalanceBatchInsertBuilder := &history.MockClaimableBalanceBatchInsertBuilder{}
	if mockExec {
		mockClaimableBalanceBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	}
	q.MockQClaimableBalances.On("NewClaimableBalanceBatchInsertBuilder").
		Return(mockClaimableBalanceBatchInsertBuilder).Twice()

	mockOfferBatchInsertBuilder := &history.MockOffersBatchInsertBuilder{}
	if mockExec {
		mockOfferBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	}
	q.MockQOffers.On("NewOffersBatchInsertBuilder").
		Return(mockOfferBatchInsertBuilder).Twice()

	mockAccountDataBatchInsertBuilder := &history.MockAccountDataBatchInsertBuilder{}
	if mockExec {
		mockAccountDataBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	}
	q.MockQData.On("NewAccountDataBatchInsertBuilder").
		Return(mockAccountDataBatchInsertBuilder).Twice()

	mockTrustLinesBatchInsertBuilder := &history.MockTrustLinesBatchInsertBuilder{}
	if mockExec {
		mockTrustLinesBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	}
	q.MockQTrustLines.On("NewTrustLinesBatchInsertBuilder").
		Return(mockTrustLinesBatchInsertBuilder)

	return []interface{}{mockAccountSignersBatchInsertBuilder,
		mockAccountsBatchInsertBuilder,
		mockClaimableBalanceBatchInsertBuilder,
		mockClaimableBalanceClaimantBatchInsertBuilder,
		mockOfferBatchInsertBuilder,
		mockAccountDataBatchInsertBuilder,
		mockTrustLinesBatchInsertBuilder,
	}
}

func mockFilteredOutProcessorsForNoRules(q *mockDBQ, mockSession *db.MockSession, ctx context.Context) []interface{} {
	mockTransactionsFilteredTmpBatchInsertBuilder := &history.MockTransactionsBatchInsertBuilder{}
	// since no filter rules are used on tests in this suite, we do not need to mock the "Add" call
	// the "Exec" call gets run by flush all the time
	mockTransactionsFilteredTmpBatchInsertBuilder.On("Exec", ctx, mockSession).Return(nil).Once()
	q.MockQTransactions.On("NewTransactionFilteredTmpBatchInsertBuilder").
		Return(mockTransactionsFilteredTmpBatchInsertBuilder)
	q.On("DeleteTransactionsFilteredTmpOlderThan", ctx, mock.AnythingOfType("uint64")).
		Return(int64(0), nil)

	return []interface{}{
		mockTransactionsFilteredTmpBatchInsertBuilder,
	}
}

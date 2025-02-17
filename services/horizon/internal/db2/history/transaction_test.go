package history

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/guregu/null"

	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/xdr"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/toid"
)

func TestTransactionQueries(t *testing.T) {
	tt := test.Start(t)
	test.ResetHorizonDB(t, tt.HorizonDB)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	// Test TransactionByHash
	var tx Transaction
	real := "2374e99349b9ef7dba9a5db3339b78fda8f34777b1af33ba468ad5c0df946d4d"
	err := q.TransactionByHash(tt.Ctx, &tx, real)
	tt.Assert.NoError(err)

	fake := "not_real"
	err = q.TransactionByHash(tt.Ctx, &tx, fake)
	tt.Assert.Equal(err, sql.ErrNoRows)
}

func TestTransactionByLiquidityPool(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	txIndex := int32(1)
	sequence := int32(56)
	txID := toid.New(sequence, int32(1), 0).ToInt64()

	// Insert a phony ledger
	ledgerCloseTime := time.Now().Unix()
	ledgerBatch := q.NewLedgerBatchInsertBuilder()
	err := ledgerBatch.Add(xdr.LedgerHeaderHistoryEntry{
		Header: xdr.LedgerHeader{
			LedgerSeq: xdr.Uint32(sequence),
			ScpValue: xdr.StellarValue{
				CloseTime: xdr.TimePoint(ledgerCloseTime),
			},
		},
	}, 0, 0, 0, 0, 0)
	tt.Assert.NoError(err)

	tt.Assert.NoError(q.Begin(tt.Ctx))

	tt.Assert.NoError(ledgerBatch.Exec(tt.Ctx, q.SessionInterface))

	// Insert a phony transaction
	transactionBuilder := q.NewTransactionBatchInsertBuilder()
	firstTransaction := buildLedgerTransaction(tt.T, testTransaction{
		index:         uint32(txIndex),
		envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
		resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
		feeChangesXDR: "AAAAAA==",
		metaXDR:       "AAAAAQAAAAAAAAAA",
		hash:          "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
	})
	err = transactionBuilder.Add(firstTransaction, uint32(sequence))
	tt.Assert.NoError(err)
	err = transactionBuilder.Exec(tt.Ctx, q)
	tt.Assert.NoError(err)

	// Insert Liquidity Pool history
	liquidityPoolID := "a2f38836a839de008cf1d782c81f45e1253cc5d3dad9110b872965484fec0a49"
	lpLoader := NewLiquidityPoolLoader(ConcurrentInserts)
	lpTransactionBuilder := q.NewTransactionLiquidityPoolBatchInsertBuilder()
	tt.Assert.NoError(lpTransactionBuilder.Add(txID, lpLoader.GetFuture(liquidityPoolID)))
	tt.Assert.NoError(lpLoader.Exec(tt.Ctx, q))
	tt.Assert.NoError(lpTransactionBuilder.Exec(tt.Ctx, q))
	tt.Assert.NoError(q.Commit())

	var records []Transaction
	tt.Assert.NoError(
		q.Transactions().ForLiquidityPool(tt.Ctx, liquidityPoolID).Select(tt.Ctx, &records),
	)
	tt.Assert.Len(records, 1)

}

// TestTransactionSuccessfulOnly tests if default query returns successful
// transactions only.
// If it's not enclosed in brackets, it may return incorrect result when mixed
// with `ForAccount` or `ForLedger` filters.
func TestTransactionSuccessfulOnly(t *testing.T) {
	tt := test.Start(t)
	test.ResetHorizonDB(t, tt.HorizonDB)
	tt.Scenario("failed_transactions")
	defer tt.Finish()

	var transactions []Transaction

	q := &Q{tt.HorizonSession()}
	query := q.Transactions().
		ForAccount(tt.Ctx, "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2")

	err := query.Select(tt.Ctx, &transactions)
	tt.Assert.NoError(err)

	tt.Assert.Equal(3, len(transactions))

	for _, transaction := range transactions {
		tt.Assert.True(transaction.Successful)
	}

	sql, _, err := query.sql.ToSql()
	tt.Assert.NoError(err)
	// Note: brackets around `(ht.successful = true OR ht.successful IS NULL)` are critical!
	tt.Assert.Contains(sql, "WHERE htp.history_account_id = ? AND (ht.successful = true OR ht.successful IS NULL)")
}

// TestTransactionIncludeFailed tests `IncludeFailed` method.
func TestTransactionIncludeFailed(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("failed_transactions")
	defer tt.Finish()

	var transactions []Transaction

	q := &Q{tt.HorizonSession()}
	query := q.Transactions().
		ForAccount(tt.Ctx, "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2").
		IncludeFailed()

	err := query.Select(tt.Ctx, &transactions)
	tt.Assert.NoError(err)

	var failed, successful int
	for _, transaction := range transactions {
		if transaction.Successful {
			successful++
		} else {
			failed++
		}
	}

	tt.Assert.Equal(3, successful)
	tt.Assert.Equal(1, failed)

	sql, _, err := query.sql.ToSql()
	tt.Assert.NoError(err)
	tt.Assert.Equal("SELECT ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures FROM history_transactions ht LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence JOIN history_transaction_participants htp ON htp.history_transaction_id = ht.id WHERE htp.history_account_id = ?", sql)
}

func TestExtraChecksTransactionSuccessfulTrueResultFalse(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("failed_transactions")
	defer tt.Finish()

	// successful `true` but tx result `false`
	_, err := tt.HorizonDB.Exec(
		`UPDATE history_transactions SET successful = true WHERE transaction_hash = 'aa168f12124b7c196c0adaee7c73a64d37f99428cacb59a91ff389626845e7cf'`,
	)
	tt.Require.NoError(err)

	var transactions []Transaction

	q := &Q{tt.HorizonSession()}
	query := q.Transactions().
		ForAccount(tt.Ctx, "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2").
		IncludeFailed()

	err = query.Select(tt.Ctx, &transactions)
	tt.Assert.Error(err)
	tt.Assert.Contains(err.Error(), "Corrupted data! `successful=true` but returned transaction is not success")
}

func TestExtraChecksTransactionSuccessfulFalseResultTrue(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("failed_transactions")
	defer tt.Finish()

	// successful `false` but tx result `true`
	_, err := tt.HorizonDB.Exec(
		`UPDATE history_transactions SET successful = false WHERE transaction_hash = 'a2dabf4e9d1642722602272e178a37c973c9177b957da86192a99b3e9f3a9aa4'`,
	)
	tt.Require.NoError(err)

	var transactions []Transaction

	q := &Q{tt.HorizonSession()}
	query := q.Transactions().
		ForAccount(tt.Ctx, "GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON").
		IncludeFailed()

	err = query.Select(tt.Ctx, &transactions)
	tt.Assert.Error(err)
	tt.Assert.Contains(err.Error(), "Corrupted data! `successful=false` but returned transaction is success")
}

func TestInsertTransactionDoesNotAllowDuplicateIndex(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.Begin(tt.Ctx))

	sequence := uint32(123)
	insertBuilder := q.NewTransactionBatchInsertBuilder()

	firstTransaction := buildLedgerTransaction(tt.T, testTransaction{
		index:         1,
		envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
		resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
		feeChangesXDR: "AAAAAA==",
		metaXDR:       "AAAAAQAAAAAAAAAA",
		hash:          "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
	})
	secondTransaction := buildLedgerTransaction(tt.T, testTransaction{
		index:         1,
		envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAIAAAAAAAAAewAAAAEAAAAAAAAACwEXUhsAAFfhAAAAAAAAAAA=",
		resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
		feeChangesXDR: "AAAAAA==",
		metaXDR:       "AAAAAQAAAAAAAAAA",
		hash:          "7e2def20d5a21a56be2a457b648f702ee1af889d3df65790e92a05081e9fabf1",
	})

	tt.Assert.NoError(insertBuilder.Add(firstTransaction, sequence))
	tt.Assert.NoError(insertBuilder.Exec(tt.Ctx, q))
	tt.Assert.NoError(q.Commit())

	tt.Assert.NoError(q.Begin(tt.Ctx))
	insertBuilder = q.NewTransactionBatchInsertBuilder()
	tt.Assert.NoError(insertBuilder.Add(secondTransaction, sequence))
	tt.Assert.EqualError(
		insertBuilder.Exec(tt.Ctx, q),
		"pq: duplicate key value violates unique constraint \"hs_transaction_by_id\"",
	)
	tt.Assert.NoError(q.Rollback())

	ledger := Ledger{
		Sequence:                   int32(sequence),
		LedgerHash:                 "4db1e4f145e9ee75162040d26284795e0697e2e84084624e7c6c723ebbf80118",
		PreviousLedgerHash:         null.NewString("4b0b8bace3b2438b2404776ce57643966855487ba6384724a3c664c7aa4cd9e4", true),
		TotalOrderID:               TotalOrderID{toid.New(int32(69859), 0, 0).ToInt64()},
		ImporterVersion:            321,
		TransactionCount:           12,
		SuccessfulTransactionCount: new(int32),
		FailedTransactionCount:     new(int32),
		OperationCount:             23,
		TotalCoins:                 23451,
		FeePool:                    213,
		BaseReserve:                687,
		MaxTxSetSize:               345,
		ProtocolVersion:            12,
		BaseFee:                    100,
		ClosedAt:                   time.Now().UTC().Truncate(time.Second),
		LedgerHeaderXDR:            null.NewString("temp", true),
	}
	*ledger.SuccessfulTransactionCount = 12
	*ledger.FailedTransactionCount = 3
	_, err := q.Exec(tt.Ctx, sq.Insert("history_ledgers").SetMap(ledgerToMap(ledger)))
	tt.Assert.NoError(err)

	var transactions []Transaction
	tt.Assert.NoError(q.Transactions().Select(tt.Ctx, &transactions))
	tt.Assert.Len(transactions, 1)
	tt.Assert.Equal(
		"19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
		transactions[0].TransactionHash,
	)
}

func TestInsertTransaction(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	sequence := uint32(123)
	ledger := Ledger{
		Sequence:                   int32(sequence),
		LedgerHash:                 "4db1e4f145e9ee75162040d26284795e0697e2e84084624e7c6c723ebbf80118",
		PreviousLedgerHash:         null.NewString("4b0b8bace3b2438b2404776ce57643966855487ba6384724a3c664c7aa4cd9e4", true),
		TotalOrderID:               TotalOrderID{toid.New(int32(69859), 0, 0).ToInt64()},
		ImporterVersion:            321,
		TransactionCount:           12,
		SuccessfulTransactionCount: new(int32),
		FailedTransactionCount:     new(int32),
		OperationCount:             23,
		TotalCoins:                 23451,
		FeePool:                    213,
		BaseReserve:                687,
		MaxTxSetSize:               345,
		ProtocolVersion:            12,
		BaseFee:                    100,
		ClosedAt:                   time.Now().UTC().Truncate(time.Second),
		LedgerHeaderXDR:            null.NewString("temp", true),
	}
	*ledger.SuccessfulTransactionCount = 12
	*ledger.FailedTransactionCount = 3
	_, err := q.Exec(tt.Ctx, sq.Insert("history_ledgers").SetMap(ledgerToMap(ledger)))
	tt.Assert.NoError(err)

	success := true

	emptySignatures := []string{}
	var nullSignatures []string

	nullTimeBounds := TimeBounds{Null: true}

	infiniteTimeBounds := TimeBounds{Lower: null.IntFrom(0)}
	timeBoundWithMin := TimeBounds{Lower: null.IntFrom(1576195867)}
	timeBoundWithMax := TimeBounds{Lower: null.IntFrom(0), Upper: null.IntFrom(1576195867)}
	timeboundsWithMinAndMax := TimeBounds{Lower: null.IntFrom(1576095867), Upper: null.IntFrom(1576195867)}
	v2TimeboundsWithMinAndMax := TimeBounds{Lower: null.IntFrom(0), Upper: null.IntFrom(1648153609)}
	v2LedgerboundsWithMinAndMax := LedgerBounds{MinLedger: null.IntFrom(0), MaxLedger: null.IntFrom(1)}

	withMultipleSignatures := []string{
		"MID8kIOLP/yEymCyhU7A/YeVpnVTDzAqszWtv8c+/qAw542BaKWxCJxl/jsggY0mF+SR8X0bvWXvPBgyYcDZDw==",
		"J0J8qTsKREW29GAmZMXXBTVkYKkGbOk1AUPUalbIiDdDjd8mpIIdMStqo9w+k5A8UKRTm/iO2V/riQ14CF9IAg==",
	}

	withSingleSignature := []string{
		"MID8kIOLP/yEymCyhU7A/YeVpnVTDzAqszWtv8c+/qAw542BaKWxCJxl/jsggY0mF+SR8X0bvWXvPBgyYcDZDw==",
	}

	for _, testCase := range []struct {
		name     string
		toInsert ingest.LedgerTransaction
		expected Transaction
	}{
		{
			"successful transaction without signatures",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  78621794419880145,
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					Successful:       success,
					TimeBounds:       nullTimeBounds,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
				},
			},
		},
		{
			"successful transaction with multiple signatures",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAACQmz0pAAAAEAwgPyQg4s//ITKYLKFTsD9h5WmdVMPMCqzNa2/xz7+oDDnjYFopbEInGX+OyCBjSYX5JHxfRu9Ze88GDJhwNkPto+xlgAAAEAnQnypOwpERbb0YCZkxdcFNWRgqQZs6TUBQ9RqVsiIN0ON3yakgh0xK2qj3D6TkDxQpFOb+I7ZX+uJDXgIX0gC",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  78621794419880145,
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAACQmz0pAAAAEAwgPyQg4s//ITKYLKFTsD9h5WmdVMPMCqzNa2/xz7+oDDnjYFopbEInGX+OyCBjSYX5JHxfRu9Ze88GDJhwNkPto+xlgAAAEAnQnypOwpERbb0YCZkxdcFNWRgqQZs6TUBQ9RqVsiIN0ON3yakgh0xK2qj3D6TkDxQpFOb+I7ZX+uJDXgIX0gC",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       withMultipleSignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       nullTimeBounds,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
		{
			"failed transaction",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAABQmz0pAAAAEAwgPyQg4s//ITKYLKFTsD9h5WmdVMPMCqzNa2/xz7+oDDnjYFopbEInGX+OyCBjSYX5JHxfRu9Ze88GDJhwNkP",
				resultXDR:     "AAAAAAAAAHv////6AAAAAA==",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  78621794419880145,
					MaxFee:           200,
					FeeCharged:       123,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAABQmz0pAAAAEAwgPyQg4s//ITKYLKFTsD9h5WmdVMPMCqzNa2/xz7+oDDnjYFopbEInGX+OyCBjSYX5JHxfRu9Ze88GDJhwNkP",
					TxResult:         "AAAAAAAAAHv////6AAAAAA==",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       withSingleSignature,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       nullTimeBounds,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       false,
				},
			},
		},
		{
			"transaction with 64 bit fee charged",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rlQL5AAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAABAAAACXRlc3QgbWVtbwAAAAAAAAEAAAAAAAAACwEXUhsAAFfhAAAAAAAAAAA=",
				resultXDR:     "AAAAAgAAAAAAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "edba3051b2f2d9b713e8a08709d631eccb72c59864ff3c564c68792271bb24a7",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "edba3051b2f2d9b713e8a08709d631eccb72c59864ff3c564c68792271bb24a7",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  78621794419880145,
					// set max fee to a value larger than MAX_INT32 but less than or equal to MAX_UINT32
					MaxFee:          2500000000,
					FeeCharged:      int64(1 << 33),
					OperationCount:  1,
					TxEnvelope:      "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rlQL5AAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAABAAAACXRlc3QgbWVtbwAAAAAAAAEAAAAAAAAACwEXUhsAAFfhAAAAAAAAAAA=",
					TxResult:        "AAAAAgAAAAAAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:       "AAAAAA==",
					TxMeta:          "AAAAAQAAAAAAAAAA",
					Signatures:      emptySignatures,
					InnerSignatures: nullSignatures,
					MemoType:        "text",
					Memo:            null.NewString("test memo", true),
					TimeBounds:      infiniteTimeBounds,
					LedgerBounds:    LedgerBounds{Null: true},
					ExtraSigners:    nil,
					Successful:      success,
				},
			},
		},
		{
			"transaction with text memo",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAABAAAACXRlc3QgbWVtbwAAAAAAAAEAAAAAAAAACwEXUhsAAFfhAAAAAAAAAAA=",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "edba3051b2f2d9b713e8a08709d631eccb72c59864ff3c564c68792271bb24a7",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "edba3051b2f2d9b713e8a08709d631eccb72c59864ff3c564c68792271bb24a7",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  78621794419880145,
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAABAAAACXRlc3QgbWVtbwAAAAAAAAEAAAAAAAAACwEXUhsAAFfhAAAAAAAAAAA=",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "text",
					Memo:             null.NewString("test memo", true),
					TimeBounds:       infiniteTimeBounds,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
		{
			"transaction with id memo",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAIAAAAAAAAAewAAAAEAAAAAAAAACwEXUhsAAFfhAAAAAAAAAAA=",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "7e2def20d5a21a56be2a457b648f702ee1af889d3df65790e92a05081e9fabf1",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "7e2def20d5a21a56be2a457b648f702ee1af889d3df65790e92a05081e9fabf1",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  78621794419880145,
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAIAAAAAAAAAewAAAAEAAAAAAAAACwEXUhsAAFfhAAAAAAAAAAA=",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "id",
					Memo:             null.NewString("123", true),
					TimeBounds:       nullTimeBounds,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
		{
			"transaction with hash memo",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAADfi3vINWiGla+KkV7ZI9wLuGviJ099leQ6SoFCB6fq/EAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "8aba253c2adc4083f35830cec37d9c6226b757ab3a94f15a12d6c22973fd5f3f",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "8aba253c2adc4083f35830cec37d9c6226b757ab3a94f15a12d6c22973fd5f3f",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  78621794419880145,
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAADfi3vINWiGla+KkV7ZI9wLuGviJ099leQ6SoFCB6fq/EAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "hash",
					Memo:             null.NewString("fi3vINWiGla+KkV7ZI9wLuGviJ099leQ6SoFCB6fq/E=", true),
					TimeBounds:       infiniteTimeBounds,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
		{
			"transaction with return memo",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAEzdjArlILa/LNv7o7lo/qv5+fVVPNl0yPgZQWB6u+gL4AAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "9e932a86cea43239aed62d8cd3b6b5ad2d8eb0a63247355e4ab44f2994209f2a",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "9e932a86cea43239aed62d8cd3b6b5ad2d8eb0a63247355e4ab44f2994209f2a",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  78621794419880145,
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAEzdjArlILa/LNv7o7lo/qv5+fVVPNl0yPgZQWB6u+gL4AAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "return",
					Memo:             null.NewString("zdjArlILa/LNv7o7lo/qv5+fVVPNl0yPgZQWB6u+gL4=", true),
					TimeBounds:       infiniteTimeBounds,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
		{
			"transaction with min time bound",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAZAAAAAAAAeJAAAAAAQAAAABd8tcbAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAALAAAAAAAS1ocAAAAAAAAAAA==",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "2a9ec3733989aa9a542ed6d0adbcc664915b1c3a72a019e6e23c2860f1ab342b",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{

					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "2a9ec3733989aa9a542ed6d0adbcc664915b1c3a72a019e6e23c2860f1ab342b",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  123456,
					MaxFee:           100,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAZAAAAAAAAeJAAAAAAQAAAABd8tcbAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAALAAAAAAAS1ocAAAAAAAAAAA==",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       timeBoundWithMin,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
		{
			"transaction with max time bound",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAZAAAAAAAAeJAAAAAAQAAAAAAAAAAAAAAAF3y1xsAAAAAAAAAAQAAAAAAAAALAAAAAAAS1ocAAAAAAAAAAA==",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "5858709ae02992431f98f7410be3d3586c5a83e9e7105d270ce1ddc2cf45358a",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "5858709ae02992431f98f7410be3d3586c5a83e9e7105d270ce1ddc2cf45358a",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  123456,
					MaxFee:           100,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAZAAAAAAAAeJAAAAAAQAAAAAAAAAAAAAAAF3y1xsAAAAAAAAAAQAAAAAAAAALAAAAAAAS1ocAAAAAAAAAAA==",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       timeBoundWithMax,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
		{
			"transaction with min and max time bound",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAZAAAAAAAAeJAAAAAAQAAAABd8VB7AAAAAF3y1xsAAAAAAAAAAQAAAAAAAAALAAAAAAAS1ocAAAAAAAAAAA==",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "7aa3419a833fb14e312ae47a98e565f668a72f23c39e0cf79f598d3d3e793b2d",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "7aa3419a833fb14e312ae47a98e565f668a72f23c39e0cf79f598d3d3e793b2d",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  123456,
					MaxFee:           100,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAZAAAAAAAAeJAAAAAAQAAAABd8VB7AAAAAF3y1xsAAAAAAAAAAQAAAAAAAAALAAAAAAAS1ocAAAAAAAAAAA==",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       timeboundsWithMinAndMax,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
		{
			"transaction with v2 preconditions",
			buildLedgerTransaction(tt.T, testTransaction{
				index:       1,
				envelopeXDR: "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAAAAAAAAAAQAAAAIAAAABAAAAAAAAAAAAAAAAYjzUCQAAAAEAAAAAAAAAAQAAAAAAAAAAAAAACgAAAAIAAAAAAAAAAAAAAAEAAAAAAAAACwAAAAAAAAAAAAAAAAAAAAA=",
				// Real values pending core accepting these txns
				resultXDR: "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				// Real values pending core accepting these txns
				feeChangesXDR: "AAAAAA==",
				// Real values pending core accepting these txns
				metaXDR: "AAAAAQAAAAAAAAAA",
				hash:    "7ad89c184dd2aa17f9f96105f9508521b52bb36f74cf57d5ffd6f7205a737764",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:                TotalOrderID{528280981504},
					TransactionHash:             "7ad89c184dd2aa17f9f96105f9508521b52bb36f74cf57d5ffd6f7205a737764",
					LedgerSequence:              ledger.Sequence,
					ApplicationOrder:            1,
					Account:                     "GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3",
					AccountSequence:             1,
					MaxFee:                      100,
					FeeCharged:                  300,
					OperationCount:              1,
					TxEnvelope:                  "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAAAAAAAAAAQAAAAIAAAABAAAAAAAAAAAAAAAAYjzUCQAAAAEAAAAAAAAAAQAAAAAAAAAAAAAACgAAAAIAAAAAAAAAAAAAAAEAAAAAAAAACwAAAAAAAAAAAAAAAAAAAAA=",
					TxResult:                    "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:                   "AAAAAA==",
					TxMeta:                      "AAAAAQAAAAAAAAAA",
					Signatures:                  emptySignatures,
					InnerSignatures:             nullSignatures,
					MemoType:                    "none",
					Memo:                        null.NewString("", false),
					TimeBounds:                  v2TimeboundsWithMinAndMax,
					LedgerBounds:                v2LedgerboundsWithMinAndMax,
					MinAccountSequence:          null.Int{},
					MinAccountSequenceAge:       null.StringFrom("10"),
					MinAccountSequenceLedgerGap: null.IntFrom(2),
					ExtraSigners:                nil,
					Successful:                  success,
				},
			},
		},
		{
			"transaction with multiple operations",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAAAAAAAAeJAAAAAAAAAAAAAAAACAAAAAAAAAAsAAAAAABLWhwAAAAAAAAALAAAAAAAS1ogAAAAAAAAAAA==",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "6a3698a409141c6e45cb254aaaf94254c36a34323146a58214ce47b9f921174c",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "6a3698a409141c6e45cb254aaaf94254c36a34323146a58214ce47b9f921174c",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  123456,
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   2,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAAAAAAAAeJAAAAAAAAAAAAAAAACAAAAAAAAAAsAAAAAABLWhwAAAAAAAAALAAAAAAAS1ogAAAAAAAAAAA==",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       nullTimeBounds,
					LedgerBounds:     LedgerBounds{Null: true},
					ExtraSigners:     nil,
					Successful:       success,
				},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			insertBuilder := q.NewTransactionBatchInsertBuilder()
			tt.Assert.NoError(q.Begin(tt.Ctx))
			tt.Assert.NoError(insertBuilder.Add(testCase.toInsert, sequence))
			tt.Assert.NoError(insertBuilder.Exec(tt.Ctx, q))
			tt.Assert.NoError(q.Commit())

			var transactions []Transaction
			tt.Assert.NoError(q.Transactions().IncludeFailed().Select(tt.Ctx, &transactions))
			tt.Assert.Len(transactions, 1)

			transaction := transactions[0]

			// ignore created time and updated time
			transaction.CreatedAt = testCase.expected.CreatedAt
			transaction.UpdatedAt = testCase.expected.UpdatedAt

			// compare ClosedAt separately because reflect.DeepEqual does not handle time.Time
			closedAt := transaction.LedgerCloseTime
			transaction.LedgerCloseTime = testCase.expected.LedgerCloseTime

			tt.Assert.True(closedAt.Equal(testCase.expected.LedgerCloseTime))
			tt.Assert.Equal(transaction, testCase.expected)

			_, err = q.Exec(tt.Ctx, sq.Delete("history_transactions"))
			tt.Assert.NoError(err)
		})
	}
}

func TestFetchFeeBumpTransaction(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	fixture := FeeBumpScenario(tt, q, true)

	var byOuterhash, byInnerHash Transaction
	tt.Assert.NoError(q.TransactionByHash(tt.Ctx, &byOuterhash, fixture.OuterHash))
	tt.Assert.NoError(q.TransactionByHash(tt.Ctx, &byInnerHash, fixture.InnerHash))

	tt.Assert.Equal(byOuterhash, byInnerHash)
	tt.Assert.Equal(byOuterhash, fixture.Transaction)

	outerOps, outerTransactions, err := q.Operations().IncludeTransactions().
		ForTransaction(tt.Ctx, fixture.OuterHash).Fetch(tt.Ctx)
	tt.Assert.NoError(err)
	tt.Assert.Len(outerTransactions, 1)
	tt.Assert.Len(outerOps, 1)

	innerOps, innerTransactions, err := q.Operations().IncludeTransactions().
		ForTransaction(tt.Ctx, fixture.InnerHash).Fetch(tt.Ctx)
	tt.Assert.NoError(err)
	tt.Assert.Len(innerTransactions, 1)
	tt.Assert.Equal(innerOps, outerOps)

	for _, tx := range append(outerTransactions, innerTransactions...) {
		tt.Assert.True(byOuterhash.CreatedAt.Equal(tx.CreatedAt))
		tt.Assert.True(byOuterhash.LedgerCloseTime.Equal(tx.LedgerCloseTime))
		byOuterhash.CreatedAt = tx.CreatedAt
		byOuterhash.LedgerCloseTime = tx.LedgerCloseTime
		tt.Assert.Equal(byOuterhash, byInnerHash)
	}

	var innerEffects []Effect
	outerEffects, err := q.EffectsForTransaction(tt.Ctx, fixture.OuterHash, db2.PageQuery{
		Cursor: "0-0",
		Order:  "asc",
		Limit:  200,
	})
	tt.Assert.NoError(err)
	tt.Assert.Len(outerEffects, 1)

	innerEffects, err = q.EffectsForTransaction(tt.Ctx, fixture.InnerHash, db2.PageQuery{
		Cursor: "0-0",
		Order:  "asc",
		Limit:  200,
	})
	tt.Assert.NoError(err)
	tt.Assert.Equal(outerEffects, innerEffects)
}

func TestHistoryTransactionSchemasMatch(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	db := tt.HorizonSession()
	type column struct {
		Name     string `db:"column_name"`
		DataType string `db:"data_type"`
	}
	query := `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = ?`
	var txColumns []column
	err := db.SelectRaw(context.Background(), &txColumns, query, "history_transactions")
	tt.Assert.NoError(err)

	var txTmpFilteredTmpColumns []column
	err = db.SelectRaw(context.Background(), &txTmpFilteredTmpColumns, query, "history_transactions_filtered_tmp")
	tt.Assert.NoError(err)

	tt.Assert.ElementsMatch(txColumns, txTmpFilteredTmpColumns)
}

func TestTransactionQueryBuilder(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.Begin(tt.Ctx))

	address := "GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON"
	accountLoader := NewAccountLoader(ConcurrentInserts)
	accountLoader.GetFuture(address)

	cbID := "00000000178826fbfe339e1f5c53417c6fedfe2c05e8bec14303143ec46b38981b09c3f9"
	cbLoader := NewClaimableBalanceLoader(ConcurrentInserts)
	cbLoader.GetFuture(cbID)

	lpID := "0000a8198b5e25994c1ca5b0556faeb27325ac746296944144e0a7406d501e8a"
	lpLoader := NewLiquidityPoolLoader(ConcurrentInserts)
	lpLoader.GetFuture(lpID)

	tt.Assert.NoError(accountLoader.Exec(tt.Ctx, q))
	tt.Assert.NoError(cbLoader.Exec(tt.Ctx, q))
	tt.Assert.NoError(lpLoader.Exec(tt.Ctx, q))

	tt.Assert.NoError(q.Commit())

	for _, testCase := range []struct {
		q            *TransactionsQ
		expectedSQL  string
		expectedArgs []interface{}
	}{
		{
			q.Transactions().ForAccount(tt.Ctx, address).Page(db2.PageQuery{Cursor: "8589938689", Order: "asc", Limit: 10}, 0),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_participants htp ON htp.history_transaction_id = ht.id " +
				"WHERE htp.history_account_id = ? AND htp.history_transaction_id > ? " +
				"ORDER BY htp.history_transaction_id asc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().ForClaimableBalance(tt.Ctx, cbID).Page(db2.PageQuery{Cursor: "8589938689", Order: "asc", Limit: 10}, 0),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_claimable_balances htcb ON htcb.history_transaction_id = ht.id " +
				"WHERE htcb.history_claimable_balance_id = ? AND htcb.history_transaction_id > ? " +
				"ORDER BY htcb.history_transaction_id asc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().ForLiquidityPool(tt.Ctx, lpID).Page(db2.PageQuery{Cursor: "8589938689", Order: "asc", Limit: 10}, 0),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_liquidity_pools htlp ON htlp.history_transaction_id = ht.id " +
				"WHERE htlp.history_liquidity_pool_id = ? AND htlp.history_transaction_id > ? " +
				"ORDER BY htlp.history_transaction_id asc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().Page(db2.PageQuery{Cursor: "8589938689", Order: "asc", Limit: 10}, 0),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"WHERE ht.id > ? " +
				"ORDER BY ht.id asc LIMIT 10",
			[]interface{}{int64(8589938689)},
		},
		{
			q.Transactions().ForAccount(tt.Ctx, address).Page(db2.PageQuery{Cursor: "8589938689", Order: "asc", Limit: 10}, 100),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_participants htp ON htp.history_transaction_id = ht.id " +
				"WHERE htp.history_account_id = ? AND htp.history_transaction_id > ? " +
				"ORDER BY htp.history_transaction_id asc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().ForClaimableBalance(tt.Ctx, cbID).Page(db2.PageQuery{Cursor: "8589938689", Order: "asc", Limit: 10}, 100),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_claimable_balances htcb ON htcb.history_transaction_id = ht.id " +
				"WHERE htcb.history_claimable_balance_id = ? AND htcb.history_transaction_id > ? " +
				"ORDER BY htcb.history_transaction_id asc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().ForLiquidityPool(tt.Ctx, lpID).Page(db2.PageQuery{Cursor: "8589938689", Order: "asc", Limit: 10}, 100),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_liquidity_pools htlp ON htlp.history_transaction_id = ht.id " +
				"WHERE htlp.history_liquidity_pool_id = ? AND htlp.history_transaction_id > ? " +
				"ORDER BY htlp.history_transaction_id asc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().Page(db2.PageQuery{Cursor: "8589938689", Order: "asc", Limit: 10}, 100),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"WHERE ht.id > ? " +
				"ORDER BY ht.id asc LIMIT 10",
			[]interface{}{int64(8589938689)},
		},
		{
			q.Transactions().ForAccount(tt.Ctx, address).Page(db2.PageQuery{Cursor: "8589938689", Order: "desc", Limit: 10}, 0),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_participants htp ON htp.history_transaction_id = ht.id " +
				"WHERE htp.history_account_id = ? AND htp.history_transaction_id < ? " +
				"ORDER BY htp.history_transaction_id desc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().ForClaimableBalance(tt.Ctx, cbID).Page(db2.PageQuery{Cursor: "8589938689", Order: "desc", Limit: 10}, 0),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_claimable_balances htcb ON htcb.history_transaction_id = ht.id " +
				"WHERE htcb.history_claimable_balance_id = ? AND htcb.history_transaction_id < ? " +
				"ORDER BY htcb.history_transaction_id desc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().ForLiquidityPool(tt.Ctx, lpID).Page(db2.PageQuery{Cursor: "8589938689", Order: "desc", Limit: 10}, 0),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_liquidity_pools htlp ON htlp.history_transaction_id = ht.id " +
				"WHERE htlp.history_liquidity_pool_id = ? AND htlp.history_transaction_id < ? " +
				"ORDER BY htlp.history_transaction_id desc LIMIT 10",
			[]interface{}{int64(1), int64(8589938689)},
		},
		{
			q.Transactions().Page(db2.PageQuery{Cursor: "8589938689", Order: "desc", Limit: 10}, 0),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"WHERE ht.id < ? " +
				"ORDER BY ht.id desc LIMIT 10",
			[]interface{}{int64(8589938689)},
		},
		{
			q.Transactions().ForAccount(tt.Ctx, address).Page(db2.PageQuery{Cursor: "8589938689", Order: "desc", Limit: 10}, 100),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_participants htp ON htp.history_transaction_id = ht.id " +
				"WHERE htp.history_account_id = ? AND htp.history_transaction_id > ? " +
				"AND htp.history_transaction_id < ? " +
				"ORDER BY htp.history_transaction_id desc LIMIT 10",
			[]interface{}{int64(1), int64(429496729599), int64(8589938689)},
		},
		{
			q.Transactions().ForClaimableBalance(tt.Ctx, cbID).Page(db2.PageQuery{Cursor: "8589938689", Order: "desc", Limit: 10}, 100),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_claimable_balances htcb ON htcb.history_transaction_id = ht.id " +
				"WHERE htcb.history_claimable_balance_id = ? AND htcb.history_transaction_id > ? " +
				"AND htcb.history_transaction_id < ? " +
				"ORDER BY htcb.history_transaction_id desc LIMIT 10",
			[]interface{}{int64(1), int64(429496729599), int64(8589938689)},
		},
		{
			q.Transactions().ForLiquidityPool(tt.Ctx, lpID).Page(db2.PageQuery{Cursor: "8589938689", Order: "desc", Limit: 10}, 100),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"JOIN history_transaction_liquidity_pools htlp ON htlp.history_transaction_id = ht.id " +
				"WHERE htlp.history_liquidity_pool_id = ? AND htlp.history_transaction_id > ? " +
				"AND htlp.history_transaction_id < ? " +
				"ORDER BY htlp.history_transaction_id desc LIMIT 10",
			[]interface{}{int64(1), int64(429496729599), int64(8589938689)},
		},
		{
			q.Transactions().Page(db2.PageQuery{Cursor: "8589938689", Order: "desc", Limit: 10}, 100),
			"SELECT " +
				"ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, " +
				"ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, " +
				"COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, " +
				"ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, " +
				"ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, " +
				"ht.memo_type, ht.memo, ht.time_bounds, ht.ledger_bounds, ht.min_account_sequence, " +
				"ht.min_account_sequence_age, ht.min_account_sequence_ledger_gap, ht.extra_signers, " +
				"hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, " +
				"ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures " +
				"FROM history_transactions ht " +
				"LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence " +
				"WHERE ht.id > ? AND ht.id < ? " +
				"ORDER BY ht.id desc LIMIT 10",
			[]interface{}{int64(429496729599), int64(8589938689)},
		},
	} {
		tt.Assert.NoError(testCase.q.Err)
		got, args, err := testCase.q.sql.ToSql()
		tt.Assert.NoError(err)
		tt.Assert.Equal(got, testCase.expectedSQL)
		tt.Assert.Equal(args, testCase.expectedArgs)
	}
}

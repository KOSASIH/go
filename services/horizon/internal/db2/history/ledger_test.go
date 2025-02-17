package history

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/guregu/null"

	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

func TestLedgerQueries(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	// Test LedgerBySequence
	var l Ledger
	err := q.LedgerBySequence(tt.Ctx, &l, 3)
	tt.Assert.NoError(err)

	err = q.LedgerBySequence(tt.Ctx, &l, 100000)
	tt.Assert.Equal(err, sql.ErrNoRows)

	// Test Ledgers()
	ls := []Ledger{}
	err = q.Ledgers().Select(tt.Ctx, &ls)

	if tt.Assert.NoError(err) {
		tt.Assert.Len(ls, 3)
	}

	// LedgersBySequence
	err = q.LedgersBySequence(tt.Ctx, &ls, 1, 2, 3)

	if tt.Assert.NoError(err) {
		tt.Assert.Len(ls, 3)

		foundSeqs := make([]int32, len(ls))
		for i := range ls {
			foundSeqs[i] = ls[i].Sequence
		}

		tt.Assert.Contains(foundSeqs, int32(1))
		tt.Assert.Contains(foundSeqs, int32(2))
		tt.Assert.Contains(foundSeqs, int32(3))
	}
}

func TestInsertLedger(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	var ledgerFromDB Ledger
	var ledgerHeaderBase64 string
	var err error
	err = q.LedgerBySequence(tt.Ctx, &ledgerFromDB, 69859)
	tt.Assert.Error(err)

	expectedLedger := Ledger{
		Sequence:                   69859,
		LedgerHash:                 "4db1e4f145e9ee75162040d26284795e0697e2e84084624e7c6c723ebbf80118",
		PreviousLedgerHash:         null.NewString("4b0b8bace3b2438b2404776ce57643966855487ba6384724a3c664c7aa4cd9e4", true),
		TotalOrderID:               TotalOrderID{toid.New(int32(69859), 0, 0).ToInt64()},
		ImporterVersion:            123,
		TransactionCount:           12,
		SuccessfulTransactionCount: new(int32),
		FailedTransactionCount:     new(int32),
		TxSetOperationCount:        new(int32),
		OperationCount:             23,
		TotalCoins:                 23451,
		FeePool:                    213,
		BaseReserve:                687,
		MaxTxSetSize:               345,
		ProtocolVersion:            12,
		BaseFee:                    100,
		ClosedAt:                   time.Now().UTC().Truncate(time.Second),
	}
	*expectedLedger.SuccessfulTransactionCount = 12
	*expectedLedger.FailedTransactionCount = 3
	*expectedLedger.TxSetOperationCount = 26

	var ledgerHash, previousLedgerHash xdr.Hash

	written, err := hex.Decode(ledgerHash[:], []byte(expectedLedger.LedgerHash))
	tt.Assert.NoError(err)
	tt.Assert.Equal(len(ledgerHash), written)

	written, err = hex.Decode(previousLedgerHash[:], []byte(expectedLedger.PreviousLedgerHash.String))
	tt.Assert.NoError(err)
	tt.Assert.Equal(len(previousLedgerHash), written)

	ledgerEntry := xdr.LedgerHeaderHistoryEntry{
		Hash: ledgerHash,
		Header: xdr.LedgerHeader{
			LedgerVersion:      12,
			PreviousLedgerHash: previousLedgerHash,
			LedgerSeq:          xdr.Uint32(expectedLedger.Sequence),
			TotalCoins:         xdr.Int64(expectedLedger.TotalCoins),
			FeePool:            xdr.Int64(expectedLedger.FeePool),
			BaseFee:            xdr.Uint32(expectedLedger.BaseFee),
			BaseReserve:        xdr.Uint32(expectedLedger.BaseReserve),
			MaxTxSetSize:       xdr.Uint32(expectedLedger.MaxTxSetSize),
			ScpValue: xdr.StellarValue{
				CloseTime: xdr.TimePoint(expectedLedger.ClosedAt.Unix()),
			},
		},
	}
	ledgerHeaderBase64, err = xdr.MarshalBase64(ledgerEntry.Header)
	tt.Assert.NoError(err)
	expectedLedger.LedgerHeaderXDR = null.NewString(ledgerHeaderBase64, true)

	ledgerBatch := q.NewLedgerBatchInsertBuilder()
	err = ledgerBatch.Add(
		ledgerEntry,
		12,
		3,
		23,
		26,
		int(expectedLedger.ImporterVersion),
	)
	tt.Assert.NoError(err)
	tt.Assert.NoError(q.Begin(tt.Ctx))
	tt.Assert.NoError(ledgerBatch.Exec(tt.Ctx, q.SessionInterface))
	tt.Assert.NoError(q.Commit())

	err = q.LedgerBySequence(tt.Ctx, &ledgerFromDB, 69859)
	tt.Assert.NoError(err)

	expectedLedger.CreatedAt = ledgerFromDB.CreatedAt
	expectedLedger.UpdatedAt = ledgerFromDB.UpdatedAt
	tt.Assert.True(ledgerFromDB.CreatedAt.After(expectedLedger.ClosedAt))
	tt.Assert.True(ledgerFromDB.UpdatedAt.After(expectedLedger.ClosedAt))
	tt.Assert.True(ledgerFromDB.CreatedAt.Before(expectedLedger.ClosedAt.Add(time.Hour)))
	tt.Assert.True(ledgerFromDB.UpdatedAt.Before(expectedLedger.ClosedAt.Add(time.Hour)))

	tt.Assert.True(expectedLedger.ClosedAt.Equal(ledgerFromDB.ClosedAt))
	expectedLedger.ClosedAt = ledgerFromDB.ClosedAt

	tt.Assert.Equal(expectedLedger, ledgerFromDB)
}

func insertLedgerWithSequence(tt *test.T, q *Q, seq uint32) {
	// generate random hashes to avoid insert clashes due to UNIQUE constraints
	var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
	ledgerHashHex := fmt.Sprintf("%064x", rnd.Uint32())
	previousLedgerHashHex := fmt.Sprintf("%064x", rnd.Uint32())

	expectedLedger := Ledger{
		Sequence:                   int32(seq),
		LedgerHash:                 ledgerHashHex,
		PreviousLedgerHash:         null.NewString(previousLedgerHashHex, true),
		TotalOrderID:               TotalOrderID{toid.New(int32(69859), 0, 0).ToInt64()},
		ImporterVersion:            123,
		TransactionCount:           12,
		SuccessfulTransactionCount: new(int32),
		FailedTransactionCount:     new(int32),
		TxSetOperationCount:        new(int32),
		OperationCount:             23,
		TotalCoins:                 23451,
		FeePool:                    213,
		BaseReserve:                687,
		MaxTxSetSize:               345,
		ProtocolVersion:            12,
		BaseFee:                    100,
		ClosedAt:                   time.Now().UTC().Truncate(time.Second),
	}
	*expectedLedger.SuccessfulTransactionCount = 12
	*expectedLedger.FailedTransactionCount = 3
	*expectedLedger.TxSetOperationCount = 26

	var ledgerHash, previousLedgerHash xdr.Hash

	written, err := hex.Decode(ledgerHash[:], []byte(expectedLedger.LedgerHash))
	tt.Assert.NoError(err)
	tt.Assert.Equal(len(ledgerHash), written)

	written, err = hex.Decode(previousLedgerHash[:], []byte(expectedLedger.PreviousLedgerHash.String))
	tt.Assert.NoError(err)
	tt.Assert.Equal(len(previousLedgerHash), written)

	ledgerEntry := xdr.LedgerHeaderHistoryEntry{
		Hash: ledgerHash,
		Header: xdr.LedgerHeader{
			LedgerVersion:      12,
			PreviousLedgerHash: previousLedgerHash,
			LedgerSeq:          xdr.Uint32(expectedLedger.Sequence),
			TotalCoins:         xdr.Int64(expectedLedger.TotalCoins),
			FeePool:            xdr.Int64(expectedLedger.FeePool),
			BaseFee:            xdr.Uint32(expectedLedger.BaseFee),
			BaseReserve:        xdr.Uint32(expectedLedger.BaseReserve),
			MaxTxSetSize:       xdr.Uint32(expectedLedger.MaxTxSetSize),
			ScpValue: xdr.StellarValue{
				CloseTime: xdr.TimePoint(expectedLedger.ClosedAt.Unix()),
			},
		},
	}
	ledgerHeaderBase64, err := xdr.MarshalBase64(ledgerEntry.Header)
	tt.Assert.NoError(err)
	expectedLedger.LedgerHeaderXDR = null.NewString(ledgerHeaderBase64, true)
	ledgerBatch := q.NewLedgerBatchInsertBuilder()
	err = ledgerBatch.Add(
		ledgerEntry,
		12,
		3,
		23,
		26,
		int(expectedLedger.ImporterVersion),
	)
	tt.Assert.NoError(err)
	tt.Assert.NoError(q.Begin(tt.Ctx))
	tt.Assert.NoError(ledgerBatch.Exec(tt.Ctx, q.SessionInterface))
	tt.Assert.NoError(q.Commit())
}

func TestGetLedgerGaps(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}

	// The DB is empty, so there shouldn't be any gaps
	gaps, err := q.GetLedgerGaps(context.Background())
	tt.Assert.NoError(err)
	tt.Assert.Len(gaps, 0)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 1, 100)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 1, EndSequence: 100}}, gaps)

	// Lets insert a few gaps and make sure they are detected incrementally
	insertLedgerWithSequence(tt, q, 4)
	insertLedgerWithSequence(tt, q, 5)
	insertLedgerWithSequence(tt, q, 6)
	insertLedgerWithSequence(tt, q, 7)

	// since there is a single ledger cluster, there should still be no gaps
	// (we don't start from ledger 0)
	gaps, err = q.GetLedgerGaps(context.Background())
	tt.Assert.NoError(err)
	tt.Assert.Len(gaps, 0)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 1, 2)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 1, EndSequence: 2}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 1, 3)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 1, EndSequence: 3}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 1, 6)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 1, EndSequence: 3}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 3, 5)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 3, EndSequence: 3}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 4, 6)
	tt.Assert.NoError(err)
	tt.Assert.Len(gaps, 0)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 4, 8)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 8, EndSequence: 8}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 4, 10)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 8, EndSequence: 10}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 8, 10)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 8, EndSequence: 10}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 9, 11)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 9, EndSequence: 11}}, gaps)

	var expectedGaps []LedgerRange

	insertLedgerWithSequence(tt, q, 99)
	insertLedgerWithSequence(tt, q, 100)
	insertLedgerWithSequence(tt, q, 101)
	insertLedgerWithSequence(tt, q, 102)

	gaps, err = q.GetLedgerGaps(context.Background())
	tt.Assert.NoError(err)
	expectedGaps = append(expectedGaps, LedgerRange{8, 98})
	tt.Assert.Equal(expectedGaps, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 10, 11)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 10, EndSequence: 11}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 4, 11)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 8, EndSequence: 11}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 1, 11)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 1, EndSequence: 3}, {StartSequence: 8, EndSequence: 11}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 10, 105)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 10, EndSequence: 98}, {StartSequence: 103, EndSequence: 105}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 100, 105)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 103, EndSequence: 105}}, gaps)

	gaps, err = q.GetLedgerGapsInRange(context.Background(), 105, 110)
	tt.Assert.NoError(err)
	tt.Assert.Equal([]LedgerRange{{StartSequence: 105, EndSequence: 110}}, gaps)

	// Yet another gap, this time to a single-ledger cluster
	insertLedgerWithSequence(tt, q, 1000)

	gaps, err = q.GetLedgerGaps(context.Background())
	tt.Assert.NoError(err)
	expectedGaps = append(expectedGaps, LedgerRange{103, 999})
	tt.Assert.Equal(expectedGaps, gaps)

	// Yet another gap, this time the gap only contains a ledger
	insertLedgerWithSequence(tt, q, 1002)
	gaps, err = q.GetLedgerGaps(context.Background())
	tt.Assert.NoError(err)
	expectedGaps = append(expectedGaps, LedgerRange{1001, 1001})
	tt.Assert.Equal(expectedGaps, gaps)
}

func TestGetNextLedgerSequence(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}

	_, ok, err := q.GetNextLedgerSequence(context.Background(), 0)
	tt.Assert.NoError(err)
	tt.Assert.False(ok)

	insertLedgerWithSequence(tt, q, 4)
	insertLedgerWithSequence(tt, q, 5)
	insertLedgerWithSequence(tt, q, 6)
	insertLedgerWithSequence(tt, q, 7)

	insertLedgerWithSequence(tt, q, 99)
	insertLedgerWithSequence(tt, q, 100)
	insertLedgerWithSequence(tt, q, 101)
	insertLedgerWithSequence(tt, q, 102)

	seq, ok, err := q.GetNextLedgerSequence(context.Background(), 0)
	tt.Assert.NoError(err)
	tt.Assert.True(ok)
	tt.Assert.Equal(uint32(4), seq)

	seq, ok, err = q.GetNextLedgerSequence(context.Background(), 4)
	tt.Assert.NoError(err)
	tt.Assert.True(ok)
	tt.Assert.Equal(uint32(5), seq)

	seq, ok, err = q.GetNextLedgerSequence(context.Background(), 10)
	tt.Assert.NoError(err)
	tt.Assert.True(ok)
	tt.Assert.Equal(uint32(99), seq)

	seq, ok, err = q.GetNextLedgerSequence(context.Background(), 101)
	tt.Assert.NoError(err)
	tt.Assert.True(ok)
	tt.Assert.Equal(uint32(102), seq)

	_, ok, err = q.GetNextLedgerSequence(context.Background(), 102)
	tt.Assert.NoError(err)
	tt.Assert.False(ok)

	_, ok, err = q.GetNextLedgerSequence(context.Background(), 110)
	tt.Assert.NoError(err)
	tt.Assert.False(ok)
}

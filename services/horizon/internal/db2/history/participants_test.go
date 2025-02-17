package history

import (
	"testing"

	sq "github.com/Masterminds/squirrel"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/test"
)

type transactionParticipant struct {
	TransactionID int64 `db:"history_transaction_id"`
	AccountID     int64 `db:"history_account_id"`
}

func getTransactionParticipants(tt *test.T, q *Q) []transactionParticipant {
	var participants []transactionParticipant
	sql := sq.Select("history_transaction_id", "history_account_id").
		From("history_transaction_participants").
		OrderBy("(history_transaction_id, history_account_id) asc")

	err := q.Select(tt.Ctx, &participants, sql)
	if err != nil {
		tt.T.Fatal(err)
	}

	return participants
}

func TestTransactionParticipantsBatch(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	batch := q.NewTransactionParticipantsBatchInsertBuilder()
	accountLoader := NewAccountLoader(ConcurrentInserts)

	transactionID := int64(1)
	otherTransactionID := int64(2)
	var addresses []string
	for i := int64(0); i < 3; i++ {
		address := keypair.MustRandom().Address()
		addresses = append(addresses, address)
		tt.Assert.NoError(batch.Add(transactionID, accountLoader.GetFuture(address)))
	}

	address := keypair.MustRandom().Address()
	addresses = append(addresses, address)
	tt.Assert.NoError(batch.Add(otherTransactionID, accountLoader.GetFuture(address)))

	tt.Assert.NoError(q.Begin(tt.Ctx))
	tt.Assert.NoError(accountLoader.Exec(tt.Ctx, q))
	tt.Assert.NoError(batch.Exec(tt.Ctx, q))
	tt.Assert.NoError(q.Commit())

	participants := getTransactionParticipants(tt, q)
	expected := []transactionParticipant{
		{TransactionID: 1},
		{TransactionID: 1},
		{TransactionID: 1},
		{TransactionID: 2},
	}
	for i := range expected {
		val, err := accountLoader.GetNow(addresses[i])
		tt.Assert.NoError(err)
		expected[i].AccountID = val
	}
	tt.Assert.ElementsMatch(expected, participants)
}

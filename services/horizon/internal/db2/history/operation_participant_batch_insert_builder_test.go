package history

import (
	"testing"

	sq "github.com/Masterminds/squirrel"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/test"
)

func TestAddOperationParticipants(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	accountLoader := NewAccountLoader(ConcurrentInserts)
	address := keypair.MustRandom().Address()
	tt.Assert.NoError(q.Begin(tt.Ctx))
	builder := q.NewOperationParticipantBatchInsertBuilder()
	err := builder.Add(240518172673, accountLoader.GetFuture(address))
	tt.Assert.NoError(err)

	tt.Assert.NoError(accountLoader.Exec(tt.Ctx, q))
	tt.Assert.NoError(builder.Exec(tt.Ctx, q))
	tt.Assert.NoError(q.Commit())

	type hop struct {
		OperationID int64 `db:"history_operation_id"`
		AccountID   int64 `db:"history_account_id"`
	}

	ops := []hop{}
	err = q.Select(tt.Ctx, &ops, sq.Select(
		"hopp.history_operation_id, "+
			"hopp.history_account_id").
		From("history_operation_participants hopp"),
	)

	if tt.Assert.NoError(err) {
		tt.Assert.Len(ops, 1)

		op := ops[0]
		tt.Assert.Equal(int64(240518172673), op.OperationID)
		val, err := accountLoader.GetNow(address)
		tt.Assert.NoError(err)
		tt.Assert.Equal(val, op.AccountID)
	}
}

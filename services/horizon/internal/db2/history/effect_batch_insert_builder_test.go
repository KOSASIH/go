package history

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/guregu/null"

	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/toid"
)

func TestAddEffect(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}
	tt.Require.NoError(q.Begin(tt.Ctx))

	address := "GAQAA5L65LSYH7CQ3VTJ7F3HHLGCL3DSLAR2Y47263D56MNNGHSQSTVY"
	muxedAddres := "MAQAA5L65LSYH7CQ3VTJ7F3HHLGCL3DSLAR2Y47263D56MNNGHSQSAAAAAAAAAAE2LP26"
	accountLoader := NewAccountLoader(ConcurrentInserts)

	builder := q.NewEffectBatchInsertBuilder()
	sequence := int32(56)
	details, err := json.Marshal(map[string]string{
		"amount":     "1000.0000000",
		"asset_type": "native",
	})

	err = builder.Add(
		accountLoader.GetFuture(address),
		null.StringFrom(muxedAddres),
		toid.New(sequence, 1, 1).ToInt64(),
		1,
		3,
		details,
	)
	tt.Require.NoError(err)

	tt.Require.NoError(accountLoader.Exec(tt.Ctx, q))
	tt.Require.NoError(builder.Exec(tt.Ctx, q))
	tt.Require.NoError(q.Commit())

	effects, err := q.Effects(tt.Ctx, db2.PageQuery{
		Cursor: "0-0",
		Order:  "asc",
		Limit:  200,
	}, 0)
	tt.Require.NoError(err)
	tt.Require.Len(effects, 1)

	effect := effects[0]
	tt.Require.Equal(address, effect.Account)
	tt.Require.Equal(muxedAddres, effect.AccountMuxed.String)
	tt.Require.Equal(int64(240518172673), effect.HistoryOperationID)
	tt.Require.Equal(int32(1), effect.Order)
	tt.Require.Equal(EffectType(3), effect.Type)
	tt.Require.Equal("{\"amount\": \"1000.0000000\", \"asset_type\": \"native\"}", effect.DetailsString.String)

	effects, err = q.Effects(tt.Ctx, db2.PageQuery{
		Cursor: fmt.Sprintf("%d-0", toid.New(sequence+2, 0, 0).ToInt64()),
		Order:  "desc",
		Limit:  200,
	}, sequence-3)
	tt.Require.NoError(err)
	tt.Require.Len(effects, 1)
	tt.Require.Equal(effects[0], effect)

	effects, err = q.Effects(tt.Ctx, db2.PageQuery{
		Cursor: fmt.Sprintf("%d-0", toid.New(sequence+5, 0, 0).ToInt64()),
		Order:  "desc",
		Limit:  200,
	}, sequence+2)
	tt.Require.NoError(err)
	tt.Require.Empty(effects)
}

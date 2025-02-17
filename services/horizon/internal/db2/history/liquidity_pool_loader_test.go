package history

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
)

func TestLiquidityPoolLoader(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	session := tt.HorizonSession()

	testLPLoader(t, tt, session, ConcurrentInserts)
	test.ResetHorizonDB(t, tt.HorizonDB)
	testLPLoader(t, tt, session, ConcurrentDeletes)
}

func testLPLoader(t *testing.T, tt *test.T, session *db.Session, mode ConcurrencyMode) {
	var ids []string
	for i := 0; i < 100; i++ {
		poolID := xdr.PoolId{byte(i)}
		id, err := xdr.MarshalHex(poolID)
		tt.Assert.NoError(err)
		ids = append(ids, id)
	}

	loader := NewLiquidityPoolLoader(mode)
	for _, id := range ids {
		future := loader.GetFuture(id)
		_, err := future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid loader state,`)
		duplicateFuture := loader.GetFuture(id)
		assert.Equal(t, future, duplicateFuture)
	}

	err := loader.Exec(context.Background(), session)
	assert.NoError(t, err)
	assert.Equal(t, LoaderStats{
		Total:    100,
		Inserted: 100,
	}, loader.Stats())
	assert.Panics(t, func() {
		loader.GetFuture("not-present")
	})

	q := &Q{session}
	for _, id := range ids {
		var internalID int64
		internalID, err = loader.GetNow(id)
		assert.NoError(t, err)
		var lp HistoryLiquidityPool
		lp, err = q.LiquidityPoolByID(context.Background(), id)
		assert.NoError(t, err)
		assert.Equal(t, lp.PoolID, id)
		assert.Equal(t, lp.InternalID, internalID)
	}

	_, err = loader.GetNow("not present")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `was not found`)

	// check that Loader works when all the previous values are already
	// present in the db and also add 10 more rows to insert
	loader = NewLiquidityPoolLoader(mode)
	for i := 100; i < 110; i++ {
		poolID := xdr.PoolId{byte(i)}
		var id string
		id, err = xdr.MarshalHex(poolID)
		tt.Assert.NoError(err)
		ids = append(ids, id)
	}

	for _, id := range ids {
		future := loader.GetFuture(id)
		_, err = future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid loader state,`)
	}

	assert.NoError(t, loader.Exec(context.Background(), session))
	assert.Equal(t, LoaderStats{
		Total:    110,
		Inserted: 10,
	}, loader.Stats())

	for _, id := range ids {
		var internalID int64
		internalID, err = loader.GetNow(id)
		assert.NoError(t, err)
		var lp HistoryLiquidityPool
		lp, err = q.LiquidityPoolByID(context.Background(), id)
		assert.NoError(t, err)
		assert.Equal(t, lp.PoolID, id)
		assert.Equal(t, lp.InternalID, internalID)
	}
}

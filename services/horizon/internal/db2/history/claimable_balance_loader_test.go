package history

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
)

func TestClaimableBalanceLoader(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	session := tt.HorizonSession()

	testCBLoader(t, tt, session, ConcurrentInserts)
	test.ResetHorizonDB(t, tt.HorizonDB)
	testCBLoader(t, tt, session, ConcurrentDeletes)
}

func testCBLoader(t *testing.T, tt *test.T, session *db.Session, mode ConcurrencyMode) {
	var ids []string
	for i := 0; i < 100; i++ {
		balanceID := xdr.ClaimableBalanceId{
			Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
			V0:   &xdr.Hash{byte(i)},
		}
		id, err := xdr.MarshalHex(balanceID)
		tt.Assert.NoError(err)
		ids = append(ids, id)
	}

	loader := NewClaimableBalanceLoader(mode)
	var futures []FutureClaimableBalanceID
	for _, id := range ids {
		future := loader.GetFuture(id)
		futures = append(futures, future)
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
	for i, id := range ids {
		future := futures[i]
		var internalID driver.Value
		internalID, err = future.Value()
		assert.NoError(t, err)
		var cb HistoryClaimableBalance
		cb, err = q.ClaimableBalanceByID(context.Background(), id)
		assert.NoError(t, err)
		assert.Equal(t, cb.BalanceID, id)
		assert.Equal(t, cb.InternalID, internalID)
	}

	futureCb := &FutureClaimableBalanceID{key: "not-present", loader: loader}
	_, err = futureCb.Value()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `was not found`)

	// check that Loader works when all the previous values are already
	// present in the db and also add 10 more rows to insert
	loader = NewClaimableBalanceLoader(mode)
	for i := 100; i < 110; i++ {
		balanceID := xdr.ClaimableBalanceId{
			Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
			V0:   &xdr.Hash{byte(i)},
		}
		var id string
		id, err = xdr.MarshalHex(balanceID)
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
		internalID, err := loader.GetNow(id)
		assert.NoError(t, err)
		var cb HistoryClaimableBalance
		cb, err = q.ClaimableBalanceByID(context.Background(), id)
		assert.NoError(t, err)
		assert.Equal(t, cb.BalanceID, id)
		assert.Equal(t, cb.InternalID, internalID)
	}
}

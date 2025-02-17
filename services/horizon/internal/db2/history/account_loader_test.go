package history

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/support/db"
)

func TestAccountLoader(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	session := tt.HorizonSession()

	testAccountLoader(t, session, ConcurrentInserts)
	test.ResetHorizonDB(t, tt.HorizonDB)
	testAccountLoader(t, session, ConcurrentDeletes)
}

func testAccountLoader(t *testing.T, session *db.Session, mode ConcurrencyMode) {
	var addresses []string
	for i := 0; i < 100; i++ {
		addresses = append(addresses, keypair.MustRandom().Address())
	}

	loader := NewAccountLoader(mode)
	for _, address := range addresses {
		future := loader.GetFuture(address)
		_, err := future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid loader state,`)
		duplicateFuture := loader.GetFuture(address)
		assert.Equal(t, future, duplicateFuture)
	}

	err := loader.Exec(context.Background(), session)
	assert.NoError(t, err)
	assert.Equal(t, LoaderStats{
		Total:    100,
		Inserted: 100,
	}, loader.Stats())
	assert.Panics(t, func() {
		loader.GetFuture(keypair.MustRandom().Address())
	})

	q := &Q{session}
	for _, address := range addresses {
		var internalId int64
		internalId, err = loader.GetNow(address)
		assert.NoError(t, err)
		var account Account
		assert.NoError(t, q.AccountByAddress(context.Background(), &account, address))
		assert.Equal(t, account.ID, internalId)
		assert.Equal(t, account.Address, address)
	}

	_, err = loader.GetNow("not present")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `was not found`)

	// check that Loader works when all the previous values are already
	// present in the db and also add 10 more rows to insert
	loader = NewAccountLoader(mode)
	for i := 0; i < 10; i++ {
		addresses = append(addresses, keypair.MustRandom().Address())
	}

	for _, address := range addresses {
		future := loader.GetFuture(address)
		_, err = future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid loader state,`)
	}

	assert.NoError(t, loader.Exec(context.Background(), session))
	assert.Equal(t, LoaderStats{
		Total:    110,
		Inserted: 10,
	}, loader.Stats())

	for _, address := range addresses {
		var internalId int64
		internalId, err = loader.GetNow(address)
		assert.NoError(t, err)
		var account Account
		assert.NoError(t, q.AccountByAddress(context.Background(), &account, address))
		assert.Equal(t, account.ID, internalId)
		assert.Equal(t, account.Address, address)
	}
}

package actions

import (
	"context"
	"net/http"
	"testing"

	horizonContext "github.com/stellar/go/services/horizon/internal/context"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stretchr/testify/assert"
)

func TestAssetsForAddressRequiresTransaction(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &history.Q{tt.HorizonSession()}

	r := &http.Request{}
	ctx := context.WithValue(
		r.Context(),
		&horizonContext.SessionContextKey,
		q,
	)

	_, _, err := assetsForAddress(r.WithContext(ctx), "GCATOZ7YJV2FANQQLX47TIV6P7VMPJCEEJGQGR6X7TONPKBN3UCLKEIS")
	assert.EqualError(t, err, "cannot be called outside of a transaction")

	assert.NoError(t, q.Begin(ctx))
	defer q.Rollback()

	_, _, err = assetsForAddress(r.WithContext(ctx), "GCATOZ7YJV2FANQQLX47TIV6P7VMPJCEEJGQGR6X7TONPKBN3UCLKEIS")
	assert.EqualError(t, err, "should only be called in a repeatable read transaction")
}

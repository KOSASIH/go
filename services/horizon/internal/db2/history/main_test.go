package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/services/horizon/internal/test"
)

func TestLatestLedger(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	var seq int
	err := q.LatestLedger(tt.Ctx, &seq)

	if tt.Assert.NoError(err) {
		tt.Assert.Equal(3, seq)
	}
}

func TestLatestLedgerSequenceClosedAt(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	sequence, closedAt, err := q.LatestLedgerSequenceClosedAt(tt.Ctx)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(int32(3), sequence)
		tt.Assert.Equal("2019-10-31T13:19:46Z", closedAt.Format(time.RFC3339))
	}

	test.ResetHorizonDB(t, tt.HorizonDB)

	sequence, closedAt, err = q.LatestLedgerSequenceClosedAt(tt.Ctx)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(int32(0), sequence)
		tt.Assert.Equal("0001-01-01T00:00:00Z", closedAt.Format(time.RFC3339))
	}
}

func TestGetLatestHistoryLedgerEmptyDB(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	value, err := q.GetLatestHistoryLedger(tt.Ctx)
	tt.Assert.NoError(err)
	tt.Assert.Equal(uint32(0), value)
}

func TestElderLedger(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	var seq int
	err := q.ElderLedger(tt.Ctx, &seq)

	if tt.Assert.NoError(err) {
		tt.Assert.Equal(1, seq)
	}
}

func TestConstructDeleteLookupTableRowsQuery(t *testing.T) {
	query := constructDeleteLookupTableRowsQuery(
		"history_accounts",
		[]int64{100, 20, 30},
	)

	assert.Equal(t,
		"WITH ha_batch AS (SELECT id FROM history_accounts WHERE id IN (100, 20, 30) ORDER BY id asc FOR UPDATE) "+
			"DELETE FROM history_accounts WHERE id IN (SELECT e1.id as id FROM ha_batch e1 "+
			"WHERE NOT EXISTS ( SELECT 1 as row FROM history_transaction_participants WHERE history_transaction_participants.history_account_id = id LIMIT 1) "+
			"AND NOT EXISTS ( SELECT 1 as row FROM history_effects WHERE history_effects.history_account_id = id LIMIT 1) "+
			"AND NOT EXISTS ( SELECT 1 as row FROM history_operation_participants WHERE history_operation_participants.history_account_id = id LIMIT 1) "+
			"AND NOT EXISTS ( SELECT 1 as row FROM history_trades WHERE history_trades.base_account_id = id LIMIT 1) "+
			"AND NOT EXISTS ( SELECT 1 as row FROM history_trades WHERE history_trades.counter_account_id = id LIMIT 1))", query)
}

func TestConstructReapLookupTablesQuery(t *testing.T) {
	query := constructFindReapLookupTablesQuery(
		"history_accounts",
		10,
		0,
	)

	assert.Equal(t,
		"WITH ha_batch AS (SELECT id FROM history_accounts WHERE id >= 0 ORDER BY id ASC limit 10) SELECT e1.id as id FROM ha_batch e1 "+
			"WHERE NOT EXISTS ( SELECT 1 as row FROM history_transaction_participants WHERE history_transaction_participants.history_account_id = id LIMIT 1) "+
			"AND NOT EXISTS ( SELECT 1 as row FROM history_effects WHERE history_effects.history_account_id = id LIMIT 1) "+
			"AND NOT EXISTS ( SELECT 1 as row FROM history_operation_participants WHERE history_operation_participants.history_account_id = id LIMIT 1) "+
			"AND NOT EXISTS ( SELECT 1 as row FROM history_trades WHERE history_trades.base_account_id = id LIMIT 1) "+
			"AND NOT EXISTS ( SELECT 1 as row FROM history_trades WHERE history_trades.counter_account_id = id LIMIT 1)", query)
}

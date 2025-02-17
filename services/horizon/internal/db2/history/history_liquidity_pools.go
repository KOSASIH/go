package history

import (
	"context"
	"sort"

	sq "github.com/Masterminds/squirrel"

	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
)

// QHistoryLiquidityPools defines account related queries.
type QHistoryLiquidityPools interface {
	CreateHistoryLiquidityPools(ctx context.Context, poolIDs []string, batchSize int) (map[string]int64, error)
	NewOperationLiquidityPoolBatchInsertBuilder() OperationLiquidityPoolBatchInsertBuilder
	NewTransactionLiquidityPoolBatchInsertBuilder() TransactionLiquidityPoolBatchInsertBuilder
}

// CreateHistoryLiquidityPools creates rows in the history_liquidity_pools table for a given list of ids.
// CreateHistoryLiquidityPools returns a mapping of id to its corresponding internal id in the history_liquidity_pools table
func (q *Q) CreateHistoryLiquidityPools(ctx context.Context, poolIDs []string, batchSize int) (map[string]int64, error) {
	if len(poolIDs) == 0 {
		return nil, nil
	}

	builder := &db.BatchInsertBuilder{
		Table:        q.GetTable("history_liquidity_pools"),
		MaxBatchSize: batchSize,
		Suffix:       "ON CONFLICT (liquidity_pool_id) DO NOTHING",
	}

	// sort before inserting to prevent deadlocks on acquiring a ShareLock
	// https://github.com/stellar/go/issues/2370
	sort.Strings(poolIDs)
	var deduped []string
	for i, id := range poolIDs {
		if i > 0 && id == poolIDs[i-1] {
			// skip duplicates
			continue
		}
		deduped = append(deduped, id)
		err := builder.Row(ctx, map[string]interface{}{
			"liquidity_pool_id": id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "could not insert history_liquidity_pools row")
		}
	}

	err := builder.Exec(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not exec claimable balance insert builder")
	}

	var lps []HistoryLiquidityPool
	toInternalID := map[string]int64{}
	const selectBatchSize = 10000

	for i := 0; i < len(deduped); i += selectBatchSize {
		end := i + selectBatchSize
		if end > len(deduped) {
			end = len(deduped)
		}
		subset := deduped[i:end]

		lps, err = q.LiquidityPoolsByIDs(ctx, subset)
		if err != nil {
			return nil, errors.Wrap(err, "could not select claimable balances")
		}

		for _, lp := range lps {
			toInternalID[lp.PoolID] = lp.InternalID
		}
	}

	return toInternalID, nil
}

// HistoryLiquidityPool is a row of data from the `history_liquidity_pools` table
type HistoryLiquidityPool struct {
	PoolID     string `db:"liquidity_pool_id"`
	InternalID int64  `db:"id"`
}

var selectHistoryLiquidityPool = sq.Select("hlp.*").From("history_liquidity_pools hlp")

// LiquidityPoolsByIDs loads rows from `history_liquidity_pools`, by liquidity_pool_id
func (q *Q) LiquidityPoolsByIDs(ctx context.Context, poolIDs []string) (dest []HistoryLiquidityPool, err error) {
	sql := selectHistoryLiquidityPool.Where(map[string]interface{}{
		"hlp.liquidity_pool_id": poolIDs, // hlp.liquidity_pool_id IN (...)
	})
	err = q.Select(ctx, &dest, sql)
	return dest, err
}

// LiquidityPoolByID loads a row from `history_liquidity_pools`, by liquidity_pool_id
func (q *Q) LiquidityPoolByID(ctx context.Context, poolID string) (dest HistoryLiquidityPool, err error) {
	sql := selectHistoryLiquidityPool.Limit(1).Where("hlp.liquidity_pool_id = ?", poolID)
	err = q.Get(ctx, &dest, sql)
	return dest, err
}

type OperationLiquidityPoolBatchInsertBuilder interface {
	Add(operationID int64, lp FutureLiquidityPoolID) error
	Exec(ctx context.Context, session db.SessionInterface) error
}

type operationLiquidityPoolBatchInsertBuilder struct {
	table   string
	builder db.FastBatchInsertBuilder
}

func (q *Q) NewOperationLiquidityPoolBatchInsertBuilder() OperationLiquidityPoolBatchInsertBuilder {
	return &operationLiquidityPoolBatchInsertBuilder{
		table:   "history_operation_liquidity_pools",
		builder: db.FastBatchInsertBuilder{},
	}
}

// Add adds a new operation claimable balance to the batch
func (i *operationLiquidityPoolBatchInsertBuilder) Add(operationID int64, lp FutureLiquidityPoolID) error {
	return i.builder.Row(map[string]interface{}{
		"history_operation_id":      operationID,
		"history_liquidity_pool_id": lp,
	})
}

// Exec flushes all pending operation claimable balances to the db
func (i *operationLiquidityPoolBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	return i.builder.Exec(ctx, session, i.table)
}

type TransactionLiquidityPoolBatchInsertBuilder interface {
	Add(transactionID int64, lp FutureLiquidityPoolID) error
	Exec(ctx context.Context, session db.SessionInterface) error
}

type transactionLiquidityPoolBatchInsertBuilder struct {
	table   string
	builder db.FastBatchInsertBuilder
}

func (q *Q) NewTransactionLiquidityPoolBatchInsertBuilder() TransactionLiquidityPoolBatchInsertBuilder {
	return &transactionLiquidityPoolBatchInsertBuilder{
		table:   "history_transaction_liquidity_pools",
		builder: db.FastBatchInsertBuilder{},
	}
}

// Add adds a new transaction claimable balance to the batch
func (i *transactionLiquidityPoolBatchInsertBuilder) Add(transactionID int64, lp FutureLiquidityPoolID) error {
	return i.builder.Row(map[string]interface{}{
		"history_transaction_id":    transactionID,
		"history_liquidity_pool_id": lp,
	})
}

// Exec flushes all pending transaction claimable balances to the db
func (i *transactionLiquidityPoolBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	return i.builder.Exec(ctx, session, i.table)
}

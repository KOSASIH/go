package history

import (
	"cmp"

	"github.com/stellar/go/support/collections/set"
)

// FutureLiquidityPoolID represents a future history liquidity pool.
// A FutureLiquidityPoolID is created by an LiquidityPoolLoader and
// the liquidity pool id is available after calling Exec() on
// the LiquidityPoolLoader.
type FutureLiquidityPoolID = future[string, HistoryLiquidityPool]

// LiquidityPoolLoader will map liquidity pools to their internal
// history ids. If there is no existing mapping for a given liquidity pool,
// the LiquidityPoolLoader will insert into the history_liquidity_pools table to
// establish a mapping.
type LiquidityPoolLoader = loader[string, HistoryLiquidityPool]

// NewLiquidityPoolLoader will construct a new LiquidityPoolLoader instance.
func NewLiquidityPoolLoader(concurrencyMode ConcurrencyMode) *LiquidityPoolLoader {
	return &LiquidityPoolLoader{
		sealed: false,
		set:    set.Set[string]{},
		ids:    map[string]int64{},
		stats:  LoaderStats{},
		name:   "LiquidityPoolLoader",
		table:  "history_liquidity_pools",
		columnsForKeys: func(keys []string) []columnValues {
			return []columnValues{
				{
					name:    "liquidity_pool_id",
					dbType:  "text",
					objects: keys,
				},
			}
		},
		mappingFromRow: func(row HistoryLiquidityPool) (string, int64) {
			return row.PoolID, row.InternalID
		},
		less:            cmp.Less[string],
		concurrencyMode: concurrencyMode,
	}
}

// LiquidityPoolLoaderStub is a stub wrapper around LiquidityPoolLoader which allows
// you to manually configure the mapping of liquidity pools to history liquidity ppol ids
type LiquidityPoolLoaderStub struct {
	Loader *LiquidityPoolLoader
}

// NewLiquidityPoolLoaderStub returns a new LiquidityPoolLoader instance
func NewLiquidityPoolLoaderStub() LiquidityPoolLoaderStub {
	return LiquidityPoolLoaderStub{Loader: NewLiquidityPoolLoader(ConcurrentInserts)}
}

// Insert updates the wrapped LiquidityPoolLoader so that the given liquidity pool
// is mapped to the provided history liquidity pool id
func (a LiquidityPoolLoaderStub) Insert(lp string, id int64) {
	a.Loader.sealed = true
	a.Loader.ids[lp] = id
}

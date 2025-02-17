package history

import (
	"cmp"

	"github.com/stellar/go/support/collections/set"
)

// FutureClaimableBalanceID represents a future history claimable balance.
// A FutureClaimableBalanceID is created by a ClaimableBalanceLoader and
// the claimable balance id is available after calling Exec() on
// the ClaimableBalanceLoader.
type FutureClaimableBalanceID = future[string, HistoryClaimableBalance]

// ClaimableBalanceLoader will map claimable balance ids to their internal
// history ids. If there is no existing mapping for a given claimable balance id,
// the ClaimableBalanceLoader will insert into the history_claimable_balances table to
// establish a mapping.
type ClaimableBalanceLoader = loader[string, HistoryClaimableBalance]

// NewClaimableBalanceLoader will construct a new ClaimableBalanceLoader instance.
func NewClaimableBalanceLoader(concurrencyMode ConcurrencyMode) *ClaimableBalanceLoader {
	return &ClaimableBalanceLoader{
		sealed: false,
		set:    set.Set[string]{},
		ids:    map[string]int64{},
		stats:  LoaderStats{},
		name:   "ClaimableBalanceLoader",
		table:  "history_claimable_balances",
		columnsForKeys: func(keys []string) []columnValues {
			return []columnValues{
				{
					name:    "claimable_balance_id",
					dbType:  "text",
					objects: keys,
				},
			}
		},
		mappingFromRow: func(row HistoryClaimableBalance) (string, int64) {
			return row.BalanceID, row.InternalID
		},
		less:            cmp.Less[string],
		concurrencyMode: concurrencyMode,
	}
}

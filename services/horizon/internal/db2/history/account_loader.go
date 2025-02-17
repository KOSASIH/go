package history

import (
	"cmp"

	"github.com/stellar/go/support/collections/set"
)

// FutureAccountID represents a future history account.
// A FutureAccountID is created by an AccountLoader and
// the account id is available after calling Exec() on
// the AccountLoader.
type FutureAccountID = future[string, Account]

// AccountLoader will map account addresses to their history
// account ids. If there is no existing mapping for a given address,
// the AccountLoader will insert into the history_accounts table to
// establish a mapping.
type AccountLoader = loader[string, Account]

// NewAccountLoader will construct a new AccountLoader instance.
func NewAccountLoader(concurrencyMode ConcurrencyMode) *AccountLoader {
	return &AccountLoader{
		sealed: false,
		set:    set.Set[string]{},
		ids:    map[string]int64{},
		stats:  LoaderStats{},
		name:   "AccountLoader",
		table:  "history_accounts",
		columnsForKeys: func(addresses []string) []columnValues {
			return []columnValues{
				{
					name:    "address",
					dbType:  "character varying(64)",
					objects: addresses,
				},
			}
		},
		mappingFromRow: func(account Account) (string, int64) {
			return account.Address, account.ID
		},
		less:            cmp.Less[string],
		concurrencyMode: concurrencyMode,
	}
}

// AccountLoaderStub is a stub wrapper around AccountLoader which allows
// you to manually configure the mapping of addresses to history account ids
type AccountLoaderStub struct {
	Loader *AccountLoader
}

// NewAccountLoaderStub returns a new AccountLoaderStub instance
func NewAccountLoaderStub() AccountLoaderStub {
	return AccountLoaderStub{Loader: NewAccountLoader(ConcurrentInserts)}
}

// Insert updates the wrapped AccountLoader so that the given account
// address is mapped to the provided history account id
func (a AccountLoaderStub) Insert(address string, id int64) {
	a.Loader.sealed = true
	a.Loader.ids[address] = id
}

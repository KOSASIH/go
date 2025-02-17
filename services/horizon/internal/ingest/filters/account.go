package filters

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/support/collections/set"
)

type accountFilter struct {
	whitelistedAccountsSet set.Set[string]
	lastModified           int64
	enabled                bool
}

type AccountFilter interface {
	processors.LedgerTransactionFilterer
	RefreshAccountFilter(filterConfig *history.AccountFilterConfig) error
}

func NewAccountFilter() AccountFilter {
	return &accountFilter{
		whitelistedAccountsSet: set.Set[string]{},
	}
}

func (f *accountFilter) Name() string {
	return "filters.accountFilter"
}

func (f *accountFilter) RefreshAccountFilter(filterConfig *history.AccountFilterConfig) error {
	// only need to re-initialize the filter config state(rules) if its cached version(in  memory)
	// is older than the incoming config version based on lastModified epoch timestamp
	if filterConfig.LastModified > f.lastModified {
		logger.Infof("New Account Filter config detected, reloading new config %v ", *filterConfig)

		f.enabled = filterConfig.Enabled
		f.whitelistedAccountsSet = listToSet(filterConfig.Whitelist)
		f.lastModified = filterConfig.LastModified
	}

	return nil
}

func (f *accountFilter) FilterTransaction(ctx context.Context, transaction ingest.LedgerTransaction) (bool, bool, error) {
	if !f.isEnabled() {
		return false, true, nil
	}

	participants, err := processors.ParticipantsForTransaction(0, transaction)
	if err != nil {
		return true, false, err
	}

	// NOTE: this assumes that the participant list has a small memory footprint
	//       otherwise, we should be doing the filtering on the DB side
	for _, p := range participants {
		if f.whitelistedAccountsSet.Contains(p.Address()) {
			return true, true, nil
		}
	}
	return true, false, nil
}

func (f accountFilter) isEnabled() bool {
	// filtering is disabled if the whitelist is empty for now, as that is the only filter rule
	return len(f.whitelistedAccountsSet) >= 1 && f.enabled
}

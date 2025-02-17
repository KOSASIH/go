package ingest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/xdr"
)

func TestStatsChangeProcessor(t *testing.T) {
	ctx := context.Background()
	processor := &StatsChangeProcessor{}

	for ledgerEntryType := range xdr.LedgerEntryTypeMap {
		// Created
		assert.NoError(t, processor.ProcessChange(ctx, Change{
			Type: xdr.LedgerEntryType(ledgerEntryType),
			Pre:  nil,
			Post: &xdr.LedgerEntry{},
		}))
		// Updated
		assert.NoError(t, processor.ProcessChange(ctx, Change{
			Type: xdr.LedgerEntryType(ledgerEntryType),
			Pre:  &xdr.LedgerEntry{},
			Post: &xdr.LedgerEntry{},
		}))
		// Removed
		assert.NoError(t, processor.ProcessChange(ctx, Change{
			Type: xdr.LedgerEntryType(ledgerEntryType),
			Pre:  &xdr.LedgerEntry{},
			Post: nil,
		}))
	}

	results := processor.GetResults()

	assert.Equal(t, int64(1), results.AccountsCreated)
	assert.Equal(t, int64(1), results.ClaimableBalancesCreated)
	assert.Equal(t, int64(1), results.DataCreated)
	assert.Equal(t, int64(1), results.OffersCreated)
	assert.Equal(t, int64(1), results.TrustLinesCreated)
	assert.Equal(t, int64(1), results.LiquidityPoolsCreated)
	assert.Equal(t, int64(1), results.ContractDataCreated)
	assert.Equal(t, int64(1), results.ContractCodeCreated)
	assert.Equal(t, int64(1), results.ConfigSettingsCreated)
	assert.Equal(t, int64(1), results.TtlCreated)

	assert.Equal(t, int64(1), results.AccountsUpdated)
	assert.Equal(t, int64(1), results.ClaimableBalancesUpdated)
	assert.Equal(t, int64(1), results.DataUpdated)
	assert.Equal(t, int64(1), results.OffersUpdated)
	assert.Equal(t, int64(1), results.TrustLinesUpdated)
	assert.Equal(t, int64(1), results.LiquidityPoolsUpdated)
	assert.Equal(t, int64(1), results.ContractDataUpdated)
	assert.Equal(t, int64(1), results.ContractCodeUpdated)
	assert.Equal(t, int64(1), results.ConfigSettingsUpdated)
	assert.Equal(t, int64(1), results.TtlUpdated)

	assert.Equal(t, int64(1), results.AccountsRemoved)
	assert.Equal(t, int64(1), results.ClaimableBalancesRemoved)
	assert.Equal(t, int64(1), results.DataRemoved)
	assert.Equal(t, int64(1), results.OffersRemoved)
	assert.Equal(t, int64(1), results.TrustLinesRemoved)
	assert.Equal(t, int64(1), results.LiquidityPoolsRemoved)
	assert.Equal(t, int64(1), results.ContractCodeRemoved)
	assert.Equal(t, int64(1), results.ContractDataRemoved)
	assert.Equal(t, int64(1), results.ConfigSettingsRemoved)
	assert.Equal(t, int64(1), results.TtlRemoved)

	assert.Equal(t, len(xdr.LedgerEntryTypeMap)*3, len(results.Map()))
}

package resourceadapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/protocols/horizon"
	protocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/xdr"
)

func TestPopulateExpAssetStat(t *testing.T) {
	row := history.AssetAndContractStat{
		ExpAssetStat: history.ExpAssetStat{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
			AssetCode:   "XIM",
			AssetIssuer: "GBZ35ZJRIKJGYH5PBKLKOZ5L6EXCNTO7BKIL7DAVVDFQ2ODJEEHHJXIM",
			Accounts: history.ExpAssetStatAccounts{
				Authorized:                      429,
				AuthorizedToMaintainLiabilities: 214,
				Unauthorized:                    107,
				ClaimableBalances:               12,
			},
			Balances: history.ExpAssetStatBalances{
				Authorized:                      "100000000000000000000",
				AuthorizedToMaintainLiabilities: "50000000000000000000",
				Unauthorized:                    "2500000000000000000",
				ClaimableBalances:               "1200000000000000000",
				LiquidityPools:                  "7700000000000000000",
			},
		},
		Contracts: history.ContractStat{
			ActiveBalance:   "900000000000000000",
			ActiveHolders:   6,
			ArchivedBalance: "700000000000000000",
			ArchivedHolders: 3,
		},
	}
	issuer := history.AccountEntry{
		AccountID:  "GBZ35ZJRIKJGYH5PBKLKOZ5L6EXCNTO7BKIL7DAVVDFQ2ODJEEHHJXIM",
		Flags:      0,
		HomeDomain: "xim.com",
	}

	var res protocol.AssetStat
	err := PopulateAssetStat(context.Background(), &res, row, issuer)
	assert.NoError(t, err)

	assert.Equal(t, "credit_alphanum4", res.Type)
	assert.Equal(t, "XIM", res.Code)
	assert.Equal(t, "GBZ35ZJRIKJGYH5PBKLKOZ5L6EXCNTO7BKIL7DAVVDFQ2ODJEEHHJXIM", res.Issuer)
	assert.Equal(t, int32(429), res.Accounts.Authorized)
	assert.Equal(t, int32(214), res.Accounts.AuthorizedToMaintainLiabilities)
	assert.Equal(t, int32(107), res.Accounts.Unauthorized)
	assert.Equal(t, int32(12), res.NumClaimableBalances)
	assert.Equal(t, int32(6), res.NumContracts)
	assert.Equal(t, int32(3), res.NumArchivedContracts)
	assert.Equal(t, "10000000000000.0000000", res.Balances.Authorized)
	assert.Equal(t, "5000000000000.0000000", res.Balances.AuthorizedToMaintainLiabilities)
	assert.Equal(t, "250000000000.0000000", res.Balances.Unauthorized)
	assert.Equal(t, "120000000000.0000000", res.ClaimableBalancesAmount)
	assert.Equal(t, "770000000000.0000000", res.LiquidityPoolsAmount)
	assert.Equal(t, "90000000000.0000000", res.ContractsAmount)
	assert.Equal(t, "70000000000.0000000", res.ArchivedContractsAmount)
	assert.Equal(t, horizon.AccountFlags{}, res.Flags)
	assert.Equal(t, "https://xim.com/.well-known/stellar.toml", res.Links.Toml.Href)
	assert.Equal(t, "", res.ContractID)
	assert.Equal(t, row.PagingToken(), res.PagingToken())

	contractID := [32]byte{}
	row.SetContractID(contractID)
	assert.NoError(t, PopulateAssetStat(context.Background(), &res, row, issuer))
	assert.Equal(t, "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4", res.ContractID)

	issuer.HomeDomain = ""
	issuer.Flags = uint32(xdr.AccountFlagsAuthRequiredFlag) |
		uint32(xdr.AccountFlagsAuthImmutableFlag) |
		uint32(xdr.AccountFlagsAuthClawbackEnabledFlag)

	err = PopulateAssetStat(context.Background(), &res, row, issuer)
	assert.NoError(t, err)

	assert.Equal(t, "credit_alphanum4", res.Type)
	assert.Equal(t, "XIM", res.Code)
	assert.Equal(t, "GBZ35ZJRIKJGYH5PBKLKOZ5L6EXCNTO7BKIL7DAVVDFQ2ODJEEHHJXIM", res.Issuer)
	assert.Equal(
		t,
		horizon.AccountFlags{
			AuthRequired:        true,
			AuthImmutable:       true,
			AuthClawbackEnabled: true,
		},
		res.Flags,
	)
	assert.Equal(t, "", res.Links.Toml.Href)
	assert.Equal(t, row.PagingToken(), res.PagingToken())
}

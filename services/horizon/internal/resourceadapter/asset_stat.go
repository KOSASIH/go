package resourceadapter

import (
	"context"
	"strings"

	"github.com/stellar/go/amount"
	protocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/render/hal"
	"github.com/stellar/go/xdr"
)

// PopulateAssetStat populates an AssetStat using asset stats and account entries
// generated from the ingestion system.
func PopulateAssetStat(
	ctx context.Context,
	res *protocol.AssetStat,
	row history.AssetAndContractStat,
	issuer history.AccountEntry,
) (err error) {
	if row.ContractID != nil {
		res.ContractID, err = strkey.Encode(strkey.VersionByteContract, *row.ContractID)
		if err != nil {
			return
		}
	}
	res.Asset.Type = xdr.AssetTypeToString[row.AssetType]
	res.Asset.Code = row.AssetCode
	res.Asset.Issuer = row.AssetIssuer
	res.Accounts = protocol.AssetStatAccounts{
		Authorized:                      row.Accounts.Authorized,
		AuthorizedToMaintainLiabilities: row.Accounts.AuthorizedToMaintainLiabilities,
		Unauthorized:                    row.Accounts.Unauthorized,
	}
	res.NumClaimableBalances = row.Accounts.ClaimableBalances
	res.NumLiquidityPools = row.Accounts.LiquidityPools
	res.NumContracts = row.Contracts.ActiveHolders
	res.NumArchivedContracts = row.Contracts.ArchivedHolders
	err = populateAssetStatBalances(res, row)
	if err != nil {
		return err
	}
	flags := int8(issuer.Flags)
	res.Flags = protocol.AccountFlags{
		(flags & int8(xdr.AccountFlagsAuthRequiredFlag)) != 0,
		(flags & int8(xdr.AccountFlagsAuthRevocableFlag)) != 0,
		(flags & int8(xdr.AccountFlagsAuthImmutableFlag)) != 0,
		(flags & int8(xdr.AccountFlagsAuthClawbackEnabledFlag)) != 0,
	}
	res.PT = row.PagingToken()

	trimmed := strings.TrimSpace(issuer.HomeDomain)
	var toml string
	if trimmed != "" {
		toml = "https://" + issuer.HomeDomain + "/.well-known/stellar.toml"
	}
	res.Links.Toml = hal.NewLink(toml)
	return
}

func populateAssetStatBalances(res *protocol.AssetStat, row history.AssetAndContractStat) (err error) {
	if err != nil {
		return errors.Wrap(err, "Invalid amount in PopulateAssetStat")
	}

	res.Balances.Authorized, err = amount.IntStringToAmount(row.Balances.Authorized)
	if err != nil {
		return errors.Wrapf(err, "Invalid amount in PopulateAssetStatBalances: %q", row.Balances.Authorized)
	}

	res.Balances.AuthorizedToMaintainLiabilities, err = amount.IntStringToAmount(row.Balances.AuthorizedToMaintainLiabilities)
	if err != nil {
		return errors.Wrapf(err, "Invalid amount in PopulateAssetStatBalances: %q", row.Balances.AuthorizedToMaintainLiabilities)
	}

	res.Balances.Unauthorized, err = amount.IntStringToAmount(row.Balances.Unauthorized)
	if err != nil {
		return errors.Wrapf(err, "Invalid amount in PopulateAssetStatBalances: %q", row.Balances.Unauthorized)
	}

	res.ClaimableBalancesAmount, err = amount.IntStringToAmount(row.Balances.ClaimableBalances)
	if err != nil {
		return errors.Wrapf(err, "Invalid amount in PopulateAssetStatBalances: %q", row.Balances.ClaimableBalances)
	}

	res.LiquidityPoolsAmount, err = amount.IntStringToAmount(row.Balances.LiquidityPools)
	if err != nil {
		return errors.Wrapf(err, "Invalid amount in PopulateAssetStatBalances: %q", row.Balances.LiquidityPools)
	}

	res.ContractsAmount, err = amount.IntStringToAmount(row.Contracts.ActiveBalance)
	if err != nil {
		return errors.Wrapf(err, "Invalid amount in PopulateAssetStatBalances: %q", row.Contracts.ActiveBalance)
	}

	res.ArchivedContractsAmount, err = amount.IntStringToAmount(row.Contracts.ArchivedBalance)
	if err != nil {
		return errors.Wrapf(err, "Invalid amount in PopulateAssetStatBalances: %q", row.Contracts.ArchivedBalance)
	}

	return nil
}

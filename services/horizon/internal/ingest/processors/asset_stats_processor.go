package processors

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type AssetStatsProcessor struct {
	assetStatsQ              history.QAssetStats
	currentLedger            uint32
	assetStatSet             AssetStatSet
	contractDataChanges      []ingest.Change
	removedExpirationEntries map[xdr.Hash]uint32
	createdExpirationEntries map[xdr.Hash]uint32
	updatedExpirationEntries map[xdr.Hash][2]uint32
	ingestFromHistoryArchive bool
	networkPassphrase        string
}

// NewAssetStatsProcessor constructs a new AssetStatsProcessor instance.
func NewAssetStatsProcessor(
	assetStatsQ history.QAssetStats,
	networkPassphrase string,
	ingestFromHistoryArchive bool,
	currentLedger uint32,
) *AssetStatsProcessor {
	p := &AssetStatsProcessor{
		currentLedger:            currentLedger,
		assetStatsQ:              assetStatsQ,
		ingestFromHistoryArchive: ingestFromHistoryArchive,
		networkPassphrase:        networkPassphrase,
		assetStatSet:             NewAssetStatSet(),
		contractDataChanges:      []ingest.Change{},
		removedExpirationEntries: map[xdr.Hash]uint32{},
		createdExpirationEntries: map[xdr.Hash]uint32{},
		updatedExpirationEntries: map[xdr.Hash][2]uint32{},
	}
	return p
}

func (p *AssetStatsProcessor) Name() string {
	return "processors.AssetStatsProcessor"
}

func (p *AssetStatsProcessor) ProcessChange(ctx context.Context, change ingest.Change) error {
	if change.Type != xdr.LedgerEntryTypeLiquidityPool &&
		change.Type != xdr.LedgerEntryTypeClaimableBalance &&
		change.Type != xdr.LedgerEntryTypeTrustline &&
		change.Type != xdr.LedgerEntryTypeContractData &&
		change.Type != xdr.LedgerEntryTypeTtl {
		return nil
	}

	var err error
	switch change.Type {
	case xdr.LedgerEntryTypeLiquidityPool:
		err = p.assetStatSet.AddLiquidityPool(change)
	case xdr.LedgerEntryTypeClaimableBalance:
		err = p.assetStatSet.AddClaimableBalance(change)
	case xdr.LedgerEntryTypeTrustline:
		err = p.assetStatSet.AddTrustline(change)
	case xdr.LedgerEntryTypeContractData:
		// only ingest contract data entries which could be relevant to
		// asset stats
		ledgerEntry := change.Post
		if ledgerEntry == nil {
			ledgerEntry = change.Pre
		}
		asset := AssetFromContractData(*ledgerEntry, p.networkPassphrase)
		_, _, balanceFound := ContractBalanceFromContractData(*ledgerEntry, p.networkPassphrase)
		if asset == nil && !balanceFound {
			return nil
		}
		p.contractDataChanges = append(p.contractDataChanges, change)
	case xdr.LedgerEntryTypeTtl:
		err = p.addExpirationChange(change)
	default:
		return errors.Errorf("Change type %v is unexpected", change.Type)
	}

	return err
}

func (p *AssetStatsProcessor) addExpirationChange(change ingest.Change) error {
	switch {
	case change.Pre == nil && change.Post != nil: // created
		post := change.Post.Data.MustTtl()
		p.createdExpirationEntries[post.KeyHash] = uint32(post.LiveUntilLedgerSeq)
	case change.Pre != nil && change.Post == nil: // removed
		pre := change.Pre.Data.MustTtl()
		p.removedExpirationEntries[pre.KeyHash] = uint32(pre.LiveUntilLedgerSeq)
	case change.Pre != nil && change.Post != nil: // updated
		pre := change.Pre.Data.MustTtl()
		post := change.Post.Data.MustTtl()
		// it's unclear if core could emit a ledger entry change where the
		// expiration ledger remains the same
		if pre.LiveUntilLedgerSeq == post.LiveUntilLedgerSeq {
			return nil
		}
		// but we expect that the expiration ledger will never decrease
		if pre.LiveUntilLedgerSeq > post.LiveUntilLedgerSeq {
			return errors.Errorf(
				"unexpected change in expiration ledger Pre: %v Post: %v",
				pre.LiveUntilLedgerSeq,
				post.LiveUntilLedgerSeq,
			)
		}
		// also the new expiration ledger must always be greater than or equal
		// to the current ledger
		if uint32(post.LiveUntilLedgerSeq) < p.currentLedger {
			return errors.Errorf(
				"post expiration ledger is less than current ledger."+
					" Pre: %v Post: %v current ledger: %v",
				pre.LiveUntilLedgerSeq,
				post.LiveUntilLedgerSeq,
				p.currentLedger,
			)
		}
		p.updatedExpirationEntries[pre.KeyHash] = [2]uint32{
			uint32(pre.LiveUntilLedgerSeq),
			uint32(post.LiveUntilLedgerSeq),
		}
	default:
		return errors.Errorf("unexpected change Pre: %v Post: %v", change.Pre, change.Post)
	}
	return nil
}

func (p *AssetStatsProcessor) Commit(ctx context.Context) error {

	contractAssetStatSet := NewContractAssetStatSet(
		p.assetStatsQ,
		p.networkPassphrase,
		p.removedExpirationEntries,
		p.createdExpirationEntries,
		p.updatedExpirationEntries,
		p.currentLedger,
	)
	for _, change := range p.contractDataChanges {
		if err := contractAssetStatSet.AddContractData(ctx, change); err != nil {
			return errors.Wrap(err, "Error ingesting contract data")
		}
	}

	return p.updateDB(ctx, contractAssetStatSet)
}

func (p *AssetStatsProcessor) updateDB(
	ctx context.Context,
	contractAssetStatSet *ContractAssetStatSet,
) error {
	if p.ingestFromHistoryArchive {
		// When ingesting from the history archives we can take advantage of the fact
		// that there are only created ledger entries. We don't need to execute any
		// updates or removals on the asset stats tables. And we can also skip
		// ingesting restored contract balances and expired contract balances.
		assetStatsDeltas := p.assetStatSet.All()
		if len(assetStatsDeltas) > 0 {
			var err error
			assetStatsDeltas, err = IncludeContractIDsInAssetStats(
				p.networkPassphrase,
				assetStatsDeltas,
				contractAssetStatSet.contractToAsset,
			)
			if err != nil {
				return errors.Wrap(err, "Error extracting asset stat rows")
			}
			if err := p.assetStatsQ.InsertAssetStats(ctx, assetStatsDeltas); err != nil {
				return errors.Wrap(err, "Error inserting asset stats")
			}
		}

		if rows := contractAssetStatSet.GetContractStats(); len(rows) > 0 {
			if err := p.assetStatsQ.InsertContractAssetStats(ctx, rows); err != nil {
				return errors.Wrap(err, "Error inserting asset contract stats")
			}
		}

		if len(contractAssetStatSet.createdBalances) > 0 {
			if err := p.assetStatsQ.InsertContractAssetBalances(ctx, contractAssetStatSet.createdBalances); err != nil {
				return errors.Wrap(err, "Error inserting asset contract stats")
			}
		}
		return nil
	}

	assetStatsDeltas := p.assetStatSet.All()

	if err := p.updateAssetStats(ctx, assetStatsDeltas, contractAssetStatSet.contractToAsset); err != nil {
		return err
	}
	if err := p.updateContractIDs(ctx, contractAssetStatSet.contractToAsset); err != nil {
		return err
	}

	if err := p.assetStatsQ.RemoveContractAssetBalances(ctx, contractAssetStatSet.removedBalances); err != nil {
		return errors.Wrap(err, "Error removing contract asset balances")
	}

	if err := p.updateContractAssetBalanceAmounts(ctx, contractAssetStatSet.updatedBalances); err != nil {
		return err
	}

	if err := p.assetStatsQ.InsertContractAssetBalances(ctx, contractAssetStatSet.createdBalances); err != nil {
		return errors.Wrap(err, "Error inserting contract asset balances")
	}

	if err := contractAssetStatSet.ingestRestoredBalances(ctx); err != nil {
		return err
	}

	if err := p.updateContractAssetBalanceExpirations(ctx); err != nil {
		return err
	}

	if err := contractAssetStatSet.ingestExpiredBalances(ctx); err != nil {
		return err
	}

	return p.updateContractAssetStats(ctx, contractAssetStatSet.contractAssetStats)
}

func (p *AssetStatsProcessor) updateContractAssetBalanceAmounts(ctx context.Context, updatedBalances map[xdr.Hash]*big.Int) error {
	keys := make([]xdr.Hash, 0, len(updatedBalances))
	amounts := make([]string, 0, len(updatedBalances))
	for key, amount := range updatedBalances {
		keys = append(keys, key)
		amounts = append(amounts, amount.String())
	}
	if err := p.assetStatsQ.UpdateContractAssetBalanceAmounts(ctx, keys, amounts); err != nil {
		return errors.Wrap(err, "Error updating contract asset balance amounts")
	}
	return nil
}

func (p *AssetStatsProcessor) updateContractAssetBalanceExpirations(ctx context.Context) error {
	keys := make([]xdr.Hash, 0, len(p.updatedExpirationEntries))
	expirationLedgers := make([]uint32, 0, len(p.updatedExpirationEntries))
	for key, update := range p.updatedExpirationEntries {
		keys = append(keys, key)
		expirationLedgers = append(expirationLedgers, update[1])
	}
	if err := p.assetStatsQ.UpdateContractAssetBalanceExpirations(ctx, keys, expirationLedgers); err != nil {
		return errors.Wrap(err, "Error updating contract asset balance expirations")
	}
	return nil
}

func IncludeContractIDsInAssetStats(
	networkPassphrase string,
	assetStatsDeltas []history.ExpAssetStat,
	contractToAsset map[xdr.Hash]*xdr.Asset,
) ([]history.ExpAssetStat, error) {
	included := map[xdr.Hash]bool{}
	// modify the asset stat row to update the contract_id column whenever we encounter a
	// contract data ledger entry with the Stellar asset metadata.
	for i, assetStatDelta := range assetStatsDeltas {
		// asset stats only supports non-native assets
		asset := xdr.MustNewCreditAsset(assetStatDelta.AssetCode, assetStatDelta.AssetIssuer)
		contractID, err := asset.ContractID(networkPassphrase)
		if err != nil {
			return nil, errors.Wrap(err, "cannot compute contract id for asset")
		}
		if asset, ok := contractToAsset[contractID]; ok && asset == nil {
			return nil, ingest.NewStateError(fmt.Errorf(
				"unexpected contract data removal in history archives: %s",
				hex.EncodeToString(contractID[:]),
			))
		} else if ok {
			assetStatDelta.SetContractID(contractID)
			included[contractID] = true
		}

		assetStatsDeltas[i] = assetStatDelta
	}

	// There is also a corner case where a Stellar Asset contract is initialized before there exists any
	// trustlines / claimable balances for the Stellar Asset. In this case, when ingesting contract data
	// ledger entries, there will be no existing asset stat row. We handle this case by inserting a row
	// with zero stats just so we can populate the contract id.
	for contractID, asset := range contractToAsset {
		if included[contractID] {
			continue
		}
		if asset == nil {
			return nil, ingest.NewStateError(fmt.Errorf(
				"unexpected contract data removal in history archives: %s",
				hex.EncodeToString(contractID[:]),
			))
		}
		var assetType xdr.AssetType
		var assetCode, assetIssuer string
		asset.MustExtract(&assetType, &assetCode, &assetIssuer)
		row := history.ExpAssetStat{
			AssetType:   assetType,
			AssetCode:   assetCode,
			AssetIssuer: assetIssuer,
			Accounts:    history.ExpAssetStatAccounts{},
			Balances:    newAssetStatBalance().ConvertToHistoryObject(),
		}
		row.SetContractID(contractID)
		assetStatsDeltas = append(assetStatsDeltas, row)
	}

	return assetStatsDeltas, nil
}

func (p *AssetStatsProcessor) updateAssetStats(
	ctx context.Context,
	assetStatsDeltas []history.ExpAssetStat,
	contractToAsset map[xdr.Hash]*xdr.Asset,
) error {
	for _, delta := range assetStatsDeltas {
		var rowsAffected int64
		var stat history.ExpAssetStat
		var err error
		// asset stats only supports non-native assets
		asset := xdr.MustNewCreditAsset(delta.AssetCode, delta.AssetIssuer)
		contractID, err := asset.ContractID(p.networkPassphrase)
		if err != nil {
			return errors.Wrap(err, "cannot compute contract id for asset")
		}

		stat, err = p.assetStatsQ.GetAssetStat(ctx,
			delta.AssetType,
			delta.AssetCode,
			delta.AssetIssuer,
		)
		assetStatNotFound := err == sql.ErrNoRows
		if !assetStatNotFound && err != nil {
			return errors.Wrap(err, "could not fetch asset stat from db")
		}
		assetStatFound := !assetStatNotFound
		if assetStatFound {
			delta.ContractID = stat.ContractID
		}

		if asset, ok := contractToAsset[contractID]; ok && asset == nil {
			if assetStatFound && stat.ContractID == nil {
				return ingest.NewStateError(errors.Errorf(
					"row has no contract id to remove %s: %s %s %s",
					hex.EncodeToString(contractID[:]),
					stat.AssetType,
					stat.AssetCode,
					stat.AssetIssuer,
				))
			}
			delta.ContractID = nil
		} else if ok {
			if assetStatFound && stat.ContractID != nil {
				return ingest.NewStateError(errors.Errorf(
					"attempting to set contract id %s but row %s already has contract id set: %s",
					hex.EncodeToString(contractID[:]),
					asset.String(),
					hex.EncodeToString((*stat.ContractID)[:]),
				))
			}
			delta.SetContractID(contractID)
		}
		delete(contractToAsset, contractID)

		if assetStatNotFound {
			// Safety checks
			if delta.Accounts.Authorized < 0 {
				return ingest.NewStateError(errors.Errorf(
					"Authorized accounts negative but DB entry does not exist for asset: %s %s %s",
					delta.AssetType,
					delta.AssetCode,
					delta.AssetIssuer,
				))
			} else if delta.Accounts.AuthorizedToMaintainLiabilities < 0 {
				return ingest.NewStateError(errors.Errorf(
					"AuthorizedToMaintainLiabilities accounts negative but DB entry does not exist for asset: %s %s %s",
					delta.AssetType,
					delta.AssetCode,
					delta.AssetIssuer,
				))
			} else if delta.Accounts.Unauthorized < 0 {
				return ingest.NewStateError(errors.Errorf(
					"Unauthorized accounts negative but DB entry does not exist for asset: %s %s %s",
					delta.AssetType,
					delta.AssetCode,
					delta.AssetIssuer,
				))
			} else if delta.Accounts.ClaimableBalances < 0 {
				return ingest.NewStateError(errors.Errorf(
					"Claimable balance negative but DB entry does not exist for asset: %s %s %s",
					delta.AssetType,
					delta.AssetCode,
					delta.AssetIssuer,
				))
			} else if delta.Accounts.LiquidityPools < 0 {
				return ingest.NewStateError(errors.Errorf(
					"Liquidity pools negative but DB entry does not exist for asset: %s %s %s",
					delta.AssetType,
					delta.AssetCode,
					delta.AssetIssuer,
				))
			}

			// Insert
			var errInsert error
			rowsAffected, errInsert = p.assetStatsQ.InsertAssetStat(ctx, delta)
			if errInsert != nil {
				return errors.Wrap(errInsert, "could not insert asset stat")
			}
		} else {
			var statBalances assetStatBalances
			if err = statBalances.Parse(stat.Balances); err != nil {
				return errors.Wrap(err, "Error parsing balances")
			}

			var deltaBalances assetStatBalances
			if err = deltaBalances.Parse(delta.Balances); err != nil {
				return errors.Wrap(err, "Error parsing balances")
			}

			statBalances = statBalances.Add(deltaBalances)
			statAccounts := stat.Accounts.Add(delta.Accounts)

			// only remove asset stat if the Metadata contract data ledger entry for the token contract
			// has also been removed.
			if statAccounts.IsZero() && delta.ContractID == nil {
				// Remove stats
				if !statBalances.IsZero() {
					return ingest.NewStateError(errors.Errorf(
						"Removing asset stat by final amount non-zero for: %s %s %s",
						delta.AssetType,
						delta.AssetCode,
						delta.AssetIssuer,
					))
				}
				rowsAffected, err = p.assetStatsQ.RemoveAssetStat(ctx,
					delta.AssetType,
					delta.AssetCode,
					delta.AssetIssuer,
				)
				if err != nil {
					return errors.Wrap(err, "could not remove asset stat")
				}
			} else {
				// Update
				rowsAffected, err = p.assetStatsQ.UpdateAssetStat(ctx, history.ExpAssetStat{
					AssetType:   delta.AssetType,
					AssetCode:   delta.AssetCode,
					AssetIssuer: delta.AssetIssuer,
					Accounts:    statAccounts,
					Balances:    statBalances.ConvertToHistoryObject(),
					ContractID:  delta.ContractID,
				})
				if err != nil {
					return errors.Wrap(err, "could not update asset stat")
				}
			}
		}

		if rowsAffected != 1 {
			return ingest.NewStateError(errors.Errorf(
				"%d rows affected (expected exactly 1) when adjusting asset stat for asset: %s %s %s",
				rowsAffected,
				delta.AssetType,
				delta.AssetCode,
				delta.AssetIssuer,
			))
		}
	}
	return nil
}

func (p *AssetStatsProcessor) updateContractIDs(
	ctx context.Context,
	contractToAsset map[xdr.Hash]*xdr.Asset,
) error {
	for contractID, asset := range contractToAsset {
		if err := p.updateContractID(ctx, contractID, asset); err != nil {
			return err
		}
	}
	return nil
}

// updateContractID will update the asset stat row for the corresponding asset to either
// add or remove the given contract id
func (p *AssetStatsProcessor) updateContractID(
	ctx context.Context,
	contractID xdr.Hash,
	asset *xdr.Asset,
) error {
	var rowsAffected int64
	// asset is nil so we need to set the contract_id column to NULL
	if asset == nil {
		stat, err := p.assetStatsQ.GetAssetStatByContract(ctx, contractID)
		if err == sql.ErrNoRows {
			return ingest.NewStateError(errors.Errorf(
				"row for asset with contract %s is missing",
				hex.EncodeToString(contractID[:]),
			))
		}
		if err != nil {
			return errors.Wrap(err, "error querying asset by contract id")
		}

		if stat.Accounts.IsZero() {
			if !stat.Balances.IsZero() {
				return ingest.NewStateError(errors.Errorf(
					"asset stat has 0 holders but non zero balance: %s",
					hex.EncodeToString(contractID[:]),
				))
			}
			// the asset stat is empty so we can remove the row entirely
			rowsAffected, err = p.assetStatsQ.RemoveAssetStat(ctx,
				stat.AssetType,
				stat.AssetCode,
				stat.AssetIssuer,
			)
			if err != nil {
				return errors.Wrap(err, "could not remove asset stat")
			}
		} else {
			// update the row to set the contract_id column to NULL
			stat.ContractID = nil
			rowsAffected, err = p.assetStatsQ.UpdateAssetStat(ctx, stat)
			if err != nil {
				return errors.Wrap(err, "could not update asset stat")
			}
		}
	} else { // asset is non nil, so we need to populate the contract_id column
		var assetType xdr.AssetType
		var assetCode, assetIssuer string
		asset.MustExtract(&assetType, &assetCode, &assetIssuer)
		stat, err := p.assetStatsQ.GetAssetStat(ctx, assetType, assetCode, assetIssuer)
		if err == sql.ErrNoRows {
			// there is no asset stat for the given asset so we need to create a new row
			row := history.ExpAssetStat{
				AssetType:   assetType,
				AssetCode:   assetCode,
				AssetIssuer: assetIssuer,
				Accounts:    history.ExpAssetStatAccounts{},
				Balances:    newAssetStatBalance().ConvertToHistoryObject(),
			}
			row.SetContractID(contractID)

			rowsAffected, err = p.assetStatsQ.InsertAssetStat(ctx, row)
			if err != nil {
				return errors.Wrap(err, "could not insert asset stat")
			}
		} else if err != nil {
			return errors.Wrap(err, "error querying asset by asset code and issuer")
		} else if dbContractID, ok := stat.GetContractID(); ok {
			// the asset stat already has a column_id set which is unexpected (the column should be NULL)
			return ingest.NewStateError(errors.Errorf(
				"attempting to set contract id %s but row %s already has contract id set: %s",
				hex.EncodeToString(contractID[:]),
				asset.String(),
				hex.EncodeToString(dbContractID[:]),
			))
		} else {
			// update the column_id column
			stat.SetContractID(contractID)
			rowsAffected, err = p.assetStatsQ.UpdateAssetStat(ctx, stat)
			if err != nil {
				return errors.Wrap(err, "could not update asset stat")
			}
		}
	}

	if rowsAffected != 1 {
		// assert that we have updated exactly one row
		return ingest.NewStateError(errors.Errorf(
			"%d rows affected (expected exactly 1) when adjusting asset stat for asset: %s",
			rowsAffected,
			asset.String(),
		))
	}
	return nil
}

func (p *AssetStatsProcessor) addContractAssetStat(contractAssetStat assetContractStatValue, row *history.ContractAssetStatRow) error {
	row.Stat.ActiveHolders += contractAssetStat.activeHolders
	row.Stat.ArchivedHolders += contractAssetStat.archivedHolders
	activeBalance, ok := new(big.Int).SetString(row.Stat.ActiveBalance, 10)
	if !ok {
		return errors.New("Error parsing: " + row.Stat.ActiveBalance)
	}
	row.Stat.ActiveBalance = activeBalance.Add(activeBalance, contractAssetStat.activeBalance).String()
	archivedBalance, ok := new(big.Int).SetString(row.Stat.ArchivedBalance, 10)
	if !ok {
		return errors.New("Error parsing: " + row.Stat.ArchivedBalance)
	}
	row.Stat.ArchivedBalance = archivedBalance.Add(archivedBalance, contractAssetStat.archivedBalance).String()
	return nil
}

func (p *AssetStatsProcessor) updateContractAssetStats(
	ctx context.Context,
	contractAssetStats map[xdr.Hash]assetContractStatValue,
) error {
	for contractID, contractAssetStat := range contractAssetStats {
		if err := p.updateAssetContractStats(ctx, contractID, contractAssetStat); err != nil {
			return err
		}
	}
	return nil
}

// updateAssetContractStats will look up an asset contract stat by contract id and
// it will adjust the contract balance and holders based on assetContractStat
func (p *AssetStatsProcessor) updateAssetContractStats(
	ctx context.Context,
	contractID xdr.Hash,
	assetContractStat assetContractStatValue,
) error {
	var rowsAffected int64
	row, err := p.assetStatsQ.GetContractAssetStat(ctx, contractID[:])
	if err == sql.ErrNoRows {
		rowsAffected, err = p.assetStatsQ.InsertContractAssetStat(ctx, assetContractStat.ConvertToHistoryObject())
		if err != nil {
			return errors.Wrap(err, "error inserting asset contract stat")
		}
	} else if err != nil {
		return errors.Wrap(err, "error querying asset by contract id")
	} else {
		if err = p.addContractAssetStat(assetContractStat, &row); err != nil {
			return errors.Wrapf(err, "could not update asset stat with contract id %v with contract delta", contractID)
		}

		if row.Stat == (history.ContractStat{
			ActiveBalance:   "0",
			ActiveHolders:   0,
			ArchivedBalance: "0",
			ArchivedHolders: 0,
		}) {
			rowsAffected, err = p.assetStatsQ.RemoveAssetContractStat(ctx, contractID[:])
		} else {
			rowsAffected, err = p.assetStatsQ.UpdateContractAssetStat(ctx, row)
		}

		if err != nil {
			return errors.Wrap(err, "could not update asset stat")
		}
	}

	if rowsAffected != 1 {
		// assert that we have updated exactly one row
		return ingest.NewStateError(errors.Errorf(
			"%d rows affected (expected exactly 1) when adjusting asset contract stat for contract: %s",
			rowsAffected,
			contractID,
		))
	}
	return nil
}

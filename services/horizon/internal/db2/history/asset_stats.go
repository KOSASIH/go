package history

import (
	"context"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"

	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

func assetStatToMap(assetStat ExpAssetStat) map[string]interface{} {
	return map[string]interface{}{
		"asset_type":   assetStat.AssetType,
		"asset_code":   assetStat.AssetCode,
		"asset_issuer": assetStat.AssetIssuer,
		"accounts":     assetStat.Accounts,
		"balances":     assetStat.Balances,
		"contract_id":  assetStat.ContractID,
	}
}

func assetStatToPrimaryKeyMap(assetStat ExpAssetStat) map[string]interface{} {
	return map[string]interface{}{
		"asset_type":   assetStat.AssetType,
		"asset_code":   assetStat.AssetCode,
		"asset_issuer": assetStat.AssetIssuer,
	}
}

// ContractAssetStatRow represents a row in the contract_asset_stats table
type ContractAssetStatRow struct {
	// ContractID is the contract id of the stellar asset contract
	ContractID []byte `db:"contract_id"`
	// Stat is a json blob containing statistics on the contract holders
	// this asset
	Stat ContractStat `db:"stat"`
}

// InsertAssetStats a set of asset stats into the exp_asset_stats
func (q *Q) InsertAssetStats(ctx context.Context, assetStats []ExpAssetStat) error {
	if len(assetStats) == 0 {
		return nil
	}

	builder := &db.FastBatchInsertBuilder{}

	for _, assetStat := range assetStats {
		if err := builder.Row(assetStatToMap(assetStat)); err != nil {
			return errors.Wrap(err, "could not insert asset assetStat row")
		}
	}

	if err := builder.Exec(ctx, q, "exp_asset_stats"); err != nil {
		return errors.Wrap(err, "could not exec asset assetStats insert builder")
	}

	return nil
}

// InsertContractAssetStats inserts the given list of rows into the contract_asset_stats table
func (q *Q) InsertContractAssetStats(ctx context.Context, rows []ContractAssetStatRow) error {
	if len(rows) == 0 {
		return nil
	}
	builder := &db.FastBatchInsertBuilder{}

	for _, row := range rows {
		if err := builder.RowStruct(row); err != nil {
			return errors.Wrap(err, "could not insert asset assetStat row")
		}
	}

	if err := builder.Exec(ctx, q, "contract_asset_stats"); err != nil {
		return errors.Wrap(err, "could not exec asset assetStats insert builder")
	}

	return nil
}

// ContractAssetBalance represents a row in the contract_asset_balances table
type ContractAssetBalance struct {
	// KeyHash is a hash of the contract balance's ledger entry key
	KeyHash []byte `db:"key_hash"`
	// ContractID is the contract id of the stellar asset contract
	ContractID []byte `db:"asset_contract_id"`
	// Amount is the amount held by the contract
	Amount string `db:"amount"`
	// ExpirationLedger is the latest ledger for which this contract balance
	// ledger entry is active
	ExpirationLedger uint32 `db:"expiration_ledger"`
}

// InsertContractAssetBalances will insert the given list of rows into the contract_asset_balances table
func (q *Q) InsertContractAssetBalances(ctx context.Context, rows []ContractAssetBalance) error {
	if len(rows) == 0 {
		return nil
	}
	builder := &db.FastBatchInsertBuilder{}

	for _, row := range rows {
		if err := builder.RowStruct(row); err != nil {
			return errors.Wrap(err, "could not insert asset assetStat row")
		}
	}

	if err := builder.Exec(ctx, q, "contract_asset_balances"); err != nil {
		return errors.Wrap(err, "could not exec asset assetStats insert builder")
	}

	return nil
}

const maxUpdateBatchSize = 30000

// UpdateContractAssetBalanceAmounts will update the expiration ledgers for the given list of keys
// (if they exist in the db).
func (q *Q) UpdateContractAssetBalanceAmounts(ctx context.Context, keys []xdr.Hash, amounts []string) error {
	for len(keys) > 0 {
		var args []interface{}
		var values []string

		for i := 0; len(keys) > 0 && i < maxUpdateBatchSize; i++ {
			args = append(args, keys[0][:], amounts[0])
			values = append(values, "(cast(? as bytea), cast(? as numeric))")
			keys = keys[1:]
			amounts = amounts[1:]
		}

		sql := fmt.Sprintf(`
			UPDATE contract_asset_balances
			SET
			  amount = myvalues.amount
			FROM (
			  VALUES
				%s
			) AS myvalues (key_hash, amount)
			WHERE contract_asset_balances.key_hash = myvalues.key_hash`,
			strings.Join(values, ","),
		)

		_, err := q.ExecRaw(ctx, sql, args...)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateContractAssetBalanceExpirations will update the expiration ledgers for the given list of keys
// (if they exist in the db).
func (q *Q) UpdateContractAssetBalanceExpirations(ctx context.Context, keys []xdr.Hash, expirationLedgers []uint32) error {
	for len(keys) > 0 {
		var args []interface{}
		var values []string

		for i := 0; len(keys) > 0 && i < maxUpdateBatchSize; i++ {
			args = append(args, keys[0][:], expirationLedgers[0])
			values = append(values, "(cast(? as bytea), cast(? as integer))")
			keys = keys[1:]
			expirationLedgers = expirationLedgers[1:]
		}

		sql := fmt.Sprintf(`
			UPDATE contract_asset_balances
			SET
			  expiration_ledger = myvalues.expiration
			FROM (
			  VALUES
				%s
			) AS myvalues (key_hash, expiration)
			WHERE contract_asset_balances.key_hash = myvalues.key_hash`,
			strings.Join(values, ","),
		)

		_, err := q.ExecRaw(ctx, sql, args...)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetContractAssetBalancesExpiringAt returns all contract asset balances which are active
// at `ledger` and expired at `ledger+1`
func (q *Q) GetContractAssetBalancesExpiringAt(ctx context.Context, ledger uint32) ([]ContractAssetBalance, error) {
	sql := sq.Select("contract_asset_balances.*").From("contract_asset_balances").
		Where(map[string]interface{}{"expiration_ledger": ledger})
	var balances []ContractAssetBalance
	err := q.Select(ctx, &balances, sql)
	return balances, err
}

// GetContractAssetBalances fetches all contract_asset_balances rows for the
// given list of key hashes.
func (q *Q) GetContractAssetBalances(ctx context.Context, keys []xdr.Hash) ([]ContractAssetBalance, error) {
	keyBytes := make([][]byte, len(keys))
	for i := range keys {
		keyBytes[i] = keys[i][:]
	}
	sql := sq.Select("contract_asset_balances.*").From("contract_asset_balances").
		Where(map[string]interface{}{"key_hash": keyBytes})
	var balances []ContractAssetBalance
	err := q.Select(ctx, &balances, sql)
	return balances, err
}

// RemoveContractAssetBalances removes rows from the contract_asset_balances table
func (q *Q) RemoveContractAssetBalances(ctx context.Context, keys []xdr.Hash) error {
	if len(keys) == 0 {
		return nil
	}
	keyBytes := make([][]byte, len(keys))
	for i := range keys {
		keyBytes[i] = keys[i][:]
	}

	_, err := q.Exec(ctx, sq.Delete("contract_asset_balances").
		Where(map[string]interface{}{
			"key_hash": keyBytes,
		}))
	return err
}

// InsertAssetStat a single asset assetStat row into the exp_asset_stats
// Returns number of rows affected and error.
func (q *Q) InsertAssetStat(ctx context.Context, assetStat ExpAssetStat) (int64, error) {
	sql := sq.Insert("exp_asset_stats").SetMap(assetStatToMap(assetStat))
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// InsertContractAssetStat inserts a row into the contract_asset_stats table
func (q *Q) InsertContractAssetStat(ctx context.Context, row ContractAssetStatRow) (int64, error) {
	sql := sq.Insert("contract_asset_stats").SetMap(map[string]interface{}{
		"contract_id": row.ContractID,
		"stat":        row.Stat,
	})
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// UpdateAssetStat updates a row in the exp_asset_stats table.
// Returns number of rows affected and error.
func (q *Q) UpdateAssetStat(ctx context.Context, assetStat ExpAssetStat) (int64, error) {
	sql := sq.Update("exp_asset_stats").
		SetMap(assetStatToMap(assetStat)).
		Where(assetStatToPrimaryKeyMap(assetStat))
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// UpdateContractAssetStat updates a row in the contract_asset_stats table.
// Returns number of rows afected and error.
func (q *Q) UpdateContractAssetStat(ctx context.Context, row ContractAssetStatRow) (int64, error) {
	sql := sq.Update("contract_asset_stats").Set("stat", row.Stat).
		Where("contract_id = ?", row.ContractID)
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// RemoveAssetStat removes a row in the exp_asset_stats table.
func (q *Q) RemoveAssetStat(ctx context.Context, assetType xdr.AssetType, assetCode, assetIssuer string) (int64, error) {
	sql := sq.Delete("exp_asset_stats").
		Where(map[string]interface{}{
			"asset_type":   assetType,
			"asset_code":   assetCode,
			"asset_issuer": assetIssuer,
		})
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// RemoveAssetContractStat removes a row in the contract_asset_stats table.
func (q *Q) RemoveAssetContractStat(ctx context.Context, contractID []byte) (int64, error) {
	sql := sq.Delete("contract_asset_stats").
		Where("contract_id = ?", contractID)
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetAssetStat returns a row in the exp_asset_stats table.
func (q *Q) GetAssetStat(ctx context.Context, assetType xdr.AssetType, assetCode, assetIssuer string) (ExpAssetStat, error) {
	sql := selectAssetStats.Where(map[string]interface{}{
		"asset_type":   assetType,
		"asset_code":   assetCode,
		"asset_issuer": assetIssuer,
	})
	var assetStat ExpAssetStat
	err := q.Get(ctx, &assetStat, sql)
	return assetStat, err
}

// GetContractAssetStat returns a row in the contract_asset_stats table.
func (q *Q) GetContractAssetStat(ctx context.Context, contractID []byte) (ContractAssetStatRow, error) {
	sql := sq.Select("*").From("contract_asset_stats").Where("contract_id = ?", contractID)
	var assetStat ContractAssetStatRow
	err := q.Get(ctx, &assetStat, sql)
	return assetStat, err
}

// GetAssetStatByContract returns the row in the exp_asset_stats table corresponding
// to the given contract id
func (q *Q) GetAssetStatByContract(ctx context.Context, contractID xdr.Hash) (ExpAssetStat, error) {
	sql := selectAssetStats.Where("contract_id = ?", contractID[:])
	var assetStat ExpAssetStat
	err := q.Get(ctx, &assetStat, sql)
	return assetStat, err
}

func parseAssetStatsCursor(cursor string) (string, string, error) {
	parts := strings.SplitN(cursor, "_", 3)
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid asset stats cursor: %v", cursor)
	}

	code, issuer, assetType := parts[0], parts[1], parts[2]
	var issuerAccount xdr.AccountId
	var asset xdr.Asset

	if err := issuerAccount.SetAddress(issuer); err != nil {
		return "", "", errors.Wrap(
			err,
			fmt.Sprintf("invalid issuer in asset stats cursor: %v", cursor),
		)
	}

	if err := asset.SetCredit(code, issuerAccount); err != nil {
		return "", "", errors.Wrap(
			err,
			fmt.Sprintf("invalid asset stats cursor: %v", cursor),
		)
	}

	if _, ok := xdr.StringToAssetType[assetType]; !ok {
		return "", "", errors.Errorf("invalid asset type in asset stats cursor: %v", cursor)
	}

	return code, issuer, nil
}

// GetAssetStats returns a page of exp_asset_stats rows.
func (q *Q) GetAssetStats(ctx context.Context, assetCode, assetIssuer string, page db2.PageQuery) ([]AssetAndContractStat, error) {
	sql := sq.Select("exp_asset_stats.*, contract_asset_stats.stat as contracts").
		From("exp_asset_stats").
		LeftJoin("contract_asset_stats ON exp_asset_stats.contract_id = contract_asset_stats.contract_id")
	filters := map[string]interface{}{}
	if assetCode != "" {
		filters["asset_code"] = assetCode
	}
	if assetIssuer != "" {
		filters["asset_issuer"] = assetIssuer
	}

	if len(filters) > 0 {
		sql = sql.Where(filters)
	}

	var cursorComparison, orderBy string
	switch page.Order {
	case "asc":
		cursorComparison, orderBy = ">", "asc"
	case "desc":
		cursorComparison, orderBy = "<", "desc"
	default:
		return nil, fmt.Errorf("invalid page order %s", page.Order)
	}

	if page.Cursor != "" {
		cursorCode, cursorIssuer, err := parseAssetStatsCursor(page.Cursor)
		if err != nil {
			return nil, err
		}

		sql = sql.Where("((asset_code, asset_issuer) "+cursorComparison+" (?,?))", cursorCode, cursorIssuer)
	}

	sql = sql.OrderBy("(asset_code, asset_issuer) " + orderBy).Limit(page.Limit)

	var results []AssetAndContractStat
	if err := q.Select(ctx, &results, sql); err != nil {
		return nil, errors.Wrap(err, "could not run select query")
	}

	return results, nil
}

var selectAssetStats = sq.Select("exp_asset_stats.*").From("exp_asset_stats")

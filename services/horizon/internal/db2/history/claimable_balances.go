package history

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/guregu/null"

	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

// ClaimableBalancesQuery is a helper struct to configure queries to claimable balances
type ClaimableBalancesQuery struct {
	PageQuery db2.PageQuery
	Asset     *xdr.Asset
	Sponsor   *xdr.AccountId
	Claimant  *xdr.AccountId
}

// Cursor validates and returns the query page cursor
func (cbq ClaimableBalancesQuery) Cursor() (int64, string, error) {
	p := cbq.PageQuery
	var l int64
	var r string
	var err error

	if p.Cursor != "" {
		parts := strings.SplitN(p.Cursor, "-", 2)
		if len(parts) != 2 {
			return l, r, errors.New("Invalid cursor")
		}

		l, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return l, r, errors.Wrap(err, "Invalid cursor - first value should be higher than 0")
		}

		var balanceID xdr.ClaimableBalanceId
		if err = xdr.SafeUnmarshalHex(parts[1], &balanceID); err != nil {
			return l, r, errors.Wrap(err, "Invalid cursor - second value should be a valid claimable balance id")
		}
		r = parts[1]
		if l < 0 {
			return l, r, errors.New("invalid cursor - first value should be higher than 0")
		}
	}

	return l, r, nil
}

// ApplyCursor applies cursor to the given sql. For performance reason the limit
// is not applied here. This allows us to hint the planner later to use the right
// indexes.
func applyClaimableBalancesQueriesCursor(sql sq.SelectBuilder, tableName string, lCursor int64, rCursor string, order string) (sq.SelectBuilder, error) {
	hasPagedLimit := false
	if lCursor > 0 && rCursor != "" {
		hasPagedLimit = true
	}

	switch order {
	case db2.OrderAscending:
		if hasPagedLimit {
			sql = sql.
				Where(
					sq.Expr(
						fmt.Sprintf("(%s.last_modified_ledger, %s.id) > (?, ?)", tableName, tableName),
						lCursor, rCursor,
					),
				)
		}
		sql = sql.OrderBy(fmt.Sprintf("%s.last_modified_ledger asc, %s.id asc", tableName, tableName))
	case db2.OrderDescending:
		if hasPagedLimit {
			sql = sql.
				Where(
					sq.Expr(
						fmt.Sprintf("(%s.last_modified_ledger, %s.id) < (?, ?)", tableName, tableName),
						lCursor,
						rCursor,
					),
				)
		}
		sql = sql.OrderBy(fmt.Sprintf("%s.last_modified_ledger desc, %s.id desc", tableName, tableName))
	default:
		return sql, errors.Errorf("invalid order: %s", order)
	}

	return sql, nil
}

// ClaimableBalanceClaimant is a row of data from the `claimable_balances_claimants` table.
// This table exists to allow faster querying for claimable balances for a specific claimant.
type ClaimableBalanceClaimant struct {
	BalanceID          string `db:"id"`
	Destination        string `db:"destination"`
	LastModifiedLedger uint32 `db:"last_modified_ledger"`
}

// ClaimableBalance is a row of data from the `claimable_balances` table.
type ClaimableBalance struct {
	BalanceID          string      `db:"id"`
	Claimants          Claimants   `db:"claimants"`
	Asset              xdr.Asset   `db:"asset"`
	Amount             xdr.Int64   `db:"amount"`
	Sponsor            null.String `db:"sponsor"`
	LastModifiedLedger uint32      `db:"last_modified_ledger"`
	Flags              uint32      `db:"flags"`
}

type Claimants []Claimant

func (c Claimants) Value() (driver.Value, error) {
	// Convert the byte array into a string as a workaround to bypass buggy encoding in the pq driver
	// (More info about this bug here https://github.com/stellar/go/issues/5086#issuecomment-1773215436).
	// By doing so, the data will be written as a string rather than hex encoded bytes.
	val, err := json.Marshal(c)
	return string(val), err
}

func (c *Claimants) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &c)
}

type Claimant struct {
	Destination string             `json:"destination"`
	Predicate   xdr.ClaimPredicate `json:"predicate"`
}

// QClaimableBalances defines claimable-balance-related related queries.
type QClaimableBalances interface {
	UpsertClaimableBalances(ctx context.Context, cb []ClaimableBalance) error
	RemoveClaimableBalances(ctx context.Context, ids []string) (int64, error)
	RemoveClaimableBalanceClaimants(ctx context.Context, ids []string) (int64, error)
	GetClaimableBalancesByID(ctx context.Context, ids []string) ([]ClaimableBalance, error)
	CountClaimableBalances(ctx context.Context) (int, error)
	NewClaimableBalanceClaimantBatchInsertBuilder() ClaimableBalanceClaimantBatchInsertBuilder
	NewClaimableBalanceBatchInsertBuilder() ClaimableBalanceBatchInsertBuilder
	GetClaimantsByClaimableBalances(ctx context.Context, ids []string) (map[string][]ClaimableBalanceClaimant, error)
}

// CountClaimableBalances returns the total number of claimable balances in the DB
func (q *Q) CountClaimableBalances(ctx context.Context) (int, error) {
	sql := sq.Select("count(*)").From("claimable_balances")

	var count int
	if err := q.Get(ctx, &count, sql); err != nil {
		return 0, errors.Wrap(err, "could not run select query")
	}

	return count, nil
}

// GetClaimableBalancesByID finds all claimable balances by ClaimableBalanceId
func (q *Q) GetClaimableBalancesByID(ctx context.Context, ids []string) ([]ClaimableBalance, error) {
	var cBalances []ClaimableBalance
	sql := selectClaimableBalances.Where(map[string]interface{}{"cb.id": ids})
	err := q.Select(ctx, &cBalances, sql)
	return cBalances, err
}

// GetClaimantsByClaimableBalances finds all claimants for ClaimableBalanceIds.
// The returned list is sorted by ids and then destination ids for each balance id.
func (q *Q) GetClaimantsByClaimableBalances(ctx context.Context, ids []string) (map[string][]ClaimableBalanceClaimant, error) {
	var claimants []ClaimableBalanceClaimant
	sql := sq.Select("*").From("claimable_balance_claimants cbc").
		Where(map[string]interface{}{"cbc.id": ids}).
		OrderBy("id asc, destination asc")
	err := q.Select(ctx, &claimants, sql)

	claimantsMap := make(map[string][]ClaimableBalanceClaimant)
	for _, claimant := range claimants {
		claimantsMap[claimant.BalanceID] = append(claimantsMap[claimant.BalanceID], claimant)
	}
	return claimantsMap, err
}

// UpsertClaimableBalances upserts a batch of claimable balances in the claimable_balances table.
// It also upserts the corresponding claimants in the claimable_balance_claimants table.
func (q *Q) UpsertClaimableBalances(ctx context.Context, cbs []ClaimableBalance) error {
	if err := q.upsertCBs(ctx, cbs); err != nil {
		return errors.Wrap(err, "could not upsert claimable balances")
	}

	if err := q.upsertCBClaimants(ctx, cbs); err != nil {
		return errors.Wrap(err, "could not upsert claimable balance claimants")
	}

	return nil
}

func (q *Q) upsertCBClaimants(ctx context.Context, cbs []ClaimableBalance) error {
	var id, lastModifiedLedger, destination []interface{}

	for _, cb := range cbs {
		for _, claimant := range cb.Claimants {
			id = append(id, cb.BalanceID)
			lastModifiedLedger = append(lastModifiedLedger, cb.LastModifiedLedger)
			destination = append(destination, claimant.Destination)
		}
	}

	upsertFields := []upsertField{
		{"id", "text", id},
		{"destination", "text", destination},
		{"last_modified_ledger", "integer", lastModifiedLedger},
	}

	return q.upsertRows(ctx, "claimable_balance_claimants", "id, destination", upsertFields)
}

func (q *Q) upsertCBs(ctx context.Context, cbs []ClaimableBalance) error {
	var id, claimants, asset, amount, sponsor, lastModifiedLedger, flags []interface{}

	for _, cb := range cbs {
		id = append(id, cb.BalanceID)
		claimants = append(claimants, cb.Claimants)
		asset = append(asset, cb.Asset)
		amount = append(amount, cb.Amount)
		sponsor = append(sponsor, cb.Sponsor)
		lastModifiedLedger = append(lastModifiedLedger, cb.LastModifiedLedger)
		flags = append(flags, cb.Flags)
	}

	upsertFields := []upsertField{
		{"id", "text", id},
		{"claimants", "jsonb", claimants},
		{"asset", "text", asset},
		{"amount", "bigint", amount},
		{"sponsor", "text", sponsor},
		{"last_modified_ledger", "integer", lastModifiedLedger},
		{"flags", "int", flags},
	}

	return q.upsertRows(ctx, "claimable_balances", "id", upsertFields)
}

// RemoveClaimableBalances deletes claimable balances table.
// Returns number of rows affected and error.
func (q *Q) RemoveClaimableBalances(ctx context.Context, ids []string) (int64, error) {
	sql := sq.Delete("claimable_balances").
		Where(sq.Eq{"id": ids})
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// RemoveClaimableBalanceClaimants deletes claimable balance claimants.
// Returns number of rows affected and error.
func (q *Q) RemoveClaimableBalanceClaimants(ctx context.Context, ids []string) (int64, error) {
	sql := sq.Delete("claimable_balance_claimants").
		Where(sq.Eq{"id": ids})
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// FindClaimableBalanceByID returns a claimable balance.
func (q *Q) FindClaimableBalanceByID(ctx context.Context, balanceID string) (ClaimableBalance, error) {
	var claimableBalance ClaimableBalance
	sql := selectClaimableBalances.Limit(1).Where("cb.id = ?", balanceID)
	err := q.Get(ctx, &claimableBalance, sql)
	return claimableBalance, err
}

// GetClaimableBalances finds all claimable balances where accountID is one of the claimants
func (q *Q) GetClaimableBalances(ctx context.Context, query ClaimableBalancesQuery) ([]ClaimableBalance, error) {
	l, r, err := query.Cursor()
	if err != nil {
		return nil, errors.Wrap(err, "error getting cursor")
	}

	sql, err := applyClaimableBalancesQueriesCursor(selectClaimableBalances, "cb", l, r, query.PageQuery.Order)
	if err != nil {
		return nil, errors.Wrap(err, "could not apply query to page")
	}

	if query.Asset != nil || query.Sponsor != nil {

		// JOIN with claimable_balance_claimants table to query by claimants
		if query.Claimant != nil {
			sql = sql.Join("claimable_balance_claimants on claimable_balance_claimants.id = cb.id")
			sql = sql.Where("claimable_balance_claimants.destination = ?", query.Claimant.Address())
		}

		// Apply filters for asset and sponsor
		if query.Asset != nil {
			sql = sql.Where("cb.asset = ?", query.Asset)
		}
		if query.Sponsor != nil {
			sql = sql.Where("cb.sponsor = ?", query.Sponsor.Address())
		}

	} else if query.Claimant != nil {
		// If only the claimant is provided without additional filters, a JOIN with claimable_balance_claimants
		// does not perform efficiently. Instead, use a subquery (with LIMIT) to retrieve claimable balances based on
		// the claimant's address.

		var selectClaimableBalanceClaimants = sq.Select("claimable_balance_claimants.id").From("claimable_balance_claimants").
			Where("claimable_balance_claimants.destination = ?", query.Claimant.Address()).Limit(query.PageQuery.Limit)

		subSql, err := applyClaimableBalancesQueriesCursor(selectClaimableBalanceClaimants, "claimable_balance_claimants", l, r, query.PageQuery.Order)
		if err != nil {
			return nil, errors.Wrap(err, "could not apply subquery to page")
		}

		subSqlString, subSqlArgs, err := subSql.ToSql()
		if err != nil {
			return nil, errors.Wrap(err, "could not build subquery")
		}

		sql = sql.
			Where(fmt.Sprintf("cb.id IN (%s)", subSqlString), subSqlArgs...)
	}

	sql = sql.Limit(query.PageQuery.Limit)

	var results []ClaimableBalance
	if err := q.Select(ctx, &results, sql); err != nil {
		return nil, errors.Wrap(err, "could not run select query")
	}

	return results, nil
}

var claimableBalancesSelectStatement = "cb.id, " +
	"cb.claimants, " +
	"cb.asset, " +
	"cb.amount, " +
	"cb.sponsor, " +
	"cb.last_modified_ledger, " +
	"cb.flags"

var selectClaimableBalances = sq.Select(claimableBalancesSelectStatement).From("claimable_balances cb")

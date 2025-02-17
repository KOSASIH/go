package history

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	sq "github.com/Masterminds/squirrel"

	"github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/toid"
)

const genesisLedger = 2

// UnmarshalDetails unmarshals the details of this effect into `dest`
func (r *Effect) UnmarshalDetails(dest interface{}) error {
	if !r.DetailsString.Valid {
		return nil
	}

	err := errors.Wrap(json.Unmarshal([]byte(r.DetailsString.String), &dest), "unmarshal effect details failed")
	if err == nil {
		// In 2.9.0 a new `asset_type` was introduced to include liquidity
		// pools. Instead of reingesting entire history, let's fill the
		// `asset_type` here if it's empty.
		// (I hate to convert to `protocol` types here but there's no other way
		// without larger refactor.)
		switch dest := dest.(type) {
		case *effects.TrustlineSponsorshipCreated:
			if dest.Type == "" {
				dest.Type = getAssetTypeForCanonicalAsset(dest.Asset)
			}
		case *effects.TrustlineSponsorshipUpdated:
			if dest.Type == "" {
				dest.Type = getAssetTypeForCanonicalAsset(dest.Asset)
			}
		case *effects.TrustlineSponsorshipRemoved:
			if dest.Type == "" {
				dest.Type = getAssetTypeForCanonicalAsset(dest.Asset)
			}
		}
	}
	return err
}

func getAssetTypeForCanonicalAsset(canonicalAsset string) string {
	if len(canonicalAsset) <= 61 {
		return "credit_alphanum4"
	} else {
		return "credit_alphanum12"
	}
}

// ID returns a lexically ordered id for this effect record
func (r *Effect) ID() string {
	return fmt.Sprintf("%019d-%010d", r.HistoryOperationID, r.Order)
}

// LedgerSequence return the ledger in which the effect occurred.
func (r *Effect) LedgerSequence() int32 {
	id := toid.Parse(r.HistoryOperationID)
	return id.LedgerSequence
}

// PagingToken returns a cursor for this effect
func (r *Effect) PagingToken() string {
	return fmt.Sprintf("%d-%d", r.HistoryOperationID, r.Order)
}

// Effects returns a page of effects without any filters besides the cursor
func (q *Q) Effects(ctx context.Context, page db2.PageQuery, oldestLedger int32) ([]Effect, error) {
	op, idx, err := parseEffectsCursor(page)
	if err != nil {
		return nil, err
	}

	var rows []Effect
	query := selectEffect
	// we do not use selectEffectsPage() because we have found the
	// query below to be more efficient when there are no other constraints
	// such as filtering by account / ledger / transaction / etc
	switch page.Order {
	case "asc":
		query = query.
			Where("(heff.history_operation_id, heff.order) > (?, ?)", op, idx).
			OrderBy("heff.history_operation_id asc, heff.order asc")
	case "desc":
		if lowerBound := lowestLedgerBound(oldestLedger); lowerBound > 0 {
			query = query.Where("heff.history_operation_id > ?", lowerBound)
		}
		query = query.
			Where("(heff.history_operation_id, heff.order) < (?, ?)", op, idx).
			OrderBy("heff.history_operation_id desc, heff.order desc")
	}

	query = query.Limit(page.Limit)

	if err = q.Select(ctx, &rows, query); err != nil {
		return nil, err
	}
	return rows, nil
}

// EffectsForAccount returns a page of effects for a given account
func (q *Q) EffectsForAccount(ctx context.Context, aid string, page db2.PageQuery, oldestLedger int32) ([]Effect, error) {
	var account Account
	if err := q.AccountByAddress(ctx, &account, aid); err != nil {
		return nil, err
	}

	query := selectEffect.Where("heff.history_account_id = ?", account.ID)
	return q.selectEffectsPage(ctx, query, page, oldestLedger)
}

// EffectsForLedger returns a page of effects for a given ledger sequence
func (q *Q) EffectsForLedger(ctx context.Context, seq int32, page db2.PageQuery) ([]Effect, error) {
	var ledger Ledger
	if err := q.LedgerBySequence(ctx, &ledger, seq); err != nil {
		return nil, err
	}

	start := toid.ID{LedgerSequence: seq}
	end := toid.ID{LedgerSequence: seq + 1}
	query := selectEffect.Where(
		"heff.history_operation_id >= ? AND heff.history_operation_id < ?",
		start.ToInt64(),
		end.ToInt64(),
	)
	return q.selectEffectsPage(ctx, query, page, 0)
}

// EffectsForOperation returns a page of effects for a given operation id.
func (q *Q) EffectsForOperation(ctx context.Context, id int64, page db2.PageQuery) ([]Effect, error) {
	start := toid.Parse(id)
	end := start
	end.IncOperationOrder()
	query := selectEffect.Where(
		"heff.history_operation_id >= ? AND heff.history_operation_id < ?",
		start.ToInt64(),
		end.ToInt64(),
	)
	return q.selectEffectsPage(ctx, query, page, 0)
}

// EffectsForLiquidityPool returns a page of effects for a given liquidity pool.
func (q *Q) EffectsForLiquidityPool(ctx context.Context, id string, page db2.PageQuery, oldestLedger int32) ([]Effect, error) {
	op, _, err := page.CursorInt64Pair(db2.DefaultPairSep)
	if err != nil {
		return nil, err
	}

	query := `SELECT holp.history_operation_id
	FROM history_operation_liquidity_pools holp
	WHERE holp.history_liquidity_pool_id = (SELECT id FROM history_liquidity_pools WHERE liquidity_pool_id =  ?)
	`
	switch page.Order {
	case "asc":
		query += "AND holp.history_operation_id >= ? ORDER BY holp.history_operation_id asc LIMIT ?"
	case "desc":
		query += "AND holp.history_operation_id <= ? ORDER BY holp.history_operation_id desc LIMIT ?"
	default:
		return nil, errors.Errorf("invalid paging order: %s", page.Order)
	}

	var liquidityPoolOperationIDs []int64
	err = q.SelectRaw(ctx, &liquidityPoolOperationIDs, query, id, op, page.Limit)
	if err != nil {
		return nil, err
	}

	return q.selectEffectsPage(
		ctx,
		selectEffect.Where(map[string]interface{}{
			"heff.history_operation_id": liquidityPoolOperationIDs,
		}),
		page,
		oldestLedger,
	)
}

// EffectsForTransaction returns a page of effects for a given transaction
func (q *Q) EffectsForTransaction(ctx context.Context, hash string, page db2.PageQuery) ([]Effect, error) {
	var tx Transaction
	if err := q.TransactionByHash(ctx, &tx, hash); err != nil {
		return nil, err
	}

	start := toid.Parse(tx.ID)
	end := start
	end.TransactionOrder++

	return q.selectEffectsPage(
		ctx,
		selectEffect.Where("heff.history_operation_id >= ? AND heff.history_operation_id < ?",
			start.ToInt64(),
			end.ToInt64(),
		),
		page,
		0,
	)
}

func parseEffectsCursor(page db2.PageQuery) (int64, int64, error) {
	op, idx, err := page.CursorInt64Pair(db2.DefaultPairSep)
	if err != nil {
		return 0, 0, err
	}

	if idx > math.MaxInt32 {
		idx = math.MaxInt32
	}
	return op, idx, nil
}

func lowestLedgerBound(oldestLedger int32) int64 {
	if oldestLedger <= genesisLedger {
		return 0
	}
	return toid.AfterLedger(oldestLedger - 1).ToInt64()
}

func (q *Q) selectEffectsPage(ctx context.Context, query sq.SelectBuilder, page db2.PageQuery, oldestLedger int32) ([]Effect, error) {
	op, idx, err := parseEffectsCursor(page)
	if err != nil {
		return nil, err
	}

	// NOTE: Remember to test the queries below with EXPLAIN / EXPLAIN ANALYZE
	// before changing them.
	// This condition is using multicolumn index and it's easy to write it in a way that
	// DB will perform a full table scan.
	switch page.Order {
	case "asc":
		query = query.
			Where(`(
					 heff.history_operation_id >= ?
				AND (
					 heff.history_operation_id > ? OR
					(heff.history_operation_id = ? AND heff.order > ?)
				))`, op, op, op, idx).
			OrderBy("heff.history_operation_id asc, heff.order asc")
	case "desc":
		if lowerBound := lowestLedgerBound(oldestLedger); lowerBound > 0 {
			query = query.Where("heff.history_operation_id > ?", lowerBound)
		}
		query = query.
			Where(`(
					 heff.history_operation_id <= ?
				AND (
					 heff.history_operation_id < ? OR
					(heff.history_operation_id = ? AND heff.order < ?)
				))`, op, op, op, idx).
			OrderBy("heff.history_operation_id desc, heff.order desc")
	}

	query = query.Limit(page.Limit)

	var rows []Effect
	if err = q.Select(ctx, &rows, query); err != nil {
		return nil, err
	}

	return rows, nil
}

// QEffects defines history_effects related queries.
type QEffects interface {
	QCreateAccountsHistory
	NewEffectBatchInsertBuilder() EffectBatchInsertBuilder
}

var selectEffect = sq.Select("heff.*, hacc.address").
	From("history_effects heff").
	LeftJoin("history_accounts hacc ON hacc.id = heff.history_account_id")

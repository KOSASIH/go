package history

import (
	"context"
	"time"

	"github.com/guregu/null"

	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
)

// TradeType is an enum which indicates the type of trade
type TradeType int16

const (
	// OrderbookTradeType is a trade which exercises an offer on the orderbook.
	OrderbookTradeType = TradeType(1)
	// LiquidityPoolTradeType is a trade which exercises a liquidity pool.
	LiquidityPoolTradeType = TradeType(2)
)

// InsertTrade represents the arguments to TradeBatchInsertBuilder.Add() which is used to insert
// rows into the history_trades table
type InsertTrade struct {
	HistoryOperationID int64     `db:"history_operation_id"`
	Order              int32     `db:"order"`
	LedgerCloseTime    time.Time `db:"ledger_closed_at"`

	CounterAssetID         int64    `db:"counter_asset_id"`
	CounterAmount          int64    `db:"counter_amount"`
	CounterAccountID       null.Int `db:"counter_account_id"`
	CounterOfferID         null.Int `db:"counter_offer_id"`
	CounterLiquidityPoolID null.Int `db:"counter_liquidity_pool_id"`

	LiquidityPoolFee null.Int `db:"liquidity_pool_fee"`

	BaseAssetID         int64    `db:"base_asset_id"`
	BaseAmount          int64    `db:"base_amount"`
	BaseAccountID       null.Int `db:"base_account_id"`
	BaseOfferID         null.Int `db:"base_offer_id"`
	BaseLiquidityPoolID null.Int `db:"base_liquidity_pool_id"`

	BaseIsSeller bool      `db:"base_is_seller"`
	BaseIsExact  null.Bool `db:"base_is_exact"`

	Type TradeType `db:"trade_type"`

	PriceN int64 `db:"price_n"`
	PriceD int64 `db:"price_d"`

	RoundingSlippage null.Int `db:"rounding_slippage"`
}

// TradeBatchInsertBuilder is used to insert trades into the
// history_trades table
type TradeBatchInsertBuilder interface {
	Add(entries ...InsertTrade) error
	Exec(ctx context.Context, session db.SessionInterface) error
}

// tradeBatchInsertBuilder is a simple wrapper around db.BatchInsertBuilder
type tradeBatchInsertBuilder struct {
	builder db.FastBatchInsertBuilder
	table   string
}

// NewTradeBatchInsertBuilder constructs a new TradeBatchInsertBuilder instance
func (q *Q) NewTradeBatchInsertBuilder() TradeBatchInsertBuilder {
	return &tradeBatchInsertBuilder{
		table:   "history_trades",
		builder: db.FastBatchInsertBuilder{},
	}
}

// Exec flushes all outstanding trades to the database
func (i *tradeBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	return i.builder.Exec(ctx, session, i.table)
}

// Add adds a new trade to the batch
func (i *tradeBatchInsertBuilder) Add(entries ...InsertTrade) error {
	for _, entry := range entries {
		err := i.builder.RowStruct(entry)
		if err != nil {
			return errors.Wrap(err, "failed to add trade")
		}
	}

	return nil
}

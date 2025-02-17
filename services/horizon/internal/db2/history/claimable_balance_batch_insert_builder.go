package history

import (
	"context"

	"github.com/stellar/go/support/db"
)

// ClaimableBalanceBatchInsertBuilder is used to insert claimable balance into the
// claimable_balances table
type ClaimableBalanceBatchInsertBuilder interface {
	Add(claimableBalance ClaimableBalance) error
	Exec(ctx context.Context) error
	Len() int
}

// ClaimableBalanceBatchInsertBuilder is a simple wrapper around db.FastBatchInsertBuilder
type claimableBalanceBatchInsertBuilder struct {
	session db.SessionInterface
	builder db.FastBatchInsertBuilder
	table   string
}

// NewClaimableBalanceBatchInsertBuilder constructs a new ClaimableBalanceBatchInsertBuilder instance
func (q *Q) NewClaimableBalanceBatchInsertBuilder() ClaimableBalanceBatchInsertBuilder {
	return &claimableBalanceBatchInsertBuilder{
		session: q,
		builder: db.FastBatchInsertBuilder{},
		table:   "claimable_balances",
	}
}

// Add adds a new claimable balance to the batch
func (i *claimableBalanceBatchInsertBuilder) Add(claimableBalance ClaimableBalance) error {
	return i.builder.RowStruct(claimableBalance)
}

// Exec writes the batch of claimable balances to the database.
func (i *claimableBalanceBatchInsertBuilder) Exec(ctx context.Context) error {
	return i.builder.Exec(ctx, i.session, i.table)
}

// Len returns the number of items in the batch.
func (i *claimableBalanceBatchInsertBuilder) Len() int {
	return i.builder.Len()
}

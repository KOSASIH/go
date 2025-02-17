package history

import (
	"context"

	"github.com/stellar/go/support/db"
)

// OffersBatchInsertBuilder is used to insert offers into the offers table
type OffersBatchInsertBuilder interface {
	Add(offer Offer) error
	Exec(ctx context.Context) error
	Len() int
}

// OffersBatchInsertBuilder is a simple wrapper around db.FastBatchInsertBuilder
type offersBatchInsertBuilder struct {
	session db.SessionInterface
	builder db.FastBatchInsertBuilder
	table   string
}

// NewOffersBatchInsertBuilder constructs a new OffersBatchInsertBuilder instance
func (q *Q) NewOffersBatchInsertBuilder() OffersBatchInsertBuilder {
	return &offersBatchInsertBuilder{
		session: q,
		builder: db.FastBatchInsertBuilder{},
		table:   "offers",
	}
}

// Add adds a new offer to the batch
func (i *offersBatchInsertBuilder) Add(offer Offer) error {
	return i.builder.RowStruct(offer)
}

// Exec writes the batch of offers to the database.
func (i *offersBatchInsertBuilder) Exec(ctx context.Context) error {
	return i.builder.Exec(ctx, i.session, i.table)
}

// Len returns the number of items in the batch.
func (i *offersBatchInsertBuilder) Len() int {
	return i.builder.Len()
}

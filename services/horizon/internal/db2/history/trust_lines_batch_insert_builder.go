package history

import (
	"context"

	"github.com/stellar/go/support/db"
)

// TrustLinesBatchInsertBuilder is used to insert trustlines into the trust_lines table
type TrustLinesBatchInsertBuilder interface {
	Add(line TrustLine) error
	Exec(ctx context.Context) error
	Len() int
}

// trustLinesBatchInsertBuilder is a simple wrapper around db.FastBatchInsertBuilder
type trustLinesBatchInsertBuilder struct {
	session db.SessionInterface
	builder db.FastBatchInsertBuilder
	table   string
}

// NewTrustLinesBatchInsertBuilder constructs a new TrustLinesBatchInsertBuilder instance
func (q *Q) NewTrustLinesBatchInsertBuilder() TrustLinesBatchInsertBuilder {
	return &trustLinesBatchInsertBuilder{
		session: q,
		builder: db.FastBatchInsertBuilder{},
		table:   "trust_lines",
	}
}

// Add adds a new trustline to the batch
func (i *trustLinesBatchInsertBuilder) Add(line TrustLine) error {
	return i.builder.RowStruct(line)
}

// Exec writes the batch of trust lines to the database.
func (i *trustLinesBatchInsertBuilder) Exec(ctx context.Context) error {
	return i.builder.Exec(ctx, i.session, i.table)
}

// Len returns the number of items in the batch.
func (i *trustLinesBatchInsertBuilder) Len() int {
	return i.builder.Len()
}

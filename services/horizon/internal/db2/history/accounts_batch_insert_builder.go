package history

import (
	"context"

	"github.com/stellar/go/support/db"
)

// AccountsBatchInsertBuilder is used to insert accounts into the accounts table
type AccountsBatchInsertBuilder interface {
	Add(account AccountEntry) error
	Exec(ctx context.Context) error
	Len() int
}

// AccountsBatchInsertBuilder is a simple wrapper around db.FastBatchInsertBuilder
type accountsBatchInsertBuilder struct {
	session db.SessionInterface
	builder db.FastBatchInsertBuilder
	table   string
}

// NewAccountsBatchInsertBuilder constructs a new AccountsBatchInsertBuilder instance
func (q *Q) NewAccountsBatchInsertBuilder() AccountsBatchInsertBuilder {
	return &accountsBatchInsertBuilder{
		session: q,
		builder: db.FastBatchInsertBuilder{},
		table:   "accounts",
	}
}

// Add adds a new account to the batch
func (i *accountsBatchInsertBuilder) Add(account AccountEntry) error {
	return i.builder.RowStruct(account)
}

// Exec writes the batch of accounts to the database.
func (i *accountsBatchInsertBuilder) Exec(ctx context.Context) error {
	return i.builder.Exec(ctx, i.session, i.table)
}

// Len returns the number of elements in the batch
func (i *accountsBatchInsertBuilder) Len() int {
	return i.builder.Len()
}

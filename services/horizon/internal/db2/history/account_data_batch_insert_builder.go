package history

import (
	"context"

	"github.com/stellar/go/support/db"
)

type AccountDataBatchInsertBuilder interface {
	Add(data Data) error
	Exec(ctx context.Context) error
	Len() int
}

type accountDataBatchInsertBuilder struct {
	session db.SessionInterface
	builder db.FastBatchInsertBuilder
	table   string
}

func (q *Q) NewAccountDataBatchInsertBuilder() AccountDataBatchInsertBuilder {
	return &accountDataBatchInsertBuilder{
		session: q,
		builder: db.FastBatchInsertBuilder{},
		table:   "accounts_data",
	}
}

// Add adds a new account data to the batch
func (i *accountDataBatchInsertBuilder) Add(data Data) error {
	ledgerKey, err := accountDataKeyToString(AccountDataKey{
		AccountID: data.AccountID,
		DataName:  data.Name,
	})
	if err != nil {
		return err
	}
	return i.builder.Row(map[string]interface{}{
		"ledger_key":           ledgerKey,
		"account_id":           data.AccountID,
		"name":                 data.Name,
		"value":                data.Value,
		"last_modified_ledger": data.LastModifiedLedger,
		"sponsor":              data.Sponsor,
	})
}

// Exec writes the batch of account data to the database.
func (i *accountDataBatchInsertBuilder) Exec(ctx context.Context) error {
	return i.builder.Exec(ctx, i.session, i.table)
}

// Len returns the number of elements in the batch
func (i *accountDataBatchInsertBuilder) Len() int {
	return i.builder.Len()
}

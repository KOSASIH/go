package history

import (
	"context"

	"github.com/guregu/null"

	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
)

// OperationBatchInsertBuilder is used to insert a transaction's operations into the
// history_operations table
type OperationBatchInsertBuilder interface {
	Add(
		id int64,
		transactionID int64,
		applicationOrder uint32,
		operationType xdr.OperationType,
		details []byte,
		sourceAccount string,
		sourceAcccountMuxed null.String,
		isPayment bool,
	) error
	Exec(ctx context.Context, session db.SessionInterface) error
}

// operationBatchInsertBuilder is a simple wrapper around db.BatchInsertBuilder
type operationBatchInsertBuilder struct {
	builder db.FastBatchInsertBuilder
	table   string
}

// NewOperationBatchInsertBuilder constructs a new TransactionBatchInsertBuilder instance
func (q *Q) NewOperationBatchInsertBuilder() OperationBatchInsertBuilder {
	return &operationBatchInsertBuilder{
		table:   "history_operations",
		builder: db.FastBatchInsertBuilder{},
	}
}

// Add adds a transaction's operations to the batch
func (i *operationBatchInsertBuilder) Add(
	id int64,
	transactionID int64,
	applicationOrder uint32,
	operationType xdr.OperationType,
	details []byte,
	sourceAccount string,
	sourceAccountMuxed null.String,
	isPayment bool,
) error {
	row := map[string]interface{}{
		"id":                   id,
		"transaction_id":       transactionID,
		"application_order":    applicationOrder,
		"type":                 operationType,
		"details":              string(details),
		"source_account":       sourceAccount,
		"source_account_muxed": sourceAccountMuxed,
		"is_payment":           nil,
	}
	if isPayment {
		row["is_payment"] = true
	}
	return i.builder.Row(row)
}

func (i *operationBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	return i.builder.Exec(ctx, session, i.table)
}

package history

import (
	"context"

	"github.com/stellar/go/support/db"
)

// QParticipants defines ingestion participant related queries.
type QParticipants interface {
	QCreateAccountsHistory
	NewTransactionParticipantsBatchInsertBuilder() TransactionParticipantsBatchInsertBuilder
	NewOperationParticipantBatchInsertBuilder() OperationParticipantBatchInsertBuilder
}

// TransactionParticipantsBatchInsertBuilder is used to insert transaction participants into the
// history_transaction_participants table
type TransactionParticipantsBatchInsertBuilder interface {
	Add(transactionID int64, accountID FutureAccountID) error
	Exec(ctx context.Context, session db.SessionInterface) error
}

type transactionParticipantsBatchInsertBuilder struct {
	tableName string
	builder   db.FastBatchInsertBuilder
}

// NewTransactionParticipantsBatchInsertBuilder constructs a new TransactionParticipantsBatchInsertBuilder instance
func (q *Q) NewTransactionParticipantsBatchInsertBuilder() TransactionParticipantsBatchInsertBuilder {
	return &transactionParticipantsBatchInsertBuilder{
		tableName: "history_transaction_participants",
		builder:   db.FastBatchInsertBuilder{},
	}
}

// Add adds a new transaction participant to the batch
func (i *transactionParticipantsBatchInsertBuilder) Add(transactionID int64, accountID FutureAccountID) error {
	return i.builder.Row(map[string]interface{}{
		"history_transaction_id": transactionID,
		"history_account_id":     accountID,
	})
}

// Exec flushes all pending transaction participants to the db
func (i *transactionParticipantsBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	return i.builder.Exec(ctx, session, i.tableName)
}

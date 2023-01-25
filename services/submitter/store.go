package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/stellar/go/support/db"
)

// TransactionState represents transaction state
type TransactionState string

// Scan implements database/sql.Scanner interface
func (s *TransactionState) Scan(src interface{}) error {
	value, ok := src.([]byte)
	if !ok {
		return errors.New("cannot convert value to TransactionState")
	}
	*s = TransactionState(value)
	return nil
}

// Value implements database/sql/driver.Valuer interface
func (s TransactionState) Value() (driver.Value, error) {
	return string(s), nil
}

// Possible states of a transaction.
const (
	// TransactionStatePending indicates that a transaction is ready to be sent.
	TransactionStatePending TransactionState = "pending"
	// TransactionStateSending indicates that a transaction is submitted to Horizon and awaiting response.
	TransactionStateSending TransactionState = "sending"
	// TransactionStateSent indicates that a transaction was successfully sent and is in the ledger.
	TransactionStateSent TransactionState = "sent"
	// TransactionStateSent indicates that there was an error when trying to send this transaction.
	// Right now it requires a manual check. More complicated logic to determine if tx should be resent
	// could be built.
	TransactionStateError TransactionState = "error"
)

type Transaction struct {
	ID int64 `db:"id"`
	// Contains data that allows to identify origin of this transaction
	ExternalID string `db:"external_id"`
	// It's not safe to change this field directly. Use Store methods that will change this in safe DB transaction.
	State TransactionState `db:"state"`
	// Started sending a transaction
	SendingAt *time.Time `db:"sending_at"`
	// Transaction in the ledger
	SentAt *time.Time `db:"sent_at"`
	// Stellar account ID
	Destination string `db:"destination"`
	// Amount in lumens to send. Other assets TBD.
	Amount string `db:"amount"`
	// Transaction hash
	Hash sql.NullString `db:"hash"`
}

type PostgresStore struct {
	session *db.Session
}

// LoadPendingTransactionsAndMarkSending starts a new DB transaction and:
//   - Loads `n` Transaction setting exclusive locks on each row (SELECT ... FOR UPDATE),
//   - Changes the state of these transactions to TransactionStateSending,
//   - Saves them in a DB.
//
// Additionally it will add additional `and` condition to the query (`addQuery`). DO NOT pass user input to this variable!
func (s *PostgresStore) LoadPendingTransactionsAndMarkSending(ctx context.Context, n int) ([]*Transaction, error) {
	err := s.session.Begin()
	if err != nil {
		return nil, err
	}

	committed := false
	defer func() {
		if !committed {
			s.session.Rollback()
		}
	}()

	var transactions []*Transaction
	// SELECT FOR UPDATE reads the latest available data, setting exclusive locks on each row it reads.
	query := "SELECT * FROM transactions WHERE state = ? LIMIT ? FOR UPDATE;"
	err = s.session.SelectRaw(ctx, &transactions, query, string(TransactionStatePending), n)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(transactions))
	now := time.Now()
	for _, transaction := range transactions {
		if transaction.State != TransactionStatePending {
			return nil, errors.New("trying to update transaction state `pending` -> `sending` but state is not `pending`")
		}
		transaction.State = TransactionStateSending
		transaction.SendingAt = &now
		ids = append(ids, transaction.ID)
	}

	_, err = s.session.ExecRaw(ctx, "UPDATE transactions SET state = ?, sending_at = ? where id in ?", TransactionStateSending, now, ids)
	if err != nil {
		return nil, err
	}

	err = s.session.Commit()
	if err == nil {
		committed = true
	}
	return transactions, err
}

func (s *PostgresStore) UpdateTransactionHash(ctx context.Context, tx *Transaction, hash string) error {
	_, err := s.session.ExecRaw(ctx, "UPDATE transactions SET hash = ? where id in ?", hash, tx.ID)
	return err
}

func (s *PostgresStore) UpdateTransactionError(ctx context.Context, tx *Transaction) error {
	_, err := s.session.ExecRaw(ctx, "UPDATE transactions SET state = ? where id in ?", TransactionStateError, tx.ID)
	return err
}

func (s *PostgresStore) UpdateTransactionSuccess(ctx context.Context, tx *Transaction) error {
	_, err := s.session.ExecRaw(ctx, "UPDATE transactions SET state = ?, sent_at = ? where id in ?", TransactionStateSent, time.Now(), tx.ID)
	return err
}

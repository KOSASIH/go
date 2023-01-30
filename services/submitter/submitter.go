package main

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/network"
	"github.com/stellar/go/support/log"
	"github.com/stellar/go/txnbuild"
)

// TransactionSubmitter is responsible for sending transactions to Stellar network
type TransactionSubmitter struct {
	Horizon             horizonclient.ClientInterface
	MasterAccount       string
	Channels            []*Channel
	Store               PostgresStore
	log                 *log.Entry
	pendingTransactions chan *Transaction
}

// init initialized struct fields that couldn't be injected
func (ts *TransactionSubmitter) init() (err error) {
	// Logger
	ts.log = log.WithFields(log.F{
		"service": "TransactionSubmitter",
	})

	// Load channels
	for i, channel := range ts.Channels {
		ts.log.WithField("i", i).Info("Initializing channel")
		accountID, sequenceNumber, err := channel.LoadState(ts.Horizon)
		if err != nil {
			return err
		}
		ts.log.WithFields(log.F{
			"i":               i,
			"account_id":      accountID,
			"sequence_number": sequenceNumber,
		}).Info("Channel initialized")
	}

	// pendingTransactions channel
	ts.pendingTransactions = make(chan *Transaction, len(ts.Channels))

	return
}

// Start starts the service. This service works in the following way.
//
//  1. It initializes channels by loading their sequence numbers.
//
//  2. It starts a go routine to listen on the pendningTransactions buffered channel
//
//  3. Every second, it:
//
//     - checks if the buffered channel is full
//     - if the channel is not full, it loads transactions with state equal `pending`
//     - updates state of loaded transactions to `sending`
//     - queues them in the buffered channel
//
// See listenForPendingTransactions() for information on how transactions are processed.
func (ts *TransactionSubmitter) Start(ctx context.Context) {
	err := ts.init()
	if err != nil {
		ts.log.WithError(err).Fatal("Could not initialize TransactionSubmitter")
	}

	for _, channel := range ts.Channels {
		go ts.listenForPendingTransactions(ctx, channel)
	}

	for {
		time.Sleep(time.Second)
		if len(ts.Channels) == len(ts.pendingTransactions) {
			continue
		}
		newPendingTransactions, err := ts.Store.LoadPendingTransactionsAndMarkSending(ctx, len(ts.Channels)-len(ts.pendingTransactions))
		if err != nil {
			ts.log.WithError(err).Error("Error loading queued transactions")
			continue
		}
		for _, transaction := range newPendingTransactions {
			ts.pendingTransactions <- transaction
		}
	}
}

// listenForPendingTransactions
// If bad_seq error is returned it will call Channel.ReloadState()
func (ts *TransactionSubmitter) listenForPendingTransactions(ctx context.Context, channel *Channel) {
	for transaction := range ts.pendingTransactions {
		ts.processTransaction(ctx, transaction, channel)
	}
}

// processTrannsaction manages the database state for a transaction being submitted by a channel
func (ts *TransactionSubmitter) processTransaction(ctx context.Context, transaction *Transaction, channel *Channel) {
	log := ts.log.WithFields(log.F{
		"transaction_id": transaction.ID,
		"destination":    transaction.Destination,
		"amount":         transaction.Amount,
		"channel":        channel.GetAccountID(),
	})

	log.Info("Processing transaction")

	if transaction.State != TransactionStateSending {
		log.WithField("state", transaction.State).Error("transaction in an invalid state")
		return
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &txnbuild.SimpleAccount{
				AccountID: channel.GetAccountID(),
				Sequence:  channel.GetSequenceNumber(),
			},
			Operations: []txnbuild.Operation{
				&txnbuild.Payment{
					SourceAccount: ts.MasterAccount,
					Amount:        transaction.Amount,
					Destination:   transaction.Destination,
					Asset:         txnbuild.NativeAsset{},
				},
			},
			BaseFee: txnbuild.MinBaseFee,
			Preconditions: txnbuild.Preconditions{
				TimeBounds: txnbuild.NewInfiniteTimeout(),
			},
		},
	)
	if err != nil {
		log.WithError(err).Error("error building transaction")
		return
	}

	txHash, err := tx.Hash(network.PublicNetworkPassphrase)
	if err != nil {
		log.WithError(err).Error("error building transaction")
		return
	}

	// Important: We need to save tx hash before submitting a transaction.
	// If the script/server crashes after transaction is submitted but before the response
	// is processed, we can easily determine whether tx was sent or not later using tx hash.
	err = ts.Store.UpdateTransactionHash(ctx, transaction, hex.EncodeToString(txHash[:]))
	if err != nil {
		log.WithError(err).Error("error saving transaction hash")
		return
	}

	err = ts.Submit(tx)
	if err != nil {
		log.Info("Success submitting transaction")
		err = ts.Store.UpdateTransactionSuccess(ctx, transaction)
	} else {
		log.WithError(err).Error("Error submitting transaction")
		err = ts.Store.UpdateTransactionError(ctx, transaction)
	}

	if err != nil {
		log.WithError(err).Error("Error saving transaction sent state")
		return
	}
}

// Submits the transaction and handles any recoverable errors until it gets included in a ledger.
// Returns any error that is not recoverable.
func (ts *TransactionSubmitter) Submit(t *txnbuild.Transaction) (err error) {
	_, err = ts.Horizon.SubmitTransaction(t)
	return err
}

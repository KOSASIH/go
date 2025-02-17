package processors

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

type ClaimableBalancesTransactionProcessor struct {
	cbLoader *history.ClaimableBalanceLoader
	txBatch  history.TransactionClaimableBalanceBatchInsertBuilder
	opBatch  history.OperationClaimableBalanceBatchInsertBuilder
}

func NewClaimableBalancesTransactionProcessor(
	cbLoader *history.ClaimableBalanceLoader,
	txBatch history.TransactionClaimableBalanceBatchInsertBuilder,
	opBatch history.OperationClaimableBalanceBatchInsertBuilder,
) *ClaimableBalancesTransactionProcessor {
	return &ClaimableBalancesTransactionProcessor{
		cbLoader: cbLoader,
		txBatch:  txBatch,
		opBatch:  opBatch,
	}
}

func (p *ClaimableBalancesTransactionProcessor) Name() string {
	return "processors.ClaimableBalancesTransactionProcessor"
}

func (p *ClaimableBalancesTransactionProcessor) ProcessTransaction(
	lcm xdr.LedgerCloseMeta, transaction ingest.LedgerTransaction,
) error {
	err := p.addTransactionClaimableBalances(lcm.LedgerSequence(), transaction)
	if err != nil {
		return err
	}

	err = p.addOperationClaimableBalances(lcm.LedgerSequence(), transaction)
	if err != nil {
		return err
	}

	return nil
}

func (p *ClaimableBalancesTransactionProcessor) addTransactionClaimableBalances(
	sequence uint32, transaction ingest.LedgerTransaction,
) error {
	transactionID := toid.New(int32(sequence), int32(transaction.Index), 0).ToInt64()
	transactionClaimableBalances, err := claimableBalancesForTransaction(transaction)
	if err != nil {
		return errors.Wrap(err, "Could not determine claimable balances for transaction")
	}

	for _, cb := range dedupeStrings(transactionClaimableBalances) {
		if err = p.txBatch.Add(transactionID, p.cbLoader.GetFuture(cb)); err != nil {
			return err
		}
	}

	return nil
}

func claimableBalancesForTransaction(
	transaction ingest.LedgerTransaction,
) ([]string, error) {
	changes, err := transaction.GetChanges()
	if err != nil {
		return nil, err
	}
	cbs, err := claimableBalancesForChanges(changes)
	if err != nil {
		return nil, errors.Wrapf(err, "reading transaction %v claimable balances", transaction.Index)
	}
	return cbs, nil
}

func claimableBalancesForChanges(
	changes []ingest.Change,
) ([]string, error) {
	var cbs []string

	for _, c := range changes {
		if c.Type != xdr.LedgerEntryTypeClaimableBalance {
			continue
		}

		if c.Pre == nil && c.Post == nil {
			return nil, errors.New("Invalid io.Change: change.Pre == nil && change.Post == nil")
		}

		var claimableBalanceID xdr.ClaimableBalanceId
		if c.Pre != nil {
			claimableBalanceID = c.Pre.Data.MustClaimableBalance().BalanceId
		}
		if c.Post != nil {
			claimableBalanceID = c.Post.Data.MustClaimableBalance().BalanceId
		}
		id, err := xdr.MarshalHex(claimableBalanceID)
		if err != nil {
			return nil, err
		}
		cbs = append(cbs, id)
	}

	return cbs, nil
}

func (p *ClaimableBalancesTransactionProcessor) addOperationClaimableBalances(
	sequence uint32, transaction ingest.LedgerTransaction,
) error {
	for opi, op := range transaction.Envelope.Operations() {
		operation := transactionOperationWrapper{
			index:          uint32(opi),
			transaction:    transaction,
			operation:      op,
			ledgerSequence: sequence,
		}

		changes, err := transaction.GetOperationChanges(uint32(opi))
		if err != nil {
			return err
		}
		cbs, err := claimableBalancesForChanges(changes)
		if err != nil {
			return errors.Wrapf(err, "reading operation %v claimable balances", operation.ID())
		}

		for _, cb := range dedupeStrings(cbs) {
			if err = p.opBatch.Add(operation.ID(), p.cbLoader.GetFuture(cb)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *ClaimableBalancesTransactionProcessor) Flush(ctx context.Context, session db.SessionInterface) error {
	err := p.txBatch.Exec(ctx, session)
	if err != nil {
		return err
	}

	err = p.opBatch.Exec(ctx, session)
	if err != nil {
		return err
	}

	return nil
}

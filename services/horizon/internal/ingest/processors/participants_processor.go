//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

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

// ParticipantsProcessor is a processor which ingests various participants
// from different sources (transactions, operations, etc)
type ParticipantsProcessor struct {
	accountLoader *history.AccountLoader
	txBatch       history.TransactionParticipantsBatchInsertBuilder
	opBatch       history.OperationParticipantBatchInsertBuilder
	network       string
}

func NewParticipantsProcessor(
	accountLoader *history.AccountLoader,
	txBatch history.TransactionParticipantsBatchInsertBuilder,
	opBatch history.OperationParticipantBatchInsertBuilder,
	network string,

) *ParticipantsProcessor {
	return &ParticipantsProcessor{
		accountLoader: accountLoader,
		txBatch:       txBatch,
		opBatch:       opBatch,
		network:       network,
	}
}

func participantsForChanges(
	changes xdr.LedgerEntryChanges,
) ([]xdr.AccountId, error) {
	var participants []xdr.AccountId

	for _, c := range changes {
		var participant *xdr.AccountId

		switch c.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			participant = participantsForLedgerEntry(c.MustCreated())
		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			participant = participantsForLedgerKey(c.MustRemoved())
		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			participant = participantsForLedgerEntry(c.MustUpdated())
		case xdr.LedgerEntryChangeTypeLedgerEntryState:
			participant = participantsForLedgerEntry(c.MustState())
		default:
			return nil, errors.Errorf("Unknown change type: %s", c.Type)
		}

		if participant != nil {
			participants = append(participants, *participant)
		}
	}

	return participants, nil
}

func participantsForLedgerEntry(le xdr.LedgerEntry) *xdr.AccountId {
	if le.Data.Type != xdr.LedgerEntryTypeAccount {
		return nil
	}
	aid := le.Data.MustAccount().AccountId
	return &aid
}

func participantsForLedgerKey(lk xdr.LedgerKey) *xdr.AccountId {
	if lk.Type != xdr.LedgerEntryTypeAccount {
		return nil
	}
	aid := lk.MustAccount().AccountId
	return &aid
}

func participantsForMeta(
	meta xdr.TransactionMeta,
) ([]xdr.AccountId, error) {
	var participants []xdr.AccountId
	if meta.Operations == nil {
		return participants, nil
	}

	for _, op := range *meta.Operations {
		var accounts []xdr.AccountId
		accounts, err := participantsForChanges(op.Changes)
		if err != nil {
			return nil, err
		}

		participants = append(participants, accounts...)
	}

	return participants, nil
}

func (p *ParticipantsProcessor) Name() string {
	return "processors.ParticipantsProcessor"
}

func (p *ParticipantsProcessor) addTransactionParticipants(
	sequence uint32,
	transaction ingest.LedgerTransaction,
) error {
	transactionID := toid.New(int32(sequence), int32(transaction.Index), 0).ToInt64()
	transactionParticipants, err := ParticipantsForTransaction(
		sequence,
		transaction,
	)
	if err != nil {
		return errors.Wrap(err, "Could not determine participants for transaction")
	}

	for _, participant := range transactionParticipants {
		if err := p.txBatch.Add(transactionID, p.accountLoader.GetFuture(participant.Address())); err != nil {
			return err
		}
	}

	return nil
}

func (p *ParticipantsProcessor) addOperationsParticipants(
	sequence uint32,
	transaction ingest.LedgerTransaction,
) error {
	participants, err := operationsParticipants(transaction, sequence, p.network)
	if err != nil {
		return errors.Wrap(err, "could not determine operation participants")
	}

	for operationID, addresses := range participants {
		for _, participant := range addresses {
			address := participant.Address()
			if err := p.opBatch.Add(operationID, p.accountLoader.GetFuture(address)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *ParticipantsProcessor) ProcessTransaction(lcm xdr.LedgerCloseMeta, transaction ingest.LedgerTransaction) error {

	if err := p.addTransactionParticipants(lcm.LedgerSequence(), transaction); err != nil {
		return err
	}

	if err := p.addOperationsParticipants(lcm.LedgerSequence(), transaction); err != nil {
		return err
	}

	return nil
}

func (p *ParticipantsProcessor) Flush(ctx context.Context, session db.SessionInterface) error {
	if err := p.txBatch.Exec(ctx, session); err != nil {
		return errors.Wrap(err, "Could not flush transaction participants to db")
	}
	if err := p.opBatch.Exec(ctx, session); err != nil {
		return errors.Wrap(err, "Could not flush operation participants to db")
	}
	return nil
}

func ParticipantsForTransaction(
	sequence uint32,
	transaction ingest.LedgerTransaction,
) ([]xdr.AccountId, error) {
	participants := []xdr.AccountId{
		transaction.Envelope.SourceAccount().ToAccountId(),
	}
	if transaction.Envelope.IsFeeBump() {
		participants = append(participants, transaction.Envelope.FeeBumpAccount().ToAccountId())
	}

	p, err := participantsForMeta(transaction.UnsafeMeta)
	if err != nil {
		return nil, err
	}
	participants = append(participants, p...)

	p, err = participantsForChanges(transaction.FeeChanges)
	if err != nil {
		return nil, err
	}
	participants = append(participants, p...)

	for opi, op := range transaction.Envelope.Operations() {
		operation := transactionOperationWrapper{
			index:          uint32(opi),
			transaction:    transaction,
			operation:      op,
			ledgerSequence: sequence,
		}

		p, err := operation.Participants()
		if err != nil {
			return nil, errors.Wrapf(
				err, "could not determine operation %v participants", operation.ID(),
			)
		}
		participants = append(participants, p...)
	}

	return dedupeParticipants(participants), nil
}

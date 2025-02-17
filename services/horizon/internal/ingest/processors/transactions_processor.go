package processors

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type TransactionProcessor struct {
	batch      history.TransactionBatchInsertBuilder
	skipTxmeta bool
	name       string
}

func NewTransactionFilteredTmpProcessor(batch history.TransactionBatchInsertBuilder, skipTxmeta bool) *TransactionProcessor {
	return &TransactionProcessor{
		batch:      batch,
		skipTxmeta: skipTxmeta,
		name:       "processors.TransactionFilteredTmpProcessor",
	}
}

func NewTransactionProcessor(batch history.TransactionBatchInsertBuilder, skipTxmeta bool) *TransactionProcessor {
	return &TransactionProcessor{
		batch:      batch,
		skipTxmeta: skipTxmeta,
		name:       "processors.TransactionProcessor",
	}
}

func (p *TransactionProcessor) Name() string {
	return p.name
}

func (p *TransactionProcessor) ProcessTransaction(lcm xdr.LedgerCloseMeta, transaction ingest.LedgerTransaction) error {
	elidedTransaction := transaction

	if p.skipTxmeta {
		switch elidedTransaction.UnsafeMeta.V {
		case 3:
			elidedTransaction.UnsafeMeta.V3 = &xdr.TransactionMetaV3{
				Ext:             xdr.ExtensionPoint{},
				TxChangesBefore: xdr.LedgerEntryChanges{},
				Operations:      []xdr.OperationMeta{},
				TxChangesAfter:  xdr.LedgerEntryChanges{},
				SorobanMeta:     nil,
			}
		case 2:
			elidedTransaction.UnsafeMeta.V2 = &xdr.TransactionMetaV2{
				TxChangesBefore: xdr.LedgerEntryChanges{},
				Operations:      []xdr.OperationMeta{},
				TxChangesAfter:  xdr.LedgerEntryChanges{},
			}
		case 1:
			elidedTransaction.UnsafeMeta.V1 = &xdr.TransactionMetaV1{
				TxChanges:  xdr.LedgerEntryChanges{},
				Operations: []xdr.OperationMeta{},
			}
		default:
			return errors.Errorf("SKIP_TXMETA is enabled, but received an un-supported tx-meta version %v, can't proceed with removal", elidedTransaction.UnsafeMeta.V)
		}
	}

	if err := p.batch.Add(elidedTransaction, lcm.LedgerSequence()); err != nil {
		return errors.Wrap(err, "Error batch inserting transaction rows")
	}

	return nil
}

func (p *TransactionProcessor) Flush(ctx context.Context, session db.SessionInterface) error {
	return p.batch.Exec(ctx, session)
}

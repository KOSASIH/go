package processors

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type AccountDataProcessor struct {
	dataQ history.QData

	batchInsertBuilder history.AccountDataBatchInsertBuilder
	dataToUpdate       []history.Data
	dataToDelete       []history.AccountDataKey
}

func NewAccountDataProcessor(dataQ history.QData) *AccountDataProcessor {
	p := &AccountDataProcessor{dataQ: dataQ}
	p.reset()
	return p
}

func (p *AccountDataProcessor) reset() {
	p.batchInsertBuilder = p.dataQ.NewAccountDataBatchInsertBuilder()
	p.dataToUpdate = []history.Data{}
	p.dataToDelete = []history.AccountDataKey{}
}

func (p *AccountDataProcessor) Name() string {
	return "processors.AccountDataProcessor"
}

func (p *AccountDataProcessor) ProcessChange(ctx context.Context, change ingest.Change) error {
	// We're interested in data only
	if change.Type != xdr.LedgerEntryTypeData {
		return nil
	}

	switch {
	case change.Pre == nil && change.Post != nil:
		// Created
		err := p.batchInsertBuilder.Add(p.ledgerEntryToRow(change.Post))
		if err != nil {
			return errors.Wrap(err, "Error adding to AccountDataBatchInsertBuilder")
		}
	case change.Pre != nil && change.Post == nil:
		// Removed
		data := change.Pre.Data.MustData()
		key := history.AccountDataKey{
			AccountID: data.AccountId.Address(),
			DataName:  string(data.DataName),
		}
		p.dataToDelete = append(p.dataToDelete, key)
	default:
		// Updated
		p.dataToUpdate = append(p.dataToUpdate, p.ledgerEntryToRow(change.Post))
	}

	if p.batchInsertBuilder.Len()+len(p.dataToUpdate)+len(p.dataToDelete) > maxBatchSize {

		if err := p.Commit(ctx); err != nil {
			return errors.Wrap(err, "error in Commit")
		}
	}

	return nil
}

func (p *AccountDataProcessor) Commit(ctx context.Context) error {
	defer p.reset()

	err := p.batchInsertBuilder.Exec(ctx)
	if err != nil {
		return errors.Wrap(err, "Error executing AccountDataBatchInsertBuilder")
	}

	if len(p.dataToUpdate) > 0 {
		if err := p.dataQ.UpsertAccountData(ctx, p.dataToUpdate); err != nil {
			return errors.Wrap(err, "error executing upsert")
		}
	}

	if len(p.dataToDelete) > 0 {
		count, err := p.dataQ.RemoveAccountData(ctx, p.dataToDelete)
		if err != nil {
			return errors.Wrap(err, "error executing removal")
		}
		if count != int64(len(p.dataToDelete)) {
			return ingest.NewStateError(errors.Errorf(
				"%d rows affected when deleting %d account data",
				count,
				len(p.dataToDelete),
			))
		}
	}

	return nil
}

func (p *AccountDataProcessor) ledgerEntryToRow(entry *xdr.LedgerEntry) history.Data {
	data := entry.Data.MustData()
	return history.Data{
		AccountID:          data.AccountId.Address(),
		Name:               string(data.DataName),
		Value:              history.AccountDataValue(data.DataValue),
		LastModifiedLedger: uint32(entry.LastModifiedLedgerSeq),
		Sponsor:            ledgerEntrySponsorToNullString(*entry),
	}
}

package processors

import (
	"context"

	"github.com/guregu/null/zero"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type AccountsProcessor struct {
	accountsQ history.QAccounts

	batchUpdateAccounts []history.AccountEntry
	removeBatch         []string
	batchInsertBuilder  history.AccountsBatchInsertBuilder
}

func NewAccountsProcessor(accountsQ history.QAccounts) *AccountsProcessor {
	p := &AccountsProcessor{accountsQ: accountsQ}
	p.reset()
	return p
}

func (p *AccountsProcessor) reset() {
	p.batchInsertBuilder = p.accountsQ.NewAccountsBatchInsertBuilder()
	p.batchUpdateAccounts = []history.AccountEntry{}
	p.removeBatch = []string{}
}

func (p *AccountsProcessor) Name() string {
	return "processors.AccountsProcessor"
}

func (p *AccountsProcessor) ProcessChange(ctx context.Context, change ingest.Change) error {
	if change.Type != xdr.LedgerEntryTypeAccount {
		return nil
	}

	changed, err := change.AccountChangedExceptSigners()
	if err != nil {
		return errors.Wrap(err, "Error running change.AccountChangedExceptSigners")
	}

	if !changed {
		return nil
	}

	switch {
	case change.Pre == nil && change.Post != nil:
		// Created
		row := p.ledgerEntryToRow(*change.Post)
		err = p.batchInsertBuilder.Add(row)
		if err != nil {
			return errors.Wrap(err, "Error adding to AccountsBatchInsertBuilder")
		}
	case change.Pre != nil && change.Post != nil:
		// Updated
		row := p.ledgerEntryToRow(*change.Post)
		p.batchUpdateAccounts = append(p.batchUpdateAccounts, row)
	case change.Pre != nil && change.Post == nil:
		// Removed
		account := change.Pre.Data.MustAccount()
		accountID := account.AccountId.Address()
		p.removeBatch = append(p.removeBatch, accountID)
	default:
		return errors.New("Invalid io.Change: change.Pre == nil && change.Post == nil")
	}

	if p.batchInsertBuilder.Len()+len(p.batchUpdateAccounts)+len(p.removeBatch) > maxBatchSize {
		err = p.Commit(ctx)
		if err != nil {
			return errors.Wrap(err, "error in Commit")
		}
	}

	return nil
}

func (p *AccountsProcessor) Commit(ctx context.Context) error {
	defer p.reset()

	err := p.batchInsertBuilder.Exec(ctx)
	if err != nil {
		return errors.Wrap(err, "Error executing AccountsBatchInsertBuilder")
	}

	// Upsert accounts
	if len(p.batchUpdateAccounts) > 0 {
		err := p.accountsQ.UpsertAccounts(ctx, p.batchUpdateAccounts)
		if err != nil {
			return errors.Wrap(err, "errors in UpsertAccounts")
		}
	}

	if len(p.removeBatch) > 0 {
		rowsAffected, err := p.accountsQ.RemoveAccounts(ctx, p.removeBatch)
		if err != nil {
			return errors.Wrap(err, "error in RemoveAccounts")
		}

		if rowsAffected != int64(len(p.removeBatch)) {
			return ingest.NewStateError(errors.Errorf(
				"%d rows affected when removing %d accounts",
				rowsAffected,
				len(p.removeBatch),
			))
		}
	}

	return nil
}

func (p *AccountsProcessor) ledgerEntryToRow(entry xdr.LedgerEntry) history.AccountEntry {
	account := entry.Data.MustAccount()
	liabilities := account.Liabilities()

	var inflationDestination = ""
	if account.InflationDest != nil {
		inflationDestination = account.InflationDest.Address()
	}

	return history.AccountEntry{
		AccountID:            account.AccountId.Address(),
		Balance:              int64(account.Balance),
		BuyingLiabilities:    int64(liabilities.Buying),
		SellingLiabilities:   int64(liabilities.Selling),
		SequenceNumber:       int64(account.SeqNum),
		SequenceLedger:       zero.IntFrom(int64(account.SeqLedger())),
		SequenceTime:         zero.IntFrom(int64(account.SeqTime())),
		NumSubEntries:        uint32(account.NumSubEntries),
		InflationDestination: inflationDestination,
		Flags:                uint32(account.Flags),
		HomeDomain:           string(account.HomeDomain),
		MasterWeight:         account.MasterKeyWeight(),
		ThresholdLow:         account.ThresholdLow(),
		ThresholdMedium:      account.ThresholdMedium(),
		ThresholdHigh:        account.ThresholdHigh(),
		LastModifiedLedger:   uint32(entry.LastModifiedLedgerSeq),
		Sponsor:              ledgerEntrySponsorToNullString(entry),
		NumSponsored:         uint32(account.NumSponsored()),
		NumSponsoring:        uint32(account.NumSponsoring()),
	}
}

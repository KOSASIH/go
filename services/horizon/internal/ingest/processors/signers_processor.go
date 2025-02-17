package processors

import (
	"context"

	"github.com/guregu/null"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type SignersProcessor struct {
	signersQ history.QSigners

	batchInsertBuilder history.AccountSignersBatchInsertBuilder
}

func NewSignersProcessor(
	signersQ history.QSigners,
) *SignersProcessor {
	p := &SignersProcessor{signersQ: signersQ}
	p.reset()
	return p
}

func (p *SignersProcessor) Name() string {
	return "processors.SignersProcessor"
}

func (p *SignersProcessor) reset() {
	p.batchInsertBuilder = p.signersQ.NewAccountSignersBatchInsertBuilder()
}

func accountSignersDiff(change ingest.Change) ([]string, map[string]int32, map[string]string) {
	var preSignerSummary map[string]int32
	var preSponsorsPerSigner map[string]string
	var postSignerSummary map[string]int32
	var postSponsorsPerSigner map[string]string
	var removedSigners []string

	if change.Pre != nil {
		accountEntry := change.Pre.Data.MustAccount()
		preSignerSummary = accountEntry.SignerSummary()
		preSponsorsPerSigner = map[string]string{}
		for signer, sponsor := range accountEntry.SponsorPerSigner() {
			preSponsorsPerSigner[signer] = sponsor.Address()
		}
	}

	if change.Post != nil {
		accountEntry := change.Post.Data.MustAccount()
		postSignerSummary = accountEntry.SignerSummary()
		postSponsorsPerSigner = map[string]string{}
		for signer, sponsor := range accountEntry.SponsorPerSigner() {
			postSponsorsPerSigner[signer] = sponsor.Address()
		}
	}

	for signer, preWeight := range preSignerSummary {
		postWeight, ok := postSignerSummary[signer]
		if ok && preWeight == postWeight && preSponsorsPerSigner[signer] == postSponsorsPerSigner[signer] {
			delete(postSignerSummary, signer)
			delete(postSponsorsPerSigner, signer)
			continue
		}
		removedSigners = append(removedSigners, signer)
	}
	return removedSigners, postSignerSummary, postSponsorsPerSigner
}

func (p *SignersProcessor) ProcessChange(ctx context.Context, change ingest.Change) error {
	if change.Type != xdr.LedgerEntryTypeAccount {
		return nil
	}

	removed, signerSummary, sponsersPerSigner := accountSignersDiff(change)
	if len(removed) == 0 && len(signerSummary) == 0 {
		return nil
	}

	if len(removed) > 0 {
		accountAddress := change.Pre.Data.MustAccount().AccountId.Address()
		if err := p.removeAccountSigners(ctx, accountAddress, removed); err != nil {
			return err
		}
	}

	if len(signerSummary) > 0 {
		accountAddress := change.Post.Data.MustAccount().AccountId.Address()
		if err := p.addAccountSigners(accountAddress, signerSummary, sponsersPerSigner); err != nil {
			return err
		}
	}

	if p.batchInsertBuilder.Len() > maxBatchSize {
		if err := p.Commit(ctx); err != nil {
			return errors.Wrap(err, "error in Commit")
		}
	}

	return nil
}

func (p *SignersProcessor) Commit(ctx context.Context) error {
	defer p.reset()

	err := p.batchInsertBuilder.Exec(ctx)
	if err != nil {
		return errors.Wrap(err, "error executing AccountSignersBatchInsertBuilder")
	}

	return nil
}

func (p *SignersProcessor) removeAccountSigners(ctx context.Context, accountAddress string, signers []string) error {
	rowsAffected, err := p.signersQ.RemoveAccountSigners(ctx, accountAddress, signers)
	if err != nil {
		return errors.Wrap(err, "Error removing a signer")
	}

	if rowsAffected != int64(len(signers)) {
		return ingest.NewStateError(errors.Errorf(
			"Expected account=%s signers=%s in database but not found when removing (rows affected = %d)",
			accountAddress,
			signers,
			rowsAffected,
		))
	}

	return nil
}

func (p *SignersProcessor) addAccountSigners(
	accountAddress string,
	signerSummary map[string]int32,
	sponsorsPerSigner map[string]string,
) error {
	for signer, weight := range signerSummary {
		// Ignore master key
		var sponsor null.String
		if signer != accountAddress {
			if sponsorDesc, isSponsored := sponsorsPerSigner[signer]; isSponsored {
				sponsor = null.StringFrom(sponsorDesc)
			}
		}

		if err := p.batchInsertBuilder.Add(history.AccountSigner{
			Account: accountAddress,
			Signer:  signer,
			Weight:  weight,
			Sponsor: sponsor,
		}); err != nil {
			return errors.Wrapf(err, "Error adding signer (%s) to AccountSignersBatchInsertBuilder", signer)
		}
	}
	return nil
}

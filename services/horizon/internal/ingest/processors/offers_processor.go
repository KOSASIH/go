package processors

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

// The offers processor can be configured to trim the offers table
// by removing all offer rows which were marked for deletion at least 100 ledgers ago
const compactionWindow = uint32(100)

type OffersProcessor struct {
	offersQ  history.QOffers
	sequence uint32

	batchUpdateOffers  []history.Offer
	insertBatchBuilder history.OffersBatchInsertBuilder
}

func NewOffersProcessor(offersQ history.QOffers, sequence uint32) *OffersProcessor {
	p := &OffersProcessor{offersQ: offersQ, sequence: sequence}
	p.reset()
	return p
}

func (p *OffersProcessor) Name() string {
	return "processors.OffersProcessor"
}

func (p *OffersProcessor) reset() {
	p.batchUpdateOffers = []history.Offer{}
	p.insertBatchBuilder = p.offersQ.NewOffersBatchInsertBuilder()
}

func (p *OffersProcessor) ProcessChange(ctx context.Context, change ingest.Change) error {
	if change.Type != xdr.LedgerEntryTypeOffer {
		return nil
	}

	switch {
	case change.Pre == nil && change.Post != nil:
		// Created
		err := p.insertBatchBuilder.Add(p.ledgerEntryToRow(change.Post))
		if err != nil {
			return errors.New("Error adding to OffersBatchInsertBuilder")
		}
	case change.Pre != nil && change.Post != nil:
		// Updated
		row := p.ledgerEntryToRow(change.Post)
		p.batchUpdateOffers = append(p.batchUpdateOffers, row)
	case change.Pre != nil && change.Post == nil:
		// Removed
		row := p.ledgerEntryToRow(change.Pre)
		row.Deleted = true
		row.LastModifiedLedger = p.sequence
		p.batchUpdateOffers = append(p.batchUpdateOffers, row)
	default:
		return errors.New("Invalid io.Change: change.Pre == nil && change.Post == nil")
	}

	if p.insertBatchBuilder.Len()+len(p.batchUpdateOffers) > maxBatchSize {
		if err := p.flushCache(ctx); err != nil {
			return errors.Wrap(err, "error in Commit")
		}
	}

	return nil
}

func (p *OffersProcessor) ledgerEntryToRow(entry *xdr.LedgerEntry) history.Offer {
	offer := entry.Data.MustOffer()
	return history.Offer{
		SellerID:           offer.SellerId.Address(),
		OfferID:            int64(offer.OfferId),
		SellingAsset:       offer.Selling,
		BuyingAsset:        offer.Buying,
		Amount:             int64(offer.Amount),
		Pricen:             int32(offer.Price.N),
		Priced:             int32(offer.Price.D),
		Price:              float64(offer.Price.N) / float64(offer.Price.D),
		Flags:              int32(offer.Flags),
		LastModifiedLedger: uint32(entry.LastModifiedLedgerSeq),
		Sponsor:            ledgerEntrySponsorToNullString(*entry),
	}
}

func (p *OffersProcessor) flushCache(ctx context.Context) error {
	defer p.reset()

	err := p.insertBatchBuilder.Exec(ctx)
	if err != nil {
		return errors.New("Error executing OffersBatchInsertBuilder")
	}

	if len(p.batchUpdateOffers) > 0 {
		err := p.offersQ.UpsertOffers(ctx, p.batchUpdateOffers)
		if err != nil {
			return errors.Wrap(err, "errors in UpsertOffers")
		}
	}

	return nil
}

func (p *OffersProcessor) Commit(ctx context.Context) error {
	if err := p.flushCache(ctx); err != nil {
		return errors.Wrap(err, "error flushing cache")
	}

	if p.sequence > compactionWindow {
		// trim offers table by removing offers which were deleted before the cutoff ledger
		if offerRowsRemoved, err := p.offersQ.CompactOffers(ctx, p.sequence-compactionWindow); err != nil {
			return errors.Wrap(err, "could not compact offers")
		} else {
			log.WithField("offer_rows_removed", offerRowsRemoved).Info("Trimmed offers table")
		}
	}

	return nil
}

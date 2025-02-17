package processors

import (
	"context"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type TrustLinesProcessor struct {
	trustLinesQ history.QTrustLines

	batchUpdateTrustlines    []history.TrustLine
	batchRemoveTrustLineKeys []string
	batchInsertBuilder       history.TrustLinesBatchInsertBuilder
}

func NewTrustLinesProcessor(trustLinesQ history.QTrustLines) *TrustLinesProcessor {
	p := &TrustLinesProcessor{trustLinesQ: trustLinesQ}
	p.reset()
	return p
}

func (p *TrustLinesProcessor) Name() string {
	return "processors.TrustLinesProcessor"
}

func (p *TrustLinesProcessor) reset() {
	p.batchUpdateTrustlines = []history.TrustLine{}
	p.batchRemoveTrustLineKeys = []string{}
	p.batchInsertBuilder = p.trustLinesQ.NewTrustLinesBatchInsertBuilder()
}

func (p *TrustLinesProcessor) ProcessChange(ctx context.Context, change ingest.Change) error {
	if change.Type != xdr.LedgerEntryTypeTrustline {
		return nil
	}

	switch {
	case change.Pre == nil && change.Post != nil:
		// Created
		line, err := xdrToTrustline(*change.Post)
		if err != nil {
			return errors.Wrap(err, "Error extracting trustline")
		}

		err = p.batchInsertBuilder.Add(line)
		if err != nil {
			return errors.Wrap(err, "Error adding to TrustLinesBatchInsertBuilder")
		}
	case change.Pre != nil && change.Post != nil:
		// Updated
		tl, err := xdrToTrustline(*change.Post)
		if err != nil {
			return errors.Wrap(err, "Error extracting trustline")
		}
		p.batchUpdateTrustlines = append(p.batchUpdateTrustlines, tl)
	case change.Pre != nil && change.Post == nil:
		// Removed
		trustLineEntry := change.Pre.Data.MustTrustLine()
		ledgerKeyString, err := trustLineLedgerKey(trustLineEntry)
		if err != nil {
			return errors.Wrap(err, "Error extracting ledger key")
		}
		p.batchRemoveTrustLineKeys = append(p.batchRemoveTrustLineKeys, ledgerKeyString)

	default:
		return errors.New("Invalid io.Change: change.Pre == nil && change.Post == nil")
	}

	if p.batchInsertBuilder.Len()+len(p.batchUpdateTrustlines)+len(p.batchRemoveTrustLineKeys) > maxBatchSize {

		if err := p.Commit(ctx); err != nil {
			return errors.Wrap(err, "error in Commit")
		}
	}

	return nil
}

func trustLineLedgerKey(trustLineEntry xdr.TrustLineEntry) (string, error) {
	var ledgerKey xdr.LedgerKey
	var ledgerKeyString string

	err := ledgerKey.SetTrustline(trustLineEntry.AccountId, trustLineEntry.Asset)
	if err != nil {
		return "", errors.Wrap(err, "Error creating ledger key")
	}
	ledgerKeyString, err = ledgerKey.MarshalBinaryBase64()
	if err != nil {
		return "", errors.Wrap(err, "Error marshaling ledger key")
	}
	return ledgerKeyString, nil
}

func xdrToTrustline(ledgerEntry xdr.LedgerEntry) (history.TrustLine, error) {
	trustLineEntry := ledgerEntry.Data.MustTrustLine()
	ledgerKeyString, err := trustLineLedgerKey(trustLineEntry)
	if err != nil {
		return history.TrustLine{}, errors.Wrap(err, "Error extracting ledger key")
	}

	assetType := trustLineEntry.Asset.Type
	var assetCode, assetIssuer, poolID string
	if assetType == xdr.AssetTypeAssetTypePoolShare {
		poolID = PoolIDToString(trustLineEntry.Asset.MustLiquidityPoolId())
	} else {
		if err = trustLineEntry.Asset.ToAsset().Extract(&assetType, &assetCode, &assetIssuer); err != nil {
			return history.TrustLine{}, errors.Wrap(err, "Error extracting asset from trustline")
		}
	}

	liabilities := trustLineEntry.Liabilities()
	return history.TrustLine{
		AccountID:          trustLineEntry.AccountId.Address(),
		AssetType:          assetType,
		AssetIssuer:        assetIssuer,
		AssetCode:          assetCode,
		Balance:            int64(trustLineEntry.Balance),
		LedgerKey:          ledgerKeyString,
		Limit:              int64(trustLineEntry.Limit),
		LiquidityPoolID:    poolID,
		BuyingLiabilities:  int64(liabilities.Buying),
		SellingLiabilities: int64(liabilities.Selling),
		Flags:              uint32(trustLineEntry.Flags),
		LastModifiedLedger: uint32(ledgerEntry.LastModifiedLedgerSeq),
		Sponsor:            ledgerEntrySponsorToNullString(ledgerEntry),
	}, nil
}

func (p *TrustLinesProcessor) Commit(ctx context.Context) error {
	defer p.reset()

	err := p.batchInsertBuilder.Exec(ctx)
	if err != nil {
		return errors.Wrap(err, "Error executing TrustLinesBatchInsertBuilder")
	}

	if len(p.batchUpdateTrustlines) > 0 {
		err := p.trustLinesQ.UpsertTrustLines(ctx, p.batchUpdateTrustlines)
		if err != nil {
			return errors.Wrap(err, "errors in UpsertTrustLines")
		}
	}

	if len(p.batchRemoveTrustLineKeys) > 0 {
		rowsAffected, err := p.trustLinesQ.RemoveTrustLines(ctx, p.batchRemoveTrustLineKeys)
		if err != nil {
			return err
		}

		if rowsAffected != int64(len(p.batchRemoveTrustLineKeys)) {
			return ingest.NewStateError(errors.Errorf(
				"%d rows affected when removing %d trust lines",
				rowsAffected,
				len(p.batchRemoveTrustLineKeys),
			))
		}
	}

	return nil
}

package resourceadapter

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/guregu/null"
	"strconv"

	horizonContext "github.com/stellar/go/services/horizon/internal/context"
	"github.com/stellar/go/xdr"

	protocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/render/hal"
)

// Populate fills out the details
func PopulateTransaction(
	ctx context.Context,
	transactionHash string,
	dest *protocol.Transaction,
	row history.Transaction,
	skipTxMeta bool,
) error {
	dest.ID = transactionHash
	dest.PT = row.PagingToken()
	dest.Successful = row.Successful
	dest.Hash = transactionHash
	dest.Ledger = row.LedgerSequence
	dest.LedgerCloseTime = row.LedgerCloseTime
	dest.Account = row.Account
	if row.AccountMuxed.Valid {
		dest.AccountMuxed = row.AccountMuxed.String
		muxedAccount := xdr.MustMuxedAddress(dest.AccountMuxed)
		dest.AccountMuxedID = uint64(muxedAccount.Med25519.Id)
	}
	dest.AccountSequence = row.AccountSequence

	dest.FeeCharged = row.FeeCharged

	dest.OperationCount = row.OperationCount
	dest.EnvelopeXdr = row.TxEnvelope
	dest.ResultXdr = row.TxResult
	if skipTxMeta {
		dest.ResultMetaXdr = ""
	} else {
		dest.ResultMetaXdr = row.TxMeta
	}
	dest.FeeMetaXdr = row.TxFeeMeta
	dest.MemoType = row.MemoType
	dest.Memo = row.Memo.String
	if row.MemoType == "text" {
		if memoBytes, err := memoBytes(row.TxEnvelope); err != nil {
			return err
		} else {
			dest.MemoBytes = memoBytes
		}
	}
	dest.Signatures = row.Signatures

	// If we never use this, we'll remove it later. This just defends us against
	// nil dereferences.
	dest.Preconditions = &protocol.TransactionPreconditions{}

	if !row.TimeBounds.Null {
		dest.Preconditions.TimeBounds = &protocol.TransactionPreconditionsTimebounds{
			MaxTime: timestampString(row.TimeBounds.Upper),
			MinTime: timestampString(row.TimeBounds.Lower),
		}
	}

	if !row.LedgerBounds.Null {
		dest.Preconditions.LedgerBounds = &protocol.TransactionPreconditionsLedgerbounds{}
		if row.LedgerBounds.MinLedger.Valid {
			dest.Preconditions.LedgerBounds.MinLedger = uint32(row.LedgerBounds.MinLedger.Int64)
		}
		if row.LedgerBounds.MaxLedger.Valid {
			dest.Preconditions.LedgerBounds.MaxLedger = uint32(row.LedgerBounds.MaxLedger.Int64)
		}
	}

	if row.MinAccountSequence.Valid {
		dest.Preconditions.MinAccountSequence = fmt.Sprint(row.MinAccountSequence.Int64)
	}

	if row.MinAccountSequenceAge.Valid && row.MinAccountSequenceAge.String != "0" {
		dest.Preconditions.MinAccountSequenceAge = row.MinAccountSequenceAge.String
	}

	if row.MinAccountSequenceLedgerGap.Valid {
		dest.Preconditions.MinAccountSequenceLedgerGap = uint32(row.MinAccountSequenceLedgerGap.Int64)
	}

	if len(row.ExtraSigners) > 0 {
		dest.Preconditions.ExtraSigners = row.ExtraSigners
	}

	if row.InnerTransactionHash.Valid {
		dest.FeeAccount = row.FeeAccount.String
		if row.FeeAccountMuxed.Valid {
			dest.FeeAccountMuxed = row.FeeAccountMuxed.String
			muxedAccount := xdr.MustMuxedAddress(dest.FeeAccountMuxed)
			dest.FeeAccountMuxedID = uint64(muxedAccount.Med25519.Id)
		}
		dest.MaxFee = row.NewMaxFee.Int64
		dest.FeeBumpTransaction = &protocol.FeeBumpTransaction{
			Hash:       row.TransactionHash,
			Signatures: dest.Signatures,
		}
		dest.InnerTransaction = &protocol.InnerTransaction{
			Hash:       row.InnerTransactionHash.String,
			MaxFee:     row.MaxFee,
			Signatures: row.InnerSignatures,
		}
		if transactionHash != row.TransactionHash {
			dest.Signatures = dest.InnerTransaction.Signatures
		}
	} else {
		dest.FeeAccount = dest.Account
		dest.FeeAccountMuxed = dest.AccountMuxed
		dest.FeeAccountMuxedID = dest.AccountMuxedID
		dest.MaxFee = row.MaxFee
	}

	lb := hal.LinkBuilder{Base: horizonContext.BaseURL(ctx)}
	dest.Links.Account = lb.Link("/accounts", dest.Account)
	dest.Links.Ledger = lb.Link("/ledgers", fmt.Sprintf("%d", dest.Ledger))
	dest.Links.Operations = lb.PagedLink("/transactions", dest.ID, "operations")
	dest.Links.Effects = lb.PagedLink("/transactions", dest.ID, "effects")
	dest.Links.Self = lb.Link("/transactions", dest.ID)
	dest.Links.Transaction = dest.Links.Self
	dest.Links.Succeeds = lb.Linkf("/transactions?order=desc&cursor=%s", dest.PT)
	dest.Links.Precedes = lb.Linkf("/transactions?order=asc&cursor=%s", dest.PT)

	// If we didn't need the structure, drop it.
	if !row.HasPreconditions() {
		dest.Preconditions = nil
	}

	return nil
}

func memoBytes(envelopeXDR string) (string, error) {
	var parsedEnvelope xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshalBase64(envelopeXDR, &parsedEnvelope); err != nil {
		return "", err
	}

	memo := *parsedEnvelope.Memo().Text
	return base64.StdEncoding.EncodeToString([]byte(memo)), nil
}

func timestampString(in null.Int) string {
	if !in.Valid {
		return ""
	}

	return strconv.FormatInt(in.Int64, 10)
}

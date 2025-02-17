package history

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/guregu/null"
	"github.com/lib/pq"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/utf8"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

// TransactionBatchInsertBuilder is used to insert transactions into the
// history_transactions table
type TransactionBatchInsertBuilder interface {
	Add(transaction ingest.LedgerTransaction, sequence uint32) error
	Exec(ctx context.Context, session db.SessionInterface) error
}

// transactionBatchInsertBuilder is a simple wrapper around db.BatchInsertBuilder
type transactionBatchInsertBuilder struct {
	encodingBuffer *xdr.EncodingBuffer
	table          string
	builder        db.FastBatchInsertBuilder
}

// NewTransactionBatchInsertBuilder constructs a new TransactionBatchInsertBuilder instance
func (q *Q) NewTransactionBatchInsertBuilder() TransactionBatchInsertBuilder {
	return &transactionBatchInsertBuilder{
		encodingBuffer: xdr.NewEncodingBuffer(),
		table:          "history_transactions",
		builder:        db.FastBatchInsertBuilder{},
	}
}

// NewTransactionFilteredTmpBatchInsertBuilder constructs a new TransactionBatchInsertBuilder instance
func (q *Q) NewTransactionFilteredTmpBatchInsertBuilder() TransactionBatchInsertBuilder {
	return &transactionBatchInsertBuilder{
		encodingBuffer: xdr.NewEncodingBuffer(),
		table:          "history_transactions_filtered_tmp",
		builder:        db.FastBatchInsertBuilder{},
	}
}

// Add adds a new transaction to the batch
func (i *transactionBatchInsertBuilder) Add(transaction ingest.LedgerTransaction, sequence uint32) error {
	row, err := transactionToRow(transaction, sequence, i.encodingBuffer)
	if err != nil {
		return err
	}

	return i.builder.RowStruct(row)
}

func (i *transactionBatchInsertBuilder) Exec(ctx context.Context, session db.SessionInterface) error {
	return i.builder.Exec(ctx, session, i.table)
}

func signatures(xdrSignatures []xdr.DecoratedSignature) pq.StringArray {
	signatures := make([]string, len(xdrSignatures))
	for i, sig := range xdrSignatures {
		signatures[i] = base64.StdEncoding.EncodeToString(sig.Signature)
	}
	return signatures
}

func memoType(transaction ingest.LedgerTransaction) string {
	switch transaction.Envelope.Memo().Type {
	case xdr.MemoTypeMemoNone:
		return "none"
	case xdr.MemoTypeMemoText:
		return "text"
	case xdr.MemoTypeMemoId:
		return "id"
	case xdr.MemoTypeMemoHash:
		return "hash"
	case xdr.MemoTypeMemoReturn:
		return "return"
	default:
		panic(fmt.Errorf("invalid memo type: %v", transaction.Envelope.Memo().Type))
	}
}

func memo(transaction ingest.LedgerTransaction) null.String {
	var (
		value string
		valid bool
	)
	memo := transaction.Envelope.Memo()
	switch memo.Type {
	case xdr.MemoTypeMemoNone:
		value, valid = "", false
	case xdr.MemoTypeMemoText:
		scrubbed := utf8.Scrub(memo.MustText())
		notnull := strings.Join(strings.Split(scrubbed, "\x00"), "")
		value, valid = notnull, true
	case xdr.MemoTypeMemoId:
		value, valid = fmt.Sprintf("%d", memo.MustId()), true
	case xdr.MemoTypeMemoHash:
		hash := memo.MustHash()
		value, valid =
			base64.StdEncoding.EncodeToString(hash[:]),
			true
	case xdr.MemoTypeMemoReturn:
		hash := memo.MustRetHash()
		value, valid =
			base64.StdEncoding.EncodeToString(hash[:]),
			true
	default:
		panic(fmt.Errorf("invalid memo type: %v", memo.Type))
	}

	return null.NewString(value, valid)
}

type TransactionWithoutLedger struct {
	TotalOrderID
	TransactionHash             string         `db:"transaction_hash"`
	LedgerSequence              int32          `db:"ledger_sequence"`
	ApplicationOrder            int32          `db:"application_order"`
	Account                     string         `db:"account"`
	AccountMuxed                null.String    `db:"account_muxed"`
	AccountSequence             int64          `db:"account_sequence"`
	MaxFee                      int64          `db:"max_fee"`
	FeeCharged                  int64          `db:"fee_charged"`
	OperationCount              int32          `db:"operation_count"`
	TxEnvelope                  string         `db:"tx_envelope"`
	TxResult                    string         `db:"tx_result"`
	TxMeta                      string         `db:"tx_meta"`
	TxFeeMeta                   string         `db:"tx_fee_meta"`
	Signatures                  pq.StringArray `db:"signatures"`
	MemoType                    string         `db:"memo_type"`
	Memo                        null.String    `db:"memo"`
	TimeBounds                  TimeBounds     `db:"time_bounds"`
	LedgerBounds                LedgerBounds   `db:"ledger_bounds"`
	MinAccountSequence          null.Int       `db:"min_account_sequence"`
	MinAccountSequenceAge       null.String    `db:"min_account_sequence_age"`
	MinAccountSequenceLedgerGap null.Int       `db:"min_account_sequence_ledger_gap"`
	ExtraSigners                pq.StringArray `db:"extra_signers"`
	CreatedAt                   time.Time      `db:"created_at"`
	UpdatedAt                   time.Time      `db:"updated_at"`
	Successful                  bool           `db:"successful"`
	FeeAccount                  null.String    `db:"fee_account"`
	FeeAccountMuxed             null.String    `db:"fee_account_muxed"`
	InnerTransactionHash        null.String    `db:"inner_transaction_hash"`
	NewMaxFee                   null.Int       `db:"new_max_fee"`
	InnerSignatures             pq.StringArray `db:"inner_signatures"`
}

func transactionToRow(transaction ingest.LedgerTransaction, sequence uint32, encodingBuffer *xdr.EncodingBuffer) (TransactionWithoutLedger, error) {
	envelopeBase64, err := encodingBuffer.MarshalBase64(transaction.Envelope)
	if err != nil {
		return TransactionWithoutLedger{}, err
	}
	resultBase64, err := encodingBuffer.MarshalBase64(&transaction.Result.Result)
	if err != nil {
		return TransactionWithoutLedger{}, err
	}
	metaBase64, err := encodingBuffer.MarshalBase64(transaction.UnsafeMeta)
	if err != nil {
		return TransactionWithoutLedger{}, err
	}
	feeMetaBase64, err := encodingBuffer.MarshalBase64(transaction.FeeChanges)
	if err != nil {
		return TransactionWithoutLedger{}, err
	}

	source := transaction.Envelope.SourceAccount()
	account := source.ToAccountId()
	var accountMuxed null.String
	if source.Type == xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
		accountMuxed = null.StringFrom(source.Address())
	}

	t := TransactionWithoutLedger{
		TransactionHash:             hex.EncodeToString(transaction.Result.TransactionHash[:]),
		LedgerSequence:              int32(sequence),
		ApplicationOrder:            int32(transaction.Index),
		Account:                     account.Address(),
		AccountMuxed:                accountMuxed,
		AccountSequence:             transaction.Envelope.SeqNum(),
		MaxFee:                      int64(transaction.Envelope.Fee()),
		FeeCharged:                  int64(transaction.Result.Result.FeeCharged),
		OperationCount:              int32(len(transaction.Envelope.Operations())),
		TxEnvelope:                  envelopeBase64,
		TxResult:                    resultBase64,
		TxMeta:                      metaBase64,
		TxFeeMeta:                   feeMetaBase64,
		TimeBounds:                  formatTimeBounds(transaction.Envelope.TimeBounds()),
		LedgerBounds:                formatLedgerBounds(transaction.Envelope.LedgerBounds()),
		MinAccountSequence:          formatMinSequenceNumber(transaction.Envelope.MinSeqNum()),
		MinAccountSequenceAge:       formatDuration(transaction.Envelope.MinSeqAge()),
		MinAccountSequenceLedgerGap: formatUint32(transaction.Envelope.MinSeqLedgerGap()),
		ExtraSigners:                formatSigners(transaction.Envelope.ExtraSigners()),
		MemoType:                    memoType(transaction),
		Memo:                        memo(transaction),
		CreatedAt:                   time.Now().UTC(),
		UpdatedAt:                   time.Now().UTC(),
		Successful:                  transaction.Result.Successful(),
	}
	t.TotalOrderID.ID = toid.New(int32(sequence), int32(transaction.Index), 0).ToInt64()

	if transaction.Envelope.IsFeeBump() {
		innerHash := transaction.Result.InnerHash()
		t.InnerTransactionHash = null.StringFrom(hex.EncodeToString(innerHash[:]))
		feeBumpAccount := transaction.Envelope.FeeBumpAccount()
		feeAccount := feeBumpAccount.ToAccountId()
		var feeAccountMuxed null.String
		if feeBumpAccount.Type == xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
			feeAccountMuxed = null.StringFrom(feeBumpAccount.Address())
		}
		t.FeeAccount = null.StringFrom(feeAccount.Address())
		t.FeeAccountMuxed = feeAccountMuxed
		t.NewMaxFee = null.IntFrom(transaction.Envelope.FeeBumpFee())
		t.InnerSignatures = signatures(transaction.Envelope.Signatures())
		t.Signatures = signatures(transaction.Envelope.FeeBumpSignatures())
	} else {
		t.InnerTransactionHash = null.StringFromPtr(nil)
		t.FeeAccount = null.StringFromPtr(nil)
		t.FeeAccountMuxed = null.StringFromPtr(nil)
		t.NewMaxFee = null.IntFromPtr(nil)
		t.InnerSignatures = nil
		t.Signatures = signatures(transaction.Envelope.Signatures())
	}

	return t, nil
}

func formatMinSequenceNumber(minSeqNum *int64) null.Int {
	if minSeqNum == nil {
		return null.Int{}
	}
	return null.IntFrom(int64(*minSeqNum))
}

func formatDuration(d *xdr.Duration) null.String {
	if d == nil {
		return null.String{}
	}
	return null.StringFrom(fmt.Sprint(uint64(*d)))
}

func formatUint32(u *xdr.Uint32) null.Int {
	if u == nil {
		return null.Int{}
	}
	return null.IntFrom(int64(*u))
}

func formatSigners(s []xdr.SignerKey) pq.StringArray {
	if s == nil {
		return nil
	}

	signers := make([]string, len(s))
	for i, key := range s {
		signers[i] = key.Address()
	}
	return signers
}

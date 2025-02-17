//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"testing"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type LedgersProcessorTestSuiteLedger struct {
	suite.Suite
	processor              *LedgersProcessor
	mockSession            *db.MockSession
	mockBatchInsertBuilder *history.MockLedgersBatchInsertBuilder
	header                 xdr.LedgerHeaderHistoryEntry
	successCount           int
	failedCount            int
	opCount                int
	ingestVersion          int
	txs                    []ingest.LedgerTransaction
	txSetOpCount           int
}

func TestLedgersProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(LedgersProcessorTestSuiteLedger))
}

func createTransaction(successful bool, numOps int, metaVer int32) ingest.LedgerTransaction {
	code := xdr.TransactionResultCodeTxSuccess
	if !successful {
		code = xdr.TransactionResultCodeTxFailed
	}

	operations := []xdr.Operation{}
	op := xdr.BumpSequenceOp{BumpTo: 30000}
	for i := 0; i < numOps; i++ {
		operations = append(operations, xdr.Operation{
			Body: xdr.OperationBody{
				Type:           xdr.OperationTypeBumpSequence,
				BumpSequenceOp: &op,
			},
		})
	}
	sourceAID := xdr.MustAddress("GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY")

	var txMeta xdr.TransactionMeta
	switch metaVer {
	case 3:
		txMeta = xdr.TransactionMeta{
			V: 3,
			V3: &xdr.TransactionMetaV3{
				Operations: make([]xdr.OperationMeta, numOps, numOps),
			},
		}
	case 2:
		txMeta = xdr.TransactionMeta{
			V: 2,
			V2: &xdr.TransactionMetaV2{
				Operations: make([]xdr.OperationMeta, numOps, numOps),
			},
		}
	case 1:
		txMeta = xdr.TransactionMeta{
			V: 1,
			V1: &xdr.TransactionMetaV1{
				Operations: make([]xdr.OperationMeta, numOps, numOps),
			},
		}
	}

	return ingest.LedgerTransaction{
		Result: xdr.TransactionResultPair{
			TransactionHash: xdr.Hash{},
			Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{
					Code:            code,
					InnerResultPair: &xdr.InnerTransactionResultPair{},
					Results:         &[]xdr.OperationResult{},
				},
			},
		},
		Envelope: xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx: xdr.Transaction{
					SourceAccount: sourceAID.ToMuxedAccount(),
					Operations:    operations,
				},
			},
		},
		UnsafeMeta: txMeta,
	}
}

func (s *LedgersProcessorTestSuiteLedger) SetupTest() {
	s.mockBatchInsertBuilder = &history.MockLedgersBatchInsertBuilder{}
	s.ingestVersion = 100
	s.header = xdr.LedgerHeaderHistoryEntry{
		Header: xdr.LedgerHeader{
			LedgerSeq: xdr.Uint32(20),
		},
	}

	s.processor = NewLedgerProcessor(
		s.mockBatchInsertBuilder,
		s.ingestVersion,
	)

	s.txs = []ingest.LedgerTransaction{
		createTransaction(true, 1, 1),
		createTransaction(false, 3, 2),
		createTransaction(true, 4, 3),
	}

	s.successCount = 2
	s.failedCount = 1
	s.opCount = 5
	s.txSetOpCount = 8
}

func (s *LedgersProcessorTestSuiteLedger) TearDownTest() {
	s.mockBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *LedgersProcessorTestSuiteLedger) TestInsertLedgerSucceeds() {
	ctx := context.Background()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: s.header,
			},
		}, tx)
		s.Assert().NoError(err)
	}

	nextHeader := xdr.LedgerHeaderHistoryEntry{
		Header: xdr.LedgerHeader{
			LedgerSeq: xdr.Uint32(21),
		},
	}
	nextTransactions := []ingest.LedgerTransaction{
		createTransaction(true, 1, 2),
		createTransaction(false, 2, 2),
	}
	for _, tx := range nextTransactions {
		err := s.processor.ProcessTransaction(xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: nextHeader,
			},
		}, tx)
		s.Assert().NoError(err)
	}

	s.mockBatchInsertBuilder.On(
		"Add",
		s.header,
		s.successCount,
		s.failedCount,
		s.opCount,
		s.txSetOpCount,
		s.ingestVersion,
	).Return(nil)

	s.mockBatchInsertBuilder.On(
		"Add",
		nextHeader,
		1,
		1,
		1,
		3,
		s.ingestVersion,
	).Return(nil)

	s.mockBatchInsertBuilder.On(
		"Exec",
		ctx,
		s.mockSession,
	).Return(nil)

	err := s.processor.Flush(ctx, s.mockSession)
	s.Assert().NoError(err)
}

func (s *LedgersProcessorTestSuiteLedger) TestInsertLedgerReturnsError() {
	s.mockBatchInsertBuilder.On(
		"Add",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(errors.New("transient error"))

	err := s.processor.ProcessTransaction(xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: s.header,
		},
	}, s.txs[0])
	s.Assert().NoError(err)

	s.Assert().EqualError(s.processor.Flush(
		context.Background(), s.mockSession),
		"error adding ledger 20 to batch: transient error",
	)
}

func (s *LedgersProcessorTestSuiteLedger) TestExecFails() {
	ctx := context.Background()
	s.mockBatchInsertBuilder.On(
		"Add",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	s.mockBatchInsertBuilder.On(
		"Exec",
		ctx,
		s.mockSession,
	).Return(errors.New("transient exec error"))

	err := s.processor.ProcessTransaction(xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: s.header,
		},
	}, s.txs[0])
	s.Assert().NoError(err)

	s.Assert().EqualError(s.processor.Flush(
		context.Background(), s.mockSession),
		"error flushing ledgers 20 - 20: transient exec error",
	)
}

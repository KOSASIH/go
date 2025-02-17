//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

type ParticipantsProcessorTestSuiteLedger struct {
	suite.Suite
	ctx                              context.Context
	processor                        *ParticipantsProcessor
	mockSession                      *db.MockSession
	mockBatchInsertBuilder           *history.MockTransactionParticipantsBatchInsertBuilder
	mockOperationsBatchInsertBuilder *history.MockOperationParticipantBatchInsertBuilder
	accountLoader                    *history.AccountLoader

	lcm             xdr.LedgerCloseMeta
	firstTx         ingest.LedgerTransaction
	secondTx        ingest.LedgerTransaction
	thirdTx         ingest.LedgerTransaction
	firstTxID       int64
	secondTxID      int64
	thirdTxID       int64
	addresses       []string
	addressToFuture map[string]history.FutureAccountID
	txs             []ingest.LedgerTransaction
}

func TestParticipantsProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(ParticipantsProcessorTestSuiteLedger))
}

func (s *ParticipantsProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockBatchInsertBuilder = &history.MockTransactionParticipantsBatchInsertBuilder{}
	s.mockOperationsBatchInsertBuilder = &history.MockOperationParticipantBatchInsertBuilder{}
	sequence := uint32(20)
	s.lcm = xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}

	s.addresses = []string{
		"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
		"GAXI33UCLQTCKM2NMRBS7XYBR535LLEVAHL5YBN4FTCB4HZHT7ZA5CVK",
		"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
	}

	s.firstTx = createTransaction(true, 1, 2)
	s.firstTx.Index = 1
	aid := xdr.MustAddress(s.addresses[0])
	s.firstTx.Envelope.V1.Tx.SourceAccount = aid.ToMuxedAccount()
	s.firstTxID = toid.New(int32(sequence), 1, 0).ToInt64()

	s.secondTx = createTransaction(true, 1, 2)
	s.secondTx.Index = 2
	s.secondTx.Envelope.Operations()[0].Body = xdr.OperationBody{
		Type: xdr.OperationTypeCreateAccount,
		CreateAccountOp: &xdr.CreateAccountOp{
			Destination: xdr.MustAddress(s.addresses[1]),
		},
	}
	aid = xdr.MustAddress(s.addresses[2])
	s.secondTx.Envelope.V1.Tx.SourceAccount = aid.ToMuxedAccount()
	s.secondTxID = toid.New(int32(sequence), 2, 0).ToInt64()

	s.thirdTx = createTransaction(true, 1, 2)
	s.thirdTx.Index = 3
	aid = xdr.MustAddress(s.addresses[0])
	s.thirdTx.Envelope.V1.Tx.SourceAccount = aid.ToMuxedAccount()
	s.thirdTxID = toid.New(int32(sequence), 3, 0).ToInt64()

	s.accountLoader = history.NewAccountLoader(history.ConcurrentInserts)
	s.addressToFuture = map[string]history.FutureAccountID{}
	for _, address := range s.addresses {
		s.addressToFuture[address] = s.accountLoader.GetFuture(address)
	}

	s.processor = NewParticipantsProcessor(
		s.accountLoader,
		s.mockBatchInsertBuilder,
		s.mockOperationsBatchInsertBuilder,
		networkPassphrase,
	)

	s.txs = []ingest.LedgerTransaction{
		s.firstTx,
		s.secondTx,
		s.thirdTx,
	}
}

func (s *ParticipantsProcessorTestSuiteLedger) TearDownTest() {
	s.mockBatchInsertBuilder.AssertExpectations(s.T())
	s.mockOperationsBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *ParticipantsProcessorTestSuiteLedger) mockSuccessfulTransactionBatchAdds() {
	s.mockBatchInsertBuilder.On(
		"Add", s.firstTxID, s.addressToFuture[s.addresses[0]],
	).Return(nil).Once()

	s.mockBatchInsertBuilder.On(
		"Add", s.secondTxID, s.addressToFuture[s.addresses[1]],
	).Return(nil).Once()
	s.mockBatchInsertBuilder.On(
		"Add", s.secondTxID, s.addressToFuture[s.addresses[2]],
	).Return(nil).Once()

	s.mockBatchInsertBuilder.On(
		"Add", s.thirdTxID, s.addressToFuture[s.addresses[0]],
	).Return(nil).Once()
}

func (s *ParticipantsProcessorTestSuiteLedger) mockSuccessfulOperationBatchAdds() {
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.firstTxID+1, s.addressToFuture[s.addresses[0]],
	).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.secondTxID+1, s.addressToFuture[s.addresses[1]],
	).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.secondTxID+1, s.addressToFuture[s.addresses[2]],
	).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.thirdTxID+1, s.addressToFuture[s.addresses[0]],
	).Return(nil).Once()
}
func (s *ParticipantsProcessorTestSuiteLedger) TestEmptyParticipants() {
	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	err := s.processor.Flush(s.ctx, s.mockSession)
	s.Assert().NoError(err)
}

func (s *ParticipantsProcessorTestSuiteLedger) TestFeeBumptransaction() {
	feeBumpTx := createTransaction(true, 0, 2)
	feeBumpTx.Index = 1
	aid := xdr.MustAddress(s.addresses[0])
	feeBumpTx.Envelope.V1.Tx.SourceAccount = aid.ToMuxedAccount()
	aid = xdr.MustAddress(s.addresses[1])
	feeBumpTx.Envelope.FeeBump = &xdr.FeeBumpTransactionEnvelope{
		Tx: xdr.FeeBumpTransaction{
			FeeSource: aid.ToMuxedAccount(),
			Fee:       100,
			InnerTx: xdr.FeeBumpTransactionInnerTx{
				Type: xdr.EnvelopeTypeEnvelopeTypeTx,
				V1:   feeBumpTx.Envelope.V1,
			},
		},
	}
	feeBumpTx.Envelope.V1 = nil
	feeBumpTx.Envelope.Type = xdr.EnvelopeTypeEnvelopeTypeTxFeeBump
	feeBumpTx.Result.Result.Result.Code = xdr.TransactionResultCodeTxFeeBumpInnerSuccess
	feeBumpTx.Result.Result.Result.InnerResultPair = &xdr.InnerTransactionResultPair{
		Result: xdr.InnerTransactionResult{
			Result: xdr.InnerTransactionResultResult{
				Code:    xdr.TransactionResultCodeTxSuccess,
				Results: &[]xdr.OperationResult{},
			},
		},
	}
	feeBumpTx.Result.Result.Result.Results = nil
	feeBumpTxID := toid.New(20, 1, 0).ToInt64()

	s.mockBatchInsertBuilder.On(
		"Add", feeBumpTxID, s.addressToFuture[s.addresses[0]],
	).Return(nil).Once()
	s.mockBatchInsertBuilder.On(
		"Add", feeBumpTxID, s.addressToFuture[s.addresses[1]],
	).Return(nil).Once()

	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	s.Assert().NoError(s.processor.ProcessTransaction(s.lcm, feeBumpTx))
	s.Assert().NoError(s.processor.Flush(s.ctx, s.mockSession))
}

func (s *ParticipantsProcessorTestSuiteLedger) TestIngestParticipantsSucceeds() {
	s.mockSuccessfulTransactionBatchAdds()
	s.mockSuccessfulOperationBatchAdds()

	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(s.lcm, tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Flush(s.ctx, s.mockSession)
	s.Assert().NoError(err)
}

func (s *ParticipantsProcessorTestSuiteLedger) TestBatchAddFails() {
	s.mockBatchInsertBuilder.On(
		"Add", s.firstTxID, s.addressToFuture[s.addresses[0]],
	).Return(errors.New("transient error")).Once()

	err := s.processor.ProcessTransaction(s.lcm, s.txs[0])
	s.Assert().EqualError(err, "transient error")
}

func (s *ParticipantsProcessorTestSuiteLedger) TestOperationParticipantsBatchAddFails() {
	s.mockBatchInsertBuilder.On(
		"Add", s.firstTxID, s.addressToFuture[s.addresses[0]],
	).Return(nil).Once()

	s.mockOperationsBatchInsertBuilder.On(
		"Add", s.firstTxID+1, s.addressToFuture[s.addresses[0]],
	).Return(errors.New("transient error")).Once()

	err := s.processor.ProcessTransaction(s.lcm, s.txs[0])
	s.Assert().EqualError(err, "transient error")
}

func (s *ParticipantsProcessorTestSuiteLedger) TestBatchAddExecFails() {
	s.mockSuccessfulTransactionBatchAdds()
	s.mockSuccessfulOperationBatchAdds()

	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(errors.New("transient error")).Once()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(s.lcm, tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Flush(s.ctx, s.mockSession)
	s.Assert().EqualError(err, "Could not flush transaction participants to db: transient error")
}

func (s *ParticipantsProcessorTestSuiteLedger) TestOperationBatchAddExecFails() {
	s.mockSuccessfulTransactionBatchAdds()
	s.mockSuccessfulOperationBatchAdds()

	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()
	s.mockOperationsBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(errors.New("transient error")).Once()

	for _, tx := range s.txs {
		err := s.processor.ProcessTransaction(s.lcm, tx)
		s.Assert().NoError(err)
	}
	err := s.processor.Flush(s.ctx, s.mockSession)
	s.Assert().EqualError(err, "Could not flush operation participants to db: transient error")
}

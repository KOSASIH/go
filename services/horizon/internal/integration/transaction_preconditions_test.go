package integration

import (
	"bytes"
	"encoding/base64"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sdk "github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

func TestTransactionPreconditionsTimeBounds(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	if itest.GetEffectiveProtocolVersion() < 19 {
		t.Skip("Can't run with protocol < 19")
	}
	master := itest.Master()
	masterAccount := itest.MasterAccount()
	currentAccountSeq, err := masterAccount.GetSequenceNumber()
	tt.NoError(err)
	txParams := buildTXParams(master, masterAccount, currentAccountSeq+1)

	// this errors because the min time is > current tx submit time
	txParams.Preconditions.TimeBounds.MinTime = time.Now().Unix() + 3600
	txParams.Preconditions.TimeBounds.MaxTime = time.Now().Unix() + 7200
	_, err = itest.SubmitTransaction(master, txParams)
	tt.Error(err)

	// this errors because the max time is < current tx submit time
	txParams.Preconditions.TimeBounds.MinTime = 0
	txParams.Preconditions.TimeBounds.MaxTime = time.Now().Unix() - 3600
	_, err = itest.SubmitTransaction(master, txParams)
	tt.Error(err)

	// Now the transaction should be submitted without problems, min < current tx submit time < max
	txParams.Preconditions.TimeBounds.MinTime = time.Now().Unix() - 3600
	txParams.Preconditions.TimeBounds.MaxTime = time.Now().Unix() + 3600
	tx := itest.MustSubmitTransaction(master, txParams)

	txHistory, err := itest.Client().TransactionDetail(tx.Hash)
	assert.NoError(t, err)

	historyMinTime, err := strconv.ParseInt(txHistory.Preconditions.TimeBounds.MinTime, 10, 64)
	assert.NoError(t, err)
	historyMaxTime, err := strconv.ParseInt(txHistory.Preconditions.TimeBounds.MaxTime, 10, 64)
	assert.NoError(t, err)

	assert.Equal(t, historyMinTime, txParams.Preconditions.TimeBounds.MinTime)
	assert.Equal(t, historyMaxTime, txParams.Preconditions.TimeBounds.MaxTime)
}

func TestTransactionPreconditionsExtraSigners(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	if itest.GetEffectiveProtocolVersion() < 19 {
		t.Skip("Can't run with protocol < 19")
	}
	master := itest.Master()
	masterAccount := itest.MasterAccount()

	// create a new signed payload signer
	addtlSigners, addtlAccounts := itest.CreateAccounts(1, "1000")

	// build a tx with seqnum based on master.seqNum+1 as source account
	latestMasterAccount := itest.MustGetAccount(master)
	currentAccountSeq, err := latestMasterAccount.GetSequenceNumber()
	tt.NoError(err)
	txParams := buildTXParams(master, masterAccount, currentAccountSeq+1)

	// this errors because the tx preconditions require extra signer that
	// didn't sign this tx
	txParams.Preconditions.ExtraSigners = []string{addtlAccounts[0].GetAccountID()}
	_, err = itest.SubmitMultiSigTransaction([]*keypair.Full{master}, txParams)
	tt.Error(err)

	// Now the transaction should be submitted without problems, the extra signer specified
	// has also signed this transaction.
	txParams.Preconditions.ExtraSigners = []string{addtlAccounts[0].GetAccountID()}
	tx, err := itest.SubmitMultiSigTransaction([]*keypair.Full{master, addtlSigners[0]}, txParams)
	tt.NoError(err)

	txHistory, err := itest.Client().TransactionDetail(tx.Hash)
	assert.NoError(t, err)
	assert.ElementsMatch(t, txHistory.Preconditions.ExtraSigners, txParams.Preconditions.ExtraSigners)
}

func TestTransactionPreconditionsLedgerBounds(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	if itest.GetEffectiveProtocolVersion() < 19 {
		t.Skip("Can't run with protocol < 19")
	}
	master := itest.Master()
	masterAccount := itest.MasterAccount()
	currentAccountSeq, err := masterAccount.GetSequenceNumber()
	tt.NoError(err)

	// build a tx with seqnum based on master.seqNum+1 as source account
	txParams := buildTXParams(master, masterAccount, currentAccountSeq+1)

	coreSequence, err := itest.GetCurrentCoreLedgerSequence()
	tt.NoError(err)

	// this txsub will error because the tx preconditions require a min ledger sequence number that
	// hasn't been realized yet on the network ledger
	txParams.Preconditions.LedgerBounds = &txnbuild.LedgerBounds{
		MinLedger: uint32(coreSequence + 1000),
		MaxLedger: uint32(coreSequence + 2000),
	}
	_, err = itest.SubmitMultiSigTransaction([]*keypair.Full{master}, txParams)
	tt.Error(err)

	txParams.Preconditions.LedgerBounds = &txnbuild.LedgerBounds{
		MinLedger: uint32(coreSequence - 1),
		MaxLedger: uint32(coreSequence + 2000),
	}
	// Now the transaction should be submitted without problems, the latest network
	// ledger sequence should be above the min and below the max of preconditions ledgerbounds
	tx, err := itest.SubmitMultiSigTransaction([]*keypair.Full{master}, txParams)
	tt.NoError(err)

	//verify roundtrip to network and back through the horizon api returns same precondition values
	txHistory, err := itest.Client().TransactionDetail(tx.Hash)
	assert.NoError(t, err)
	assert.Equal(t, txHistory.Preconditions.LedgerBounds.MaxLedger, txParams.Preconditions.LedgerBounds.MaxLedger)
	assert.Equal(t, txHistory.Preconditions.LedgerBounds.MinLedger, txParams.Preconditions.LedgerBounds.MinLedger)
}

func TestTransactionPreconditionsMinSequenceNumberAge(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	if itest.GetEffectiveProtocolVersion() < 19 {
		t.Skip("Can't run with protocol < 19")
	}
	submitPhonyOp(itest) // upgrades master account to v3

	master := itest.Master()
	masterAccount := itest.MasterAccount()
	currentAccountSeq, err := masterAccount.GetSequenceNumber()
	tt.NoError(err)
	latestMasterAccount := itest.MustGetAccount(master)

	ledgerRequest := sdk.LedgerRequest{Order: sdk.OrderDesc, Limit: 1}
	ledgers, err := itest.Client().Ledgers(ledgerRequest)
	tt.NoError(err)
	tt.Len(ledgers.Embedded.Records, 1)

	// gather up the current sequence times
	signedAcctSeqTime, err := strconv.ParseInt(latestMasterAccount.SequenceTime, 10, 64)
	tt.NoError(err)
	tt.GreaterOrEqual(signedAcctSeqTime, int64(0))
	acctSeqTime := uint64(signedAcctSeqTime)
	networkSeqTime := uint64(ledgers.Embedded.Records[0].ClosedAt.UTC().Unix())
	tt.GreaterOrEqual(networkSeqTime, acctSeqTime)

	// build a tx with seqnum based on master.seqNum+1 as source account
	txParams := buildTXParams(master, masterAccount, currentAccountSeq+1)

	// This txsub will error because the tx preconditions require a min sequence
	// age which has been set 10000 seconds greater than the current difference
	// between network ledger sequence time and account sequnece time.
	txParams.Preconditions.MinSequenceNumberAge = networkSeqTime - acctSeqTime + 10000
	tx, err := itest.SubmitMultiSigTransaction([]*keypair.Full{master}, txParams)
	tt.Error(err)

	// Now the transaction should be submitted without problems, the min
	// sequence age is set to be 1s more than the current difference between
	// network time and account sequence time.
	time.Sleep(time.Second)
	txParams.Preconditions.MinSequenceNumberAge = 1
	tx, err = itest.SubmitMultiSigTransaction([]*keypair.Full{master}, txParams)
	itest.LogFailedTx(tx, err)

	//verify roundtrip to network and back through the horizon api returns same precondition values
	txHistory, err := itest.Client().TransactionDetail(tx.Hash)
	tt.NoError(err)

	expected := txParams.Preconditions.MinSequenceNumberAge
	actual, err := strconv.ParseUint(txHistory.Preconditions.MinAccountSequenceAge, 10, 64)
	tt.NoError(err)
	tt.Equal(expected, actual)
}

func TestTransactionPreconditionsMinSequenceNumberLedgerGap(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	if itest.GetEffectiveProtocolVersion() < 19 {
		t.Skip("Can't run with protocol < 19")
	}
	master := itest.Master()
	masterAccount := itest.MasterAccount()
	currentAccountSeq, err := masterAccount.GetSequenceNumber()
	tt.NoError(err)

	// gather up the current sequence number
	networkLedger, err := itest.GetCurrentCoreLedgerSequence()
	tt.NoError(err)

	// build a tx with seqnum based on master.seqNum+1 as source account
	txParams := buildTXParams(master, masterAccount, currentAccountSeq+1)

	// this txsub will error because the tx preconditions require a min sequence gap
	// which has been set 10000 sequence numbers greater than the current difference between
	// network ledger sequence and account sequnece numbers
	txParams.Preconditions.MinSequenceNumberLedgerGap = uint32(int64(networkLedger) - currentAccountSeq + 10000)
	_, err = itest.SubmitMultiSigTransaction([]*keypair.Full{master}, txParams)
	tt.Error(err)

	txParams.Preconditions.MinSequenceNumberLedgerGap = uint32(int64(networkLedger) - currentAccountSeq - 1)
	// Now the transaction should be submitted without problems, the min sequence gap
	// is set to be one less then the current difference between network sequence and account sequence number.
	tx, err := itest.SubmitMultiSigTransaction([]*keypair.Full{master}, txParams)
	tt.NoError(err)

	//verify roundtrip to network and back through the horizon api returns same precondition values
	txHistory, err := itest.Client().TransactionDetail(tx.Hash)
	assert.NoError(t, err)
	assert.Equal(t, txHistory.Preconditions.MinAccountSequenceLedgerGap, txParams.Preconditions.MinSequenceNumberLedgerGap)
}

// TestTransactionWithoutPreconditions ensures that Horizon doesn't break when
// we have a PRECOND_NONE type transaction (which is not possible to submit
// through SDKs, but is absolutely still possible).
func TestTransactionWithoutPreconditions(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	if itest.GetEffectiveProtocolVersion() < 19 {
		t.Skip("Can't run with protocol < 19")
	}

	master := itest.Master()
	masterAccount := itest.MasterAccount()
	seqNum, err := masterAccount.GetSequenceNumber()
	tt.NoError(err)

	account := xdr.MuxedAccount{}
	tt.NoError(account.SetEd25519Address(master.Address()))

	payment := txnbuild.Payment{ // dummy op
		Destination: master.Address(),
		Amount:      "1000",
		Asset:       txnbuild.NativeAsset{},
	}
	paymentOp, err := payment.BuildXDR()
	tt.NoError(err)

	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				SourceAccount: account,
				Fee:           xdr.Uint32(1000),
				SeqNum:        xdr.SequenceNumber(seqNum + 1),
				Operations:    []xdr.Operation{paymentOp},
				Cond: xdr.Preconditions{
					Type: xdr.PreconditionTypePrecondNone,
				},
			},
			Signatures: nil,
		},
	}

	// Taken from txnbuild.concatSignatures
	h, err := network.HashTransactionInEnvelope(envelope,
		itest.Config().NetworkPassphrase)
	tt.NoError(err)

	sig, err := master.SignDecorated(h[:])
	tt.NoError(err)

	// taken from txnbuild.marshallBinary
	var txBytes bytes.Buffer
	envelope.V1.Signatures = []xdr.DecoratedSignature{sig}
	_, err = xdr.Marshal(&txBytes, envelope)
	tt.NoError(err)
	b64 := base64.StdEncoding.EncodeToString(txBytes.Bytes())

	txResp, err := itest.Client().SubmitTransactionXDR(b64)
	tt.NoError(err)

	txResp2, err := itest.Client().TransactionDetail(txResp.Hash)
	tt.NoError(err)
	tt.Nil(txResp2.Preconditions)
}

func TestTransactionPreconditionsEdgeCases(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	if itest.GetEffectiveProtocolVersion() < 19 {
		t.Skip("Can't run with protocol < 19")
	}
	master := itest.Master()
	masterAccount := itest.MasterAccount()

	maxMinSeq := int64(math.MaxInt64)
	preconditionTests := []txnbuild.Preconditions{
		{LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 1, MaxLedger: 0}},
		{LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 0, MaxLedger: math.MaxUint32}},
		{LedgerBounds: &txnbuild.LedgerBounds{MinLedger: math.MaxUint32, MaxLedger: 1}},
		{
			LedgerBounds: &txnbuild.LedgerBounds{MinLedger: math.MaxUint32, MaxLedger: 1},
			ExtraSigners: []string{},
		},
		{
			MinSequenceNumber:          &maxMinSeq,
			MinSequenceNumberLedgerGap: math.MaxUint32,
			MinSequenceNumberAge:       math.MaxUint64,
			ExtraSigners:               nil,
		},
	}

	for _, precondition := range preconditionTests {
		seqNum, err := masterAccount.IncrementSequenceNumber()
		tt.NoError(err)

		params := buildTXParams(master, masterAccount, seqNum)
		precondition.TimeBounds = txnbuild.NewInfiniteTimeout()
		params.Preconditions = precondition

		// The goal here is not to check for validation or errors or responses,
		// but rather to just make sure the edge case doesn't crash Horizon.
		itest.SubmitTransaction(master, params)
	}
}

func buildTXParams(master *keypair.Full, masterAccount txnbuild.Account, txSequence int64) txnbuild.TransactionParams {
	return txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: masterAccount.GetAccountID(),
			Sequence:  txSequence,
		},
		// Phony operation to run
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{
				Destination: master.Address(),
				Amount:      "10",
				Asset:       txnbuild.NativeAsset{},
			},
		},
		BaseFee: txnbuild.MinBaseFee,
		Memo:    nil,
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewInfiniteTimeout(),
		},
	}
}

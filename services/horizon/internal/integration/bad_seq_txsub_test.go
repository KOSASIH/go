package integration

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/txnbuild"
)

func TestNegativeSequenceTxSubmission(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	master := itest.Master()

	// First, bump the sequence to the maximum value -1
	op := txnbuild.BumpSequence{
		BumpTo: int64(math.MaxInt64) - 1,
	}
	itest.MustSubmitOperations(itest.MasterAccount(), master, &op)

	account := itest.MasterAccount()
	seqnum, err := account.GetSequenceNumber()
	tt.NoError(err)
	tt.Equal(int64(math.MaxInt64)-1, seqnum)

	// Submit a simple payment
	op2 := txnbuild.Payment{
		Destination: master.Address(),
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	txResp := itest.MustSubmitOperations(account, master, &op2)
	tt.Equal(master.Address(), txResp.Account)

	// The transaction should had bumped our sequence to the maximum possible value
	seqnum, err = account.GetSequenceNumber()
	tt.NoError(err)
	tt.Equal(int64(math.MaxInt64), seqnum)

	// Using txnbuild to create another transaction should fail, since it would cause a sequence number overflow
	txResp, err = itest.SubmitOperations(account, master, &op2)
	tt.Error(err)
	tt.Contains(err.Error(), "sequence cannot be increased, it already reached MaxInt64")

	// We can enforce a negative sequence without errors by setting IncrementSequenceNum=false
	account = &txnbuild.SimpleAccount{
		AccountID: account.GetAccountID(),
		Sequence:  math.MinInt64,
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        account,
		Operations:           []txnbuild.Operation{&op2},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		IncrementSequenceNum: false,
	}
	tx, err := txnbuild.NewTransaction(txParams)
	tt.NoError(err)
	tx, err = tx.Sign(itest.Config().NetworkPassphrase, master)
	tt.NoError(err)
	txResp, err = itest.Client().SubmitTransaction(tx)
	tt.Error(err)
	clientErr, ok := err.(*horizonclient.Error)
	tt.True(ok)
	codes, err := clientErr.ResultCodes()
	tt.NoError(err)
	tt.Equal("tx_bad_seq", codes.TransactionCode)

}

func TestBadSeqTxSubmission(t *testing.T) {
	tt := assert.New(t)
	itest := integration.NewTest(t, integration.Config{})
	master := itest.Master()

	account := itest.MasterAccount()
	seqnum, err := account.GetSequenceNumber()
	tt.NoError(err)

	op2 := txnbuild.Payment{
		Destination: master.Address(),
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	// Submit a simple payment tx, but with a gapped sequence
	// that is intentionally set more than one ahead of current account seq
	// this should trigger a tx_bad_seq from core
	account = &txnbuild.SimpleAccount{
		AccountID: account.GetAccountID(),
		Sequence:  seqnum + 10,
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        account,
		Operations:           []txnbuild.Operation{&op2},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		IncrementSequenceNum: false,
	}
	tx, err := txnbuild.NewTransaction(txParams)
	tt.NoError(err)
	tx, err = tx.Sign(itest.Config().NetworkPassphrase, master)
	tt.NoError(err)
	_, err = itest.Client().SubmitTransaction(tx)
	tt.Error(err)
	clientErr, ok := err.(*horizonclient.Error)
	tt.True(ok)
	codes, err := clientErr.ResultCodes()
	tt.NoError(err)
	tt.Equal("tx_bad_seq", codes.TransactionCode)
}

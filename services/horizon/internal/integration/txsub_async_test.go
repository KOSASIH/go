package integration

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
)

func getTransaction(client *horizonclient.Client, hash string) error {
	for i := 0; i < 60; i++ {
		_, err := client.TransactionDetail(hash)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		return nil
	}
	return errors.New("transaction not found")
}

func TestAsyncTxSub_SuccessfulSubmission(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})
	master := itest.Master()
	masterAccount := itest.MasterAccount()

	txParams := txnbuild.TransactionParams{
		BaseFee:              txnbuild.MinBaseFee,
		SourceAccount:        masterAccount,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{
				Destination: master.Address(),
				Amount:      "10",
				Asset:       txnbuild.NativeAsset{},
			},
		},
		Preconditions: txnbuild.Preconditions{
			TimeBounds:   txnbuild.NewInfiniteTimeout(),
			LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 0, MaxLedger: 100},
		},
	}

	txResp, err := itest.AsyncSubmitTransaction(master, txParams)
	assert.NoError(t, err)
	assert.Equal(t, txResp, horizon.AsyncTransactionSubmissionResponse{
		TxStatus: "PENDING",
		Hash:     "6cbb7f714bd08cea7c30cab7818a35c510cbbfc0a6aa06172a1e94146ecf0165",
	})

	err = getTransaction(itest.Client(), txResp.Hash)
	assert.NoError(t, err)
}

func TestAsyncTxSub_SubmissionError(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})
	master := itest.Master()
	masterAccount := itest.MasterAccount()

	txParams := txnbuild.TransactionParams{
		BaseFee:              txnbuild.MinBaseFee,
		SourceAccount:        masterAccount,
		IncrementSequenceNum: false,
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{
				Destination: master.Address(),
				Amount:      "10",
				Asset:       txnbuild.NativeAsset{},
			},
		},
		Preconditions: txnbuild.Preconditions{
			TimeBounds:   txnbuild.NewInfiniteTimeout(),
			LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 0, MaxLedger: 100},
		},
	}

	txResp, err := itest.AsyncSubmitTransaction(master, txParams)
	assert.NoError(t, err)
	assert.Equal(t, txResp, horizon.AsyncTransactionSubmissionResponse{
		ErrorResultXDR:           "AAAAAAAAAGT////7AAAAAA==",
		DeprecatedErrorResultXDR: "AAAAAAAAAGT////7AAAAAA==",
		TxStatus:                 "ERROR",
		Hash:                     "0684df00f20efd5876f1b8d17bc6d3a68d8b85c06bb41e448815ecaa6307a251",
	})
}

func TestAsyncTxSub_SubmissionTryAgainLater(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})
	master := itest.Master()
	masterAccount := itest.MasterAccount()

	txParams := txnbuild.TransactionParams{
		BaseFee:              txnbuild.MinBaseFee,
		SourceAccount:        masterAccount,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{
				Destination: master.Address(),
				Amount:      "10",
				Asset:       txnbuild.NativeAsset{},
			},
		},
		Preconditions: txnbuild.Preconditions{
			TimeBounds:   txnbuild.NewInfiniteTimeout(),
			LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 0, MaxLedger: 100},
		},
	}

	txResp, err := itest.AsyncSubmitTransaction(master, txParams)
	assert.NoError(t, err)
	assert.Equal(t, txResp, horizon.AsyncTransactionSubmissionResponse{
		ErrorResultXDR: "",
		TxStatus:       "PENDING",
		Hash:           "6cbb7f714bd08cea7c30cab7818a35c510cbbfc0a6aa06172a1e94146ecf0165",
	})

	txResp, err = itest.AsyncSubmitTransaction(master, txParams)
	assert.NoError(t, err)
	assert.Equal(t, txResp, horizon.AsyncTransactionSubmissionResponse{
		ErrorResultXDR: "",
		TxStatus:       "TRY_AGAIN_LATER",
		Hash:           "d5eb72a4c1832b89965850fff0bd9bba4b6ca102e7c89099dcaba5e7d7d2e049",
	})
}

func TestAsyncTxSub_TransactionMalformed(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{
		EnableStellarRPC: true,
		HorizonEnvironment: map[string]string{
			"MAX_HTTP_REQUEST_SIZE": "1800",
		},
		QuickExpiration: true,
	})
	master := itest.Master()

	// establish which account will be contract owner, and load it's current seq
	sourceAccount, err := itest.Client().AccountDetail(horizonclient.AccountRequest{
		AccountID: itest.Master().Address(),
	})
	require.NoError(t, err)

	installContractOp := assembleInstallContractCodeOp(t, master.Address(), "soroban_sac_test.wasm")
	preFlightOp, minFee := itest.PreflightHostFunctions(&sourceAccount, *installContractOp)
	txParams := integration.GetBaseTransactionParamsWithFee(&sourceAccount, minFee+txnbuild.MinBaseFee, &preFlightOp)
	_, err = itest.AsyncSubmitTransaction(master, txParams)
	assert.EqualError(
		t, err,
		"horizon error: \"Transaction Malformed\" - check horizon.Error.Problem for more information",
	)
}

func TestAsyncTxSub_GetOpenAPISpecResponse(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})
	res, err := http.Get(itest.AsyncTxSubOpenAPISpecURL())
	assert.NoError(t, err)
	assert.Equal(t, res.StatusCode, 200)

	bytes, err := io.ReadAll(res.Body)
	res.Body.Close()
	assert.NoError(t, err)

	openAPISpec := string(bytes)
	assert.Contains(t, openAPISpec, "openapi: 3.0.0")
}

package integration

import (
	"strconv"
	"testing"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
)

// TestProtocol19Upgrade tests that crossing the upgrade boundary results in the
// correct behavior and no crashes.
func TestProtocol19Upgrade(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{ProtocolVersion: 18})

	master := itest.Master()
	masterAccount := itest.MasterAccount()

	if integration.GetCoreMaxSupportedProtocol() < 19 {
		t.Skip("This test run does not support Protocol 19")
	}

	// Note: These tests are combined to avoid the extra setup/teardown.

	// TestTransactionPreconditionsPremature ensures that submitting
	// transactions that use Protocol 19 features fail correctly.
	t.Run("TestTransactionPreconditionsPremature", func(t *testing.T) {
		tt := assert.New(t)

		// Submit a transaction with extra preconditions set too early.
		txParams := txnbuild.TransactionParams{
			BaseFee:              txnbuild.MinBaseFee,
			SourceAccount:        masterAccount,
			IncrementSequenceNum: true,
			// Phony operation to run
			Operations: []txnbuild.Operation{
				&txnbuild.Payment{
					Destination: masterAccount.GetAccountID(),
					Amount:      "10",
					Asset:       txnbuild.NativeAsset{},
				},
			},
			Preconditions: txnbuild.Preconditions{
				TimeBounds:   txnbuild.NewInfiniteTimeout(),
				LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 0, MaxLedger: 100},
			},
		}
		_, err := itest.SubmitTransaction(master, txParams)

		tt.Error(err)
		if prob := horizonclient.GetError(err); prob != nil {
			if results, ok := prob.Problem.Extras["result_codes"].(map[string]interface{}); ok {
				tt.Equal("tx_not_supported", results["transaction"])
			} else {
				tt.FailNow("result_codes couldn't be parsed: %+v", results)
			}
		} else {
			tt.Error(prob)
		}
	})

	// TestTransactionAccountV3Upgrade ensures that upgrading over the
	// Protocol 19 boundary correctly adds the V3 fields.
	t.Run("TestTransactionAccountV3Upgrade", func(t *testing.T) {
		var account horizon.Account
		tt := assert.New(t)

		// Submit phony operation which should bump the sequence number but not
		// actually track it in the extension.
		tx := submitPhonyOp(itest)
		account = itest.MasterAccountDetails()

		// Check that the account response has V3 fields omitted.
		tt.EqualValues(0, account.SequenceLedger)
		tt.Equal("", account.SequenceTime)

		itest.UpgradeProtocol(19)

		// Submit phony operation which should trigger the new fields.
		tx = submitPhonyOp(itest)

		// Refresh master account and check that the account response has the new
		// AccountV3 fields
		account = itest.MasterAccountDetails()
		tt.Equal(uint32(tx.Ledger), account.SequenceLedger)
		tt.Equal(strconv.FormatInt(tx.LedgerCloseTime.Unix(), 10), account.SequenceTime)
	})
}

func submitPhonyOp(itest *integration.Test) horizon.Transaction {
	master := itest.Master()
	account := itest.MasterAccount()

	return itest.MustSubmitOperations(account, master,
		&txnbuild.Payment{
			Destination: master.Address(),
			Amount:      "10",
			Asset:       txnbuild.NativeAsset{},
		},
	)
}

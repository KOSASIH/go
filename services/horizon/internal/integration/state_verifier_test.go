package integration

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/test/integration"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
)

func TestStateVerifier(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})

	sponsored := keypair.MustRandom()
	sponsoredSource := &txnbuild.SimpleAccount{
		AccountID: sponsored.Address(),
		Sequence:  1,
	}
	signer1 := keypair.MustParseAddress("GAB3CVX6C2KCDZUUS4FIMP5Z2IUDTMKMRKADOFOCNOB437VCPS5DRG3Z")
	signer2 := keypair.MustParseAddress("GBUERII77FW6Z7SPOIMFQQT63PMUQRWTIAARR3QVSXTRULNQSUQVIYRC")
	signer3 := keypair.MustParseAddress("GCNLAKGPBL4H6CQRITHSDTJZ6RLTP3WY2OJZJN4EWKRSNM2A23CV6VD3")

	// The operations below create a sponsorship sandwich, sponsoring an
	// account, its trustlines, offers, data, and claimable balances.
	// Then 3 signers are created with the middle one sponsored.
	master := itest.Master()
	ops := []txnbuild.Operation{
		&txnbuild.BeginSponsoringFutureReserves{
			SponsoredID: sponsored.Address(),
		},
		&txnbuild.CreateAccount{
			Destination: sponsored.Address(),
			Amount:      "100",
		},
		&txnbuild.ChangeTrust{
			SourceAccount: sponsoredSource.AccountID,
			Line:          txnbuild.CreditAsset{Code: "ABCD", Issuer: master.Address()}.MustToChangeTrustAsset(),
			Limit:         txnbuild.MaxTrustlineLimit,
		},
		&txnbuild.ChangeTrust{
			Line: txnbuild.LiquidityPoolShareChangeTrustAsset{
				LiquidityPoolParameters: txnbuild.LiquidityPoolParameters{
					AssetA: txnbuild.NativeAsset{},
					AssetB: txnbuild.CreditAsset{
						Code:   "ABCD",
						Issuer: master.Address(),
					},
					Fee: 30,
				},
			},
			Limit: txnbuild.MaxTrustlineLimit,
		},
		&txnbuild.ManageSellOffer{
			SourceAccount: sponsoredSource.AccountID,
			Selling:       txnbuild.NativeAsset{},
			Buying:        txnbuild.CreditAsset{Code: "ABCD", Issuer: master.Address()},
			Amount:        "3",
			Price:         xdr.Price{N: 1, D: 1},
		},
		&txnbuild.ManageData{
			SourceAccount: sponsoredSource.AccountID,
			Name:          "test",
			Value:         []byte("test"),
		},
		&txnbuild.CreateClaimableBalance{
			SourceAccount: sponsoredSource.AccountID,
			Amount:        "2",
			Asset:         txnbuild.NativeAsset{},
			Destinations: []txnbuild.Claimant{
				txnbuild.NewClaimant(keypair.MustRandom().Address(), nil),
			},
		},
		&txnbuild.EndSponsoringFutureReserves{
			SourceAccount: sponsoredSource.AccountID,
		},
		&txnbuild.SetOptions{
			SourceAccount: sponsoredSource.AccountID,
			Signer: &txnbuild.Signer{
				Address: signer1.Address(),
				Weight:  3,
			},
		},
		&txnbuild.BeginSponsoringFutureReserves{
			SponsoredID: sponsored.Address(),
		},
		&txnbuild.SetOptions{
			SourceAccount: sponsoredSource.AccountID,
			Signer: &txnbuild.Signer{
				Address: signer2.Address(),
				Weight:  3,
			},
		},
		&txnbuild.EndSponsoringFutureReserves{
			SourceAccount: sponsoredSource.AccountID,
		},
		&txnbuild.SetOptions{
			SourceAccount: sponsoredSource.AccountID,
			Signer: &txnbuild.Signer{
				Address: signer3.Address(),
				Weight:  3,
			},
		},
	}
	txResp, err := itest.SubmitMultiSigOperations(itest.MasterAccount(), []*keypair.Full{master, sponsored}, ops...)
	assert.NoError(t, err)
	assert.True(t, txResp.Successful)

	verified := waitForStateVerifications(itest, 1)
	if !verified {
		t.Fatal("State verification not run...")
	}

	// Trigger state rebuild to check if ingesting from history archive works
	session := itest.HorizonIngest().HistoryQ().Clone()
	q := &history.Q{SessionInterface: session}
	err = q.Begin(context.Background())
	assert.NoError(t, err)
	_, err = q.GetLastLedgerIngest(context.Background())
	assert.NoError(t, err)
	err = q.UpdateIngestVersion(context.Background(), 0)
	assert.NoError(t, err)
	err = q.Commit()
	assert.NoError(t, err)

	verified = waitForStateVerifications(itest, 2)
	if !verified {
		t.Fatal("State verification not run...")
	}
}

func waitForStateVerifications(itest *integration.Test, count int) bool {
	t := itest.CurrentTest()
	// Check metrics until state verification run
	for i := 0; i < 120; i++ {
		t.Logf("Checking metrics (%d attempt)\n", i)
		res, err := http.Get(itest.MetricsURL())
		assert.NoError(t, err)

		metricsBytes, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		assert.NoError(t, err)
		metrics := string(metricsBytes)

		stateInvalid := strings.Contains(metrics, "horizon_ingest_state_invalid 1")
		assert.False(t, stateInvalid, "State is invalid!")

		notVerifiedYet := strings.Contains(
			metrics,
			fmt.Sprintf("horizon_ingest_state_verify_duration_seconds_count %d", count-1),
		)
		if notVerifiedYet {
			time.Sleep(time.Second)
			continue
		}

		return true
	}

	return false
}

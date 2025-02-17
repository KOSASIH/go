package filters

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/xdr"
)

func TestAccountFilterAllowsWhenMatch(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	filterConfig := &history.AccountFilterConfig{
		Whitelist:    []string{"GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"},
		Enabled:      true,
		LastModified: 1,
	}

	filter := NewAccountFilter()
	err := filter.RefreshAccountFilter(filterConfig)
	tt.NoError(err)

	isEnabled, result, err := filter.FilterTransaction(ctx, getAccountTestTx(t,
		"GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL",
		"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"))

	tt.NoError(err)
	tt.Equal(isEnabled, true)
	tt.Equal(result, true)
}

func TestAccountFilterAllowsWhenDisabled(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	filterConfig := &history.AccountFilterConfig{
		Whitelist:    []string{"GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"},
		Enabled:      false,
		LastModified: 1,
	}
	filter := NewAccountFilter()
	err := filter.RefreshAccountFilter(filterConfig)
	tt.NoError(err)

	isEnabled, result, err := filter.FilterTransaction(ctx, getAccountTestTx(t,
		"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"))

	tt.NoError(err)

	// there is no match on filter rule, but since filter is disabled, it should allow all
	tt.Equal(isEnabled, false)
	tt.Equal(result, true)
}

func TestAccountFilterAllowsWhenEmptyWhitelist(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	filterConfig := &history.AccountFilterConfig{
		Whitelist:    []string{},
		Enabled:      true,
		LastModified: 1,
	}
	filter := NewAccountFilter()
	err := filter.RefreshAccountFilter(filterConfig)
	tt.NoError(err)

	isEnabled, result, err := filter.FilterTransaction(ctx, getAccountTestTx(t,
		"GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL",
		"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"))

	tt.NoError(err)
	tt.Equal(isEnabled, false)
	tt.Equal(result, true)
}

func TestAccountFilterDoesNotAllowWhenNoMatch(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	filterConfig := &history.AccountFilterConfig{
		Whitelist:    []string{"GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"},
		Enabled:      true,
		LastModified: 1,
	}

	filter := NewAccountFilter()
	err := filter.RefreshAccountFilter(filterConfig)
	tt.NoError(err)

	isEnabled, result, err := filter.FilterTransaction(ctx, getAccountTestTx(t,
		"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"))

	tt.NoError(err)
	tt.Equal(isEnabled, true)
	tt.Equal(result, false)
}

func getAccountTestTx(t *testing.T, accountId string, issuer string) ingest.LedgerTransaction {

	var xdrAssetCode [12]byte
	var xdrIssuer xdr.AccountId
	copy(xdrAssetCode[:], "USDC")
	require.NoError(t, xdrIssuer.SetAddress(issuer))

	return ingest.LedgerTransaction{
		UnsafeMeta: xdr.TransactionMeta{
			V: 1,
			V1: &xdr.TransactionMetaV1{
				Operations: []xdr.OperationMeta{},
			},
		},
		Result: xdr.TransactionResultPair{
			Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{
					Code: xdr.TransactionResultCodeTxSuccess,
				},
			},
		},
		Envelope: xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx: xdr.Transaction{
					SourceAccount: xdr.MustMuxedAddress(accountId),
					Operations: []xdr.Operation{
						{Body: xdr.OperationBody{
							Type: xdr.OperationTypePayment,
							PaymentOp: &xdr.PaymentOp{
								Destination: xdr.MustMuxedAddress(accountId),
								Asset: xdr.Asset{
									Type: xdr.AssetTypeAssetTypeCreditAlphanum12,
									AlphaNum12: &xdr.AlphaNum12{
										AssetCode: xdrAssetCode,
										Issuer:    xdrIssuer,
									},
								},
								Amount: 100,
							},
						}},
					},
				},
			},
		},
	}
}

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

func TestAssetFilterAllowsOnMatch(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	filterConfig := &history.AssetFilterConfig{
		Whitelist:    []string{"USDC:GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"},
		Enabled:      true,
		LastModified: 1,
	}
	filter := NewAssetFilter()
	err := filter.RefreshAssetFilter(filterConfig)
	tt.NoError(err)

	isEnabled, result, err := filter.FilterTransaction(ctx, getAssetTestV1Tx(t, "GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"))
	tt.NoError(err)
	tt.Equal(isEnabled, true)
	tt.Equal(result, true)

	isEnabled, result, err = filter.FilterTransaction(ctx, getAssetTestV0Tx(t, "GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"))
	tt.NoError(err)
	tt.Equal(isEnabled, true)
	tt.Equal(result, true)
}

func TestAssetFilterAllowsWhenEmptyWhitelist(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	filterConfig := &history.AssetFilterConfig{
		Whitelist:    []string{},
		Enabled:      true,
		LastModified: 1,
	}
	filter := NewAssetFilter()
	err := filter.RefreshAssetFilter(filterConfig)
	tt.NoError(err)

	isEnabled, result, err := filter.FilterTransaction(ctx, getAssetTestV1Tx(t, "GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"))
	tt.NoError(err)
	tt.Equal(isEnabled, false)
	tt.Equal(result, true)

	isEnabled, result, err = filter.FilterTransaction(ctx, getAssetTestV0Tx(t, "GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"))
	tt.NoError(err)
	tt.Equal(isEnabled, false)
	tt.Equal(result, true)
}

func TestAssetFilterAllowsWhenDisabled(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	filterConfig := &history.AssetFilterConfig{
		Whitelist:    []string{"USDX:GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"},
		Enabled:      false,
		LastModified: 1,
	}
	filter := NewAssetFilter()
	err := filter.RefreshAssetFilter(filterConfig)
	tt.NoError(err)

	isEnabled, result, err := filter.FilterTransaction(ctx, getAssetTestV1Tx(t, "GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"))
	tt.NoError(err)
	// there was no match on filter rules, but since filter was disabled also, it should allow all
	tt.Equal(isEnabled, false)
	tt.Equal(result, true)
}

func TestAssetFilterDoesNotAllowV1WhenNoMatch(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	filterConfig := &history.AssetFilterConfig{
		Whitelist:    []string{"USDX:GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"},
		Enabled:      true,
		LastModified: 1,
	}

	filter := NewAssetFilter()
	err := filter.RefreshAssetFilter(filterConfig)
	tt.NoError(err)

	isEnabled, result, err := filter.FilterTransaction(ctx, getAssetTestV1Tx(t, "GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"))
	tt.NoError(err)
	tt.Equal(isEnabled, true)
	tt.Equal(result, false)

	isEnabled, result, err = filter.FilterTransaction(ctx, getAssetTestV0Tx(t, "GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"))
	tt.NoError(err)
	tt.Equal(isEnabled, true)
	tt.Equal(result, false)
}

func getAssetTestV1Tx(t *testing.T, issuer string) ingest.LedgerTransaction {
	var xdrAssetCode [12]byte
	var xdrIssuer xdr.AccountId
	copy(xdrAssetCode[:], "USDC")
	require.NoError(t, xdrIssuer.SetAddress(issuer))

	return ingest.LedgerTransaction{
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
					Operations: []xdr.Operation{
						{Body: xdr.OperationBody{
							Type: xdr.OperationTypePayment,
							PaymentOp: &xdr.PaymentOp{
								Destination: xdr.MustMuxedAddress("GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"),
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

func getAssetTestV0Tx(t *testing.T, issuer string) ingest.LedgerTransaction {
	var xdrAssetCode [12]byte
	var xdrIssuer xdr.AccountId
	copy(xdrAssetCode[:], "USDC")
	require.NoError(t, xdrIssuer.SetAddress(issuer))

	return ingest.LedgerTransaction{
		Result: xdr.TransactionResultPair{
			Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{
					Code: xdr.TransactionResultCodeTxSuccess,
				},
			},
		},
		Envelope: xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTxV0,
			V0: &xdr.TransactionV0Envelope{
				Tx: xdr.TransactionV0{
					Operations: []xdr.Operation{
						{Body: xdr.OperationBody{
							Type: xdr.OperationTypePayment,
							PaymentOp: &xdr.PaymentOp{
								Destination: xdr.MustMuxedAddress("GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"),
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

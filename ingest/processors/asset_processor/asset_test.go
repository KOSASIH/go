package asset

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/ingest"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

var genericSourceAccount, _ = xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256([32]byte{}))

var genericBumpOperation = xdr.Operation{
	SourceAccount: &genericSourceAccount,
	Body: xdr.OperationBody{
		Type:           xdr.OperationTypeBumpSequence,
		BumpSequenceOp: &xdr.BumpSequenceOp{},
	},
}
var genericBumpOperationEnvelope = xdr.TransactionV1Envelope{
	Tx: xdr.Transaction{
		SourceAccount: genericSourceAccount,
		Memo:          xdr.Memo{},
		Operations: []xdr.Operation{
			genericBumpOperation,
		},
		Ext: xdr.TransactionExt{
			V: 0,
			SorobanData: &xdr.SorobanTransactionData{
				Ext: xdr.ExtensionPoint{
					V: 0,
				},
				Resources: xdr.SorobanResources{
					Footprint: xdr.LedgerFootprint{
						ReadOnly:  []xdr.LedgerKey{},
						ReadWrite: []xdr.LedgerKey{},
					},
				},
				ResourceFee: 100,
			},
		},
	},
}

var genericLedgerCloseMeta = xdr.LedgerCloseMeta{
	V: 0,
	V0: &xdr.LedgerCloseMetaV0{
		LedgerHeader: xdr.LedgerHeaderHistoryEntry{
			Header: xdr.LedgerHeader{
				LedgerSeq: 2,
				ScpValue: xdr.StellarValue{
					CloseTime: 10,
				},
			},
		},
	},
}

func TestTransformAsset(t *testing.T) {

	type assetInput struct {
		operation xdr.Operation
		index     int32
		txnIndex  int32
		// transaction xdr.TransactionEnvelope
		lcm xdr.LedgerCloseMeta
	}

	type transformTest struct {
		input      assetInput
		wantOutput AssetOutput
		wantErr    error
	}

	nonPaymentInput := assetInput{
		operation: genericBumpOperation,
		txnIndex:  0,
		index:     0,
		lcm:       genericLedgerCloseMeta,
	}

	tests := []transformTest{
		{
			input:      nonPaymentInput,
			wantOutput: AssetOutput{},
			wantErr:    fmt.Errorf("operation of type 11 cannot issue an asset (id 0)"),
		},
	}

	hardCodedInputTransaction, err := makeAssetTestInput()
	assert.NoError(t, err)
	hardCodedOutputArray := makeAssetTestOutput()

	for i, op := range hardCodedInputTransaction.Envelope.Operations() {
		tests = append(tests, transformTest{
			input: assetInput{
				operation: op,
				index:     int32(i),
				txnIndex:  int32(i),
				lcm:       genericLedgerCloseMeta},
			wantOutput: hardCodedOutputArray[i],
			wantErr:    nil,
		})
	}

	for _, test := range tests {
		actualOutput, actualError := TransformAsset(test.input.operation, test.input.index, test.input.txnIndex, 0, test.input.lcm)
		assert.Equal(t, test.wantErr, actualError)
		assert.Equal(t, test.wantOutput, actualOutput)
	}
}

var lpAssetA = xdr.Asset{
	Type: xdr.AssetTypeAssetTypeNative,
}

var lpAssetB = xdr.Asset{
	Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
	AlphaNum4: &xdr.AlphaNum4{
		AssetCode: xdr.AssetCode4([4]byte{0x55, 0x53, 0x53, 0x44}),
		Issuer:    testAccount4ID,
	},
}

var genericTxMeta = utils.CreateSampleTxMeta(29, lpAssetA, lpAssetB)

var genericLedgerTransaction = ingest.LedgerTransaction{
	Index: 1,
	Envelope: xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1:   &genericBumpOperationEnvelope,
	},
	Result: utils.CreateSampleResultMeta(true, 10).Result,
	UnsafeMeta: xdr.TransactionMeta{
		V:  1,
		V1: genericTxMeta,
	},
}

// a selection of hardcoded accounts with their IDs and addresses
var testAccount1Address = "GCEODJVUUVYVFD5KT4TOEDTMXQ76OPFOQC2EMYYMLPXQCUVPOB6XRWPQ"
var testAccount1ID, _ = xdr.AddressToAccountId(testAccount1Address)
var testAccount1 = testAccount1ID.ToMuxedAccount()

var testAccount2Address = "GAOEOQMXDDXPVJC3HDFX6LZFKANJ4OOLQOD2MNXJ7PGAY5FEO4BRRAQU"
var testAccount2ID, _ = xdr.AddressToAccountId(testAccount2Address)
var testAccount2 = testAccount2ID.ToMuxedAccount()

var testAccount3Address = "GBT4YAEGJQ5YSFUMNKX6BPBUOCPNAIOFAVZOF6MIME2CECBMEIUXFZZN"
var testAccount3ID, _ = xdr.AddressToAccountId(testAccount3Address)
var testAccount3 = testAccount3ID.ToMuxedAccount()

var testAccount4Address = "GBVVRXLMNCJQW3IDDXC3X6XCH35B5Q7QXNMMFPENSOGUPQO7WO7HGZPA"
var testAccount4ID, _ = xdr.AddressToAccountId(testAccount4Address)

var usdtAsset = xdr.Asset{
	Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
	AlphaNum4: &xdr.AlphaNum4{
		AssetCode: xdr.AssetCode4([4]byte{0x55, 0x53, 0x44, 0x54}),
		Issuer:    testAccount4ID,
	},
}

var nativeAsset = xdr.MustNewNativeAsset()

func makeAssetTestInput() (inputTransaction ingest.LedgerTransaction, err error) {
	inputTransaction = genericLedgerTransaction
	inputEnvelope := genericBumpOperationEnvelope

	inputEnvelope.Tx.SourceAccount = testAccount1

	inputOperations := []xdr.Operation{
		{
			SourceAccount: nil,
			Body: xdr.OperationBody{
				Type: xdr.OperationTypePayment,
				PaymentOp: &xdr.PaymentOp{
					Destination: testAccount2,
					Asset:       usdtAsset,
					Amount:      350000000,
				},
			},
		},
		{
			SourceAccount: nil,
			Body: xdr.OperationBody{
				Type: xdr.OperationTypePayment,
				PaymentOp: &xdr.PaymentOp{
					Destination: testAccount3,
					Asset:       nativeAsset,
					Amount:      350000000,
				},
			},
		},
	}

	inputEnvelope.Tx.Operations = inputOperations
	inputTransaction.Envelope.V1 = &inputEnvelope
	return
}

func makeAssetTestOutput() (transformedAssets []AssetOutput) {
	transformedAssets = []AssetOutput{
		{
			AssetCode:      "USDT",
			AssetIssuer:    "GBVVRXLMNCJQW3IDDXC3X6XCH35B5Q7QXNMMFPENSOGUPQO7WO7HGZPA",
			AssetType:      "credit_alphanum4",
			AssetID:        -8205667356306085451,
			ClosedAt:       time.Date(1970, time.January, 1, 0, 0, 10, 0, time.UTC),
			LedgerSequence: 2,
		},
		{
			AssetCode:      "",
			AssetIssuer:    "",
			AssetType:      "native",
			AssetID:        -5706705804583548011,
			ClosedAt:       time.Date(1970, time.January, 1, 0, 0, 10, 0, time.UTC),
			LedgerSequence: 2,
		},
	}
	return
}

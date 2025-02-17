//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/support/contractevents"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type OperationsProcessorTestSuiteLedger struct {
	suite.Suite
	ctx                    context.Context
	processor              *OperationProcessor
	mockSession            *db.MockSession
	mockBatchInsertBuilder *history.MockOperationsBatchInsertBuilder
}

func TestOperationProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(OperationsProcessorTestSuiteLedger))
}

func (s *OperationsProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockBatchInsertBuilder = &history.MockOperationsBatchInsertBuilder{}

	s.processor = NewOperationProcessor(
		s.mockBatchInsertBuilder,
		"test network",
	)
}

func (s *OperationsProcessorTestSuiteLedger) TearDownTest() {
	s.mockBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *OperationsProcessorTestSuiteLedger) mockBatchInsertAdds(txs []ingest.LedgerTransaction, sequence uint32) error {
	for _, t := range txs {
		for i, op := range t.Envelope.Operations() {
			expected := transactionOperationWrapper{
				index:          uint32(i),
				transaction:    t,
				operation:      op,
				ledgerSequence: sequence,
			}
			details, err := expected.Details()
			if err != nil {
				return err
			}
			detailsJSON, err := json.Marshal(details)
			if err != nil {
				return err
			}

			source := expected.SourceAccount()
			acID := source.ToAccountId()
			var muxedAccount null.String
			if source.Type == xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
				muxedAccount = null.StringFrom(source.Address())
			}
			s.mockBatchInsertBuilder.On(
				"Add",
				expected.ID(),
				expected.TransactionID(),
				expected.Order(),
				expected.OperationType(),
				detailsJSON,
				acID.Address(),
				muxedAccount,
				mock.Anything,
			).Return(nil).Once()
		}
	}

	return nil
}

func (s *OperationsProcessorTestSuiteLedger) TestOperationTypeInvokeHostFunctionDetails() {
	sourceAddress := "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY"
	source := xdr.MustMuxedAddress(sourceAddress)

	contractParamVal0 := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &xdr.Hash{0x1, 0x2},
	}
	contractParamVal1 := xdr.ScSymbol("func1")
	contractParamVal2 := xdr.Int32(-5)
	contractParamVal3 := xdr.Uint32(6)
	contractParamVal4 := xdr.Uint64(3)
	contractParamVal5 := xdr.ScBytes([]byte{0, 1, 2})
	contractParamVal6 := true

	accountId := xdr.MustAddress("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	wasm := []byte("Some contract code")

	tx := ingest.LedgerTransaction{
		UnsafeMeta: xdr.TransactionMeta{
			V:  2,
			V2: &xdr.TransactionMetaV2{},
		},
	}

	s.T().Run("InvokeContract", func(t *testing.T) {
		wrapper := transactionOperationWrapper{
			transaction: tx,
			operation: xdr.Operation{
				SourceAccount: &source,
				Body: xdr.OperationBody{
					Type: xdr.OperationTypeInvokeHostFunction,
					InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
						HostFunction: xdr.HostFunction{
							Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
							InvokeContract: &xdr.InvokeContractArgs{
								ContractAddress: contractParamVal0,
								FunctionName:    contractParamVal1,
								Args: xdr.ScVec{
									{
										Type: xdr.ScValTypeScvI32,
										I32:  &contractParamVal2,
									},
									{
										Type: xdr.ScValTypeScvU32,
										U32:  &contractParamVal3,
									},
									{
										Type: xdr.ScValTypeScvU64,
										U64:  &contractParamVal4,
									},
									{
										Type:  xdr.ScValTypeScvBytes,
										Bytes: &contractParamVal5,
									},
									{
										Type: xdr.ScValTypeScvBool,
										B:    &contractParamVal6,
									},
									{
										// invalid ScVal
										Type: 5555,
									},
								},
							},
						},
					},
				},
			},
		}

		details, err := wrapper.Details()
		s.Assert().NoError(err)

		args := []xdr.ScVal{
			{
				Type:    xdr.ScValTypeScvAddress,
				Address: &contractParamVal0,
			},
			{
				Type: xdr.ScValTypeScvSymbol,
				Sym:  &contractParamVal1,
			},
		}
		args = append(args, wrapper.operation.Body.InvokeHostFunctionOp.HostFunction.InvokeContract.Args...)
		detailsFunctionParams := details["parameters"].([]map[string]string)
		s.Assert().Equal(details["function"], "HostFunctionTypeHostFunctionTypeInvokeContract")
		s.assertInvokeHostFunctionParameter(detailsFunctionParams, 0, "Address", args[0])
		s.assertInvokeHostFunctionParameter(detailsFunctionParams, 1, "Sym", args[1])
		s.assertInvokeHostFunctionParameter(detailsFunctionParams, 2, "I32", args[2])
		s.assertInvokeHostFunctionParameter(detailsFunctionParams, 3, "U32", args[3])
		s.assertInvokeHostFunctionParameter(detailsFunctionParams, 4, "U64", args[4])
		s.assertInvokeHostFunctionParameter(detailsFunctionParams, 5, "Bytes", args[5])
		s.assertInvokeHostFunctionParameter(detailsFunctionParams, 6, "B", args[6])
		s.assertInvokeHostFunctionParameter(detailsFunctionParams, 7, "n/a", args[7])
	})

	s.T().Run("CreateContractFromAsset", func(t *testing.T) {
		wrapper := transactionOperationWrapper{
			transaction: tx,
			operation: xdr.Operation{
				SourceAccount: &source,
				Body: xdr.OperationBody{
					Type: xdr.OperationTypeInvokeHostFunction,
					InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
						HostFunction: xdr.HostFunction{
							Type: xdr.HostFunctionTypeHostFunctionTypeCreateContract,
							CreateContract: &xdr.CreateContractArgs{
								ContractIdPreimage: xdr.ContractIdPreimage{
									Type: xdr.ContractIdPreimageTypeContractIdPreimageFromAsset,
									FromAsset: &xdr.Asset{
										Type: 1,
										AlphaNum4: &xdr.AlphaNum4{
											AssetCode: xdr.AssetCode4{65, 82, 83, 0},
											Issuer:    xdr.MustAddress("GCXI6Q73J7F6EUSBZTPW4G4OUGVDHABPYF2U4KO7MVEX52OH5VMVUCRF"),
										},
									},
								},
								Executable: xdr.ContractExecutable{
									Type: xdr.ContractExecutableTypeContractExecutableStellarAsset,
								},
							},
						},
					},
				},
			},
		}

		details, err := wrapper.Details()
		s.Assert().NoError(err)

		s.Assert().Equal(details["function"], "HostFunctionTypeHostFunctionTypeCreateContract")
		s.Assert().Equal(details["from"], "asset")
		s.Assert().Equal(details["asset"], "ARS:GCXI6Q73J7F6EUSBZTPW4G4OUGVDHABPYF2U4KO7MVEX52OH5VMVUCRF")
	})

	s.T().Run("CreateContractFromAddress", func(t *testing.T) {
		wrapper := transactionOperationWrapper{
			transaction: tx,
			operation: xdr.Operation{
				SourceAccount: &source,
				Body: xdr.OperationBody{
					Type: xdr.OperationTypeInvokeHostFunction,
					InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
						HostFunction: xdr.HostFunction{
							Type: xdr.HostFunctionTypeHostFunctionTypeCreateContract,
							CreateContract: &xdr.CreateContractArgs{
								ContractIdPreimage: xdr.ContractIdPreimage{
									Type: xdr.ContractIdPreimageTypeContractIdPreimageFromAddress,
									FromAddress: &xdr.ContractIdPreimageFromAddress{
										Address: xdr.ScAddress{
											Type:      xdr.ScAddressTypeScAddressTypeAccount,
											AccountId: &accountId,
										},
										Salt: xdr.Uint256{1},
									},
								},
								Executable: xdr.ContractExecutable{},
							},
						},
					},
				},
			},
		}

		details, err := wrapper.Details()
		s.Assert().NoError(err)

		s.Assert().Equal(details["function"], "HostFunctionTypeHostFunctionTypeCreateContract")
		s.Assert().Equal(details["from"], "address")
		s.Assert().Equal(details["address"], "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
		s.Assert().Equal(details["salt"], xdr.Uint256{1}.String())
	})

	s.T().Run("UploadContractWasm", func(t *testing.T) {
		wrapper := transactionOperationWrapper{
			transaction: tx,
			operation: xdr.Operation{
				SourceAccount: &source,
				Body: xdr.OperationBody{
					Type: xdr.OperationTypeInvokeHostFunction,
					InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
						HostFunction: xdr.HostFunction{
							Type: xdr.HostFunctionTypeHostFunctionTypeUploadContractWasm,
							Wasm: &wasm,
						},
					},
				},
			},
		}

		details, err := wrapper.Details()
		s.Assert().NoError(err)
		s.Assert().Equal(details["function"], "HostFunctionTypeHostFunctionTypeUploadContractWasm")
	})

	s.T().Run("InvokeContractWithSACEventsInDetails", func(t *testing.T) {
		randomIssuer := keypair.MustRandom()
		randomAsset := xdr.MustNewCreditAsset("TESTING", randomIssuer.Address())
		passphrase := "passphrase"
		randomAccount := keypair.MustRandom().Address()
		contractId := [32]byte{}
		zeroContractStrKey, err := strkey.Encode(strkey.VersionByteContract, contractId[:])
		s.Assert().NoError(err)

		transferContractEvent := contractevents.GenerateEvent(contractevents.EventTypeTransfer, randomAccount, zeroContractStrKey, "", randomAsset, big.NewInt(10000000), passphrase)
		burnContractEvent := contractevents.GenerateEvent(contractevents.EventTypeBurn, zeroContractStrKey, "", "", randomAsset, big.NewInt(10000000), passphrase)
		mintContractEvent := contractevents.GenerateEvent(contractevents.EventTypeMint, "", zeroContractStrKey, randomAccount, randomAsset, big.NewInt(10000000), passphrase)
		clawbackContractEvent := contractevents.GenerateEvent(contractevents.EventTypeClawback, zeroContractStrKey, "", randomAccount, randomAsset, big.NewInt(10000000), passphrase)

		tx = ingest.LedgerTransaction{
			UnsafeMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: &xdr.SorobanTransactionMeta{
						Events: []xdr.ContractEvent{
							transferContractEvent,
							burnContractEvent,
							mintContractEvent,
							clawbackContractEvent,
						},
					},
				},
			},
		}
		wrapper := transactionOperationWrapper{
			transaction: tx,
			operation: xdr.Operation{
				SourceAccount: &source,
				Body: xdr.OperationBody{
					Type: xdr.OperationTypeInvokeHostFunction,
					InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
						HostFunction: xdr.HostFunction{
							Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
							InvokeContract: &xdr.InvokeContractArgs{
								ContractAddress: xdr.ScAddress{
									Type:       xdr.ScAddressTypeScAddressTypeContract,
									ContractId: &xdr.Hash{0x1, 0x2},
								},
								FunctionName: "foo",
								Args:         xdr.ScVec{},
							},
						},
					},
				},
			},
			network: passphrase,
		}

		details, err := wrapper.Details()
		s.Assert().NoError(err)
		s.Assert().Len(details["asset_balance_changes"], 4)

		found := 0
		for _, assetBalanceChanged := range details["asset_balance_changes"].([]map[string]interface{}) {
			if assetBalanceChanged["type"] == "transfer" {
				s.Assert().Equal(assetBalanceChanged["from"], randomAccount)
				s.Assert().Equal(assetBalanceChanged["to"], zeroContractStrKey)
				s.Assert().Equal(assetBalanceChanged["amount"], "1.0000000")
				found++
			}

			if assetBalanceChanged["type"] == "burn" {
				s.Assert().Equal(assetBalanceChanged["from"], zeroContractStrKey)
				s.Assert().NotContains(assetBalanceChanged, "to")
				s.Assert().Equal(assetBalanceChanged["amount"], "1.0000000")
				found++
			}

			if assetBalanceChanged["type"] == "mint" {
				s.Assert().NotContains(assetBalanceChanged, "from")
				s.Assert().Equal(assetBalanceChanged["to"], zeroContractStrKey)
				s.Assert().Equal(assetBalanceChanged["amount"], "1.0000000")
				found++
			}

			if assetBalanceChanged["type"] == "clawback" {
				s.Assert().Equal(assetBalanceChanged["from"], zeroContractStrKey)
				s.Assert().NotContains(assetBalanceChanged, "to")
				s.Assert().Equal(assetBalanceChanged["amount"], "1.0000000")
				found++
			}
		}
		s.Assert().Equal(found, 4, "should have one balance changed record for each of mint, burn, clawback, transfer")
	})
}

func (s *OperationsProcessorTestSuiteLedger) assertInvokeHostFunctionParameter(parameters []map[string]string, paramPosition int, expectedType string, expectedVal xdr.ScVal) {
	serializedParam := parameters[paramPosition]
	s.Assert().Equal(serializedParam["type"], expectedType)
	if expectedSerializedXdr, err := expectedVal.MarshalBinary(); err == nil {
		s.Assert().Equal(serializedParam["value"], base64.StdEncoding.EncodeToString(expectedSerializedXdr))
	} else {
		s.Assert().Equal(serializedParam["value"], "n/a")
	}
}

func (s *OperationsProcessorTestSuiteLedger) TestAddOperationSucceeds() {
	sequence := uint32(56)
	lcm := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}

	unmuxed := xdr.MustAddress("GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2")
	muxed := xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xdeadbeefdeadbeef,
			Ed25519: *unmuxed.Ed25519,
		},
	}
	firstTx := createTransaction(true, 1, 2)
	firstTx.Index = 1
	firstTx.Envelope.Operations()[0].Body = xdr.OperationBody{
		Type: xdr.OperationTypePayment,
		PaymentOp: &xdr.PaymentOp{
			Destination: muxed,
			Asset:       xdr.Asset{Type: xdr.AssetTypeAssetTypeNative},
			Amount:      100,
		},
	}
	firstTx.Envelope.V1.Tx.SourceAccount = muxed
	secondTx := createTransaction(false, 3, 2)
	thirdTx := createTransaction(true, 4, 2)

	txs := []ingest.LedgerTransaction{
		firstTx,
		secondTx,
		thirdTx,
	}

	var err error

	err = s.mockBatchInsertAdds(txs, sequence)
	s.Assert().NoError(err)
	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(nil).Once()

	for _, tx := range txs {
		err = s.processor.ProcessTransaction(lcm, tx)
		s.Assert().NoError(err)
	}
	s.Assert().NoError(s.processor.Flush(s.ctx, s.mockSession))
}

func (s *OperationsProcessorTestSuiteLedger) TestAddOperationFails() {
	sequence := uint32(56)
	lcm := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}
	tx := createTransaction(true, 1, 2)

	s.mockBatchInsertBuilder.
		On(
			"Add",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(errors.New("transient error")).Once()

	err := s.processor.ProcessTransaction(lcm, tx)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "Error batch inserting operation rows: transient error")
}

func (s *OperationsProcessorTestSuiteLedger) TestExecFails() {
	sequence := uint32(56)
	lcm := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}
	tx := createTransaction(true, 1, 2)

	s.mockBatchInsertBuilder.
		On(
			"Add",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil).Once()
	s.Assert().NoError(s.processor.ProcessTransaction(lcm, tx))

	s.mockBatchInsertBuilder.On("Exec", s.ctx, s.mockSession).Return(errors.New("transient error")).Once()
	err := s.processor.Flush(s.ctx, s.mockSession)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "transient error")
}

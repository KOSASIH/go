package txnbuild

import (
	"testing"

	"github.com/stellar/go/xdr"

	"github.com/stretchr/testify/assert"
)

func TestCreateInvokeHostFunctionValid(t *testing.T) {
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761100))

	invokeHostFunctionOp := InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type:           xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{},
		},
		Auth:          []xdr.SorobanAuthorizationEntry{},
		SourceAccount: sourceAccount.AccountID,
	}

	assert.NoError(t, invokeHostFunctionOp.Validate())
}

func TestCreateInvokeHostFunctionInvalid(t *testing.T) {
	invokeHostFunctionOp := InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type:           xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{},
		},
		Auth:          []xdr.SorobanAuthorizationEntry{},
		SourceAccount: "invalid account value",
	}

	assert.Error(t, invokeHostFunctionOp.Validate())
}

func TestInvokeHostFunctionRoundTrip(t *testing.T) {
	val := xdr.Int32(4)
	wasmId := xdr.Hash{1, 2, 3, 4}
	i64 := xdr.Int64(45)
	accountId := xdr.MustAddress("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	var invokeHostFunctionOp = &InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: xdr.ScAddress{
					Type:       xdr.ScAddressTypeScAddressTypeContract,
					ContractId: &xdr.Hash{0x1, 0x2},
				},
				FunctionName: "foo",
				Args: xdr.ScVec{
					xdr.ScVal{
						Type: xdr.ScValTypeScvI32,
						I32:  &val,
					},
				},
			},
		},
		Auth: []xdr.SorobanAuthorizationEntry{
			{
				Credentials: xdr.SorobanCredentials{
					Type: xdr.SorobanCredentialsTypeSorobanCredentialsAddress,
					Address: &xdr.SorobanAddressCredentials{
						Address: xdr.ScAddress{
							Type:      xdr.ScAddressTypeScAddressTypeAccount,
							AccountId: &accountId,
						},
						Nonce:                     0,
						SignatureExpirationLedger: 0,
						Signature: xdr.ScVal{
							Type: xdr.ScValTypeScvI64,
							I64:  &i64,
						},
					},
				},
				RootInvocation: xdr.SorobanAuthorizedInvocation{
					Function: xdr.SorobanAuthorizedFunction{
						Type: xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
						ContractFn: &xdr.InvokeContractArgs{
							ContractAddress: xdr.ScAddress{
								Type:       xdr.ScAddressTypeScAddressTypeContract,
								ContractId: &xdr.Hash{0x1, 0x2},
							},
							FunctionName: "foo",
							Args: xdr.ScVec{
								xdr.ScVal{
									Type: xdr.ScValTypeScvI32,
									I32:  &val,
								},
							},
						},
					},
					SubInvocations: nil,
				},
			},
		},
		SourceAccount: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Resources: xdr.SorobanResources{
					Footprint: xdr.LedgerFootprint{
						ReadOnly: []xdr.LedgerKey{
							{
								Type: xdr.LedgerEntryTypeContractData,
								ContractData: &xdr.LedgerKeyContractData{
									Contract: xdr.ScAddress{
										Type:       xdr.ScAddressTypeScAddressTypeContract,
										ContractId: &xdr.Hash{1, 2, 3},
									},
									Key: xdr.ScVal{
										Type: xdr.ScValTypeScvContractInstance,
										Instance: &xdr.ScContractInstance{
											Executable: xdr.ContractExecutable{
												Type:     xdr.ContractExecutableTypeContractExecutableWasm,
												WasmHash: &wasmId,
											},
										},
									},
								},
							},
						},
						ReadWrite: []xdr.LedgerKey{
							{
								Type: xdr.LedgerEntryTypeContractData,
								ContractData: &xdr.LedgerKeyContractData{
									Contract: xdr.ScAddress{
										Type:       xdr.ScAddressTypeScAddressTypeContract,
										ContractId: &xdr.Hash{1, 2, 3},
									},
									Key: xdr.ScVal{
										Type: xdr.ScValTypeScvI64,
										I64:  &i64,
									},
								},
							},
						},
					},
					Instructions: 0,
					ReadBytes:    0,
					WriteBytes:   0,
				},
				ResourceFee: 1,
				Ext: xdr.ExtensionPoint{
					V: 0,
				},
			},
		},
	}
	testOperationsMarshalingRoundtrip(t, []Operation{invokeHostFunctionOp}, false)

	// with muxed accounts
	invokeHostFunctionOp.SourceAccount = "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK"
	testOperationsMarshalingRoundtrip(t, []Operation{invokeHostFunctionOp}, true)
}

package txnbuild

import (
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type InvokeHostFunction struct {
	HostFunction  xdr.HostFunction
	Auth          []xdr.SorobanAuthorizationEntry
	SourceAccount string
	Ext           xdr.TransactionExt
}

func (f *InvokeHostFunction) BuildXDR() (xdr.Operation, error) {

	opType := xdr.OperationTypeInvokeHostFunction
	xdrOp := xdr.InvokeHostFunctionOp{
		HostFunction: f.HostFunction,
		Auth:         f.Auth,
	}

	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR Operation")
	}

	op := xdr.Operation{Body: body}

	SetOpSourceAccount(&op, f.SourceAccount)
	return op, nil
}

func (f *InvokeHostFunction) FromXDR(xdrOp xdr.Operation) error {
	result, ok := xdrOp.Body.GetInvokeHostFunctionOp()
	if !ok {
		return errors.New("error parsing invoke host function operation from xdr")
	}

	f.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	f.HostFunction = result.HostFunction
	f.Auth = result.Auth

	return nil
}

func (f *InvokeHostFunction) Validate() error {
	if f.SourceAccount != "" {
		_, err := xdr.AddressToMuxedAccount(f.SourceAccount)
		if err != nil {
			return NewValidationError("SourceAccount", err.Error())
		}
	}
	return nil
}

func (f *InvokeHostFunction) GetSourceAccount() string {
	return f.SourceAccount
}

func (f *InvokeHostFunction) BuildTransactionExt() (xdr.TransactionExt, error) {
	return f.Ext, nil
}

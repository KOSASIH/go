package txnbuild

import (
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

type RestoreFootprint struct {
	SourceAccount string
	Ext           xdr.TransactionExt
}

func (f *RestoreFootprint) BuildXDR() (xdr.Operation, error) {
	xdrOp := xdr.RestoreFootprintOp{
		Ext: xdr.ExtensionPoint{
			V: 0,
		},
	}

	body, err := xdr.NewOperationBody(xdr.OperationTypeRestoreFootprint, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR Operation")
	}

	op := xdr.Operation{Body: body}

	SetOpSourceAccount(&op, f.SourceAccount)
	return op, nil
}

func (f *RestoreFootprint) FromXDR(xdrOp xdr.Operation) error {
	_, ok := xdrOp.Body.GetRestoreFootprintOp()
	if !ok {
		return errors.New("error parsing invoke host function operation from xdr")
	}
	f.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	return nil
}

func (f *RestoreFootprint) Validate() error {
	if f.SourceAccount != "" {
		_, err := xdr.AddressToMuxedAccount(f.SourceAccount)
		if err != nil {
			return NewValidationError("SourceAccount", err.Error())
		}
	}
	return nil
}

func (f *RestoreFootprint) GetSourceAccount() string {
	return f.SourceAccount
}

func (f *RestoreFootprint) BuildTransactionExt() (xdr.TransactionExt, error) {
	return f.Ext, nil
}

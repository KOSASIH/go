package xdr

import (
	"encoding/hex"
	"fmt"
)

func (ce ContractEvent) String() string {
	result := ce.Type.String() + "("
	if ce.ContractId != nil {
		result += hex.EncodeToString(ce.ContractId[:]) + ","
	}
	result += ce.Body.String() + ")"
	return result
}

func (eb ContractEventBody) String() string {
	return fmt.Sprintf("%+v", *eb.V0)
}

func (de DiagnosticEvent) String() string {
	return fmt.Sprintf("%s, successful call: %t", de.Event, de.InSuccessfulContractCall)
}

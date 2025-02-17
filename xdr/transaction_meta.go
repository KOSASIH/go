package xdr

import (
	"fmt"
)

// Operations is a helper on TransactionMeta that returns operations
// meta from `TransactionMeta.Operations` or `TransactionMeta.V1.Operations`.
func (transactionMeta *TransactionMeta) OperationsMeta() []OperationMeta {
	switch transactionMeta.V {
	case 0:
		return *transactionMeta.Operations
	case 1:
		return transactionMeta.MustV1().Operations
	case 2:
		return transactionMeta.MustV2().Operations
	case 3:
		return transactionMeta.MustV3().Operations
	default:
		panic("Unsupported TransactionMeta version")
	}
}

// GetDiagnosticEvents returns all contract events emitted by a given operation.
func (t *TransactionMeta) GetDiagnosticEvents() ([]DiagnosticEvent, error) {
	switch t.V {
	case 1:
		return nil, nil
	case 2:
		return nil, nil
	case 3:
		var diagnosticEvents []DiagnosticEvent
		var contractEvents []ContractEvent
		if sorobanMeta := t.MustV3().SorobanMeta; sorobanMeta != nil {
			diagnosticEvents = sorobanMeta.DiagnosticEvents
			if len(diagnosticEvents) > 0 {
				// all contract events and diag events for a single operation(by its index in the tx) were available
				// in tx meta's DiagnosticEvents, no need to look anywhere else for events
				return diagnosticEvents, nil
			}

			contractEvents = sorobanMeta.Events
			if len(contractEvents) == 0 {
				// no events were present in this tx meta
				return nil, nil
			}
		}

		// tx meta only provided contract events, no diagnostic events, we convert the contract
		// event to a diagnostic event, to fit the response interface.
		convertedDiagnosticEvents := make([]DiagnosticEvent, len(contractEvents))
		for i, event := range contractEvents {
			convertedDiagnosticEvents[i] = DiagnosticEvent{
				InSuccessfulContractCall: true,
				Event:                    event,
			}
		}
		return convertedDiagnosticEvents, nil
	default:
		return nil, fmt.Errorf("unsupported TransactionMeta version: %v", t.V)
	}
}

package contract

import (
	"fmt"
	"time"

	"github.com/stellar/go/ingest"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// ContractCodeOutput is a representation of contract code that aligns with the Bigquery table soroban_contract_code
type ContractCodeOutput struct {
	ContractCodeHash   string    `json:"contract_code_hash"`
	ContractCodeExtV   int32     `json:"contract_code_ext_v"`
	LastModifiedLedger uint32    `json:"last_modified_ledger"`
	LedgerEntryChange  uint32    `json:"ledger_entry_change"`
	Deleted            bool      `json:"deleted"`
	ClosedAt           time.Time `json:"closed_at"`
	LedgerSequence     uint32    `json:"ledger_sequence"`
	LedgerKeyHash      string    `json:"ledger_key_hash"`
	//ContractCodeCode                string `json:"contract_code"`
	NInstructions     uint32 `json:"n_instructions"`
	NFunctions        uint32 `json:"n_functions"`
	NGlobals          uint32 `json:"n_globals"`
	NTableEntries     uint32 `json:"n_table_entries"`
	NTypes            uint32 `json:"n_types"`
	NDataSegments     uint32 `json:"n_data_segments"`
	NElemSegments     uint32 `json:"n_elem_segments"`
	NImports          uint32 `json:"n_imports"`
	NExports          uint32 `json:"n_exports"`
	NDataSegmentBytes uint32 `json:"n_data_segment_bytes"`
}

// TransformContractCode converts a contract code ledger change entry into a form suitable for BigQuery
func TransformContractCode(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) (ContractCodeOutput, error) {
	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return ContractCodeOutput{}, err
	}

	contractCode, ok := ledgerEntry.Data.GetContractCode()
	if !ok {
		return ContractCodeOutput{}, fmt.Errorf("could not extract contract code from ledger entry; actual type is %s", ledgerEntry.Data.Type)
	}

	// LedgerEntryChange must contain a contract code change to be parsed, otherwise skip
	if ledgerEntry.Data.Type != xdr.LedgerEntryTypeContractCode {
		return ContractCodeOutput{}, nil
	}

	ledgerKeyHash := utils.LedgerEntryToLedgerKeyHash(ledgerEntry)

	contractCodeExtV := contractCode.Ext.V

	contractCodeHash := contractCode.Hash.HexString()

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return ContractCodeOutput{}, err
	}

	ledgerSequence := header.Header.LedgerSeq

	var outputNInstructions uint32
	var outputNFunctions uint32
	var outputNGlobals uint32
	var outputNTableEntries uint32
	var outputNTypes uint32
	var outputNDataSegments uint32
	var outputNElemSegments uint32
	var outputNImports uint32
	var outputNExports uint32
	var outputNDataSegmentBytes uint32

	extV1, ok := contractCode.Ext.GetV1()
	if ok {
		outputNInstructions = uint32(extV1.CostInputs.NInstructions)
		outputNFunctions = uint32(extV1.CostInputs.NFunctions)
		outputNGlobals = uint32(extV1.CostInputs.NGlobals)
		outputNTableEntries = uint32(extV1.CostInputs.NTableEntries)
		outputNTypes = uint32(extV1.CostInputs.NTypes)
		outputNDataSegments = uint32(extV1.CostInputs.NDataSegments)
		outputNElemSegments = uint32(extV1.CostInputs.NElemSegments)
		outputNImports = uint32(extV1.CostInputs.NImports)
		outputNExports = uint32(extV1.CostInputs.NExports)
		outputNDataSegmentBytes = uint32(extV1.CostInputs.NDataSegmentBytes)
	}

	transformedCode := ContractCodeOutput{
		ContractCodeHash:   contractCodeHash,
		ContractCodeExtV:   int32(contractCodeExtV),
		LastModifiedLedger: uint32(ledgerEntry.LastModifiedLedgerSeq),
		LedgerEntryChange:  uint32(changeType),
		Deleted:            outputDeleted,
		ClosedAt:           closedAt,
		LedgerSequence:     uint32(ledgerSequence),
		LedgerKeyHash:      ledgerKeyHash,
		NInstructions:      outputNInstructions,
		NFunctions:         outputNFunctions,
		NGlobals:           outputNGlobals,
		NTableEntries:      outputNTableEntries,
		NTypes:             outputNTypes,
		NDataSegments:      outputNDataSegments,
		NElemSegments:      outputNElemSegments,
		NImports:           outputNImports,
		NExports:           outputNExports,
		NDataSegmentBytes:  outputNDataSegmentBytes,
	}
	return transformedCode, nil
}

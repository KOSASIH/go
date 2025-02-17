package ledger

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/stellar/go/historyarchive"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

// LedgerOutput is a representation of a ledger that aligns with the BigQuery table history_ledgers
type LedgerOutput struct {
	Sequence                   uint32    `json:"sequence"` // sequence number of the ledger
	LedgerHash                 string    `json:"ledger_hash"`
	PreviousLedgerHash         string    `json:"previous_ledger_hash"`
	LedgerHeader               string    `json:"ledger_header"` // base 64 encoding of the ledger header
	TransactionCount           int32     `json:"transaction_count"`
	OperationCount             int32     `json:"operation_count"` // counts only operations that were a part of successful transactions
	SuccessfulTransactionCount int32     `json:"successful_transaction_count"`
	FailedTransactionCount     int32     `json:"failed_transaction_count"`
	TxSetOperationCount        string    `json:"tx_set_operation_count"` // counts all operations, even those that are part of failed transactions
	ClosedAt                   time.Time `json:"closed_at"`              // UTC timestamp
	TotalCoins                 int64     `json:"total_coins"`
	FeePool                    int64     `json:"fee_pool"`
	BaseFee                    uint32    `json:"base_fee"`
	BaseReserve                uint32    `json:"base_reserve"`
	MaxTxSetSize               uint32    `json:"max_tx_set_size"`
	ProtocolVersion            uint32    `json:"protocol_version"`
	LedgerID                   int64     `json:"id"`
	SorobanFeeWrite1Kb         int64     `json:"soroban_fee_write_1kb"`
	NodeID                     string    `json:"node_id"`
	Signature                  string    `json:"signature"`
	TotalByteSizeOfBucketList  uint64    `json:"total_byte_size_of_bucket_list"`
}

// TransformLedger converts a ledger from the history archive ingestion system into a form suitable for BigQuery
func TransformLedger(inputLedger historyarchive.Ledger, lcm xdr.LedgerCloseMeta) (LedgerOutput, error) {
	ledgerHeader := inputLedger.Header.Header

	outputSequence := uint32(ledgerHeader.LedgerSeq)

	outputLedgerID := toid.New(int32(outputSequence), 0, 0).ToInt64()

	outputLedgerHash := utils.HashToHexString(inputLedger.Header.Hash)
	outputPreviousHash := utils.HashToHexString(ledgerHeader.PreviousLedgerHash)

	outputLedgerHeader, err := xdr.MarshalBase64(ledgerHeader)
	if err != nil {
		return LedgerOutput{}, fmt.Errorf("for ledger %d (ledger id=%d): %v", outputSequence, outputLedgerID, err)
	}

	outputTransactionCount, outputOperationCount, outputSuccessfulCount, outputFailedCount, outputTxSetOperationCount, err := extractCounts(inputLedger)
	if err != nil {
		return LedgerOutput{}, fmt.Errorf("for ledger %d (ledger id=%d): %v", outputSequence, outputLedgerID, err)
	}

	outputCloseTime, err := utils.TimePointToUTCTimeStamp(ledgerHeader.ScpValue.CloseTime)
	if err != nil {
		return LedgerOutput{}, err
	}

	outputTotalCoins := int64(ledgerHeader.TotalCoins)
	if outputTotalCoins < 0 {
		return LedgerOutput{}, fmt.Errorf("the total number of coins (%d) is negative for ledger %d (ledger id=%d)", outputTotalCoins, outputSequence, outputLedgerID)
	}

	outputFeePool := int64(ledgerHeader.FeePool)
	if outputFeePool < 0 {
		return LedgerOutput{}, fmt.Errorf("the fee pool (%d) is negative for ledger %d (ledger id=%d)", outputFeePool, outputSequence, outputLedgerID)
	}

	outputBaseFee := uint32(ledgerHeader.BaseFee)

	outputBaseReserve := uint32(ledgerHeader.BaseReserve)

	outputMaxTxSetSize := uint32(ledgerHeader.MaxTxSetSize)

	outputProtocolVersion := uint32(ledgerHeader.LedgerVersion)

	var outputSorobanFeeWrite1Kb int64
	var outputTotalByteSizeOfBucketList uint64

	lcmV1, ok := lcm.GetV1()
	if ok {
		var extV1 xdr.LedgerCloseMetaExtV1
		extV1, ok = lcmV1.Ext.GetV1()
		if ok {
			outputSorobanFeeWrite1Kb = int64(extV1.SorobanFeeWrite1Kb)
		}
		totalByteSizeOfBucketList := lcmV1.TotalByteSizeOfBucketList
		outputTotalByteSizeOfBucketList = uint64(totalByteSizeOfBucketList)
	}

	var outputNodeID string
	var outputSignature string
	LedgerCloseValueSignature, ok := ledgerHeader.ScpValue.Ext.GetLcValueSignature()
	if ok {
		outputNodeID, err = getAddress(LedgerCloseValueSignature.NodeId)
		if err != nil {
			return LedgerOutput{}, err
		}
		outputSignature = base64.StdEncoding.EncodeToString(LedgerCloseValueSignature.Signature)
	}

	transformedLedger := LedgerOutput{
		Sequence:                   outputSequence,
		LedgerID:                   outputLedgerID,
		LedgerHash:                 outputLedgerHash,
		PreviousLedgerHash:         outputPreviousHash,
		LedgerHeader:               outputLedgerHeader,
		TransactionCount:           outputTransactionCount,
		OperationCount:             outputOperationCount,
		SuccessfulTransactionCount: outputSuccessfulCount,
		FailedTransactionCount:     outputFailedCount,
		TxSetOperationCount:        outputTxSetOperationCount,
		ClosedAt:                   outputCloseTime,
		TotalCoins:                 outputTotalCoins,
		FeePool:                    outputFeePool,
		BaseFee:                    outputBaseFee,
		BaseReserve:                outputBaseReserve,
		MaxTxSetSize:               outputMaxTxSetSize,
		ProtocolVersion:            outputProtocolVersion,
		SorobanFeeWrite1Kb:         outputSorobanFeeWrite1Kb,
		NodeID:                     outputNodeID,
		Signature:                  outputSignature,
		TotalByteSizeOfBucketList:  outputTotalByteSizeOfBucketList,
	}
	return transformedLedger, nil
}

func TransactionProcessing(l xdr.LedgerCloseMeta) []xdr.TransactionResultMeta {
	switch l.V {
	case 0:
		return l.MustV0().TxProcessing
	case 1:
		return l.MustV1().TxProcessing
	default:
		panic(fmt.Sprintf("Unsupported LedgerCloseMeta.V: %d", l.V))
	}
}

func extractCounts(ledger historyarchive.Ledger) (transactionCount int32, operationCount int32, successTxCount int32, failedTxCount int32, txSetOperationCount string, err error) {
	transactions := GetTransactionSet(ledger)
	results := ledger.TransactionResult.TxResultSet.Results
	txCount := len(transactions)
	if txCount != len(results) {
		err = fmt.Errorf("the number of transactions and results are different (%d != %d)", txCount, len(results))
		return
	}

	txSetOperationCounter := int32(0)
	for i := 0; i < txCount; i++ {
		operations := transactions[i].Operations()
		numberOfOps := int32(len(operations))
		txSetOperationCounter += numberOfOps

		// for successful transactions, the operation count is based on the operations results slice
		if results[i].Result.Successful() {
			operationResults, ok := results[i].Result.OperationResults()
			if !ok {
				err = fmt.Errorf("could not access operation results for result %d", i)
				return
			}

			successTxCount++
			operationCount += int32(len(operationResults))
		} else {
			failedTxCount++
		}

	}
	transactionCount = int32(txCount) - failedTxCount
	txSetOperationCount = strconv.FormatInt(int64(txSetOperationCounter), 10)
	return
}

func GetTransactionSet(transactionEntry historyarchive.Ledger) (transactionProcessing []xdr.TransactionEnvelope) {
	switch transactionEntry.Transaction.Ext.V {
	case 0:
		return transactionEntry.Transaction.TxSet.Txs
	case 1:
		return getTransactionPhase(transactionEntry.Transaction.Ext.GeneralizedTxSet.V1TxSet.Phases)
	default:
		panic(fmt.Sprintf("Unsupported TransactionHistoryEntry.Ext: %d", transactionEntry.Transaction.Ext.V))
	}
}

func getTransactionPhase(transactionPhase []xdr.TransactionPhase) (transactionEnvelope []xdr.TransactionEnvelope) {
	transactionSlice := []xdr.TransactionEnvelope{}
	for _, phase := range transactionPhase {
		switch phase.V {
		case 0:
			components := phase.MustV0Components()
			for _, component := range components {
				switch component.Type {
				case 0:
					transactionSlice = append(transactionSlice, component.TxsMaybeDiscountedFee.Txs...)

				default:
					panic(fmt.Sprintf("Unsupported TxSetComponentType: %d", component.Type))
				}

			}
		default:
			panic(fmt.Sprintf("Unsupported TransactionPhase.V: %d", phase.V))
		}
	}
	return transactionSlice

}

// TODO: This should be moved into the go monorepo xdr functions
// Or nodeID should just be an xdr.AccountId but the error message would be incorrect
func getAddress(nodeID xdr.NodeId) (string, error) {
	switch nodeID.Type {
	case xdr.PublicKeyTypePublicKeyTypeEd25519:
		ed, ok := nodeID.GetEd25519()
		if !ok {
			return "", fmt.Errorf("could not get Ed25519")
		}
		raw := make([]byte, 32)
		copy(raw, ed[:])
		return strkey.Encode(strkey.VersionByteAccountID, raw)
	default:
		return "", fmt.Errorf("unknown node id type: %v", nodeID.Type)
	}
}

package ledger

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/stellar/go/xdr"
)

func Sequence(l xdr.LedgerCloseMeta) uint32 {
	return uint32(l.LedgerHeaderHistoryEntry().Header.LedgerSeq)
}

func Hash(l xdr.LedgerCloseMeta) string {
	return l.LedgerHeaderHistoryEntry().Hash.HexString()
}

func PreviousHash(l xdr.LedgerCloseMeta) string {
	return l.PreviousLedgerHash().HexString()
}

func CloseTime(l xdr.LedgerCloseMeta) int64 {
	return l.LedgerCloseTime()
}

func ClosedAt(l xdr.LedgerCloseMeta) time.Time {
	return time.Unix(l.LedgerCloseTime(), 0).UTC()
}

func TotalCoins(l xdr.LedgerCloseMeta) int64 {
	return int64(l.LedgerHeaderHistoryEntry().Header.TotalCoins)
}

func FeePool(l xdr.LedgerCloseMeta) int64 {
	return int64(l.LedgerHeaderHistoryEntry().Header.FeePool)
}

func BaseFee(l xdr.LedgerCloseMeta) uint32 {
	return uint32(l.LedgerHeaderHistoryEntry().Header.BaseFee)
}

func BaseReserve(l xdr.LedgerCloseMeta) uint32 {
	return uint32(l.LedgerHeaderHistoryEntry().Header.BaseReserve)
}

func MaxTxSetSize(l xdr.LedgerCloseMeta) uint32 {
	return uint32(l.LedgerHeaderHistoryEntry().Header.MaxTxSetSize)
}

func LedgerVersion(l xdr.LedgerCloseMeta) uint32 {
	return uint32(l.LedgerHeaderHistoryEntry().Header.LedgerVersion)
}

func SorobanFeeWrite1Kb(l xdr.LedgerCloseMeta) (int64, bool) {
	lcmV1, ok := l.GetV1()
	if !ok {
		return 0, false
	}

	extV1, ok := lcmV1.Ext.GetV1()
	if !ok {
		return 0, false
	}

	return int64(extV1.SorobanFeeWrite1Kb), true
}

func TotalByteSizeOfBucketList(l xdr.LedgerCloseMeta) (uint64, bool) {
	lcmV1, ok := l.GetV1()
	if !ok {
		return 0, false
	}

	return uint64(lcmV1.TotalByteSizeOfBucketList), true
}

func NodeID(l xdr.LedgerCloseMeta) (string, error) {
	LedgerCloseValueSignature, ok := l.LedgerHeaderHistoryEntry().Header.ScpValue.Ext.GetLcValueSignature()
	if !ok {
		return "", fmt.Errorf("could not get LedgerCloseValueSignature")

	}
	return LedgerCloseValueSignature.NodeId.GetAddress()
}

func Signature(l xdr.LedgerCloseMeta) (string, bool) {
	LedgerCloseValueSignature, ok := l.LedgerHeaderHistoryEntry().Header.ScpValue.Ext.GetLcValueSignature()
	if !ok {
		return "", false
	}

	return base64.StdEncoding.EncodeToString(LedgerCloseValueSignature.Signature), true
}

// TransactionCounts calculates and returns the number of successful and total transactions
func TransactionCounts(l xdr.LedgerCloseMeta) (successTxCount, totalTxCount uint32) {
	transactions := l.TransactionEnvelopes()
	results, err := l.TxProcessing()
	if err != nil {
		panic(err)
	}

	txCount := len(transactions)
	if txCount != len(results) {
		panic("transaction count and number of TransactionResultMeta not equal")
	}

	for i := 0; i < txCount; i++ {
		if results[i].Result.Successful() {
			successTxCount++
		}
	}

	return successTxCount, uint32(txCount)
}

// OperationCounts calculates and returns the number of successful operations and the total operations within
// a LedgerCloseMeta
func OperationCounts(l xdr.LedgerCloseMeta) (successfulOperationCount, totalOperationCount uint32) {
	transactions := l.TransactionEnvelopes()
	results, err := l.TxProcessing()
	if err != nil {
		panic(err)
	}

	for i, result := range results {
		operations := transactions[i].OperationsCount()
		totalOperationCount += operations

		// for successful transactions, the operation count is based on the operations results slice
		if result.Result.Successful() {
			operationResults, ok := result.Result.OperationResults()
			if !ok {
				panic("could not get OperationResults")
			}

			successfulOperationCount += uint32(len(operationResults))
		}
	}

	return successfulOperationCount, totalOperationCount
}

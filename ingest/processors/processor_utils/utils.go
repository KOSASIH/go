package processor_utils

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/dgryski/go-farm"
	"github.com/guregu/null"
	"github.com/stellar/go/hash"
	"github.com/stellar/go/ingest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// ExtractEntryFromChange gets the most recent state of an entry from an ingestion change, as well as if the entry was deleted
func ExtractEntryFromChange(change ingest.Change) (xdr.LedgerEntry, xdr.LedgerEntryChangeType, bool, error) {
	switch changeType := change.LedgerEntryChangeType(); changeType {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated, xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		return *change.Post, changeType, false, nil
	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		return *change.Pre, changeType, true, nil
	default:
		return xdr.LedgerEntry{}, changeType, false, fmt.Errorf("unable to extract ledger entry type from change")
	}
}

// TimePointToUTCTimeStamp takes in an xdr TimePoint and converts it to a time.Time struct in UTC. It returns an error for negative timepoints
func TimePointToUTCTimeStamp(providedTime xdr.TimePoint) (time.Time, error) {
	intTime := int64(providedTime)
	if intTime < 0 {
		return time.Now(), fmt.Errorf("the timepoint is negative")
	}
	return time.Unix(intTime, 0).UTC(), nil
}

func CreateSampleTxMeta(subOperationCount int, AssetA, AssetB xdr.Asset) *xdr.TransactionMetaV1 {
	operationMeta := []xdr.OperationMeta{}
	for i := 0; i < subOperationCount; i++ {
		operationMeta = append(operationMeta, xdr.OperationMeta{
			Changes: xdr.LedgerEntryChanges{},
		})
	}

	operationMeta = AddLPOperations(operationMeta, AssetA, AssetB)
	operationMeta = AddLPOperations(operationMeta, AssetA, AssetB)

	operationMeta = append(operationMeta, xdr.OperationMeta{
		Changes: xdr.LedgerEntryChanges{},
	})

	return &xdr.TransactionMetaV1{
		Operations: operationMeta,
	}
}

func AddLPOperations(txMeta []xdr.OperationMeta, AssetA, AssetB xdr.Asset) []xdr.OperationMeta {
	txMeta = append(txMeta, xdr.OperationMeta{
		Changes: xdr.LedgerEntryChanges{
			xdr.LedgerEntryChange{
				Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
				State: &xdr.LedgerEntry{
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeLiquidityPool,
						LiquidityPool: &xdr.LiquidityPoolEntry{
							LiquidityPoolId: xdr.PoolId{1, 2, 3, 4, 5, 6, 7, 8, 9},
							Body: xdr.LiquidityPoolEntryBody{
								Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
								ConstantProduct: &xdr.LiquidityPoolEntryConstantProduct{
									Params: xdr.LiquidityPoolConstantProductParameters{
										AssetA: AssetA,
										AssetB: AssetB,
										Fee:    30,
									},
									ReserveA:                 100000,
									ReserveB:                 1000,
									TotalPoolShares:          500,
									PoolSharesTrustLineCount: 25,
								},
							},
						},
					},
				},
			},
			xdr.LedgerEntryChange{
				Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
				Updated: &xdr.LedgerEntry{
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeLiquidityPool,
						LiquidityPool: &xdr.LiquidityPoolEntry{
							LiquidityPoolId: xdr.PoolId{1, 2, 3, 4, 5, 6, 7, 8, 9},
							Body: xdr.LiquidityPoolEntryBody{
								Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
								ConstantProduct: &xdr.LiquidityPoolEntryConstantProduct{
									Params: xdr.LiquidityPoolConstantProductParameters{
										AssetA: AssetA,
										AssetB: AssetB,
										Fee:    30,
									},
									ReserveA:                 101000,
									ReserveB:                 1100,
									TotalPoolShares:          502,
									PoolSharesTrustLineCount: 26,
								},
							},
						},
					},
				},
			},
		}})

	return txMeta
}

// CreateSampleResultMeta creates Transaction results with the desired success flag and number of sub operation results
func CreateSampleResultMeta(successful bool, subOperationCount int) xdr.TransactionResultMeta {
	resultCode := xdr.TransactionResultCodeTxFailed
	if successful {
		resultCode = xdr.TransactionResultCodeTxSuccess
	}
	operationResults := []xdr.OperationResult{}
	operationResultTr := &xdr.OperationResultTr{
		Type: xdr.OperationTypeCreateAccount,
		CreateAccountResult: &xdr.CreateAccountResult{
			Code: 0,
		},
	}

	for i := 0; i < subOperationCount; i++ {
		operationResults = append(operationResults, xdr.OperationResult{
			Code: xdr.OperationResultCodeOpInner,
			Tr:   operationResultTr,
		})
	}

	return xdr.TransactionResultMeta{
		Result: xdr.TransactionResultPair{
			Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{
					Code:    resultCode,
					Results: &operationResults,
				},
			},
		},
	}
}

// ConvertStroopValueToReal converts a value in stroops, the smallest amount unit, into real units
func ConvertStroopValueToReal(input xdr.Int64) float64 {
	output, _ := big.NewRat(int64(input), int64(10000000)).Float64()
	return output
}

func GetCloseTime(lcm xdr.LedgerCloseMeta) (time.Time, error) {
	headerHistoryEntry := lcm.LedgerHeaderHistoryEntry()
	return ExtractLedgerCloseTime(headerHistoryEntry)
}

func GetLedgerSequence(lcm xdr.LedgerCloseMeta) uint32 {
	headerHistoryEntry := lcm.LedgerHeaderHistoryEntry()
	return uint32(headerHistoryEntry.Header.LedgerSeq)
}

// ExtractLedgerCloseTime gets the close time of the provided ledger
func ExtractLedgerCloseTime(ledger xdr.LedgerHeaderHistoryEntry) (time.Time, error) {
	return TimePointToUTCTimeStamp(ledger.Header.ScpValue.CloseTime)
}

func LedgerEntryToLedgerKeyHash(ledgerEntry xdr.LedgerEntry) string {
	ledgerKey, _ := ledgerEntry.LedgerKey()
	ledgerKeyByte, _ := ledgerKey.MarshalBinary()
	hashedLedgerKeyByte := hash.Hash(ledgerKeyByte)
	ledgerKeyHash := hex.EncodeToString(hashedLedgerKeyByte[:])

	return ledgerKeyHash
}

// HashToHexString is utility function that converts and xdr.Hash type to a hex string
func HashToHexString(inputHash xdr.Hash) string {
	sliceHash := inputHash[:]
	hexString := hex.EncodeToString(sliceHash)
	return hexString
}

func CreateSampleTx(sequence int64, operationCount int) xdr.TransactionEnvelope {
	kp, err := keypair.Random()
	PanicOnError(err)

	operations := []txnbuild.Operation{}
	operationType := &txnbuild.BumpSequence{
		BumpTo: 0,
	}
	for i := 0; i < operationCount; i++ {
		operations = append(operations, operationType)
	}

	sourceAccount := txnbuild.NewSimpleAccount(kp.Address(), int64(0))
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &sourceAccount,
			Operations:    operations,
			BaseFee:       txnbuild.MinBaseFee,
			Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		},
	)
	PanicOnError(err)

	env := tx.ToXDR()
	return env
}

// PanicOnError is a function that panics if the provided error is not nil
func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func LedgerKeyToLedgerKeyHash(ledgerKey xdr.LedgerKey) string {
	ledgerKeyByte, _ := ledgerKey.MarshalBinary()
	hashedLedgerKeyByte := hash.Hash(ledgerKeyByte)
	ledgerKeyHash := hex.EncodeToString(hashedLedgerKeyByte[:])

	return ledgerKeyHash
}

// GetAccountAddressFromMuxedAccount takes in a muxed account and returns the address of the account
func GetAccountAddressFromMuxedAccount(account xdr.MuxedAccount) (string, error) {
	providedID := account.ToAccountId()
	pointerToID := &providedID
	return pointerToID.GetAddress()
}

func LedgerEntrySponsorToNullString(entry xdr.LedgerEntry) null.String {
	sponsoringID := entry.SponsoringID()

	var sponsor null.String
	if sponsoringID != nil {
		sponsor.SetValid((*sponsoringID).Address())
	}

	return sponsor
}

func FarmHashAsset(assetCode, assetIssuer, assetType string) int64 {
	asset := fmt.Sprintf("%s%s%s", assetCode, assetIssuer, assetType)
	hash := farm.Fingerprint64([]byte(asset))

	return int64(hash)
}

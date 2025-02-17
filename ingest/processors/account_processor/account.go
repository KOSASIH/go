package account

import (
	"fmt"
	"time"

	"github.com/guregu/null"
	"github.com/guregu/null/zero"
	"github.com/stellar/go/ingest"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// AccountOutput is a representation of an account that aligns with the BigQuery table accounts
type AccountOutput struct {
	AccountID            string      `json:"account_id"` // account address
	Balance              float64     `json:"balance"`
	BuyingLiabilities    float64     `json:"buying_liabilities"`
	SellingLiabilities   float64     `json:"selling_liabilities"`
	SequenceNumber       int64       `json:"sequence_number"`
	SequenceLedger       zero.Int    `json:"sequence_ledger"`
	SequenceTime         zero.Int    `json:"sequence_time"`
	NumSubentries        uint32      `json:"num_subentries"`
	InflationDestination string      `json:"inflation_destination"`
	Flags                uint32      `json:"flags"`
	HomeDomain           string      `json:"home_domain"`
	MasterWeight         int32       `json:"master_weight"`
	ThresholdLow         int32       `json:"threshold_low"`
	ThresholdMedium      int32       `json:"threshold_medium"`
	ThresholdHigh        int32       `json:"threshold_high"`
	Sponsor              null.String `json:"sponsor"`
	NumSponsored         uint32      `json:"num_sponsored"`
	NumSponsoring        uint32      `json:"num_sponsoring"`
	LastModifiedLedger   uint32      `json:"last_modified_ledger"`
	LedgerEntryChange    uint32      `json:"ledger_entry_change"`
	Deleted              bool        `json:"deleted"`
	ClosedAt             time.Time   `json:"closed_at"`
	LedgerSequence       uint32      `json:"ledger_sequence"`
}

// TransformAccount converts an account from the history archive ingestion system into a form suitable for BigQuery
func TransformAccount(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) (AccountOutput, error) {
	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return AccountOutput{}, err
	}

	accountEntry, accountFound := ledgerEntry.Data.GetAccount()
	if !accountFound {
		return AccountOutput{}, fmt.Errorf("could not extract account data from ledger entry; actual type is %s", ledgerEntry.Data.Type)
	}

	outputID, err := accountEntry.AccountId.GetAddress()
	if err != nil {
		return AccountOutput{}, err
	}

	outputBalance := accountEntry.Balance
	if outputBalance < 0 {
		return AccountOutput{}, fmt.Errorf("balance is negative (%d) for account: %s", outputBalance, outputID)
	}

	//The V1 struct is the first version of the extender from accountEntry. It contains information on liabilities, and in the future
	//more extensions may contain extra information
	accountExtensionInfo, V1Found := accountEntry.Ext.GetV1()
	var outputBuyingLiabilities, outputSellingLiabilities xdr.Int64
	if V1Found {
		liabilities := accountExtensionInfo.Liabilities
		outputBuyingLiabilities, outputSellingLiabilities = liabilities.Buying, liabilities.Selling
		if outputBuyingLiabilities < 0 {
			return AccountOutput{}, fmt.Errorf("the buying liabilities count is negative (%d) for account: %s", outputBuyingLiabilities, outputID)
		}

		if outputSellingLiabilities < 0 {
			return AccountOutput{}, fmt.Errorf("the selling liabilities count is negative (%d) for account: %s", outputSellingLiabilities, outputID)
		}
	}

	outputSequenceNumber := int64(accountEntry.SeqNum)
	if outputSequenceNumber < 0 {
		return AccountOutput{}, fmt.Errorf("account sequence number is negative (%d) for account: %s", outputSequenceNumber, outputID)
	}
	outputSequenceLedger := accountEntry.SeqLedger()
	outputSequenceTime := accountEntry.SeqTime()

	outputNumSubentries := uint32(accountEntry.NumSubEntries)

	inflationDestAccountID := accountEntry.InflationDest
	var outputInflationDest string
	if inflationDestAccountID != nil {
		outputInflationDest, err = inflationDestAccountID.GetAddress()
		if err != nil {
			return AccountOutput{}, err
		}
	}

	outputFlags := uint32(accountEntry.Flags)

	outputHomeDomain := string(accountEntry.HomeDomain)

	outputMasterWeight := int32(accountEntry.MasterKeyWeight())
	outputThreshLow := int32(accountEntry.ThresholdLow())
	outputThreshMed := int32(accountEntry.ThresholdMedium())
	outputThreshHigh := int32(accountEntry.ThresholdHigh())

	outputLastModifiedLedger := uint32(ledgerEntry.LastModifiedLedgerSeq)

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return AccountOutput{}, err
	}

	ledgerSequence := header.Header.LedgerSeq

	transformedAccount := AccountOutput{
		AccountID:            outputID,
		Balance:              utils.ConvertStroopValueToReal(outputBalance),
		BuyingLiabilities:    utils.ConvertStroopValueToReal(outputBuyingLiabilities),
		SellingLiabilities:   utils.ConvertStroopValueToReal(outputSellingLiabilities),
		SequenceNumber:       outputSequenceNumber,
		SequenceLedger:       zero.IntFrom(int64(outputSequenceLedger)),
		SequenceTime:         zero.IntFrom(int64(outputSequenceTime)),
		NumSubentries:        outputNumSubentries,
		InflationDestination: outputInflationDest,
		Flags:                outputFlags,
		HomeDomain:           outputHomeDomain,
		MasterWeight:         outputMasterWeight,
		ThresholdLow:         outputThreshLow,
		ThresholdMedium:      outputThreshMed,
		ThresholdHigh:        outputThreshHigh,
		LastModifiedLedger:   outputLastModifiedLedger,
		Sponsor:              utils.LedgerEntrySponsorToNullString(ledgerEntry),
		NumSponsored:         uint32(accountEntry.NumSponsored()),
		NumSponsoring:        uint32(accountEntry.NumSponsoring()),
		LedgerEntryChange:    uint32(changeType),
		Deleted:              outputDeleted,
		ClosedAt:             closedAt,
		LedgerSequence:       uint32(ledgerSequence),
	}
	return transformedAccount, nil
}

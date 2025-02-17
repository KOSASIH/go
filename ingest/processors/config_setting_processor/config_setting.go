package configsetting

import (
	"fmt"
	"strconv"
	"time"

	"github.com/stellar/go/ingest"
	utils "github.com/stellar/go/ingest/processors/processor_utils"
	"github.com/stellar/go/xdr"
)

// ConfigSettingOutput is a representation of soroban config settings that aligns with the Bigquery table config_settings
type ConfigSettingOutput struct {
	ConfigSettingId                 int32               `json:"config_setting_id"`
	ContractMaxSizeBytes            uint32              `json:"contract_max_size_bytes"`
	LedgerMaxInstructions           int64               `json:"ledger_max_instructions"`
	TxMaxInstructions               int64               `json:"tx_max_instructions"`
	FeeRatePerInstructionsIncrement int64               `json:"fee_rate_per_instructions_increment"`
	TxMemoryLimit                   uint32              `json:"tx_memory_limit"`
	LedgerMaxReadLedgerEntries      uint32              `json:"ledger_max_read_ledger_entries"`
	LedgerMaxReadBytes              uint32              `json:"ledger_max_read_bytes"`
	LedgerMaxWriteLedgerEntries     uint32              `json:"ledger_max_write_ledger_entries"`
	LedgerMaxWriteBytes             uint32              `json:"ledger_max_write_bytes"`
	TxMaxReadLedgerEntries          uint32              `json:"tx_max_read_ledger_entries"`
	TxMaxReadBytes                  uint32              `json:"tx_max_read_bytes"`
	TxMaxWriteLedgerEntries         uint32              `json:"tx_max_write_ledger_entries"`
	TxMaxWriteBytes                 uint32              `json:"tx_max_write_bytes"`
	FeeReadLedgerEntry              int64               `json:"fee_read_ledger_entry"`
	FeeWriteLedgerEntry             int64               `json:"fee_write_ledger_entry"`
	FeeRead1Kb                      int64               `json:"fee_read_1kb"`
	BucketListTargetSizeBytes       int64               `json:"bucket_list_target_size_bytes"`
	WriteFee1KbBucketListLow        int64               `json:"write_fee_1kb_bucket_list_low"`
	WriteFee1KbBucketListHigh       int64               `json:"write_fee_1kb_bucket_list_high"`
	BucketListWriteFeeGrowthFactor  uint32              `json:"bucket_list_write_fee_growth_factor"`
	FeeHistorical1Kb                int64               `json:"fee_historical_1kb"`
	TxMaxContractEventsSizeBytes    uint32              `json:"tx_max_contract_events_size_bytes"`
	FeeContractEvents1Kb            int64               `json:"fee_contract_events_1kb"`
	LedgerMaxTxsSizeBytes           uint32              `json:"ledger_max_txs_size_bytes"`
	TxMaxSizeBytes                  uint32              `json:"tx_max_size_bytes"`
	FeeTxSize1Kb                    int64               `json:"fee_tx_size_1kb"`
	ContractCostParamsCpuInsns      []map[string]string `json:"contract_cost_params_cpu_insns"`
	ContractCostParamsMemBytes      []map[string]string `json:"contract_cost_params_mem_bytes"`
	ContractDataKeySizeBytes        uint32              `json:"contract_data_key_size_bytes"`
	ContractDataEntrySizeBytes      uint32              `json:"contract_data_entry_size_bytes"`
	MaxEntryTtl                     uint32              `json:"max_entry_ttl"`
	MinTemporaryTtl                 uint32              `json:"min_temporary_ttl"`
	MinPersistentTtl                uint32              `json:"min_persistent_ttl"`
	AutoBumpLedgers                 uint32              `json:"auto_bump_ledgers"`
	PersistentRentRateDenominator   int64               `json:"persistent_rent_rate_denominator"`
	TempRentRateDenominator         int64               `json:"temp_rent_rate_denominator"`
	MaxEntriesToArchive             uint32              `json:"max_entries_to_archive"`
	BucketListSizeWindowSampleSize  uint32              `json:"bucket_list_size_window_sample_size"`
	EvictionScanSize                uint64              `json:"eviction_scan_size"`
	StartingEvictionScanLevel       uint32              `json:"starting_eviction_scan_level"`
	LedgerMaxTxCount                uint32              `json:"ledger_max_tx_count"`
	BucketListSizeWindow            []uint64            `json:"bucket_list_size_window"`
	LastModifiedLedger              uint32              `json:"last_modified_ledger"`
	LedgerEntryChange               uint32              `json:"ledger_entry_change"`
	Deleted                         bool                `json:"deleted"`
	ClosedAt                        time.Time           `json:"closed_at"`
	LedgerSequence                  uint32              `json:"ledger_sequence"`
}

// TransformConfigSetting converts an config setting ledger change entry into a form suitable for BigQuery
func TransformConfigSetting(ledgerChange ingest.Change, header xdr.LedgerHeaderHistoryEntry) (ConfigSettingOutput, error) {
	ledgerEntry, changeType, outputDeleted, err := utils.ExtractEntryFromChange(ledgerChange)
	if err != nil {
		return ConfigSettingOutput{}, err
	}

	configSetting, ok := ledgerEntry.Data.GetConfigSetting()
	if !ok {
		return ConfigSettingOutput{}, fmt.Errorf("could not extract config setting from ledger entry; actual type is %s", ledgerEntry.Data.Type)
	}

	configSettingId := configSetting.ConfigSettingId

	contractMaxSizeBytes, _ := configSetting.GetContractMaxSizeBytes()

	contractCompute, _ := configSetting.GetContractCompute()
	ledgerMaxInstructions := contractCompute.LedgerMaxInstructions
	txMaxInstructions := contractCompute.TxMaxInstructions
	feeRatePerInstructionsIncrement := contractCompute.FeeRatePerInstructionsIncrement
	txMemoryLimit := contractCompute.TxMemoryLimit

	contractLedgerCost, _ := configSetting.GetContractLedgerCost()
	ledgerMaxReadLedgerEntries := contractLedgerCost.LedgerMaxReadLedgerEntries
	ledgerMaxReadBytes := contractLedgerCost.LedgerMaxReadBytes
	ledgerMaxWriteLedgerEntries := contractLedgerCost.LedgerMaxWriteLedgerEntries
	ledgerMaxWriteBytes := contractLedgerCost.LedgerMaxWriteBytes
	txMaxReadLedgerEntries := contractLedgerCost.TxMaxReadLedgerEntries
	txMaxReadBytes := contractLedgerCost.TxMaxReadBytes
	txMaxWriteLedgerEntries := contractLedgerCost.TxMaxWriteLedgerEntries
	txMaxWriteBytes := contractLedgerCost.TxMaxWriteBytes
	feeReadLedgerEntry := contractLedgerCost.FeeReadLedgerEntry
	feeWriteLedgerEntry := contractLedgerCost.FeeWriteLedgerEntry
	feeRead1Kb := contractLedgerCost.FeeRead1Kb
	bucketListTargetSizeBytes := contractLedgerCost.BucketListTargetSizeBytes
	writeFee1KbBucketListLow := contractLedgerCost.WriteFee1KbBucketListLow
	writeFee1KbBucketListHigh := contractLedgerCost.WriteFee1KbBucketListHigh
	bucketListWriteFeeGrowthFactor := contractLedgerCost.BucketListWriteFeeGrowthFactor

	contractHistoricalData, _ := configSetting.GetContractHistoricalData()
	feeHistorical1Kb := contractHistoricalData.FeeHistorical1Kb

	contractMetaData, _ := configSetting.GetContractEvents()
	txMaxContractEventsSizeBytes := contractMetaData.TxMaxContractEventsSizeBytes
	feeContractEvents1Kb := contractMetaData.FeeContractEvents1Kb

	contractBandwidth, _ := configSetting.GetContractBandwidth()
	ledgerMaxTxsSizeBytes := contractBandwidth.LedgerMaxTxsSizeBytes
	txMaxSizeBytes := contractBandwidth.TxMaxSizeBytes
	feeTxSize1Kb := contractBandwidth.FeeTxSize1Kb

	paramsCpuInsns, _ := configSetting.GetContractCostParamsCpuInsns()
	contractCostParamsCpuInsns := serializeParams(paramsCpuInsns)

	paramsMemBytes, _ := configSetting.GetContractCostParamsMemBytes()
	contractCostParamsMemBytes := serializeParams(paramsMemBytes)

	contractDataKeySizeBytes, _ := configSetting.GetContractDataKeySizeBytes()

	contractDataEntrySizeBytes, _ := configSetting.GetContractDataEntrySizeBytes()

	stateArchivalSettings, _ := configSetting.GetStateArchivalSettings()
	maxEntryTtl := stateArchivalSettings.MaxEntryTtl
	minTemporaryTtl := stateArchivalSettings.MinTemporaryTtl
	minPersistentTtl := stateArchivalSettings.MinPersistentTtl
	persistentRentRateDenominator := stateArchivalSettings.PersistentRentRateDenominator
	tempRentRateDenominator := stateArchivalSettings.TempRentRateDenominator
	maxEntriesToArchive := stateArchivalSettings.MaxEntriesToArchive
	bucketListSizeWindowSampleSize := stateArchivalSettings.BucketListSizeWindowSampleSize
	evictionScanSize := stateArchivalSettings.EvictionScanSize
	startingEvictionScanLevel := stateArchivalSettings.StartingEvictionScanLevel

	contractExecutionLanes, _ := configSetting.GetContractExecutionLanes()
	ledgerMaxTxCount := contractExecutionLanes.LedgerMaxTxCount

	bucketList, _ := configSetting.GetBucketListSizeWindow()
	bucketListSizeWindow := make([]uint64, 0, len(bucketList))
	for _, sizeWindow := range bucketList {
		bucketListSizeWindow = append(bucketListSizeWindow, uint64(sizeWindow))
	}

	closedAt, err := utils.TimePointToUTCTimeStamp(header.Header.ScpValue.CloseTime)
	if err != nil {
		return ConfigSettingOutput{}, err
	}

	ledgerSequence := header.Header.LedgerSeq

	transformedConfigSetting := ConfigSettingOutput{
		ConfigSettingId:                 int32(configSettingId),
		ContractMaxSizeBytes:            uint32(contractMaxSizeBytes),
		LedgerMaxInstructions:           int64(ledgerMaxInstructions),
		TxMaxInstructions:               int64(txMaxInstructions),
		FeeRatePerInstructionsIncrement: int64(feeRatePerInstructionsIncrement),
		TxMemoryLimit:                   uint32(txMemoryLimit),
		LedgerMaxReadLedgerEntries:      uint32(ledgerMaxReadLedgerEntries),
		LedgerMaxReadBytes:              uint32(ledgerMaxReadBytes),
		LedgerMaxWriteLedgerEntries:     uint32(ledgerMaxWriteLedgerEntries),
		LedgerMaxWriteBytes:             uint32(ledgerMaxWriteBytes),
		TxMaxReadLedgerEntries:          uint32(txMaxReadLedgerEntries),
		TxMaxReadBytes:                  uint32(txMaxReadBytes),
		TxMaxWriteLedgerEntries:         uint32(txMaxWriteLedgerEntries),
		TxMaxWriteBytes:                 uint32(txMaxWriteBytes),
		FeeReadLedgerEntry:              int64(feeReadLedgerEntry),
		FeeWriteLedgerEntry:             int64(feeWriteLedgerEntry),
		FeeRead1Kb:                      int64(feeRead1Kb),
		BucketListTargetSizeBytes:       int64(bucketListTargetSizeBytes),
		WriteFee1KbBucketListLow:        int64(writeFee1KbBucketListLow),
		WriteFee1KbBucketListHigh:       int64(writeFee1KbBucketListHigh),
		BucketListWriteFeeGrowthFactor:  uint32(bucketListWriteFeeGrowthFactor),
		FeeHistorical1Kb:                int64(feeHistorical1Kb),
		TxMaxContractEventsSizeBytes:    uint32(txMaxContractEventsSizeBytes),
		FeeContractEvents1Kb:            int64(feeContractEvents1Kb),
		LedgerMaxTxsSizeBytes:           uint32(ledgerMaxTxsSizeBytes),
		TxMaxSizeBytes:                  uint32(txMaxSizeBytes),
		FeeTxSize1Kb:                    int64(feeTxSize1Kb),
		ContractCostParamsCpuInsns:      contractCostParamsCpuInsns,
		ContractCostParamsMemBytes:      contractCostParamsMemBytes,
		ContractDataKeySizeBytes:        uint32(contractDataKeySizeBytes),
		ContractDataEntrySizeBytes:      uint32(contractDataEntrySizeBytes),
		MaxEntryTtl:                     uint32(maxEntryTtl),
		MinTemporaryTtl:                 uint32(minTemporaryTtl),
		MinPersistentTtl:                uint32(minPersistentTtl),
		PersistentRentRateDenominator:   int64(persistentRentRateDenominator),
		TempRentRateDenominator:         int64(tempRentRateDenominator),
		MaxEntriesToArchive:             uint32(maxEntriesToArchive),
		BucketListSizeWindowSampleSize:  uint32(bucketListSizeWindowSampleSize),
		EvictionScanSize:                uint64(evictionScanSize),
		StartingEvictionScanLevel:       uint32(startingEvictionScanLevel),
		LedgerMaxTxCount:                uint32(ledgerMaxTxCount),
		BucketListSizeWindow:            bucketListSizeWindow,
		LastModifiedLedger:              uint32(ledgerEntry.LastModifiedLedgerSeq),
		LedgerEntryChange:               uint32(changeType),
		Deleted:                         outputDeleted,
		ClosedAt:                        closedAt,
		LedgerSequence:                  uint32(ledgerSequence),
	}
	return transformedConfigSetting, nil
}

func serializeParams(costParams xdr.ContractCostParams) []map[string]string {
	params := make([]map[string]string, 0, len(costParams))
	for _, contractCostParam := range costParams {
		serializedParam := map[string]string{}
		serializedParam["ExtV"] = strconv.Itoa(int(contractCostParam.Ext.V))
		serializedParam["ConstTerm"] = strconv.Itoa(int(contractCostParam.ConstTerm))
		serializedParam["LinearTerm"] = strconv.Itoa(int(contractCostParam.LinearTerm))
		params = append(params, serializedParam)
	}

	return params
}

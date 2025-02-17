package datastore

import "strconv"

type MetaData struct {
	StartLedger          uint32
	EndLedger            uint32
	StartLedgerCloseTime int64
	EndLedgerCloseTime   int64
	ProtocolVersion      uint32
	CoreVersion          string
	NetworkPassPhrase    string
	CompressionType      string
	Version              string
}

func (m MetaData) ToMap() map[string]string {
	return map[string]string{
		"start-ledger":            strconv.FormatUint(uint64(m.StartLedger), 10),
		"end-ledger":              strconv.FormatUint(uint64(m.EndLedger), 10),
		"start-ledger-close-time": strconv.FormatInt(m.StartLedgerCloseTime, 10),
		"end-ledger-close-time":   strconv.FormatInt(m.EndLedgerCloseTime, 10),
		"protocol-version":        strconv.FormatInt(int64(m.ProtocolVersion), 10),
		"core-version":            m.CoreVersion,
		"network-passphrase":      m.NetworkPassPhrase,
		"compression-type":        m.CompressionType,
		"version":                 m.Version,
	}
}
func NewMetaDataFromMap(data map[string]string) (MetaData, error) {
	var metaData MetaData

	if val, ok := data["start-ledger"]; ok {
		startLedger, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return metaData, err
		}
		metaData.StartLedger = uint32(startLedger)
	}

	if val, ok := data["end-ledger"]; ok {
		endLedger, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return metaData, err
		}
		metaData.EndLedger = uint32(endLedger)
	}

	if val, ok := data["start-ledger-close-time"]; ok {
		startLedgerCloseTime, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return metaData, err
		}
		metaData.StartLedgerCloseTime = startLedgerCloseTime
	}

	if val, ok := data["end-ledger-close-time"]; ok {
		endLedgerCloseTime, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return metaData, err
		}
		metaData.EndLedgerCloseTime = endLedgerCloseTime
	}

	if val, ok := data["protocol-version"]; ok {
		protocolVersion, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return metaData, err
		}
		metaData.ProtocolVersion = uint32(protocolVersion)
	}

	metaData.CoreVersion = data["core-version"]
	metaData.NetworkPassPhrase = data["network-passphrase"]
	metaData.CompressionType = data["compression-type"]
	metaData.Version = data["version"]

	return metaData, nil
}

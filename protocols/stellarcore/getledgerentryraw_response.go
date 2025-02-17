package stellarcore

// GetLedgerEntryRawResponse is the structure of Stellar Core's /getledgerentryraw
type GetLedgerEntryRawResponse struct {
	Ledger  uint32                   `json:"ledger"`
	Entries []RawLedgerEntryResponse `json:"entries"`
}

type RawLedgerEntryResponse struct {
	Entry string `json:"le"` // base64-encoded xdr.LedgerEntry
}

package stellarcore

const (
	// Indicates that the entry is live in the current state
	LedgerEntryStateLive = "live"
	// Indicates that the entry is proven to be brand new and will live in the
	// current state when created. In this case, the `Entry` field will be an
	// xdr.LedgerKey matching the one requested rather than an xdr.LedgerEntry.
	LedgerEntryStateNew = "new"
	// Indicates that the entry has been archived due to its TTL but still lives
	// in the live state
	LedgerEntryStateArchived = "archived"
	// Indicates that the entry has been evicted from the live state and now
	// lives in the "hot archive" state.
	LedgerEntryStateEvicted = "evicted"
)

// GetLedgerEntryResponse is the structure of Stellar Core's /getledgerentry
type GetLedgerEntryResponse struct {
	Ledger  uint32                `json:"ledger"`
	Entries []LedgerEntryResponse `json:"entries"`
}

type LedgerEntryResponse struct {
	Entry string `json:"e"`             // base64-encoded xdr.LedgerEntry, or xdr.LedgerKey if state == new
	State string `json:"state"`         // one of the above states
	Ttl   uint32 `json:"ttl,omitempty"` // optionally, a Soroban entry's `liveUntilLedgerSeq`
}

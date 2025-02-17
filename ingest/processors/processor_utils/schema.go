package processor_utils

import (
	"github.com/stellar/go/historyarchive"
	"github.com/stellar/go/xdr"
)

// Claimants
type Claimant struct {
	Destination string             `json:"destination"`
	Predicate   xdr.ClaimPredicate `json:"predicate"`
}

// Price represents the price of an asset as a fraction
type Price struct {
	Numerator   int32 `json:"n"`
	Denominator int32 `json:"d"`
}

// Path is a representation of an asset without an ID that forms part of a path in a path payment
type Path struct {
	AssetCode   string `json:"asset_code"`
	AssetIssuer string `json:"asset_issuer"`
	AssetType   string `json:"asset_type"`
}

type SponsorshipOutput struct {
	Operation      xdr.Operation
	OperationIndex uint32
}

// TestTransaction transaction meta
type TestTransaction struct {
	Index         uint32
	EnvelopeXDR   string
	ResultXDR     string
	FeeChangesXDR string
	MetaXDR       string
	Hash          string
}

type HistoryArchiveLedgerAndLCM struct {
	Ledger historyarchive.Ledger
	LCM    xdr.LedgerCloseMeta
}

package ingest

import (
	"context"

	"github.com/stellar/go/historyarchive"
	"github.com/stellar/go/ingest"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

// historyArchiveAdapter is an adapter for the historyarchive package to read from history archives
type historyArchiveAdapter struct {
	archive historyarchive.ArchiveInterface
}

type verifiableChangeReader interface {
	ingest.ChangeReader
	VerifyBucketList(expectedHash xdr.Hash) error
}

type historyArchiveAdapterInterface interface {
	GetLatestLedgerSequence() (uint32, error)
	GetState(ctx context.Context, sequence uint32) (verifiableChangeReader, error)
	GetStats() []historyarchive.ArchiveStats
}

// newHistoryArchiveAdapter is a constructor to make a historyArchiveAdapter
func newHistoryArchiveAdapter(archive historyarchive.ArchiveInterface) historyArchiveAdapterInterface {
	return &historyArchiveAdapter{archive: archive}
}

// GetLatestLedgerSequence returns the latest ledger sequence or an error
func (haa *historyArchiveAdapter) GetLatestLedgerSequence() (uint32, error) {
	has, err := haa.archive.GetRootHAS()
	if err != nil {
		return 0, errors.Wrap(err, "could not get root HAS")
	}

	return has.CurrentLedger, nil
}

// GetState returns a reader with the state of the ledger at the provided sequence number.
func (haa *historyArchiveAdapter) GetState(ctx context.Context, sequence uint32) (verifiableChangeReader, error) {
	exists, err := haa.archive.CategoryCheckpointExists("history", sequence)
	if err != nil {
		return nil, errors.Wrap(err, "error checking if category checkpoint exists")
	}
	if !exists {
		return nil, errors.Errorf("history checkpoint does not exist for ledger %d", sequence)
	}

	sr, e := ingest.NewCheckpointChangeReader(ctx, haa.archive, sequence)
	if e != nil {
		return nil, errors.Wrap(e, "could not make memory state reader")
	}

	return sr, nil
}

func (haa *historyArchiveAdapter) GetStats() []historyarchive.ArchiveStats {
	return haa.archive.GetStats()
}

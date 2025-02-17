package datastore

import (
	"context"
	"sort"

	"github.com/pkg/errors"

	"github.com/stellar/go/historyarchive"
	"github.com/stellar/go/support/log"
)

type ResumableManager interface {
	// Given a requested ledger range, return the first absent ledger within the
	// requested range of [start, end].
	//
	// start - begin search inclusive from this ledger, must be greater than 0.
	// end   - stop search inclusive to this ledger.
	//
	// If start=0, invalid, error will be returned.
	//
	// If end=0, is provided as a convenience, to allow requesting an effectively
	// dynamic end value for the range, which will be an approximation of the network's
	// most recent checkpointed ledger + (2 * checkpoint_frequency).
	//
	// return:
	// absentLedger      - will be non-zero, the oldest ledger sequence between range of [start, end]
	//                     which is not populated on data store.
	// ok                - if true, 'absentLedger' has a usable non-zero value, if false, there is no absent ledger in the requested range and 'absentLedger' is set to zero.
	// err               - the search was cancelled due to this unexpected error, 'absentLedger' and 'ok' return values should be ignored.
	//
	// When no error, the two return values will compose the following truth table:
	//    1. datastore had no data in the requested range: absentLedger={start}, ok=true
	//    2. datastore had partial data in the requested range: absentLedger={a value > start and <= end}, ok=true
	//    3. datastore had all data in the requested range: absentLedger=0, ok=false
	FindStart(ctx context.Context, start, end uint32) (absentLedger uint32, ok bool, err error)
}

type resumableManagerService struct {
	ledgerBatchConfig DataStoreSchema
	dataStore         DataStore
	archive           historyarchive.ArchiveInterface
}

func NewResumableManager(dataStore DataStore,
	ledgerBatchConfig DataStoreSchema,
	archive historyarchive.ArchiveInterface) ResumableManager {
	return &resumableManagerService{
		ledgerBatchConfig: ledgerBatchConfig,
		dataStore:         dataStore,
		archive:           archive,
	}
}

func (rm resumableManagerService) FindStart(ctx context.Context, start, end uint32) (absentLedger uint32, ok bool, err error) {
	if start < 1 {
		return 0, false, errors.New("Invalid start value, must be greater than zero")
	}

	log.WithField("start", start).WithField("end", end)

	networkLatest := uint32(0)
	if end < 1 {
		var latestErr error
		networkLatest, latestErr = rm.archive.GetLatestLedgerSequence()
		if latestErr != nil {
			err := errors.Wrap(latestErr, "Resumability of requested export ledger range, was not able to get latest ledger from network")
			return 0, false, err
		}
		networkLatest = networkLatest + (rm.archive.GetCheckpointManager().GetCheckpointFrequency() * 2)
		log.Infof("Resumability computed effective latest network ledger including padding of checkpoint frequency to be %d", networkLatest)

		if start > networkLatest {
			// requested to start at a point beyond the latest network, resume not applicable.
			return 0, false, errors.Errorf("Invalid start value of %v, it is greater than network's latest ledger of %v", start, networkLatest)
		}
		end = networkLatest
	} else if end >= rm.ledgerBatchConfig.LedgersPerFile {
		// Adjacent ranges may end up overlapping due to the clamping behavior in adjustLedgerRange()
		// https://github.com/stellar/go/blob/fff01229a5af77dee170a37bf0c71b2ce8bb8474/exp/services/ledgerexporter/internal/config.go#L173-L192
		// For example, assuming 64 ledgers per file, [2, 100] and [101, 150] get adjusted to [2, 127] and [64, 191]
		// If we export [64, 191] and then try to resume on [2, 127], the binary search logic will determine that
		// [2, 127] is fully exported because the midpoint of [2, 127] is present.
		// To fix this issue we query the end ledger and if it is present, we only do the binary search on the
		// preceding sub range. This will allow resumability to work on adjacent ranges that end up overlapping
		// due to adjustLedgerRange().
		// Note that if there is an overlap the size of the overlap will never be larger than the number of files
		// per partition and that is why it is sufficient to only check if the end ledger is present.
		exists, err := rm.dataStore.Exists(ctx, rm.ledgerBatchConfig.GetObjectKeyFromSequenceNumber(end))
		if err != nil {
			return 0, false, err
		}
		if exists {
			end -= rm.ledgerBatchConfig.LedgersPerFile
			if start > end {
				// data store had all ledgers for requested range, no resumability needed.
				log.Infof("Resumability found no absent object keys in requested ledger range")
				return 0, false, nil
			}
		}
	}

	rangeSize := max(int(end-start), 1)
	var binarySearchError error
	lowestAbsentIndex := sort.Search(rangeSize, binarySearchCallbackFn(&rm, ctx, start, end, &binarySearchError))
	if binarySearchError != nil {
		return 0, false, binarySearchError
	}

	if lowestAbsentIndex < int(rangeSize) {
		nearestAbsentLedgerSequence := start + uint32(lowestAbsentIndex)
		log.Infof("Resumability determined next absent object start key of %d for requested export ledger range", nearestAbsentLedgerSequence)
		return nearestAbsentLedgerSequence, true, nil
	}

	// unbounded, and datastore had up to latest network, return that as staring point.
	if networkLatest > 0 {
		return networkLatest, true, nil
	}

	// data store had all ledgers for requested range, no resumability needed.
	log.Infof("Resumability found no absent object keys in requested ledger range")
	return 0, false, nil
}

func binarySearchCallbackFn(rm *resumableManagerService, ctx context.Context, start, end uint32, binarySearchError *error) func(ledgerSequence int) bool {
	lookupCache := map[string]bool{}

	return func(binarySearchIndex int) bool {
		if *binarySearchError != nil {
			// an error has already occured in a callback for the same binary search, exiting
			return true
		}
		objectKeyMiddle := rm.ledgerBatchConfig.GetObjectKeyFromSequenceNumber(start + uint32(binarySearchIndex))

		// there may be small occurrence of repeated queries on same object key once
		// search narrows down to a range that fits within the ledgers per file
		// worst case being 'log of ledgers_per_file' queries.
		middleFoundOnStore, foundInCache := lookupCache[objectKeyMiddle]
		if !foundInCache {
			var datastoreErr error
			middleFoundOnStore, datastoreErr = rm.dataStore.Exists(ctx, objectKeyMiddle)
			if datastoreErr != nil {
				*binarySearchError = errors.Wrapf(datastoreErr, "While searching datastore for resumability within export ledger range start=%d, end=%d, was not able to check if object key %v exists on data store", start, end, objectKeyMiddle)
				return true
			}
			lookupCache[objectKeyMiddle] = middleFoundOnStore
		}
		return !middleFoundOnStore
	}
}

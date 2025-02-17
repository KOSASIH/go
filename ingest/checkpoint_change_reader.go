package ingest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/stellar/go/historyarchive"
	"github.com/stellar/go/support/collections/set"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

// readResult is the result of reading a bucket value
type readResult struct {
	entryChange xdr.LedgerEntryChange
	e           error
}

// CheckpointChangeReader is a ChangeReader which returns Changes from a history archive
// snapshot. The Changes produced by a CheckpointChangeReader reflect the state of the Stellar
// network at a particular checkpoint ledger sequence.
type CheckpointChangeReader struct {
	ctx               context.Context
	has               *historyarchive.HistoryArchiveState
	archive           historyarchive.ArchiveInterface
	visitedLedgerKeys set.Set[string]
	sequence          uint32
	readChan          chan readResult
	streamOnce        sync.Once
	closeOnce         sync.Once
	done              chan bool

	readBytesMutex sync.RWMutex
	totalRead      int64
	totalSize      int64

	encodingBuffer *xdr.EncodingBuffer

	// This should be set to true in tests only
	disableBucketListHashValidation bool
	sleep                           func(time.Duration)
}

// Ensure CheckpointChangeReader implements ChangeReader
var _ ChangeReader = &CheckpointChangeReader{}

const (
	// maxStreamRetries defines how many times should we retry when there are errors in
	// the xdr stream returned by GetXdrStreamForHash().
	maxStreamRetries = 3
	msrBufferSize    = 50000
)

// NewCheckpointChangeReader constructs a new CheckpointChangeReader instance.
//
// The ledger sequence must be a checkpoint ledger. By default (see
// `historyarchive.ConnectOptions.CheckpointFrequency` for configuring this),
// its next sequence number would have to be a multiple of 64, e.g.
// sequence=100031 is a checkpoint ledger, since: (100031+1) mod 64 == 0
func NewCheckpointChangeReader(
	ctx context.Context,
	archive historyarchive.ArchiveInterface,
	sequence uint32,
) (*CheckpointChangeReader, error) {
	manager := archive.GetCheckpointManager()

	// The nth ledger is a checkpoint ledger iff: n+1 mod f == 0, where f is the
	// checkpoint frequency (64 by default).
	if !manager.IsCheckpoint(sequence) {
		return nil, errors.Errorf(
			"%d is not a checkpoint ledger, try %d or %d "+
				"(in general, try n where n+1 mod %d == 0)",
			sequence, manager.PrevCheckpoint(sequence),
			manager.NextCheckpoint(sequence),
			manager.GetCheckpointFrequency())
	}

	has, err := archive.GetCheckpointHAS(sequence)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get checkpoint HAS at ledger sequence %d", sequence)
	}

	return &CheckpointChangeReader{
		ctx:               ctx,
		has:               &has,
		archive:           archive,
		visitedLedgerKeys: set.Set[string]{},
		sequence:          sequence,
		readChan:          make(chan readResult, msrBufferSize),
		streamOnce:        sync.Once{},
		closeOnce:         sync.Once{},
		done:              make(chan bool),
		encodingBuffer:    xdr.NewEncodingBuffer(),
		sleep:             time.Sleep,
	}, nil
}

// VerifyBucketList verifies that the bucket list hash computed from the history archive snapshot
// associated with the CheckpointChangeReader matches the expectedHash.
// Assuming expectedHash comes from a trusted source (captive-core running in unbounded mode), this
// check will give you full security that the data returned by the CheckpointChangeReader can be trusted.
// Note that Stream will verify all the ledger entries from an individual bucket and
// VerifyBucketList() verifies the entire list of bucket hashes.
func (r *CheckpointChangeReader) VerifyBucketList(expectedHash xdr.Hash) error {
	historyBucketListHash, err := r.has.BucketListHash()
	if err != nil {
		return errors.Wrap(err, "Error getting bucket list hash")
	}

	if !bytes.Equal(historyBucketListHash[:], expectedHash[:]) {
		return fmt.Errorf(
			"bucket list hash of history archive does not match expected hash: %#x %#x",
			historyBucketListHash,
			expectedHash,
		)
	}

	return nil
}

func (r *CheckpointChangeReader) bucketExists(hash historyarchive.Hash) (bool, error) {
	return r.archive.BucketExists(hash)
}

// streamBuckets is internal method that streams buckets from the given HAS.
//
// Buckets should be processed from oldest to newest, `snap` and then `curr` at
// each level. The correct value of ledger entry is the latest seen
// `INITENTRY`/`LIVEENTRY` except the case when there's a `DEADENTRY` later
// which removes the entry.
//
// We can implement trivial algorithm (processing from oldest to newest buckets)
// but it requires to keep map of all entries in memory and stream what's left
// when all buckets are processed.
//
// However, we can modify this algorithm to work from newest to oldest ledgers:
//
//  1. For each `INITENTRY`/`LIVEENTRY` we check if we've seen the key before
//     (stored in `visitedLedgerKeys`). If the key hasn't been seen, we write that bucket
//     entry to the stream and add it to the `visitedLedgerKeys` (we don't mark `INITENTRY`,
//     see the inline comment or CAP-20).
//  2. For each `DEADENTRY` we keep track of removed bucket entries in
//     `visitedLedgerKeys` map.
//
// In such algorithm we just need to store a set of keys that require much less space.
// The memory requirements will be lowered when CAP-0020 is live and older buckets are
// rewritten. Then, we will only need to keep track of `DEADENTRY`.
func (r *CheckpointChangeReader) streamBuckets() {
	defer func() {
		r.visitedLedgerKeys = nil

		r.closeOnce.Do(r.close)
		close(r.readChan)
	}()

	var buckets []historyarchive.Hash
	for i := 0; i < len(r.has.CurrentBuckets); i++ {
		b := r.has.CurrentBuckets[i]
		for _, hashString := range []string{b.Curr, b.Snap} {
			hash, err := historyarchive.DecodeHash(hashString)
			if err != nil {
				r.readChan <- r.error(errors.Wrap(err, "Error decoding bucket hash"))
				return
			}

			if hash.IsZero() {
				continue
			}

			buckets = append(buckets, hash)
		}
	}

	for _, hash := range buckets {
		exists, err := r.bucketExists(hash)
		if err != nil {
			r.readChan <- r.error(
				errors.Wrapf(err, "error checking if bucket exists: %s", hash),
			)
			return
		}

		if !exists {
			r.readChan <- r.error(
				errors.Errorf("bucket hash does not exist: %s", hash),
			)
			return
		}

		size, err := r.archive.BucketSize(hash)
		if err != nil {
			r.readChan <- r.error(
				errors.Wrapf(err, "error checking bucket size: %s", hash),
			)
			return
		}

		r.readBytesMutex.Lock()
		r.totalSize += size
		r.readBytesMutex.Unlock()
	}

	for i, hash := range buckets {
		oldestBucket := i == len(buckets)-1
		if shouldContinue := r.streamBucketContents(hash, oldestBucket); !shouldContinue {
			break
		}
	}
}

// readBucketEntry will attempt to read a bucket entry from `stream`.
// If any errors are encountered while reading from `stream`, readBucketEntry will
// retry the operation using a new *historyarchive.XdrStream.
// The total number of retries will not exceed `maxStreamRetries`.
func (r *CheckpointChangeReader) readBucketEntry(stream *xdr.Stream, hash historyarchive.Hash) (
	xdr.BucketEntry,
	error,
) {
	var entry xdr.BucketEntry
	var err error
	currentPosition := stream.BytesRead()
	gzipCurrentPosition := stream.CompressedBytesRead()

	for attempts := 0; ; attempts++ {
		if r.ctx.Err() != nil {
			err = r.ctx.Err()
			break
		}
		if err == nil {
			err = stream.ReadOne(&entry)
			if err == nil || err == io.EOF {
				r.readBytesMutex.Lock()
				r.totalRead += stream.CompressedBytesRead() - gzipCurrentPosition
				r.readBytesMutex.Unlock()
				break
			}
		}
		if attempts >= maxStreamRetries {
			break
		}

		stream.Close()

		var retryStream *xdr.Stream
		retryStream, err = r.newXDRStream(hash)
		if err != nil {
			err = errors.Wrap(err, "Error creating new xdr stream")
			continue
		}

		*stream = *retryStream

		_, err = stream.Discard(currentPosition)
		if err != nil {
			err = errors.Wrap(err, "Error discarding from xdr stream")
			continue
		}
	}

	return entry, err
}

func (r *CheckpointChangeReader) newXDRStream(hash historyarchive.Hash) (
	*xdr.Stream,
	error,
) {
	rdr, e := r.archive.GetXdrStreamForHash(hash)
	if e == nil && !r.disableBucketListHashValidation {
		// Calling SetExpectedHash will enable validation of the stream hash. If hashes
		// don't match, rdr.Close() will return an error.
		rdr.SetExpectedHash(hash)
	}

	return rdr, e
}

// streamBucketContents pushes value onto the read channel, returning false when the channel needs to be closed otherwise true
func (r *CheckpointChangeReader) streamBucketContents(hash historyarchive.Hash, oldestBucket bool) bool {
	rdr, e := r.newXDRStream(hash)
	if e != nil {
		r.readChan <- r.error(
			errors.Wrapf(e, "cannot get xdr stream for hash '%s'", hash.String()),
		)
		return false
	}

	defer func() {
		err := rdr.Close()
		if err != nil {
			r.readChan <- r.error(errors.Wrap(err, "Error closing xdr stream"))
			// Stop streaming from the rest of the files.
			r.Close()
		}
	}()

	// bucketProtocolVersion is a protocol version read from METAENTRY or 0 when no METAENTRY.
	// No METAENTRY means that bucket originates from before protocol version 11.
	bucketProtocolVersion := uint32(0)

	for n := 0; ; n++ {
		var entry xdr.BucketEntry
		entry, e = r.readBucketEntry(rdr, hash)
		if e != nil {
			if e == io.EOF {
				// No entries loaded for this batch, nothing more to process
				return true
			}
			r.readChan <- r.error(
				errors.Wrapf(e, "Error on XDR record %d of hash '%s'", n, hash.String()),
			)
			return false
		}

		if entry.Type == xdr.BucketEntryTypeMetaentry {
			if n != 0 {
				r.readChan <- r.error(
					errors.Errorf(
						"METAENTRY not the first entry (n=%d) in the bucket hash '%s'",
						n, hash.String(),
					),
				)
				return false
			}
			// We can't use MustMetaEntry() here. Check:
			// https://github.com/golang/go/issues/32560
			bucketProtocolVersion = uint32(entry.MetaEntry.LedgerVersion)
			continue
		}

		var key xdr.LedgerKey
		var err error

		switch entry.Type {
		case xdr.BucketEntryTypeLiveentry, xdr.BucketEntryTypeInitentry:
			liveEntry := entry.MustLiveEntry()
			key, err = liveEntry.LedgerKey()
			if err != nil {
				r.readChan <- r.error(
					errors.Wrapf(err, "Error generating ledger key for XDR record %d of hash '%s'", n, hash.String()),
				)
				return false
			}
		case xdr.BucketEntryTypeDeadentry:
			key = entry.MustDeadEntry()
		default:
			r.readChan <- r.error(
				errors.Errorf("Unknown BucketEntryType=%d: %d@%s", entry.Type, n, hash.String()),
			)
			return false
		}

		// We're using compressed keys here
		// Safe, since we are converting to string right away
		keyBytes, e := r.encodingBuffer.LedgerKeyUnsafeMarshalBinaryCompress(key)
		if e != nil {
			r.readChan <- r.error(
				errors.Wrapf(
					e, "Error marshaling XDR record %d of hash '%s'", n, hash.String(),
				),
			)
			return false
		}

		h := string(keyBytes)
		// claimable balances and offers have unique ids
		// once a claimable balance or offer is created we can assume that
		// the id can never be recreated again, unlike, for example, trustlines
		// which can be deleted and then recreated
		unique := key.Type == xdr.LedgerEntryTypeClaimableBalance ||
			key.Type == xdr.LedgerEntryTypeOffer

		switch entry.Type {
		case xdr.BucketEntryTypeLiveentry, xdr.BucketEntryTypeInitentry:
			if entry.Type == xdr.BucketEntryTypeInitentry && bucketProtocolVersion < 11 {
				r.readChan <- r.error(
					errors.Errorf("Read INITENTRY from version <11 bucket: %d@%s", n, hash.String()),
				)
				return false
			}

			if !r.visitedLedgerKeys.Contains(h) {
				// Return LEDGER_ENTRY_STATE changes only now.
				liveEntry := entry.MustLiveEntry()
				entryChange := xdr.LedgerEntryChange{
					Type:  xdr.LedgerEntryChangeTypeLedgerEntryState,
					State: &liveEntry,
				}
				r.readChan <- readResult{entryChange, nil}

				// We don't update `visitedLedgerKeys` for INITENTRY because CAP-20 says:
				// > a bucket entry marked INITENTRY implies that either no entry
				// > with the same ledger key exists in an older bucket, or else
				// > that the (chronologically) preceding entry with the same ledger
				// > key was DEADENTRY.
				if entry.Type == xdr.BucketEntryTypeLiveentry {
					// We skip adding entries from the last bucket to visitedLedgerKeys because:
					// 1. Ledger keys are unique within a single bucket.
					// 2. This is the last bucket we process so there's no need to track
					//    seen last entries in this bucket.
					if oldestBucket {
						continue
					}
					r.visitedLedgerKeys.Add(h)
				}
			} else if entry.Type == xdr.BucketEntryTypeInitentry && unique {
				// we can remove the ledger key because we know that it's unique in the ledger
				// and cannot be recreated
				r.visitedLedgerKeys.Remove(h)
			}
		case xdr.BucketEntryTypeDeadentry:
			r.visitedLedgerKeys.Add(h)
		default:
			r.readChan <- r.error(
				errors.Errorf("Unexpected entry type %d: %d@%s", entry.Type, n, hash.String()),
			)
			return false
		}

		select {
		case <-r.done:
			// Close() called: stop processing buckets.
			return false
		default:
			continue
		}
	}

	panic("Shouldn't happen")
}

// Read returns a new ledger entry change on each call, returning io.EOF when the stream ends.
func (r *CheckpointChangeReader) Read() (Change, error) {
	r.streamOnce.Do(func() {
		go r.streamBuckets()
	})

	// blocking call. anytime we consume from this channel, the background goroutine will stream in the next value
	result, ok := <-r.readChan
	if !ok {
		// when channel is closed then return io.EOF
		return Change{}, io.EOF
	}

	if result.e != nil {
		return Change{}, errors.Wrap(result.e, "Error while reading from buckets")
	}
	entryType, err := result.entryChange.EntryType()
	if err != nil {
		return Change{}, errors.Wrap(err, "Error getting entry type")
	}
	return Change{
		Type: entryType,
		Post: result.entryChange.State,
	}, nil
}

func (r *CheckpointChangeReader) error(err error) readResult {
	return readResult{xdr.LedgerEntryChange{}, err}
}

func (r *CheckpointChangeReader) close() {
	close(r.done)
}

// Progress returns progress reading all buckets in percents.
func (r *CheckpointChangeReader) Progress() float64 {
	r.readBytesMutex.RLock()
	defer r.readBytesMutex.RUnlock()
	return float64(r.totalRead) / float64(r.totalSize) * 100
}

// Close should be called when reading is finished.
func (r *CheckpointChangeReader) Close() error {
	r.closeOnce.Do(r.close)
	return nil
}

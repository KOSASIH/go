package loadtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/xdr"
)

// LedgerBackend is used to load test ingestion.
// LedgerBackend will take a file of synthetically generated ledgers (see
// services/horizon/internal/integration/generate_ledgers_test.go) and merge those ledgers
// with real ledgers from the Stellar network. The merged ledgers will then be replayed to
// the ingesting down stream system at a configurable rate.
type LedgerBackend struct {
	config                LedgerBackendConfig
	mergedLedgersFilePath string
	mergedLedgersStream   *xdr.Stream
	startTime             time.Time
	startLedgerSeq        uint32
	nextLedgerSeq         uint32
	latestLedgerSeq       uint32
	preparedRange         ledgerbackend.Range
	cachedLedger          xdr.LedgerCloseMeta
}

// LedgerBackendConfig configures LedgerBackend
type LedgerBackendConfig struct {
	// NetworkPassphrase is the passphrase of the Stellar network from where the real ledgers
	// will be obtained
	NetworkPassphrase string
	// LedgerBackend is the source of the real ledgers
	LedgerBackend ledgerbackend.LedgerBackend
	// LedgersFilePath is a file containing the synthetic ledgers that will be combined with the
	// real ledgers and then replayed by LedgerBackend
	LedgersFilePath string
	// LedgerEntriesFilePath is a file containing the ledger entry fixtures for the synthetic ledgers
	LedgerEntriesFilePath string
	// LedgerCloseDuration is the rate at which ledgers will be replayed from LedgerBackend
	LedgerCloseDuration time.Duration
}

// NewLedgerBackend constructs an LedgerBackend instance
func NewLedgerBackend(config LedgerBackendConfig) *LedgerBackend {
	return &LedgerBackend{
		config: config,
	}
}

func (r *LedgerBackend) GetLatestLedgerSequence(ctx context.Context) (uint32, error) {
	if r.nextLedgerSeq == 0 {
		return 0, fmt.Errorf("PrepareRange() must be called before GetLatestLedgerSequence()")
	}

	return r.latestLedgerSeq, nil
}

func readLedgerEntries(path string) ([]xdr.LedgerEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	stream, err := xdr.NewZstdStream(file)
	if err != nil {
		return nil, err
	}

	var entries []xdr.LedgerEntry
	for {
		var entry xdr.LedgerEntry
		err = stream.ReadOne(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err = stream.Close(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *LedgerBackend) PrepareRange(ctx context.Context, ledgerRange ledgerbackend.Range) error {
	if r.nextLedgerSeq != 0 {
		if r.isPrepared(ledgerRange) {
			return nil
		}
		return fmt.Errorf("PrepareRange() already called")
	}
	generatedLedgerEntries, err := readLedgerEntries(r.config.LedgerEntriesFilePath)
	if err != nil {
		return err
	}
	generatedLedgersFile, err := os.Open(r.config.LedgersFilePath)
	if err != nil {
		return err
	}
	generatedLedgers, err := xdr.NewZstdStream(generatedLedgersFile)
	if err != nil {
		return err
	}

	err = r.config.LedgerBackend.PrepareRange(ctx, ledgerRange)
	if err != nil {
		return err
	}
	cur := ledgerRange.From()
	firstLedger, err := r.config.LedgerBackend.GetLedger(ctx, cur)
	if err != nil {
		return err
	}
	var changes xdr.LedgerEntryChanges
	for i := 0; i < len(generatedLedgerEntries); i++ {
		changes = append(changes, xdr.LedgerEntryChange{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryCreated,
			Created: &generatedLedgerEntries[i],
		})
	}
	var flag xdr.Uint32 = 1
	firstLedger.V1.UpgradesProcessing = append(firstLedger.V1.UpgradesProcessing, xdr.UpgradeEntryMeta{
		Upgrade: xdr.LedgerUpgrade{
			Type:     xdr.LedgerUpgradeTypeLedgerUpgradeFlags,
			NewFlags: &flag,
		},
		Changes: changes,
	})

	mergedLedgersFile, err := os.CreateTemp("", "merged-ledgers")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(mergedLedgersFile.Name())
		}
	}()
	writer, err := zstd.NewWriter(mergedLedgersFile)
	if err != nil {
		return err
	}

	var latestLedgerSeq uint32
	for cur = cur + 1; !ledgerRange.Bounded() || cur <= ledgerRange.To(); cur++ {
		var ledger xdr.LedgerCloseMeta
		ledger, err = r.config.LedgerBackend.GetLedger(ctx, cur)
		if err != nil {
			return err
		}
		var generatedLedger xdr.LedgerCloseMeta
		if err = generatedLedgers.ReadOne(&generatedLedger); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if err = MergeLedgers(r.config.NetworkPassphrase, &ledger, generatedLedger); err != nil {
			return err
		}
		if err = xdr.MarshalFramed(writer, ledger); err != nil {
			return err
		}
		latestLedgerSeq = cur
	}
	if err = generatedLedgers.Close(); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	if err = mergedLedgersFile.Sync(); err != nil {
		return err
	}

	if _, err = mergedLedgersFile.Seek(0, 0); err != nil {
		return err
	}
	mergedLedgersStream, err := xdr.NewZstdStream(mergedLedgersFile)
	if err != nil {
		return err
	}
	cleanup = false

	r.mergedLedgersFilePath = mergedLedgersFile.Name()
	r.mergedLedgersStream = mergedLedgersStream
	// from this point, ledgers will be available at a rate of once
	// every r.ledgerCloseDuration time has elapsed
	r.startTime = time.Now()
	r.startLedgerSeq = ledgerRange.From()
	r.nextLedgerSeq = r.startLedgerSeq + 1
	r.latestLedgerSeq = latestLedgerSeq
	r.cachedLedger = firstLedger
	r.preparedRange = ledgerRange
	return nil
}

func (r *LedgerBackend) IsPrepared(ctx context.Context, ledgerRange ledgerbackend.Range) (bool, error) {
	return r.isPrepared(ledgerRange), nil
}

func (r *LedgerBackend) isPrepared(ledgerRange ledgerbackend.Range) bool {
	if r.nextLedgerSeq == 0 {
		return false
	}

	if r.preparedRange.Bounded() != ledgerRange.Bounded() {
		return false
	}

	if ledgerRange.From() < r.cachedLedger.LedgerSequence() {
		return false
	}

	return ledgerRange.From() >= r.cachedLedger.LedgerSequence() && ledgerRange.To() <= r.preparedRange.To()
}

func (r *LedgerBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	if r.nextLedgerSeq == 0 {
		return xdr.LedgerCloseMeta{}, fmt.Errorf("PrepareRange() must be called before GetLedger()")
	}
	if sequence < r.cachedLedger.LedgerSequence() {
		return xdr.LedgerCloseMeta{}, fmt.Errorf(
			"sequence number %v is behind the ledger stream sequence %d",
			sequence,
			r.cachedLedger.LedgerSequence(),
		)
	}
	for ; r.nextLedgerSeq <= sequence; r.nextLedgerSeq++ {
		var ledger xdr.LedgerCloseMeta
		if err := r.mergedLedgersStream.ReadOne(&ledger); err == io.EOF {
			return ledger, fmt.Errorf(
				"sequence number %v is greater than the latest ledger available",
				sequence,
			)
		} else if err != nil {
			return ledger, err
		}
		if ledger.LedgerSequence() != r.nextLedgerSeq {
			return ledger, fmt.Errorf(
				"unexpected ledger sequence (expected=%d actual=%d)",
				r.nextLedgerSeq,
				ledger.LedgerSequence(),
			)
		}
		r.cachedLedger = ledger
	}
	i := int(sequence - r.startLedgerSeq)
	// the i'th ledger will only be available after (i+1) * r.ledgerCloseDuration time has elapsed
	closeTime := r.startTime.Add(time.Duration(i+1) * r.config.LedgerCloseDuration)
	time.Sleep(time.Until(closeTime))
	return r.cachedLedger, nil
}

func (r *LedgerBackend) Close() error {
	if err := r.config.LedgerBackend.Close(); err != nil {
		return err
	}
	if r.mergedLedgersStream != nil {
		// closing the stream will also close the ledgers file
		if err := r.mergedLedgersStream.Close(); err != nil {
			return err
		}
		if err := os.Remove(r.mergedLedgersFilePath); err != nil {
			return err
		}
	}
	return nil
}

func validLedger(ledger xdr.LedgerCloseMeta) error {
	if _, ok := ledger.GetV1(); !ok {
		return fmt.Errorf("ledger version %v is not supported", ledger.V)
	}
	if _, ok := ledger.MustV1().TxSet.GetV1TxSet(); !ok {
		return fmt.Errorf("ledger txset %v is not supported", ledger.MustV1().TxSet.V)
	}
	return nil
}

func extractChanges(networkPassphrase string, changeMap map[string][]ingest.Change, ledger xdr.LedgerCloseMeta) error {
	reader, err := ingest.NewLedgerChangeReaderFromLedgerCloseMeta(networkPassphrase, ledger)
	if err != nil {
		return err
	}
	for {
		var change ingest.Change
		var ledgerKey xdr.LedgerKey
		var b64 string
		change, err = reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		ledgerKey, err = change.LedgerKey()
		if err != nil {
			return err
		}
		b64, err = ledgerKey.MarshalBinaryBase64()
		if err != nil {
			return err
		}
		changeMap[b64] = append(changeMap[b64], change)
	}
	return nil
}

func changeIsEqual(a, b ingest.Change) (bool, error) {
	if a.Type != b.Type || a.Reason != b.Reason {
		return false, nil
	}
	if a.Pre == nil {
		if b.Pre != nil {
			return false, nil
		}
	} else {
		if ok, err := xdr.Equals(a.Pre, b.Pre); err != nil || !ok {
			return ok, err
		}
	}
	if a.Post == nil {
		if b.Post != nil {
			return false, nil
		}
	} else {
		if ok, err := xdr.Equals(a.Post, b.Post); err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

func changesAreEqual(a, b map[string][]ingest.Change) (bool, error) {
	if len(a) != len(b) {
		return false, nil
	}
	for key, aChanges := range a {
		bChanges := b[key]
		if len(aChanges) != len(bChanges) {
			return false, nil
		}
		for i, aChange := range aChanges {
			bChange := bChanges[i]
			if ok, err := changeIsEqual(aChange, bChange); !ok || err != nil {
				return ok, err
			}
		}
	}
	return true, nil
}

// MergeLedgers merges two xdr.LedgerCloseMeta instances.
func MergeLedgers(networkPassphrase string, dst *xdr.LedgerCloseMeta, src xdr.LedgerCloseMeta) error {
	if err := validLedger(*dst); err != nil {
		return err
	}
	if err := validLedger(src); err != nil {
		return err
	}

	combinedChangesByKey := map[string][]ingest.Change{}
	if err := extractChanges(networkPassphrase, combinedChangesByKey, *dst); err != nil {
		return err
	}
	if err := extractChanges(networkPassphrase, combinedChangesByKey, src); err != nil {
		return err
	}

	// src is merged into dst by appending all the transactions from src into dst,
	// appending all the upgrades from src into dst, and appending all the evictions
	// from src into dst
	dst.V1.TxSet.V1TxSet.Phases = append(dst.V1.TxSet.V1TxSet.Phases, src.V1.TxSet.V1TxSet.Phases...)
	dst.V1.TxProcessing = append(dst.V1.TxProcessing, src.V1.TxProcessing...)
	dst.V1.UpgradesProcessing = append(dst.V1.UpgradesProcessing, src.V1.UpgradesProcessing...)
	dst.V1.EvictedTemporaryLedgerKeys = append(dst.V1.EvictedTemporaryLedgerKeys, src.V1.EvictedTemporaryLedgerKeys...)
	dst.V1.EvictedPersistentLedgerEntries = append(dst.V1.EvictedPersistentLedgerEntries, src.V1.EvictedPersistentLedgerEntries...)

	mergedChangesByKey := map[string][]ingest.Change{}
	if err := extractChanges(networkPassphrase, mergedChangesByKey, *dst); err != nil {
		return err
	}

	// a merge is valid if the ordered list of changes emitted by the merged ledger is equal to
	// the list of changes emitted by dst concatenated by the list of changes emitted by src, or
	// in other words:
	// extractChanges(merge(dst, src)) == concat(extractChanges(dst), extractChanges(src))
	if ok, err := changesAreEqual(combinedChangesByKey, mergedChangesByKey); err != nil {
		return err
	} else if !ok {
		return errors.New("order of changes are not preserved")
	}

	return nil
}

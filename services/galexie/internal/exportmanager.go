package galexie

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/support/datastore"
	"github.com/stellar/go/xdr"
)

type ExportManager struct {
	dataStoreSchema    datastore.DataStoreSchema
	ledgerBackend      ledgerbackend.LedgerBackend
	currentMetaArchive *xdr.LedgerCloseMetaBatch
	queue              UploadQueue
	latestLedgerMetric *prometheus.GaugeVec
	networkPassPhrase  string
	coreVersion        string
}

// NewExportManager creates a new ExportManager with the provided configuration.
func NewExportManager(dataStoreSchema datastore.DataStoreSchema,
	backend ledgerbackend.LedgerBackend,
	queue UploadQueue,
	prometheusRegistry *prometheus.Registry,
	networkPassPhrase string,
	coreVersion string) (*ExportManager, error) {
	if dataStoreSchema.LedgersPerFile < 1 {
		return nil, errors.Errorf("Invalid ledgers per file (%d): must be at least 1", dataStoreSchema.LedgersPerFile)
	}

	latestLedgerMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: nameSpace, Subsystem: "export_manager", Name: "latest_ledger",
		Help: "sequence number of the latest ledger consumed by the export manager",
	}, []string{"start_ledger", "end_ledger"})
	prometheusRegistry.MustRegister(latestLedgerMetric)

	return &ExportManager{
		dataStoreSchema:    dataStoreSchema,
		ledgerBackend:      backend,
		queue:              queue,
		latestLedgerMetric: latestLedgerMetric,
		networkPassPhrase:  networkPassPhrase,
		coreVersion:        coreVersion,
	}, nil
}

// AddLedgerCloseMeta adds ledger metadata to the current export object
func (e *ExportManager) AddLedgerCloseMeta(ctx context.Context, ledgerCloseMeta xdr.LedgerCloseMeta) error {
	ledgerSeq := ledgerCloseMeta.LedgerSequence()

	// Determine the object key for the given ledger sequence
	objectKey := e.dataStoreSchema.GetObjectKeyFromSequenceNumber(ledgerSeq)

	if e.currentMetaArchive == nil {
		endSeq := ledgerSeq + e.dataStoreSchema.LedgersPerFile - 1
		if ledgerSeq < e.dataStoreSchema.LedgersPerFile {
			// Special case: Adjust the end ledger sequence for the first batch.
			// Since the start ledger is 2 instead of 0, we want to ensure that the end ledger sequence
			// does not exceed LedgersPerFile.
			// For example, if LedgersPerFile is 64, the file name for the first batch should be 0-63, not 2-66.
			endSeq = e.dataStoreSchema.LedgersPerFile - 1
		}

		// Create a new LedgerCloseMetaBatch
		e.currentMetaArchive = &xdr.LedgerCloseMetaBatch{StartSequence: xdr.Uint32(ledgerSeq), EndSequence: xdr.Uint32(endSeq)}
	}

	if err := e.currentMetaArchive.AddLedger(ledgerCloseMeta); err != nil {
		return errors.Wrapf(err, "failed to add ledger %d", ledgerSeq)
	}

	if ledgerSeq >= uint32(e.currentMetaArchive.EndSequence) {
		ledgerMetaArchive, err := NewLedgerMetaArchiveFromXDR(e.networkPassPhrase, e.coreVersion, objectKey, *e.currentMetaArchive)
		if err != nil {
			return err
		}
		if err := e.queue.Enqueue(ctx, ledgerMetaArchive); err != nil {
			return err
		}
		e.currentMetaArchive = nil
	}
	return nil
}

// Run iterates over the specified range of ledgers, retrieves ledger data
// from the backend, and processes the corresponding ledger close metadata.
// The process continues until the ending ledger number is reached or a cancellation
// signal is received.
func (e *ExportManager) Run(ctx context.Context, startLedger, endLedger uint32) error {
	defer e.queue.Close()
	labels := prometheus.Labels{
		"start_ledger": strconv.FormatUint(uint64(startLedger), 10),
		"end_ledger":   strconv.FormatUint(uint64(endLedger), 10),
	}

	var ledgerRange ledgerbackend.Range
	if endLedger < 1 {
		ledgerRange = ledgerbackend.UnboundedRange(startLedger)
	} else {
		ledgerRange = ledgerbackend.BoundedRange(startLedger, endLedger)
	}
	if err := e.ledgerBackend.PrepareRange(ctx, ledgerRange); err != nil {
		return errors.Wrap(err, "Could not prepare captive core ledger backend")
	}

	for nextLedger := startLedger; endLedger < 1 || nextLedger <= endLedger; nextLedger++ {
		ledgerCloseMeta, err := e.ledgerBackend.GetLedger(ctx, nextLedger)
		if err != nil {
			return errors.Wrapf(err, "failed to retrieve ledger %d from the ledger backend", nextLedger)
		}
		e.latestLedgerMetric.With(labels).Set(float64(nextLedger))
		err = e.AddLedgerCloseMeta(ctx, ledgerCloseMeta)
		if err != nil {
			return errors.Wrapf(err, "failed to add ledgerCloseMeta for ledger %d", nextLedger)
		}
	}
	logger.Infof("ExportManager successfully exported ledgers from %d to %d", startLedger, endLedger)
	return nil
}

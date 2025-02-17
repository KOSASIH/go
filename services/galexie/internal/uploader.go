package galexie

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go/support/compressxdr"
	"github.com/stellar/go/support/datastore"
)

// Uploader is responsible for uploading data to a storage destination.
type Uploader struct {
	dataStore            datastore.DataStore
	queue                UploadQueue
	uploadDurationMetric *prometheus.SummaryVec
	objectSizeMetrics    *prometheus.SummaryVec
	latestLedgerMetric   prometheus.Gauge
}

// NewUploader constructs a new Uploader instance
func NewUploader(
	destination datastore.DataStore,
	queue UploadQueue,
	prometheusRegistry *prometheus.Registry,
) Uploader {
	uploadDurationMetric := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: nameSpace, Subsystem: "uploader", Name: "put_duration_seconds",
			Help:       "duration for uploading a ledger batch, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"already_exists", "ledgers"},
	)
	objectSizeMetrics := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: nameSpace, Subsystem: "uploader", Name: "object_size_bytes",
			Help:       "size of a ledger batch in bytes, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"ledgers", "already_exists", "compression"},
	)
	latestLedgerMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nameSpace, Subsystem: "uploader", Name: "latest_ledger",
		Help: "sequence number of the latest ledger uploaded",
	})
	prometheusRegistry.MustRegister(uploadDurationMetric, objectSizeMetrics, latestLedgerMetric)
	return Uploader{
		dataStore:            destination,
		queue:                queue,
		uploadDurationMetric: uploadDurationMetric,
		objectSizeMetrics:    objectSizeMetrics,
		latestLedgerMetric:   latestLedgerMetric,
	}
}

type writerRecorder struct {
	io.Writer
	count *int64
}

func (r writerRecorder) Write(p []byte) (int, error) {
	total, err := r.Writer.Write(p)
	*r.count += int64(total)
	return total, err
}

type writerToRecorder struct {
	io.WriterTo
	totalCompressed   int64
	totalUncompressed int64
}

func (r *writerToRecorder) WriteTo(w io.Writer) (int64, error) {
	uncompressedCount, err := r.WriterTo.WriteTo(writerRecorder{
		Writer: w,
		count:  &r.totalCompressed,
	})
	r.totalUncompressed += uncompressedCount
	return uncompressedCount, err
}

// Upload uploads the serialized binary data of ledger TxMeta to the specified destination.
func (u Uploader) Upload(ctx context.Context, metaArchive *LedgerMetaArchive) error {
	logger.Infof("Uploading: %s", metaArchive.ObjectKey)
	startTime := time.Now()
	numLedgers := strconv.FormatUint(uint64(len(metaArchive.Data.LedgerCloseMetas)), 10)

	xdrEncoder := compressxdr.NewXDREncoder(compressxdr.DefaultCompressor, &metaArchive.Data)

	writerTo := &writerToRecorder{
		WriterTo: xdrEncoder,
	}
	ok, err := u.dataStore.PutFileIfNotExists(ctx, metaArchive.ObjectKey, writerTo, metaArchive.metaData.ToMap())
	if err != nil {
		return errors.Wrapf(err, "error uploading %s", metaArchive.ObjectKey)
	}

	logger.Infof("Uploaded %s successfully", metaArchive.ObjectKey)
	alreadyExists := strconv.FormatBool(!ok)

	u.uploadDurationMetric.With(prometheus.Labels{
		"ledgers":        numLedgers,
		"already_exists": alreadyExists,
	}).Observe(time.Since(startTime).Seconds())
	u.objectSizeMetrics.With(prometheus.Labels{
		"compression":    "none",
		"ledgers":        numLedgers,
		"already_exists": alreadyExists,
	}).Observe(float64(writerTo.totalUncompressed))
	u.objectSizeMetrics.With(prometheus.Labels{
		"compression":    xdrEncoder.Compressor.Name(),
		"ledgers":        numLedgers,
		"already_exists": alreadyExists,
	}).Observe(float64(writerTo.totalCompressed))
	u.latestLedgerMetric.Set(float64(metaArchive.Data.EndSequence))
	return nil
}

// Run starts the uploader, continuously listening for LedgerMetaArchive objects to upload.
func (u Uploader) Run(ctx context.Context, shutdownDelayTime time.Duration) error {
	uploadCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-uploadCtx.Done():
			// if uploadCtx is cancelled that means we have exited Run()
			// and therefore there are no remaining uploads
			return
		case <-ctx.Done():
			logger.Info("Received shutdown signal, waiting for remaining uploads to complete...")
		}

		select {
		case <-time.After(shutdownDelayTime):
			// wait for some time to upload remaining objects from
			// the upload queue
			logger.Info("Timeout reached, canceling remaining uploads...")
			cancel()
		case <-uploadCtx.Done():
			// if uploadCtx is cancelled that means we have exited Run()
			// and therefore there are no remaining uploads
			return
		}
	}()

	for {
		metaObject, ok, err := u.queue.Dequeue(uploadCtx)
		if err != nil {
			return err
		}
		if !ok {
			logger.Info("Meta archive channel closed, stopping uploader")
			return nil
		}

		// Upload the received LedgerMetaArchive.
		if err = u.Upload(uploadCtx, metaObject); err != nil {
			return err
		}
	}
}

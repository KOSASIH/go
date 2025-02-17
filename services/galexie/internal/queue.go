package galexie

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// UploadQueue is a queue of LedgerMetaArchive objects which are scheduled for upload
type UploadQueue struct {
	metaArchiveCh     chan *LedgerMetaArchive
	queueLengthMetric prometheus.Gauge
}

// NewUploadQueue constructs a new UploadQueue
func NewUploadQueue(size int, prometheusRegistry *prometheus.Registry) UploadQueue {
	queueLengthMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: nameSpace,
		Subsystem: "upload_queue",
		Name:      "length",
		Help:      "The number of objects queued for upload",
	})
	prometheusRegistry.MustRegister(queueLengthMetric)
	return UploadQueue{
		metaArchiveCh:     make(chan *LedgerMetaArchive, size),
		queueLengthMetric: queueLengthMetric,
	}
}

// Enqueue will add an upload task to the queue. Enqueue may block if the queue is full.
func (u UploadQueue) Enqueue(ctx context.Context, archive *LedgerMetaArchive) error {
	u.queueLengthMetric.Inc()
	select {
	case u.metaArchiveCh <- archive:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dequeue will pop a task off the queue. Dequeue may block if the queue is empty.
func (u UploadQueue) Dequeue(ctx context.Context) (*LedgerMetaArchive, bool, error) {
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()

	case metaObject, ok := <-u.metaArchiveCh:
		if ok {
			u.queueLengthMetric.Dec()
		}
		return metaObject, ok, nil
	}
}

// Close will close the queue.
func (u UploadQueue) Close() {
	close(u.metaArchiveCh)
}

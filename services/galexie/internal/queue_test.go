package galexie

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestQueueContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	queue := NewUploadQueue(0, prometheus.NewRegistry())
	cancel()

	require.ErrorIs(t, queue.Enqueue(ctx, nil), context.Canceled)
	_, _, err := queue.Dequeue(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func getMetricValue(metric prometheus.Metric) *dto.Metric {
	value := &dto.Metric{}
	err := metric.Write(value)
	if err != nil {
		panic(err)
	}
	return value
}

func TestQueue(t *testing.T) {
	queue := NewUploadQueue(3, prometheus.NewRegistry())

	require.NoError(t, queue.Enqueue(context.Background(), NewLedgerMetaArchive("test", 1, 1)))
	require.NoError(t, queue.Enqueue(context.Background(), NewLedgerMetaArchive("test", 2, 2)))
	require.NoError(t, queue.Enqueue(context.Background(), NewLedgerMetaArchive("test", 3, 3)))

	require.Equal(t, float64(3), getMetricValue(queue.queueLengthMetric).GetGauge().GetValue())
	queue.Close()

	l, ok, err := queue.Dequeue(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, float64(2), getMetricValue(queue.queueLengthMetric).GetGauge().GetValue())
	require.Equal(t, uint32(1), uint32(l.Data.StartSequence))

	l, ok, err = queue.Dequeue(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, float64(1), getMetricValue(queue.queueLengthMetric).GetGauge().GetValue())
	require.Equal(t, uint32(2), uint32(l.Data.StartSequence))

	l, ok, err = queue.Dequeue(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, float64(0), getMetricValue(queue.queueLengthMetric).GetGauge().GetValue())
	require.Equal(t, uint32(3), uint32(l.Data.StartSequence))

	l, ok, err = queue.Dequeue(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, l)
}

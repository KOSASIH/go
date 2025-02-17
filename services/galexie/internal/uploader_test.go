package galexie

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/support/compressxdr"
	"github.com/stellar/go/support/datastore"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

var testShutdownDelayTime = 300 * time.Millisecond

func TestUploaderSuite(t *testing.T) {
	suite.Run(t, new(UploaderSuite))
}

// UploaderSuite is a test suite for the Uploader.
type UploaderSuite struct {
	suite.Suite
	ctx           context.Context
	mockDataStore datastore.MockDataStore
}

func (s *UploaderSuite) SetupTest() {
	s.ctx = context.Background()
	s.mockDataStore = datastore.MockDataStore{}
}

func (s *UploaderSuite) TearDownTest() {
	s.mockDataStore.AssertExpectations(s.T())
}

func (s *UploaderSuite) TestUpload() {
	s.testUpload(false)
	s.testUpload(true)
}

func (s *UploaderSuite) TestUploadWithMetadata() {
	key, start, end := "test-1-100", uint32(1), uint32(100)
	archive := NewLedgerMetaArchive(key, start, end)
	for i := start; i <= end; i++ {
		_ = archive.Data.AddLedger(createLedgerCloseMeta(i))
	}
	metadata := datastore.MetaData{
		StartLedger:          start,
		EndLedger:            end,
		StartLedgerCloseTime: 123456789,
		EndLedgerCloseTime:   987654321,
		ProtocolVersion:      3,
		CoreVersion:          "v1.2.3",
		NetworkPassPhrase:    "testnet",
		CompressionType:      "gzip",
		Version:              "1.0.0",
	}
	archive.metaData = metadata
	var capturedBuf bytes.Buffer
	s.mockDataStore.On("PutFileIfNotExists", mock.Anything, key, mock.Anything, metadata.ToMap()).
		Run(func(args mock.Arguments) {
			_ = args.Get(1).(string)
			_, err := args.Get(2).(io.WriterTo).WriteTo(&capturedBuf)
			s.Require().NoError(err)
		}).Return(true, nil).Once()

	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	dataUploader := NewUploader(&s.mockDataStore, queue, registry)
	s.Require().NoError(dataUploader.Upload(context.Background(), archive))

}

func (s *UploaderSuite) testUpload(putOkReturnVal bool) {
	key, start, end := "test-1-100", uint32(1), uint32(100)
	archive := NewLedgerMetaArchive(key, start, end)
	for i := start; i <= end; i++ {
		_ = archive.Data.AddLedger(createLedgerCloseMeta(i))
	}

	var capturedBuf bytes.Buffer
	var capturedKey string
	s.mockDataStore.On("PutFileIfNotExists", mock.Anything, key, mock.Anything, datastore.MetaData{}.ToMap()).
		Run(func(args mock.Arguments) {
			capturedKey = args.Get(1).(string)
			_, err := args.Get(2).(io.WriterTo).WriteTo(&capturedBuf)
			s.Require().NoError(err)
		}).Return(putOkReturnVal, nil).Once()

	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	dataUploader := NewUploader(&s.mockDataStore, queue, registry)
	s.Require().NoError(dataUploader.Upload(context.Background(), archive))

	expectedCompressedLength := capturedBuf.Len()
	var decodedArchive LedgerMetaArchive
	xdrDecoder := compressxdr.NewXDRDecoder(compressxdr.DefaultCompressor, &decodedArchive.Data)

	decoder := xdrDecoder
	_, err := decoder.ReadFrom(&capturedBuf)
	s.Require().NoError(err)

	// require that the decoded data matches the original test data
	s.Require().Equal(key, capturedKey)
	s.Require().Equal(archive.Data, decodedArchive.Data)

	alreadyExists := !putOkReturnVal
	metric, err := dataUploader.uploadDurationMetric.MetricVec.GetMetricWith(prometheus.Labels{
		"ledgers":        "100",
		"already_exists": strconv.FormatBool(alreadyExists),
	})
	s.Require().NoError(err)
	s.Require().Equal(
		uint64(1),
		getMetricValue(metric).GetSummary().GetSampleCount(),
	)
	s.Require().Positive(getMetricValue(metric).GetSummary().GetSampleSum())
	metric, err = dataUploader.uploadDurationMetric.MetricVec.GetMetricWith(prometheus.Labels{
		"ledgers":        "100",
		"already_exists": strconv.FormatBool(!alreadyExists),
	})
	s.Require().NoError(err)
	s.Require().Equal(
		uint64(0),
		getMetricValue(metric).GetSummary().GetSampleCount(),
	)

	metric, err = dataUploader.objectSizeMetrics.MetricVec.GetMetricWith(prometheus.Labels{
		"ledgers":        "100",
		"compression":    decoder.Compressor.Name(),
		"already_exists": strconv.FormatBool(alreadyExists),
	})
	s.Require().NoError(err)
	s.Require().Equal(
		uint64(1),
		getMetricValue(metric).GetSummary().GetSampleCount(),
	)
	s.Require().Equal(
		float64(expectedCompressedLength),
		getMetricValue(metric).GetSummary().GetSampleSum(),
	)
	metric, err = dataUploader.objectSizeMetrics.MetricVec.GetMetricWith(prometheus.Labels{
		"ledgers":        "100",
		"compression":    decoder.Compressor.Name(),
		"already_exists": strconv.FormatBool(!alreadyExists),
	})
	s.Require().NoError(err)
	s.Require().Equal(
		uint64(0),
		getMetricValue(metric).GetSummary().GetSampleCount(),
	)

	metric, err = dataUploader.objectSizeMetrics.MetricVec.GetMetricWith(prometheus.Labels{
		"ledgers":        "100",
		"compression":    "none",
		"already_exists": strconv.FormatBool(alreadyExists),
	})
	s.Require().NoError(err)
	s.Require().Equal(
		uint64(1),
		getMetricValue(metric).GetSummary().GetSampleCount(),
	)
	uncompressedPayload, err := decodedArchive.Data.MarshalBinary()
	s.Require().NoError(err)
	s.Require().Equal(
		float64(len(uncompressedPayload)),
		getMetricValue(metric).GetSummary().GetSampleSum(),
	)
	metric, err = dataUploader.objectSizeMetrics.MetricVec.GetMetricWith(prometheus.Labels{
		"ledgers":        "100",
		"compression":    "none",
		"already_exists": strconv.FormatBool(!alreadyExists),
	})
	s.Require().NoError(err)
	s.Require().Equal(
		uint64(0),
		getMetricValue(metric).GetSummary().GetSampleCount(),
	)

	s.Require().Equal(
		float64(100),
		getMetricValue(dataUploader.latestLedgerMetric).GetGauge().GetValue(),
	)
}

func (s *UploaderSuite) TestUploadPutError() {
	s.testUploadPutError(true)
	s.testUploadPutError(false)
}

func (s *UploaderSuite) testUploadPutError(putOkReturnVal bool) {
	key, start, end := "test-1-100", uint32(1), uint32(100)
	archive := NewLedgerMetaArchive(key, start, end)

	s.mockDataStore.On("PutFileIfNotExists", context.Background(), key,
		mock.Anything, datastore.MetaData{}.ToMap()).Return(putOkReturnVal, errors.New("error in PutFileIfNotExists")).Once()

	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	dataUploader := NewUploader(&s.mockDataStore, queue, registry)
	err := dataUploader.Upload(context.Background(), archive)
	s.Require().Equal(fmt.Sprintf("error uploading %s: error in PutFileIfNotExists", key), err.Error())

	for _, alreadyExists := range []string{"true", "false"} {
		metric, err := dataUploader.uploadDurationMetric.MetricVec.GetMetricWith(prometheus.Labels{
			"ledgers":        "100",
			"already_exists": alreadyExists,
		})
		s.Require().NoError(err)
		s.Require().Equal(
			uint64(0),
			getMetricValue(metric).GetSummary().GetSampleCount(),
		)

		for _, compression := range []string{compressxdr.DefaultCompressor.Name(), "none"} {
			metric, err = dataUploader.objectSizeMetrics.MetricVec.GetMetricWith(prometheus.Labels{
				"ledgers":        "100",
				"compression":    compression,
				"already_exists": alreadyExists,
			})
			s.Require().NoError(err)
			s.Require().Equal(
				uint64(0),
				getMetricValue(metric).GetSummary().GetSampleCount(),
			)
		}

		s.Require().Equal(
			float64(0),
			getMetricValue(dataUploader.latestLedgerMetric).GetGauge().GetValue(),
		)
	}
}

func (s *UploaderSuite) TestRunUntilQueueClose() {
	var prev *mock.Call
	for i := 1; i <= 100; i++ {
		key := fmt.Sprintf("test-%d", i)
		cur := s.mockDataStore.On("PutFileIfNotExists", mock.Anything,
			key, mock.Anything, mock.Anything).Return(true, nil).Once()
		if prev != nil {
			cur.NotBefore(prev)
		}
		prev = cur
	}

	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	go func() {
		for i := uint32(1); i <= uint32(100); i++ {
			key := fmt.Sprintf("test-%d", i)
			s.Require().NoError(queue.Enqueue(s.ctx, NewLedgerMetaArchive(key, i, i)))
		}
		queue.Close()
	}()

	dataUploader := NewUploader(&s.mockDataStore, queue, registry)
	s.Require().NoError(dataUploader.Run(context.Background(), testShutdownDelayTime))

	s.Require().Equal(
		float64(100),
		getMetricValue(dataUploader.latestLedgerMetric).GetGauge().GetValue(),
	)
}

func (s *UploaderSuite) TestRunContextCancel() {
	ctx, cancel := context.WithCancel(context.Background())
	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)

	first := s.mockDataStore.On("PutFileIfNotExists", mock.Anything, "test", mock.Anything, datastore.MetaData{}.ToMap()).
		Return(true, nil).Once().Run(func(args mock.Arguments) {
		cancel()
	})
	s.mockDataStore.On("PutFileIfNotExists", mock.Anything, "test1", mock.Anything, datastore.MetaData{}.ToMap()).
		Return(true, nil).Once().NotBefore(first).Run(func(args mock.Arguments) {
		ctxArg := args.Get(0).(context.Context)
		s.Require().NoError(ctxArg.Err())
	})

	go func() {
		s.Require().NoError(queue.Enqueue(s.ctx, NewLedgerMetaArchive("test", 1, 1)))
		s.Require().NoError(queue.Enqueue(s.ctx, NewLedgerMetaArchive("test1", 2, 2)))
	}()

	dataUploader := NewUploader(&s.mockDataStore, queue, registry)
	s.Require().EqualError(dataUploader.Run(ctx, testShutdownDelayTime), "context canceled")
	s.Require().Equal(
		float64(2),
		getMetricValue(dataUploader.latestLedgerMetric).GetGauge().GetValue(),
	)
}

func (s *UploaderSuite) TestRunUploadError() {
	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(2, registry)

	s.Require().NoError(queue.Enqueue(s.ctx, NewLedgerMetaArchive("test", 1, 1)))
	s.Require().NoError(queue.Enqueue(s.ctx, NewLedgerMetaArchive("test1", 2, 2)))

	s.mockDataStore.On("PutFileIfNotExists", mock.Anything, "test",
		mock.Anything, mock.Anything).Return(false, errors.New("Put error")).Once()

	dataUploader := NewUploader(&s.mockDataStore, queue, registry)
	err := dataUploader.Run(context.Background(), testShutdownDelayTime)
	s.Require().Equal("error uploading test: Put error", err.Error())
}

func NewLedgerMetaArchive(key string, startSeq uint32, endSeq uint32) *LedgerMetaArchive {
	return &LedgerMetaArchive{
		ObjectKey: key,
		Data: xdr.LedgerCloseMetaBatch{
			StartSequence: xdr.Uint32(startSeq),
			EndSequence:   xdr.Uint32(endSeq),
		},
		metaData: datastore.MetaData{},
	}
}

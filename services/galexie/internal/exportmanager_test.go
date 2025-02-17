package galexie

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/support/collections/set"
	"github.com/stellar/go/support/datastore"
	"github.com/stellar/go/xdr"
)

func createLedgerCloseMeta(ledgerSeq uint32) xdr.LedgerCloseMeta {
	return xdr.LedgerCloseMeta{
		V: int32(0),
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerVersion: 21,
					LedgerSeq:     xdr.Uint32(ledgerSeq),
					ScpValue:      xdr.StellarValue{CloseTime: xdr.TimePoint(ledgerSeq * 100)},
				},
			},
			TxSet:              xdr.TransactionSet{},
			TxProcessing:       nil,
			UpgradesProcessing: nil,
			ScpInfo:            nil,
		},
		V1: nil,
	}
}

func TestExporterSuite(t *testing.T) {
	suite.Run(t, new(ExportManagerSuite))
}

// ExportManagerSuite is a test suite for the ExportManager.
type ExportManagerSuite struct {
	suite.Suite
	ctx         context.Context
	mockBackend ledgerbackend.MockDatabaseBackend
}

func (s *ExportManagerSuite) SetupTest() {
	s.ctx = context.Background()
	s.mockBackend = ledgerbackend.MockDatabaseBackend{}
}

func (s *ExportManagerSuite) TearDownTest() {
	s.mockBackend.AssertExpectations(s.T())
}

func (s *ExportManagerSuite) TestInvalidExportConfig() {
	config := datastore.DataStoreSchema{LedgersPerFile: 0, FilesPerPartition: 10}
	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	_, err := NewExportManager(config, &s.mockBackend, queue, registry, "passphrase", "coreversion")
	s.Require().Error(err)
}

func (s *ExportManagerSuite) TestRun() {
	config := datastore.DataStoreSchema{LedgersPerFile: 64, FilesPerPartition: 10}
	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	exporter, err := NewExportManager(config, &s.mockBackend, queue, registry, "passphrase", "coreversion")
	s.Require().NoError(err)

	start := uint32(0)
	end := uint32(255)
	expectedKeys := set.NewSet[string](10)
	s.mockBackend.On("PrepareRange", s.ctx, ledgerbackend.BoundedRange(start, end)).Return(nil)
	for i := start; i <= end; i++ {
		s.mockBackend.On("GetLedger", s.ctx, i).
			Return(createLedgerCloseMeta(i), nil)
		key := config.GetObjectKeyFromSequenceNumber(i)
		expectedKeys.Add(key)
	}

	actualKeys := set.NewSet[string](10)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			v, ok, dqErr := queue.Dequeue(s.ctx)
			s.Assert().NoError(dqErr)
			if !ok {
				break
			}
			actualKeys.Add(v.ObjectKey)
		}
	}()

	err = exporter.Run(s.ctx, start, end)
	s.Require().NoError(err)

	wg.Wait()

	s.Require().Equal(expectedKeys, actualKeys)
	s.Require().Equal(
		float64(255),
		getMetricValue(exporter.latestLedgerMetric.With(
			prometheus.Labels{
				"start_ledger": "0",
				"end_ledger":   "255",
			}),
		).GetGauge().GetValue(),
	)
}

func (s *ExportManagerSuite) TestRunContextCancel() {
	config := datastore.DataStoreSchema{LedgersPerFile: 1, FilesPerPartition: 1}

	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	exporter, err := NewExportManager(config, &s.mockBackend, queue, registry, "passphrase", "coreversion")
	s.Require().NoError(err)
	ctx, cancel := context.WithCancel(context.Background())

	s.mockBackend.On("PrepareRange", ctx, ledgerbackend.BoundedRange(0, 255)).Return(nil)
	s.mockBackend.On("GetLedger", mock.Anything, mock.Anything).
		Return(createLedgerCloseMeta(1), nil)

	go func() {
		<-time.After(time.Second * 1)
		cancel()
	}()

	go func() {
		for i := 0; i < 127; i++ {
			_, ok, dqErr := queue.Dequeue(s.ctx)
			s.Assert().NoError(dqErr)
			s.Assert().True(ok)
		}
	}()

	err = exporter.Run(ctx, 0, 255)
	s.Require().EqualError(err, "failed to add ledgerCloseMeta for ledger 128: context canceled")

}

func (s *ExportManagerSuite) TestRunWithCanceledContext() {
	config := datastore.DataStoreSchema{LedgersPerFile: 1, FilesPerPartition: 10}

	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	exporter, err := NewExportManager(config, &s.mockBackend, queue, registry, "passphrase", "coreversion")
	s.Require().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.mockBackend.On("PrepareRange", ctx, ledgerbackend.BoundedRange(1, 10)).
		Return(context.Canceled).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		s.Require().ErrorIs(ctx.Err(), context.Canceled)
	})
	err = exporter.Run(ctx, 1, 10)
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *ExportManagerSuite) TestAddLedgerCloseMeta() {
	config := datastore.DataStoreSchema{LedgersPerFile: 1, FilesPerPartition: 10}

	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	exporter, err := NewExportManager(config, &s.mockBackend, queue, registry, "passphrase", "coreversion")
	s.Require().NoError(err)

	expectedKeys := set.NewSet[string](10)
	actualKeys := set.NewSet[string](10)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			v, ok, err := queue.Dequeue(s.ctx)
			s.Assert().NoError(err)
			if !ok {
				break
			}
			actualKeys.Add(v.ObjectKey)
		}
	}()

	start := uint32(0)
	end := uint32(255)
	for i := start; i <= end; i++ {
		s.Require().NoError(exporter.AddLedgerCloseMeta(context.Background(), createLedgerCloseMeta(i)))

		key := config.GetObjectKeyFromSequenceNumber(i)
		expectedKeys.Add(key)
	}

	queue.Close()
	wg.Wait()
	s.Require().Equal(expectedKeys, actualKeys)
}

func (s *ExportManagerSuite) TestAddLedgerCloseMetaContextCancel() {
	config := datastore.DataStoreSchema{LedgersPerFile: 1, FilesPerPartition: 10}
	registry := prometheus.NewRegistry()
	queue := NewUploadQueue(1, registry)
	exporter, err := NewExportManager(config, &s.mockBackend, queue, registry, "passphrase", "coreversion")
	s.Require().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-time.After(time.Second * 1)
		cancel()
	}()

	require.NoError(s.T(), exporter.AddLedgerCloseMeta(ctx, createLedgerCloseMeta(1)))
	err = exporter.AddLedgerCloseMeta(ctx, createLedgerCloseMeta(2))
	require.EqualError(s.T(), err, "context canceled")
}

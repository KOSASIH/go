package ingest

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
)

func TestCalculateParallelLedgerBatchSize(t *testing.T) {
	config := Config{}
	result := &mockSystem{}
	factory := func(c Config) (System, error) {
		return result, nil
	}

	// worker count 0
	system, err := newParallelSystems(config, 0, MinBatchSize, MaxCaptiveCoreBackendBatchSize, factory)
	assert.EqualError(t, err, "workerCount must be > 0")

	// worker count 1, range smaller than HistoryCheckpointLedgerInterval
	system, err = newParallelSystems(config, 1, 50, 200, factory)
	assert.EqualError(t, err, fmt.Sprintf("minBatchSize must be at least the %d", HistoryCheckpointLedgerInterval))

	// worker count 1, max batch size smaller than min batch size
	system, err = newParallelSystems(config, 1, 5000, 200, factory)
	assert.EqualError(t, err, "maxBatchSize cannot be less than minBatchSize")

	// worker count 1, captive core batch size
	system, _ = newParallelSystems(config, 1, MinBatchSize, MaxCaptiveCoreBackendBatchSize, factory)
	assert.Equal(t, uint32(MaxCaptiveCoreBackendBatchSize), system.calculateParallelLedgerBatchSize(uint32(MaxCaptiveCoreBackendBatchSize)+10))
	assert.Equal(t, uint32(MinBatchSize), system.calculateParallelLedgerBatchSize(0))
	assert.Equal(t, uint32(10048), system.calculateParallelLedgerBatchSize(10048)) // exact multiple
	assert.Equal(t, uint32(10048), system.calculateParallelLedgerBatchSize(10090)) // round down

	// worker count 1, buffered storage batch size
	system, _ = newParallelSystems(config, 1, MinBatchSize, MaxBufferedStorageBackendBatchSize, factory)
	assert.Equal(t, uint32(MaxBufferedStorageBackendBatchSize), system.calculateParallelLedgerBatchSize(uint32(MaxBufferedStorageBackendBatchSize)+10))
	assert.Equal(t, uint32(MinBatchSize), system.calculateParallelLedgerBatchSize(0))
	assert.Equal(t, uint32(10048), system.calculateParallelLedgerBatchSize(10048)) // exact multiple
	assert.Equal(t, uint32(10048), system.calculateParallelLedgerBatchSize(10090)) // round down

	// worker count 1, no min/max batch size
	system, _ = newParallelSystems(config, 1, 0, 0, factory)
	assert.Equal(t, uint32(20096), system.calculateParallelLedgerBatchSize(20096)) // exact multiple
	assert.Equal(t, uint32(20032), system.calculateParallelLedgerBatchSize(20090)) // round down

	// worker count 1, min/max batch size
	system, _ = newParallelSystems(config, 1, 64, 20000, factory)
	assert.Equal(t, uint32(19968), system.calculateParallelLedgerBatchSize(20096)) // round down
	system, _ = newParallelSystems(config, 1, 64, 30000, factory)
	assert.Equal(t, uint32(20096), system.calculateParallelLedgerBatchSize(20096)) // exact multiple

	// Tests for worker count 2

	// no min/max batch size
	system, _ = newParallelSystems(config, 2, 0, 0, factory)
	assert.Equal(t, uint32(64), system.calculateParallelLedgerBatchSize(60))  // range smaller than 64
	assert.Equal(t, uint32(64), system.calculateParallelLedgerBatchSize(128)) // exact multiple
	assert.Equal(t, uint32(10048), system.calculateParallelLedgerBatchSize(20096))

	// range larger than max batch size
	system, _ = newParallelSystems(config, 2, 64, 10000, factory)
	assert.Equal(t, uint32(9984), system.calculateParallelLedgerBatchSize(20096)) // round down

	// range smaller than min batch size
	system, _ = newParallelSystems(config, 2, 64, 0, factory)
	assert.Equal(t, uint32(64), system.calculateParallelLedgerBatchSize(50))       // min batch size
	assert.Equal(t, uint32(10048), system.calculateParallelLedgerBatchSize(20096)) // exact multiple
	assert.Equal(t, uint32(64), system.calculateParallelLedgerBatchSize(100))      // min batch size

	// batch size equal to min
	system, _ = newParallelSystems(config, 2, 100, 0, factory)
	assert.Equal(t, uint32(64), system.calculateParallelLedgerBatchSize(100)) // round down

	// equal min/max batch size
	system, _ = newParallelSystems(config, 2, 5000, 5000, factory)
	assert.Equal(t, uint32(4992), system.calculateParallelLedgerBatchSize(20096)) // round down

	// worker count 3
	system, _ = newParallelSystems(config, 3, 64, 7000, factory)
	assert.Equal(t, uint32(6656), system.calculateParallelLedgerBatchSize(20096))

	// worker count 4
	system, _ = newParallelSystems(config, 4, 64, 20000, factory)
	assert.Equal(t, uint32(4992), system.calculateParallelLedgerBatchSize(20096)) //round down
	assert.Equal(t, uint32(64), system.calculateParallelLedgerBatchSize(64))
	assert.Equal(t, uint32(64), system.calculateParallelLedgerBatchSize(2))

	// max possible workers
	system, _ = newParallelSystems(config, math.MaxUint32, 0, 0, factory)
	assert.Equal(t, uint32(64), system.calculateParallelLedgerBatchSize(math.MaxUint32))
}

func TestParallelReingestRange(t *testing.T) {
	config := Config{}
	var (
		rangesCalled []history.LedgerRange
		m            sync.Mutex
	)
	result := &mockSystem{}
	result.On("ReingestRange", mock.AnythingOfType("[]history.LedgerRange"), false, false).Run(
		func(args mock.Arguments) {
			m.Lock()
			defer m.Unlock()
			rangesCalled = append(rangesCalled, args.Get(0).([]history.LedgerRange)...)
			// simulate call
			time.Sleep(time.Millisecond * time.Duration(10+rand.Int31n(50)))
		}).Return(error(nil))
	result.On("RebuildTradeAggregationBuckets", uint32(1), uint32(2050)).Return(nil).Once()
	factory := func(c Config) (System, error) {
		return result, nil
	}
	system, err := newParallelSystems(config, 3, MinBatchSize, MaxCaptiveCoreBackendBatchSize, factory)
	assert.NoError(t, err)
	err = system.ReingestRange([]history.LedgerRange{{1, 2050}})
	assert.NoError(t, err)

	sort.Slice(rangesCalled, func(i, j int) bool {
		return rangesCalled[i].StartSequence < rangesCalled[j].StartSequence
	})
	expected := []history.LedgerRange{
		{StartSequence: 1, EndSequence: 640}, {StartSequence: 641, EndSequence: 1280}, {StartSequence: 1281, EndSequence: 1920}, {StartSequence: 1921, EndSequence: 2050},
	}
	assert.Equal(t, expected, rangesCalled)

	rangesCalled = nil
	system, err = newParallelSystems(config, 1, 0, 0, factory)
	assert.NoError(t, err)
	result.On("RebuildTradeAggregationBuckets", uint32(1), uint32(1024)).Return(nil).Once()
	err = system.ReingestRange([]history.LedgerRange{{1, 1024}})
	result.AssertExpectations(t)
	expected = []history.LedgerRange{
		{StartSequence: 1, EndSequence: 1024},
	}
	assert.NoError(t, err)
	assert.Equal(t, expected, rangesCalled)
}

func TestParallelReingestRangeError(t *testing.T) {
	config := Config{}
	result := &mockSystem{}
	// Fail on the second range
	result.On("ReingestRange", []history.LedgerRange{{641, 1280}}, false, false).Return(errors.New("failed because of foo")).Once()
	result.On("ReingestRange", mock.AnythingOfType("[]history.LedgerRange"), false, false).Return(nil)
	result.On("RebuildTradeAggregationBuckets", uint32(1), uint32(641)).Return(nil).Once()

	factory := func(c Config) (System, error) {
		return result, nil
	}
	system, err := newParallelSystems(config, 3, MinBatchSize, MaxCaptiveCoreBackendBatchSize, factory)
	assert.NoError(t, err)
	err = system.ReingestRange([]history.LedgerRange{{1, 2050}})
	result.AssertExpectations(t)
	assert.Error(t, err)
	assert.Equal(t, "job failed, recommended restart range: [641, 2050]: error when processing [641, 1280] range: failed because of foo", err.Error())
}

func TestParallelReingestRangeErrorInEarlierJob(t *testing.T) {
	config := Config{}
	var wg sync.WaitGroup
	wg.Add(1)
	result := &mockSystem{}
	// Fail on an lower subrange after the first error
	result.On("ReingestRange", []history.LedgerRange{{641, 1280}}, false, false).Run(func(mock.Arguments) {
		// Wait for a more recent range to error
		wg.Wait()
		// This sleep should help making sure the result of this range is processed later than the one below
		// (there are no guarantees without instrumenting ReingestRange(), but that's too complicated)
		time.Sleep(50 * time.Millisecond)
	}).Return(errors.New("failed because of foo")).Once()
	result.On("ReingestRange", []history.LedgerRange{{1281, 1920}}, false, false).Run(func(mock.Arguments) {
		wg.Done()
	}).Return(errors.New("failed because of bar")).Once()
	result.On("ReingestRange", mock.AnythingOfType("[]history.LedgerRange"), false, false).Return(error(nil))
	result.On("RebuildTradeAggregationBuckets", uint32(1), uint32(641)).Return(nil).Once()

	factory := func(c Config) (System, error) {
		return result, nil
	}
	system, err := newParallelSystems(config, 3, 0, 0, factory)
	assert.NoError(t, err)
	err = system.ReingestRange([]history.LedgerRange{{1, 2050}})
	result.AssertExpectations(t)
	assert.Error(t, err)
	assert.Equal(t, "job failed, recommended restart range: [641, 2050]: error when processing [641, 1280] range: failed because of foo", err.Error())

}

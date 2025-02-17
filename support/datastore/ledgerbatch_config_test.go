package datastore

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetObjectKeyFromSequenceNumber(t *testing.T) {
	testCases := []struct {
		filesPerPartition uint32
		ledgerSeq         uint32
		ledgersPerFile    uint32
		expectedKey       string
	}{
		{0, 5, 1, "FFFFFFFA--5.xdr.zstd"},
		{0, 5, 10, "FFFFFFFF--0-9.xdr.zstd"},
		{2, 10, 100, "FFFFFFFF--0-199/FFFFFFFF--0-99.xdr.zstd"},
		{2, 150, 50, "FFFFFF9B--100-199/FFFFFF69--150-199.xdr.zstd"},
		{2, 300, 200, "FFFFFFFF--0-399/FFFFFF37--200-399.xdr.zstd"},
		{2, 1, 1, "FFFFFFFF--0-1/FFFFFFFE--1.xdr.zstd"},
		{4, 10, 100, "FFFFFFFF--0-399/FFFFFFFF--0-99.xdr.zstd"},
		{4, 250, 50, "FFFFFF37--200-399/FFFFFF05--250-299.xdr.zstd"},
		{1, 300, 200, "FFFFFF37--200-399.xdr.zstd"},
		{1, 1, 1, "FFFFFFFE--1.xdr.zstd"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("LedgerSeq-%d-LedgersPerFile-%d", tc.ledgerSeq, tc.ledgersPerFile), func(t *testing.T) {
			config := DataStoreSchema{FilesPerPartition: tc.filesPerPartition, LedgersPerFile: tc.ledgersPerFile}
			key := config.GetObjectKeyFromSequenceNumber(tc.ledgerSeq)
			require.Equal(t, tc.expectedKey, key)
		})
	}
}

func TestGetObjectKeyFromSequenceNumber_ObjectKeyDescOrder(t *testing.T) {
	config := DataStoreSchema{
		LedgersPerFile:    1,
		FilesPerPartition: 10,
	}
	sequenceCount := 10000
	sequenceMap := make(map[uint32]string)
	keys := make([]uint32, len(sequenceMap))
	count := 0

	// Add 0 and MaxUint32 as edge cases
	sequenceMap[0] = config.GetObjectKeyFromSequenceNumber(0)
	keys = append(keys, 0)
	sequenceMap[math.MaxUint32] = config.GetObjectKeyFromSequenceNumber(math.MaxUint32)
	keys = append(keys, math.MaxUint32)

	for {
		if count >= sequenceCount {
			break
		}
		randSequence := rand.Uint32()
		if _, ok := sequenceMap[randSequence]; ok {
			continue
		}
		sequenceMap[randSequence] = config.GetObjectKeyFromSequenceNumber(randSequence)
		keys = append(keys, randSequence)
		count++
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	prev := sequenceMap[keys[0]]
	for i := 1; i < sequenceCount; i++ {
		curr := sequenceMap[keys[i]]
		if prev <= curr {
			t.Error("sequences not in lexicographic order")
		}
		prev = curr
	}
}

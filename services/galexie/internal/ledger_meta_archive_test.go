package galexie

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go/support/datastore"
	"github.com/stellar/go/xdr"
)

func TestNewLedgerMetaArchiveFromXDR(t *testing.T) {
	data := xdr.LedgerCloseMetaBatch{
		StartSequence: 1234,
		EndSequence:   1234,
		LedgerCloseMetas: []xdr.LedgerCloseMeta{
			createLedgerCloseMeta(1234),
		},
	}

	archive, err := NewLedgerMetaArchiveFromXDR("testnet", "v1.2.3", "key", data)

	require.NoError(t, err)
	require.NotNil(t, archive)

	// Check if the metadata fields are correctly populated
	expectedMetaData := datastore.MetaData{
		StartLedger:          1234,
		EndLedger:            1234,
		StartLedgerCloseTime: 1234 * 100,
		EndLedgerCloseTime:   1234 * 100,
		NetworkPassPhrase:    "testnet",
		CompressionType:      "zstd",
		ProtocolVersion:      21,
		CoreVersion:          "v1.2.3",
		Version:              "develop",
	}

	require.Equal(t, expectedMetaData, archive.metaData)

	data = xdr.LedgerCloseMetaBatch{
		StartSequence: 1234,
		EndSequence:   1237,
		LedgerCloseMetas: []xdr.LedgerCloseMeta{
			createLedgerCloseMeta(1234),
			createLedgerCloseMeta(1235),
			createLedgerCloseMeta(1236),
			createLedgerCloseMeta(1237),
		},
	}

	archive, err = NewLedgerMetaArchiveFromXDR("testnet", "v1.2.3", "key", data)

	require.NoError(t, err)
	require.NotNil(t, archive)

	// Check if the metadata fields are correctly populated
	expectedMetaData = datastore.MetaData{
		StartLedger:          1234,
		EndLedger:            1237,
		StartLedgerCloseTime: 1234 * 100,
		EndLedgerCloseTime:   1237 * 100,
		NetworkPassPhrase:    "testnet",
		CompressionType:      "zstd",
		ProtocolVersion:      21,
		CoreVersion:          "v1.2.3",
		Version:              "develop",
	}

	require.Equal(t, expectedMetaData, archive.metaData)
}

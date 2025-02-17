// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package xdr

import (
	"bytes"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXdrStreamHash(t *testing.T) {
	bucketEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	stream := CreateXdrStream(bucketEntry)

	// Stream hash should be equal sha256 hash of concatenation of:
	// - uint32 representing the number of bytes of a structure,
	// - xdr-encoded `BucketEntry` above.
	b := &bytes.Buffer{}
	err := MarshalFramed(b, bucketEntry)
	require.NoError(t, err)

	expectedHash := sha256.Sum256(b.Bytes())
	stream.SetExpectedHash(expectedHash)

	var readBucketEntry BucketEntry
	err = stream.ReadOne(&readBucketEntry)
	require.NoError(t, err)
	assert.Equal(t, bucketEntry, readBucketEntry)

	assert.Equal(t, int(stream.BytesRead()), b.Len())

	assert.Equal(t, io.EOF, stream.ReadOne(&readBucketEntry))
	assert.Equal(t, int(stream.BytesRead()), b.Len())

	assert.NoError(t, stream.Close())
}

func TestXdrStreamDiscard(t *testing.T) {
	firstEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	secondEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC23QF2HUE52AMXUFUH3AYJAXXGXXV2VHXYYR6EYXETPKDXZSAW67XO4"),
					Balance:   Int64(100000000),
				},
			},
		},
	}

	fullStream := CreateXdrStream(firstEntry, secondEntry)
	b := &bytes.Buffer{}
	require.NoError(t, MarshalFramed(b, firstEntry))
	require.NoError(t, MarshalFramed(b, secondEntry))
	expectedHash := sha256.Sum256(b.Bytes())
	fullStream.SetExpectedHash(expectedHash)

	discardStream := CreateXdrStream(firstEntry, secondEntry)
	discardStream.SetExpectedHash(expectedHash)

	var readBucketEntry BucketEntry
	require.NoError(t, fullStream.ReadOne(&readBucketEntry))
	assert.Equal(t, firstEntry, readBucketEntry)

	skipAmount := fullStream.BytesRead()
	bytesRead, err := discardStream.Discard(skipAmount)
	require.NoError(t, err)
	assert.Equal(t, bytesRead, skipAmount)

	require.NoError(t, fullStream.ReadOne(&readBucketEntry))
	assert.Equal(t, secondEntry, readBucketEntry)

	require.NoError(t, discardStream.ReadOne(&readBucketEntry))
	assert.Equal(t, secondEntry, readBucketEntry)

	assert.Equal(t, int(fullStream.BytesRead()), b.Len())
	assert.Equal(t, fullStream.BytesRead(), discardStream.BytesRead())

	assert.Equal(t, io.EOF, fullStream.ReadOne(&readBucketEntry))
	assert.Equal(t, io.EOF, discardStream.ReadOne(&readBucketEntry))

	assert.Equal(t, int(fullStream.BytesRead()), b.Len())
	assert.Equal(t, fullStream.BytesRead(), discardStream.BytesRead())

	assert.NoError(t, discardStream.Close())
	assert.NoError(t, fullStream.Close())
}

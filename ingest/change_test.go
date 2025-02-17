package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/xdr"
)

func assertChangesAreEqual(t *testing.T, a, b Change) {
	assert.Equal(t, a.Type, b.Type)
	if a.Pre == nil {
		assert.Nil(t, b.Pre)
	} else {
		aBytes, err := a.Pre.MarshalBinary()
		assert.NoError(t, err)
		bBytes, err := b.Pre.MarshalBinary()
		assert.NoError(t, err)
		assert.Equal(t, aBytes, bBytes)
	}
	if a.Post == nil {
		assert.Nil(t, b.Post)
	} else {
		aBytes, err := a.Post.MarshalBinary()
		assert.NoError(t, err)
		bBytes, err := b.Post.MarshalBinary()
		assert.NoError(t, err)
		assert.Equal(t, aBytes, bBytes)
	}
}

func TestSortChanges(t *testing.T) {
	for _, testCase := range []struct {
		input    []Change
		expected []Change
	}{
		{[]Change{}, []Change{}},
		{
			[]Change{
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre:  nil,
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							},
						},
					},
				},
			},
			[]Change{
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre:  nil,
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							},
						},
					},
				},
			},
		},
		{
			[]Change{
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre:  nil,
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
								Balance:   25,
							},
						},
					},
				},
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF"),
								Balance:   20,
							},
						},
					},
					Post: nil,
				},
				{
					Type: xdr.LedgerEntryTypeTtl,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeTtl,
							Ttl: &xdr.TtlEntry{
								KeyHash:            xdr.Hash{1},
								LiveUntilLedgerSeq: 50,
							},
						},
					},
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeTtl,
							Ttl: &xdr.TtlEntry{
								KeyHash:            xdr.Hash{1},
								LiveUntilLedgerSeq: 100,
							},
						},
					},
				},
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
								Balance:   25,
							},
						},
					},
					Post: nil,
				},
			},
			[]Change{
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF"),
								Balance:   20,
							},
						},
					},
					Post: nil,
				},
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre:  nil,
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
								Balance:   25,
							},
						},
					},
				},
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
								Balance:   25,
							},
						},
					},
					Post: nil,
				},

				{
					Type: xdr.LedgerEntryTypeTtl,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeTtl,
							Ttl: &xdr.TtlEntry{
								KeyHash:            xdr.Hash{1},
								LiveUntilLedgerSeq: 50,
							},
						},
					},
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeTtl,
							Ttl: &xdr.TtlEntry{
								KeyHash:            xdr.Hash{1},
								LiveUntilLedgerSeq: 100,
							},
						},
					},
				},
			},
		},
	} {
		sortChanges(testCase.input)
		assert.Equal(t, len(testCase.input), len(testCase.expected))
		for i := range testCase.input {
			assertChangesAreEqual(t, testCase.input[i], testCase.expected[i])
		}
	}
}

package xdr

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLedgerMetaArchive_AddLedgerValidRange(t *testing.T) {

	tests := []struct {
		name     string
		startSeq uint32
		endSeq   uint32
		seqNum   uint32
		errMsg   string
	}{
		{startSeq: 10, endSeq: 100, seqNum: 10, errMsg: ""},
		{startSeq: 10, endSeq: 100, seqNum: 11, errMsg: ""},
		{startSeq: 10, endSeq: 100, seqNum: 99, errMsg: ""},
		{startSeq: 10, endSeq: 100, seqNum: 100, errMsg: ""},
		{startSeq: 10, endSeq: 100, seqNum: 9, errMsg: "ledger sequence 9 is outside valid range [10, 100]"},
		{startSeq: 10, endSeq: 100, seqNum: 101, errMsg: "ledger sequence 101 is outside valid range [10, 100]"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("range [%d, %d]: Add seq %d", tt.startSeq, tt.endSeq, tt.seqNum),
			func(t *testing.T) {
				f := LedgerCloseMetaBatch{StartSequence: Uint32(tt.startSeq), EndSequence: Uint32(tt.endSeq)}
				err := f.AddLedger(LedgerCloseMeta{
					V: int32(0),
					V0: &LedgerCloseMetaV0{
						LedgerHeader: LedgerHeaderHistoryEntry{
							Header: LedgerHeader{
								LedgerSeq: Uint32(tt.seqNum),
							},
						},
						TxSet:              TransactionSet{},
						TxProcessing:       nil,
						UpgradesProcessing: nil,
						ScpInfo:            nil,
					},
					V1: nil,
				})
				if tt.errMsg != "" {
					require.EqualError(t, err, tt.errMsg)
				} else {
					require.NoError(t, err)
				}
			})
	}
}
func TestLedgerMetaArchive_AddLedgerSequential(t *testing.T) {
	var start, end uint32 = 1, 100
	f := LedgerCloseMetaBatch{StartSequence: Uint32(start), EndSequence: Uint32(end + 100)}

	// Add ledgers sequentially
	for i := start; i <= end; i++ {
		require.NoError(t, f.AddLedger(LedgerCloseMeta{
			V: int32(0),
			V0: &LedgerCloseMetaV0{
				LedgerHeader: LedgerHeaderHistoryEntry{
					Header: LedgerHeader{
						LedgerSeq: Uint32(i),
					},
				},
				TxSet:              TransactionSet{},
				TxProcessing:       nil,
				UpgradesProcessing: nil,
				ScpInfo:            nil,
			},
			V1: nil,
		}))
	}

	// Test out of sequence
	testCases := []struct {
		ledgerSeq      uint32
		expectedErrMsg string
	}{
		{
			end + 2,
			fmt.Sprintf("ledgers must be added sequentially: expected sequence %d, got %d", end+1, end+2),
		},
		{
			end,
			fmt.Sprintf("ledgers must be added sequentially: expected sequence %d, got %d", end+1, end),
		},
		{
			end - 1,
			fmt.Sprintf("ledgers must be added sequentially: expected sequence %d, got %d", end+1, end-1),
		},
	}

	for _, tc := range testCases {
		err := f.AddLedger(LedgerCloseMeta{
			V: int32(0),
			V0: &LedgerCloseMetaV0{
				LedgerHeader: LedgerHeaderHistoryEntry{
					Header: LedgerHeader{
						LedgerSeq: Uint32(tc.ledgerSeq),
					},
				},
				TxSet:              TransactionSet{},
				TxProcessing:       nil,
				UpgradesProcessing: nil,
				ScpInfo:            nil,
			},
			V1: nil,
		})
		require.EqualError(t, err, tc.expectedErrMsg)
	}
}

func TestGetLedger(t *testing.T) {
	var start, end uint32 = 121, 1300
	f := LedgerCloseMetaBatch{StartSequence: Uint32(start), EndSequence: Uint32(end)}

	for i := start; i <= end-10; i++ {
		f.LedgerCloseMetas = append(f.LedgerCloseMetas, LedgerCloseMeta{
			V: int32(0),
			V0: &LedgerCloseMetaV0{
				LedgerHeader: LedgerHeaderHistoryEntry{
					Header: LedgerHeader{
						LedgerSeq: Uint32(i),
					},
				},
				TxSet:              TransactionSet{},
				TxProcessing:       nil,
				UpgradesProcessing: nil,
				ScpInfo:            nil,
			},
			V1: nil,
		})
	}

	testCases := []struct {
		name           string
		ledgerSeq      uint32
		expectedErrMsg string
	}{
		{
			name:           "LedgerSequenceInRange",
			ledgerSeq:      start,
			expectedErrMsg: "",
		},
		{
			name:           "LedgerSequenceInRange",
			ledgerSeq:      start + 10,
			expectedErrMsg: "",
		},
		{
			name:      "LedgerSequenceAboveRange",
			ledgerSeq: end + 1,
			expectedErrMsg: fmt.Sprintf("ledger sequence %d is outside the valid range "+
				"of ledger sequences [%d, %d] this batch holds", end+1, start, end),
		},
		{
			name:      "LedgerSequenceBelowRange",
			ledgerSeq: start - 1,
			expectedErrMsg: fmt.Sprintf("ledger sequence %d is outside the valid range "+
				"of ledger sequences [%d, %d] this batch holds", start-1, start, end),
		},
		{
			name:           "LedgerCloseMetaNotFound",
			ledgerSeq:      end - 5,
			expectedErrMsg: fmt.Sprintf("LedgerCloseMeta for sequence %d not found in the batch", end-5),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			archive, err := f.GetLedger(tc.ledgerSeq)
			if tc.expectedErrMsg != "" {
				require.EqualError(t, err, tc.expectedErrMsg)
				require.Equal(t, archive, LedgerCloseMeta{})
			} else {
				require.NoError(t, err)
				require.Equal(t, archive.LedgerSequence(), tc.ledgerSeq)
			}
		})
	}
}

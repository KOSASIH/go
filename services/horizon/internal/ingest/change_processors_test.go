package ingest

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

func TestStreamReaderError(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	mockChangeReader := &ingest.MockChangeReader{}
	mockChangeReader.
		On("Read").
		Return(ingest.Change{}, errors.New("transient error")).Once()
	mockChangeProcessor := &processors.MockChangeProcessor{}

	err := streamChanges(ctx, mockChangeProcessor, 1, mockChangeReader)
	tt.EqualError(err, "could not read transaction: transient error")
}

func TestStreamChangeProcessorError(t *testing.T) {
	tt := assert.New(t)
	ctx := context.Background()

	change := ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
				},
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 11,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   200,
				},
			},
		},
	}
	mockChangeReader := &ingest.MockChangeReader{}
	mockChangeReader.
		On("Read").
		Return(change, nil).Once()

	mockChangeProcessor := &processors.MockChangeProcessor{}
	mockChangeProcessor.
		On(
			"ProcessChange", ctx,
			change,
		).
		Return(errors.New("transient error")).Once()

	logsGet := log.StartTest(logrus.ErrorLevel)
	err := streamChanges(ctx, mockChangeProcessor, 1, mockChangeReader)
	tt.EqualError(err, "could not process change: transient error")
	logs := logsGet()
	line, err := logs[0].String()
	tt.NoError(err)

	preB64, err := xdr.MarshalBase64(change.Pre)
	tt.NoError(err)
	postB64, err := xdr.MarshalBase64(change.Post)
	tt.NoError(err)
	expectedTokens := []string{"LedgerEntryTypeAccount", preB64, postB64}

	for _, token := range expectedTokens {
		tt.Contains(line, token)
	}
}

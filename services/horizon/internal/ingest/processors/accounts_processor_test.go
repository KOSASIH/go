//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"testing"

	"github.com/guregu/null/zero"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/xdr"
)

func TestAccountsProcessorTestSuiteState(t *testing.T) {
	suite.Run(t, new(AccountsProcessorTestSuiteState))
}

type AccountsProcessorTestSuiteState struct {
	suite.Suite
	ctx                            context.Context
	processor                      *AccountsProcessor
	mockQ                          *history.MockQAccounts
	mockAccountsBatchInsertBuilder *history.MockAccountsBatchInsertBuilder
}

func (s *AccountsProcessorTestSuiteState) SetupTest() {
	s.ctx = context.Background()
	s.mockQ = &history.MockQAccounts{}

	s.mockAccountsBatchInsertBuilder = &history.MockAccountsBatchInsertBuilder{}
	s.mockQ.On("NewAccountsBatchInsertBuilder").Return(s.mockAccountsBatchInsertBuilder).Twice()
	s.mockAccountsBatchInsertBuilder.On("Exec", s.ctx).Return(nil).Once()
	s.mockAccountsBatchInsertBuilder.On("Len").Return(1).Maybe()

	s.processor = NewAccountsProcessor(s.mockQ)
}

func (s *AccountsProcessorTestSuiteState) TearDownTest() {
	s.Assert().NoError(s.processor.Commit(s.ctx))
	s.mockQ.AssertExpectations(s.T())
}

func (s *AccountsProcessorTestSuiteState) TestNoEntries() {
	// Nothing processed, assertions in TearDownTest.
}

func (s *AccountsProcessorTestSuiteState) TestCreatesAccounts() {
	// We use LedgerEntryChangesCache so all changes are squashed
	s.mockAccountsBatchInsertBuilder.On("Add", history.AccountEntry{
		LastModifiedLedger: 123,
		AccountID:          "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
		MasterWeight:       1,
		ThresholdLow:       1,
		ThresholdMedium:    1,
		ThresholdHigh:      1,
	}).Return(nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Thresholds: [4]byte{1, 1, 1, 1},
				},
			},
			LastModifiedLedgerSeq: xdr.Uint32(123),
		},
	})
	s.Assert().NoError(err)
}

func TestAccountsProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(AccountsProcessorTestSuiteLedger))
}

type AccountsProcessorTestSuiteLedger struct {
	suite.Suite
	ctx                            context.Context
	processor                      *AccountsProcessor
	mockQ                          *history.MockQAccounts
	mockAccountsBatchInsertBuilder *history.MockAccountsBatchInsertBuilder
}

func (s *AccountsProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockQ = &history.MockQAccounts{}

	s.mockAccountsBatchInsertBuilder = &history.MockAccountsBatchInsertBuilder{}
	s.mockQ.On("NewAccountsBatchInsertBuilder").Return(s.mockAccountsBatchInsertBuilder).Twice()
	s.mockAccountsBatchInsertBuilder.On("Exec", s.ctx).Return(nil).Once()
	s.mockAccountsBatchInsertBuilder.On("Len").Return(1).Maybe()

	s.processor = NewAccountsProcessor(s.mockQ)
}

func (s *AccountsProcessorTestSuiteLedger) TearDownTest() {
	s.Assert().NoError(s.processor.Commit(s.ctx))
	s.mockQ.AssertExpectations(s.T())
}

func (s *AccountsProcessorTestSuiteLedger) TestNoTransactions() {
	// Nothing processed, assertions in TearDownTest.
}

func (s *AccountsProcessorTestSuiteLedger) TestNewAccount() {
	account := xdr.AccountEntry{
		AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Thresholds: [4]byte{1, 1, 1, 1},
	}
	lastModifiedLedgerSeq := xdr.Uint32(123)
	// We use LedgerEntryChangesCache so all changes are squashed
	s.mockAccountsBatchInsertBuilder.On("Add", history.AccountEntry{
		AccountID:          "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
		MasterWeight:       1,
		ThresholdLow:       1,
		ThresholdMedium:    1,
		ThresholdHigh:      1,
		HomeDomain:         "",
		LastModifiedLedger: uint32(123),
	}).Return(nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &account,
			},
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
		},
	})
	s.Assert().NoError(err)

	updatedAccount := xdr.AccountEntry{
		AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Thresholds: [4]byte{0, 1, 2, 3},
		HomeDomain: "stellar.org",
	}

	s.mockQ.On(
		"UpsertAccounts",
		s.ctx,
		[]history.AccountEntry{
			{
				AccountID:          "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
				MasterWeight:       0,
				ThresholdLow:       1,
				ThresholdMedium:    2,
				ThresholdHigh:      3,
				HomeDomain:         "stellar.org",
				LastModifiedLedger: uint32(123),
			},
		},
	).Return(nil).Once()

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq - 1,
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &account,
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &updatedAccount,
			},
		},
	})
	s.Assert().NoError(err)
}

func (s *AccountsProcessorTestSuiteLedger) TestNewAccountUpgrade() {
	account := xdr.AccountEntry{
		AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Thresholds: [4]byte{1, 1, 1, 1},
		Ext: xdr.AccountEntryExt{
			V: 1,
			V1: &xdr.AccountEntryExtensionV1{
				Ext: xdr.AccountEntryExtensionV1Ext{
					V:  2,
					V2: &xdr.AccountEntryExtensionV2{},
				},
			},
		},
	}
	lastModifiedLedgerSeq := xdr.Uint32(123)

	s.mockAccountsBatchInsertBuilder.On("Add", history.AccountEntry{
		AccountID:          "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
		MasterWeight:       1,
		ThresholdLow:       1,
		ThresholdMedium:    1,
		ThresholdHigh:      1,
		HomeDomain:         "",
		LastModifiedLedger: uint32(123),
	}).Return(nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &account,
			},
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
		},
	})
	s.Assert().NoError(err)

	updatedAccount := xdr.AccountEntry{
		AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Thresholds: [4]byte{0, 1, 2, 3},
		HomeDomain: "stellar.org",
		Ext: xdr.AccountEntryExt{
			V: 1,
			V1: &xdr.AccountEntryExtensionV1{
				Ext: xdr.AccountEntryExtensionV1Ext{
					V: 2,
					V2: &xdr.AccountEntryExtensionV2{
						Ext: xdr.AccountEntryExtensionV2Ext{
							V: 3,
							V3: &xdr.AccountEntryExtensionV3{
								SeqLedger: 2346,
								SeqTime:   1647265534,
							},
						},
					},
				},
			},
		},
	}

	s.mockQ.On(
		"UpsertAccounts",
		s.ctx,
		[]history.AccountEntry{
			{
				AccountID:          "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
				SequenceLedger:     zero.IntFrom(2346),
				SequenceTime:       zero.IntFrom(1647265534),
				MasterWeight:       0,
				ThresholdLow:       1,
				ThresholdMedium:    2,
				ThresholdHigh:      3,
				HomeDomain:         "stellar.org",
				LastModifiedLedger: uint32(123),
			},
		},
	).Return(nil).Once()

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq - 1,
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &account,
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &updatedAccount,
			},
		},
	})
	s.Assert().NoError(err)

}

func (s *AccountsProcessorTestSuiteLedger) TestRemoveAccount() {
	s.mockQ.On(
		"RemoveAccounts",
		s.ctx,
		[]string{"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"},
	).Return(int64(1), nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Thresholds: [4]byte{1, 1, 1, 1},
				},
			},
		},
		Post: nil,
	})
	s.Assert().NoError(err)
}

func (s *AccountsProcessorTestSuiteLedger) TestProcessUpgradeChange() {
	account := xdr.AccountEntry{
		AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Thresholds: [4]byte{1, 1, 1, 1},
	}
	lastModifiedLedgerSeq := xdr.Uint32(123)

	s.mockAccountsBatchInsertBuilder.On("Add", history.AccountEntry{
		LastModifiedLedger: uint32(lastModifiedLedgerSeq),
		AccountID:          "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
		MasterWeight:       1,
		ThresholdLow:       1,
		ThresholdMedium:    1,
		ThresholdHigh:      1,
	}).Return(nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &account,
			},
		},
	})
	s.Assert().NoError(err)

	updatedAccount := xdr.AccountEntry{
		AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Thresholds: [4]byte{0, 1, 2, 3},
		HomeDomain: "stellar.org",
	}

	s.mockQ.On(
		"UpsertAccounts",
		s.ctx,
		[]history.AccountEntry{
			{
				LastModifiedLedger: uint32(lastModifiedLedgerSeq) + 1,
				AccountID:          "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
				SequenceTime:       zero.IntFrom(0),
				SequenceLedger:     zero.IntFrom(0),
				MasterWeight:       0,
				ThresholdLow:       1,
				ThresholdMedium:    2,
				ThresholdHigh:      3,
				HomeDomain:         "stellar.org",
			},
		},
	).Return(nil).Once()

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &account,
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq + 1,
			Data: xdr.LedgerEntryData{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &updatedAccount,
			},
		},
	})
	s.Assert().NoError(err)
}

func (s *AccountsProcessorTestSuiteLedger) TestFeeProcessedBeforeEverythingElse() {
	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
					Balance:   200,
				},
			},
		},
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
					Balance:   100,
				},
			},
		},
	})
	s.Assert().NoError(err)

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
					Balance:   100,
				},
			},
		},
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
					Balance:   300,
				},
			},
		},
	})
	s.Assert().NoError(err)

	s.mockQ.On(
		"UpsertAccounts",
		s.ctx,
		[]history.AccountEntry{
			{
				LastModifiedLedger: 0,
				AccountID:          "GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A",
				Balance:            100,
			},
			{
				LastModifiedLedger: 0,
				AccountID:          "GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A",
				Balance:            300,
			},
		},
	).Return(nil).Once()
}

//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

func TestAccountsSignerProcessorTestSuiteState(t *testing.T) {
	suite.Run(t, new(AccountsSignerProcessorTestSuiteState))
}

type AccountsSignerProcessorTestSuiteState struct {
	suite.Suite
	ctx                    context.Context
	processor              *SignersProcessor
	mockQ                  *history.MockQSigners
	mockBatchInsertBuilder *history.MockAccountSignersBatchInsertBuilder
}

func (s *AccountsSignerProcessorTestSuiteState) SetupTest() {
	s.ctx = context.Background()
	s.mockQ = &history.MockQSigners{}
	s.mockBatchInsertBuilder = &history.MockAccountSignersBatchInsertBuilder{}

	s.mockQ.
		On("NewAccountSignersBatchInsertBuilder").
		Return(s.mockBatchInsertBuilder).Twice()
	s.mockBatchInsertBuilder.On("Exec", s.ctx).Return(nil)
	s.mockBatchInsertBuilder.On("Len").Return(1).Maybe()
	s.processor = NewSignersProcessor(s.mockQ)
}

func (s *AccountsSignerProcessorTestSuiteState) TearDownTest() {
	s.Assert().NoError(s.processor.Commit(s.ctx))

	s.mockQ.AssertExpectations(s.T())
	s.mockBatchInsertBuilder.AssertExpectations(s.T())
}

func (s *AccountsSignerProcessorTestSuiteState) TestNoEntries() {
	// Nothing processed, assertions in TearDownTest.
}

func (s *AccountsSignerProcessorTestSuiteState) TestCreatesSigners() {
	s.mockBatchInsertBuilder.
		On("Add", history.AccountSigner{
			Account: "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			Signer:  "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			Weight:  int32(1),
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
		},
	})
	s.Assert().NoError(err)

	s.mockBatchInsertBuilder.
		On("Add", history.AccountSigner{
			Account: "GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX",
			Signer:  "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			Weight:  int32(10),
		}).Return(nil).Once()

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Weight: 10,
						},
					},
				},
			},
		},
	})
	s.Assert().NoError(err)

}

func (s *AccountsSignerProcessorTestSuiteState) TestCreatesSignerWithSponsor() {
	s.mockBatchInsertBuilder.
		On("Add", history.AccountSigner{
			Account: "GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX",
			Signer:  "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			Weight:  int32(10),
			Sponsor: null.StringFrom("GDWZ6MKJP5ESVIB7O5RW4UFFGSCDILPEKDXWGG4HXXSHEZZPTKLR6UVG"),
		}).Return(nil).Once()

	sponsorshipDescriptor := xdr.MustAddress("GDWZ6MKJP5ESVIB7O5RW4UFFGSCDILPEKDXWGG4HXXSHEZZPTKLR6UVG")

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							Weight: 10,
						},
					},
					Ext: xdr.AccountEntryExt{
						V: 1,
						V1: &xdr.AccountEntryExtensionV1{
							Ext: xdr.AccountEntryExtensionV1Ext{
								V: 2,
								V2: &xdr.AccountEntryExtensionV2{
									NumSponsored:  1,
									NumSponsoring: 0,
									SignerSponsoringIDs: []xdr.SponsorshipDescriptor{
										&sponsorshipDescriptor,
									},
								},
							},
						},
					},
				},
			},
		},
	})
	s.Assert().NoError(err)
}

func TestAccountsSignerProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(AccountsSignerProcessorTestSuiteLedger))
}

type AccountsSignerProcessorTestSuiteLedger struct {
	suite.Suite
	ctx                                  context.Context
	processor                            *SignersProcessor
	mockQ                                *history.MockQSigners
	mockAccountSignersBatchInsertBuilder *history.MockAccountSignersBatchInsertBuilder
}

func (s *AccountsSignerProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockQ = &history.MockQSigners{}
	s.mockAccountSignersBatchInsertBuilder = &history.MockAccountSignersBatchInsertBuilder{}
	s.mockQ.
		On("NewAccountSignersBatchInsertBuilder").
		Return(s.mockAccountSignersBatchInsertBuilder)
	s.mockAccountSignersBatchInsertBuilder.On("Exec", s.ctx).Return(nil)
	s.mockAccountSignersBatchInsertBuilder.On("Len").Return(1).Maybe()

	s.processor = NewSignersProcessor(s.mockQ)
}

func (s *AccountsSignerProcessorTestSuiteLedger) TearDownTest() {
	s.mockQ.AssertExpectations(s.T())
}

func (s *AccountsSignerProcessorTestSuiteLedger) TestNoTransactions() {
	// Nothing processed, assertions in TearDownTest.
	s.Assert().NoError(s.processor.Commit(context.Background()))
}

func (s *AccountsSignerProcessorTestSuiteLedger) TestNewAccount() {
	s.mockAccountSignersBatchInsertBuilder.
		On(
			"Add",
			history.AccountSigner{
				"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
				"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
				int32(1),
				null.String{},
			},
		).
		Return(nil).Once()

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
		},
	})
	s.Assert().NoError(err)
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *AccountsSignerProcessorTestSuiteLedger) TestNoUpdatesWhenNoSignerChanges() {
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
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Thresholds: [4]byte{1, 1, 1, 1},
				},
			},
		},
	})
	s.Assert().NoError(err)
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *AccountsSignerProcessorTestSuiteLedger) TestNewSigner() {
	// Create new signer
	s.mockAccountSignersBatchInsertBuilder.
		On(
			"Add",
			history.AccountSigner{
				"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
				"GCAHY6JSXQFKWKP6R7U5JPXDVNV4DJWOWRFLY3Y6YPBF64QRL4BPFDNS",
				int32(15),
				null.StringFromPtr(nil)},
		).
		Return(nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 10,
						},
					},
				},
			},
		},
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 10,
						},
						{
							Key:    xdr.MustSigner("GCAHY6JSXQFKWKP6R7U5JPXDVNV4DJWOWRFLY3Y6YPBF64QRL4BPFDNS"),
							Weight: 15,
						},
					},
				},
			},
		},
	})
	s.Assert().NoError(err)
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *AccountsSignerProcessorTestSuiteLedger) TestSignerRemoved() {
	// Remove old signers
	s.mockQ.
		On(
			"RemoveAccountSigners",
			s.ctx,
			"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			[]string{"GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"},
		).
		Return(int64(1), nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 10,
						},
						{
							Key:    xdr.MustSigner("GCAHY6JSXQFKWKP6R7U5JPXDVNV4DJWOWRFLY3Y6YPBF64QRL4BPFDNS"),
							Weight: 15,
						},
					},
				},
			},
		},
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCAHY6JSXQFKWKP6R7U5JPXDVNV4DJWOWRFLY3Y6YPBF64QRL4BPFDNS"),
							Weight: 15,
						},
					},
				},
			},
		},
	})
	s.Assert().NoError(err)
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

// TestSignerPreAuthTxRemovedTxFailed tests if removing preauthorized transaction
// signer works even when tx failed.
func (s *AccountsSignerProcessorTestSuiteLedger) TestSignerPreAuthTxRemovedTxFailed() {
	// Remove old signers
	s.mockQ.
		On(
			"RemoveAccountSigners",
			s.ctx,
			"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			[]string{"TBU2RRGLXH3E5CQHTD3ODLDF2BWDCYUSSBLLZ5GNW7JXHDIYKXZWHXL7"},
		).
		Return(int64(1), nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 10,
						},
						{
							Key:    xdr.MustSigner("TBU2RRGLXH3E5CQHTD3ODLDF2BWDCYUSSBLLZ5GNW7JXHDIYKXZWHXL7"),
							Weight: 15,
						},
					},
				},
			},
		},
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 10,
						},
					},
				},
			},
		},
	})
	s.Assert().NoError(err)
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *AccountsSignerProcessorTestSuiteLedger) TestRemoveAccount() {
	s.mockQ.
		On(
			"RemoveAccountSigners",
			s.ctx,
			"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			[]string{"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"},
		).
		Return(int64(1), nil).Once()

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
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *AccountsSignerProcessorTestSuiteLedger) TestRemoveAccountNoRowsAffected() {
	s.mockQ.
		On(
			"RemoveAccountSigners",
			s.ctx,
			"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			[]string{"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"},
		).
		Return(int64(0), nil).Once()

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
	s.Assert().Error(err)
	s.Assert().IsType(ingest.StateError{}, errors.Cause(err))
	s.Assert().EqualError(
		err,
		"Expected "+
			"account=GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML "+
			"signers=[GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML] in database but not found when removing "+
			"(rows affected = 0)",
	)
}

func (s *AccountsSignerProcessorTestSuiteLedger) TestProcessUpgradeChange() {
	s.mockAccountSignersBatchInsertBuilder.
		On(
			"Add",
			history.AccountSigner{
				"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
				"GCAHY6JSXQFKWKP6R7U5JPXDVNV4DJWOWRFLY3Y6YPBF64QRL4BPFDNS",
				int32(15),
				null.String{},
			},
		).Return(nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 10,
						},
					},
				},
			},
		},
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 10,
						},
						{
							Key:    xdr.MustSigner("GCAHY6JSXQFKWKP6R7U5JPXDVNV4DJWOWRFLY3Y6YPBF64QRL4BPFDNS"),
							Weight: 15,
						},
					},
				},
			},
		},
	})
	s.Assert().NoError(err)

	// Remove old signer
	s.mockQ.
		On(
			"RemoveAccountSigners",
			s.ctx,
			"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			mock.MatchedBy(func(signers []string) bool {
				return assert.ElementsMatch(s.T(),
					[]string{
						"GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV",
						"GCAHY6JSXQFKWKP6R7U5JPXDVNV4DJWOWRFLY3Y6YPBF64QRL4BPFDNS",
					},
					signers,
				)
			}),
		).
		Return(int64(2), nil).Once()

	// Create new and old (updated) signer
	s.mockAccountSignersBatchInsertBuilder.
		On(
			"Add",
			history.AccountSigner{
				"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
				"GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV",
				int32(12),
				null.String{},
			},
		).
		Return(nil).Once()

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 1000,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 10,
						},
						{
							Key:    xdr.MustSigner("GCAHY6JSXQFKWKP6R7U5JPXDVNV4DJWOWRFLY3Y6YPBF64QRL4BPFDNS"),
							Weight: 15,
						},
					},
				},
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 1001,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCBBDQLCTNASZJ3MTKAOYEOWRGSHDFAJVI7VPZUOP7KXNHYR3HP2BUKV"),
							Weight: 12,
						},
					},
				},
			},
		},
	})
	s.Assert().NoError(err)
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func createTransactionMeta(opMeta []xdr.OperationMeta) xdr.TransactionMeta {
	return xdr.TransactionMeta{
		V: 1,
		V1: &xdr.TransactionMetaV1{
			Operations: opMeta,
		},
	}
}

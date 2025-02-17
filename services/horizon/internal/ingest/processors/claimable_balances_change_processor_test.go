//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"testing"

	"github.com/guregu/null"

	"github.com/stretchr/testify/suite"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/xdr"
)

func TestClaimableBalancesChangeProcessorTestSuiteState(t *testing.T) {
	suite.Run(t, new(ClaimableBalancesChangeProcessorTestSuiteState))
}

type ClaimableBalancesChangeProcessorTestSuiteState struct {
	suite.Suite
	ctx                                    context.Context
	processor                              *ClaimableBalancesChangeProcessor
	mockQ                                  *history.MockQClaimableBalances
	mockClaimantsBatchInsertBuilder        *history.MockClaimableBalanceClaimantBatchInsertBuilder
	mockClaimableBalanceBatchInsertBuilder *history.MockClaimableBalanceBatchInsertBuilder
}

func (s *ClaimableBalancesChangeProcessorTestSuiteState) SetupTest() {
	s.ctx = context.Background()
	s.mockClaimantsBatchInsertBuilder = &history.MockClaimableBalanceClaimantBatchInsertBuilder{}
	s.mockClaimableBalanceBatchInsertBuilder = &history.MockClaimableBalanceBatchInsertBuilder{}

	s.mockQ = &history.MockQClaimableBalances{}
	s.mockQ.
		On("NewClaimableBalanceClaimantBatchInsertBuilder").
		Return(s.mockClaimantsBatchInsertBuilder)
	s.mockQ.
		On("NewClaimableBalanceBatchInsertBuilder").
		Return(s.mockClaimableBalanceBatchInsertBuilder)

	s.mockClaimantsBatchInsertBuilder.On("Exec", s.ctx).Return(nil)
	s.mockClaimableBalanceBatchInsertBuilder.On("Exec", s.ctx).Return(nil)
	s.mockClaimantsBatchInsertBuilder.On("Len").Return(1).Maybe()
	s.mockClaimableBalanceBatchInsertBuilder.On("Len").Return(1).Maybe()

	s.processor = NewClaimableBalancesChangeProcessor(s.mockQ)
}

func (s *ClaimableBalancesChangeProcessorTestSuiteState) TearDownTest() {
	s.Assert().NoError(s.processor.Commit(s.ctx))
	s.mockQ.AssertExpectations(s.T())
}

func (s *ClaimableBalancesChangeProcessorTestSuiteState) TestNoEntries() {
	// Nothing processed, assertions in TearDownTest.
}

func (s *ClaimableBalancesChangeProcessorTestSuiteState) TestCreatesClaimableBalances() {
	lastModifiedLedgerSeq := xdr.Uint32(123)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}

	cBalance := xdr.ClaimableBalanceEntry{
		BalanceId: balanceID,
		Claimants: []xdr.Claimant{
			{
				V0: &xdr.ClaimantV0{
					Destination: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
				},
			},
		},
		Asset:  xdr.MustNewCreditAsset("USD", "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Amount: 10,
	}
	id, err := xdr.MarshalHex(balanceID)
	s.Assert().NoError(err)
	s.mockClaimableBalanceBatchInsertBuilder.On("Add", history.ClaimableBalance{
		BalanceID: id,
		Claimants: []history.Claimant{
			{
				Destination: "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			},
		},
		Asset:              cBalance.Asset,
		Amount:             cBalance.Amount,
		LastModifiedLedger: uint32(lastModifiedLedgerSeq),
	}).Return(nil).Once()

	s.mockClaimableBalanceBatchInsertBuilder.On("Exec", s.ctx).Return(nil).Once()

	s.mockClaimantsBatchInsertBuilder.On("Add", history.ClaimableBalanceClaimant{
		BalanceID:          id,
		Destination:        "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
		LastModifiedLedger: uint32(lastModifiedLedgerSeq),
	}).Return(nil).Once()

	s.mockClaimantsBatchInsertBuilder.On("Exec", s.ctx).Return(nil).Once()

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeClaimableBalance,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type:             xdr.LedgerEntryTypeClaimableBalance,
				ClaimableBalance: &cBalance,
			},
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
		},
	})
	s.Assert().NoError(err)
}

func TestClaimableBalancesChangeProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(ClaimableBalancesChangeProcessorTestSuiteLedger))
}

type ClaimableBalancesChangeProcessorTestSuiteLedger struct {
	suite.Suite
	ctx                                    context.Context
	processor                              *ClaimableBalancesChangeProcessor
	mockQ                                  *history.MockQClaimableBalances
	mockClaimantsBatchInsertBuilder        *history.MockClaimableBalanceClaimantBatchInsertBuilder
	mockClaimableBalanceBatchInsertBuilder *history.MockClaimableBalanceBatchInsertBuilder
}

func (s *ClaimableBalancesChangeProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockClaimantsBatchInsertBuilder = &history.MockClaimableBalanceClaimantBatchInsertBuilder{}
	s.mockClaimableBalanceBatchInsertBuilder = &history.MockClaimableBalanceBatchInsertBuilder{}
	s.mockQ = &history.MockQClaimableBalances{}
	s.mockQ.
		On("NewClaimableBalanceClaimantBatchInsertBuilder").
		Return(s.mockClaimantsBatchInsertBuilder)
	s.mockQ.
		On("NewClaimableBalanceBatchInsertBuilder").
		Return(s.mockClaimableBalanceBatchInsertBuilder)

	s.mockClaimantsBatchInsertBuilder.On("Exec", s.ctx).Return(nil)
	s.mockClaimableBalanceBatchInsertBuilder.On("Exec", s.ctx).Return(nil)
	s.mockClaimantsBatchInsertBuilder.On("Len").Return(1).Maybe()
	s.mockClaimableBalanceBatchInsertBuilder.On("Len").Return(1).Maybe()

	s.processor = NewClaimableBalancesChangeProcessor(s.mockQ)
}

func (s *ClaimableBalancesChangeProcessorTestSuiteLedger) TearDownTest() {
	s.Assert().NoError(s.processor.Commit(s.ctx))
	s.processor.reset()
	s.mockQ.AssertExpectations(s.T())
}

func (s *ClaimableBalancesChangeProcessorTestSuiteLedger) TestNoTransactions() {
	// Nothing processed, assertions in TearDownTest.
}

func (s *ClaimableBalancesChangeProcessorTestSuiteLedger) TestNewClaimableBalance() {
	lastModifiedLedgerSeq := xdr.Uint32(123)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	cBalance := xdr.ClaimableBalanceEntry{
		BalanceId: balanceID,
		Claimants: []xdr.Claimant{},
		Asset:     xdr.MustNewCreditAsset("USD", "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Amount:    10,
	}
	entry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type:             xdr.LedgerEntryTypeClaimableBalance,
			ClaimableBalance: &cBalance,
		},
		LastModifiedLedgerSeq: lastModifiedLedgerSeq,
		Ext: xdr.LedgerEntryExt{
			V: 1,
			V1: &xdr.LedgerEntryExtensionV1{
				SponsoringId: xdr.MustAddressPtr("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
			},
		},
	}

	id, err := xdr.MarshalHex(balanceID)
	s.Assert().NoError(err)

	// We use LedgerEntryChangesCache so all changes are squashed
	s.mockClaimableBalanceBatchInsertBuilder.On(
		"Add",
		history.ClaimableBalance{
			BalanceID:          id,
			Claimants:          []history.Claimant{},
			Asset:              cBalance.Asset,
			Amount:             cBalance.Amount,
			LastModifiedLedger: uint32(lastModifiedLedgerSeq),
			Sponsor:            null.StringFrom("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		},
	).Return(nil).Once()

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeClaimableBalance,
		Pre:  nil,
		Post: &entry,
	})
	s.Assert().NoError(err)
}

func (s *ClaimableBalancesChangeProcessorTestSuiteLedger) TestRemoveClaimableBalance() {
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	cBalance := xdr.ClaimableBalanceEntry{
		BalanceId: balanceID,
		Claimants: []xdr.Claimant{},
		Asset:     xdr.MustNewCreditAsset("USD", "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Amount:    10,
	}
	lastModifiedLedgerSeq := xdr.Uint32(123)
	pre := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type:             xdr.LedgerEntryTypeClaimableBalance,
			ClaimableBalance: &cBalance,
		},
		LastModifiedLedgerSeq: lastModifiedLedgerSeq - 1,
		Ext: xdr.LedgerEntryExt{
			V: 1,
			V1: &xdr.LedgerEntryExtensionV1{
				SponsoringId: nil,
			},
		},
	}
	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeClaimableBalance,
		Pre:  &pre,
		Post: nil,
	})
	s.Assert().NoError(err)

	id, err := xdr.MarshalHex(balanceID)
	s.Assert().NoError(err)
	s.mockQ.On(
		"RemoveClaimableBalances",
		s.ctx,
		[]string{id},
	).Return(int64(1), nil).Once()

	s.mockQ.On(
		"RemoveClaimableBalanceClaimants",
		s.ctx,
		[]string{id},
	).Return(int64(1), nil).Once()
}

func (s *ClaimableBalancesChangeProcessorTestSuiteLedger) TestUpdateClaimableBalanceAddSponsor() {
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	cBalance := xdr.ClaimableBalanceEntry{
		BalanceId: balanceID,
		Claimants: []xdr.Claimant{},
		Asset:     xdr.MustNewCreditAsset("USD", "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		Amount:    10,
	}
	lastModifiedLedgerSeq := xdr.Uint32(123)

	pre := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type:             xdr.LedgerEntryTypeClaimableBalance,
			ClaimableBalance: &cBalance,
		},
		LastModifiedLedgerSeq: lastModifiedLedgerSeq - 1,
		Ext: xdr.LedgerEntryExt{
			V: 1,
			V1: &xdr.LedgerEntryExtensionV1{
				SponsoringId: nil,
			},
		},
	}

	// add sponsor
	updated := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type:             xdr.LedgerEntryTypeClaimableBalance,
			ClaimableBalance: &cBalance,
		},
		LastModifiedLedgerSeq: lastModifiedLedgerSeq,
		Ext: xdr.LedgerEntryExt{
			V: 1,
			V1: &xdr.LedgerEntryExtensionV1{
				SponsoringId: xdr.MustAddressPtr("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
			},
		},
	}
	s.mockClaimableBalanceBatchInsertBuilder.On("Exec", s.ctx).Return(nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeClaimableBalance,
		Pre:  &pre,
		Post: &updated,
	})
	s.Assert().NoError(err)

	id, err := xdr.MarshalHex(balanceID)
	s.Assert().NoError(err)
	s.mockQ.On(
		"UpsertClaimableBalances",
		s.ctx,
		[]history.ClaimableBalance{
			{
				BalanceID:          id,
				Claimants:          []history.Claimant{},
				Asset:              cBalance.Asset,
				Amount:             cBalance.Amount,
				LastModifiedLedger: uint32(lastModifiedLedgerSeq),
				Sponsor:            null.StringFrom("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
			},
		},
	).Return(nil).Once()
}

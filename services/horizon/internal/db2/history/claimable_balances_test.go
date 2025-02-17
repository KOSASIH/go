package history

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/guregu/null"

	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/xdr"
)

func TestRemoveClaimableBalance(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}
	tt.Assert.NoError(q.BeginTx(tt.Ctx, &sql.TxOptions{}))
	defer func() {
		_ = q.Rollback()
	}()

	accountID := "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
	asset := xdr.MustNewCreditAsset("USD", accountID)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	id, err := xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance := ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: accountID,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 123,
		Amount:             10,
	}

	balanceBatchInsertBuilder := q.NewClaimableBalanceBatchInsertBuilder()
	tt.Assert.NoError(balanceBatchInsertBuilder.Add(cBalance))
	tt.Assert.NoError(balanceBatchInsertBuilder.Exec(tt.Ctx))

	r, err := q.FindClaimableBalanceByID(tt.Ctx, id)
	tt.Assert.NoError(err)
	tt.Assert.NotNil(r)

	removed, err := q.RemoveClaimableBalances(tt.Ctx, []string{id})
	tt.Assert.NoError(err)
	tt.Assert.Equal(int64(1), removed)

	cbs := []ClaimableBalance{}
	err = q.Select(tt.Ctx, &cbs, selectClaimableBalances)

	if tt.Assert.NoError(err) {
		tt.Assert.Len(cbs, 0)
	}
}

func TestRemoveClaimableBalanceClaimants(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}
	tt.Assert.NoError(q.BeginTx(tt.Ctx, &sql.TxOptions{}))
	defer func() {
		_ = q.Rollback()
	}()

	accountID := "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
	asset := xdr.MustNewCreditAsset("USD", accountID)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	id, err := xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance := ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: accountID,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 123,
		Amount:             10,
	}

	claimantsInsertBuilder := q.NewClaimableBalanceClaimantBatchInsertBuilder()
	tt.Assert.NoError(insertClaimants(claimantsInsertBuilder, cBalance))
	tt.Assert.NoError(claimantsInsertBuilder.Exec(tt.Ctx))

	removed, err := q.RemoveClaimableBalanceClaimants(tt.Ctx, []string{id})
	tt.Assert.NoError(err)
	tt.Assert.Equal(int64(1), removed)
}

func TestFindClaimableBalancesByDestination(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.BeginTx(tt.Ctx, &sql.TxOptions{}))
	defer func() {
		_ = q.Rollback()
	}()

	dest1 := "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
	dest2 := "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"

	asset := xdr.MustNewCreditAsset("USD", dest1)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	id, err := xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance := ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: dest1,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 123,
		Amount:             10,
	}

	balanceInsertBuilder := q.NewClaimableBalanceBatchInsertBuilder()
	claimantsInsertBuilder := q.NewClaimableBalanceClaimantBatchInsertBuilder()

	tt.Assert.NoError(balanceInsertBuilder.Add(cBalance))
	tt.Assert.NoError(insertClaimants(claimantsInsertBuilder, cBalance))

	balanceID = xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{3, 2, 1},
	}
	id, err = xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance = ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: dest1,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
			{
				Destination: dest2,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 300,
		Amount:             10,
	}

	tt.Assert.NoError(balanceInsertBuilder.Add(cBalance))
	tt.Assert.NoError(insertClaimants(claimantsInsertBuilder, cBalance))

	tt.Assert.NoError(claimantsInsertBuilder.Exec(tt.Ctx))
	tt.Assert.NoError(balanceInsertBuilder.Exec(tt.Ctx))

	query := ClaimableBalancesQuery{
		PageQuery: db2.MustPageQuery("", false, "", 10),
		Claimant:  xdr.MustAddressPtr(dest1),
	}

	// this validates the cb query with claimant parameter
	cbs, err := q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 2)

	for _, cb := range cbs {
		tt.Assert.Equal(dest1, cb.Claimants[0].Destination)
	}

	// this validates the cb query with different claimant parameter
	query.Claimant = xdr.MustAddressPtr(dest2)
	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	// this validates the cb query with claimant and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 150, cbs[0].BalanceID), false, "asc", 10)
	query.Claimant = xdr.MustAddressPtr(dest1)
	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	// this validates the cb query with no claimant parameter,
	// should still produce working sql, as it triggers different LIMIT position in sql.
	query.PageQuery = db2.MustPageQuery("", false, "desc", 1)
	query.Claimant = nil
	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
}

func TestFindClaimableBalancesByCursor(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.BeginTx(tt.Ctx, &sql.TxOptions{}))
	defer func() {
		_ = q.Rollback()
	}()

	balanceInsertBuilder := q.NewClaimableBalanceBatchInsertBuilder()
	claimantsInsertBuilder := q.NewClaimableBalanceClaimantBatchInsertBuilder()

	dest1 := "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
	dest2 := "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"

	sponsor1 := "GA25GQLHJU3LPEJXEIAXK23AWEA5GWDUGRSHTQHDFT6HXHVMRULSQJUJ"
	sponsor2 := "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"

	asset := xdr.MustNewCreditAsset("USD", dest1)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	id, err := xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance := ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: dest1,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 123,
		Amount:             10,
		Sponsor:            null.StringFrom(sponsor1),
	}

	tt.Assert.NoError(balanceInsertBuilder.Add(cBalance))
	tt.Assert.NoError(insertClaimants(claimantsInsertBuilder, cBalance))

	balanceID = xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{3, 2, 1},
	}
	id, err = xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance = ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: dest1,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
			{
				Destination: dest2,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 300,
		Amount:             10,
		Sponsor:            null.StringFrom(sponsor2),
	}

	tt.Assert.NoError(balanceInsertBuilder.Add(cBalance))
	tt.Assert.NoError(insertClaimants(claimantsInsertBuilder, cBalance))

	tt.Assert.NoError(claimantsInsertBuilder.Exec(tt.Ctx))
	tt.Assert.NoError(balanceInsertBuilder.Exec(tt.Ctx))

	query := ClaimableBalancesQuery{
		PageQuery: db2.MustPageQuery("", false, "", 10),
	}

	cbs, err := q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 2)

	order := "" // default is "asc"
	// this validates the cb query with claimant and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 150, cbs[0].BalanceID), false, order, 10)
	query.Claimant = xdr.MustAddressPtr(dest1)
	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	// this validates the cb query with claimant, asset, sponsor and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 150, cbs[0].BalanceID), false, order, 10)
	query.Claimant = xdr.MustAddressPtr(dest1)
	query.Asset = &asset
	query.Sponsor = xdr.MustAddressPtr(sponsor2)

	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	// this validates the cb query with no claimant, asset, sponsor and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 150, cbs[0].BalanceID), false, order, 10)
	query.Claimant = nil
	query.Asset = &asset
	query.Sponsor = xdr.MustAddressPtr(sponsor2)

	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	order = "desc"
	// claimant and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 301, cbs[0].BalanceID), false, order, 10)
	query.Claimant = xdr.MustAddressPtr(dest1)
	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	// claimant, asset, sponsor and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 301, cbs[0].BalanceID), false, order, 10)
	query.Claimant = xdr.MustAddressPtr(dest1)
	query.Asset = &asset
	query.Sponsor = xdr.MustAddressPtr(sponsor2)

	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	// no claimant, asset, sponsor and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 301, cbs[0].BalanceID), false, order, 10)
	query.Claimant = nil
	query.Asset = &asset
	query.Sponsor = xdr.MustAddressPtr(sponsor2)

	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	order = "asc"
	// claimant and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 150, cbs[0].BalanceID), false, order, 10)
	query.Claimant = xdr.MustAddressPtr(dest1)
	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	// claimant, asset, sponsor and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 150, cbs[0].BalanceID), false, order, 10)
	query.Claimant = xdr.MustAddressPtr(dest1)
	query.Asset = &asset
	query.Sponsor = xdr.MustAddressPtr(sponsor2)

	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)

	// no claimant, asset, sponsor and cb.id/ledger cursor parameters
	query.PageQuery = db2.MustPageQuery(fmt.Sprintf("%v-%s", 150, cbs[0].BalanceID), false, order, 10)
	query.Claimant = nil
	query.Asset = &asset
	query.Sponsor = xdr.MustAddressPtr(sponsor2)

	cbs, err = q.GetClaimableBalances(tt.Ctx, query)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal(dest2, cbs[0].Claimants[1].Destination)
}

func insertClaimants(claimantsInsertBuilder ClaimableBalanceClaimantBatchInsertBuilder, cBalance ClaimableBalance) error {
	for _, claimant := range cBalance.Claimants {
		claimant := ClaimableBalanceClaimant{
			BalanceID:          cBalance.BalanceID,
			Destination:        claimant.Destination,
			LastModifiedLedger: cBalance.LastModifiedLedger,
		}
		err := claimantsInsertBuilder.Add(claimant)
		if err != nil {
			return err
		}
	}
	return nil
}

type claimableBalanceQueryResult struct {
	Claimants []string
	Asset     string
	Sponsor   string
}

func validateClaimableBalanceQuery(t *test.T, q *Q, query ClaimableBalancesQuery, expectedQueryResult []claimableBalanceQueryResult) {
	cbs, err := q.GetClaimableBalances(t.Ctx, query)
	t.Assert.NoError(err)
	for i, expected := range expectedQueryResult {
		for j, claimant := range expected.Claimants {
			t.Assert.Equal(claimant, cbs[i].Claimants[j].Destination)
		}
		if expected.Asset != "" {
			t.Assert.Equal(expected.Asset, cbs[i].Asset.String())
		}
		if expected.Sponsor != "" {
			t.Assert.Equal(expected.Sponsor, cbs[i].Sponsor.String)
		}
	}
}

// TestFindClaimableBalancesByDestinationWithLimit tests querying claimable balances by destination and asset
func TestFindClaimableBalancesByDestinationWithLimit(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()

	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.BeginTx(tt.Ctx, &sql.TxOptions{}))
	defer func() {
		_ = q.Rollback()
	}()

	assetIssuer := "GA25GQLHJU3LPEJXEIAXK23AWEA5GWDUGRSHTQHDFT6HXHVMRULSQJUJ"
	asset1 := xdr.MustNewCreditAsset("ASSET1", assetIssuer)
	asset2 := xdr.MustNewCreditAsset("ASSET2", assetIssuer)

	sponsor1 := "GA25GQLHJU3LPEJXEIAXK23AWEA5GWDUGRSHTQHDFT6HXHVMRULSQJUJ"
	sponsor2 := "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"

	dest1 := "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
	dest2 := "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"

	claimants := []Claimant{
		{
			Destination: dest1,
			Predicate: xdr.ClaimPredicate{
				Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
			},
		},
		{
			Destination: dest2,
			Predicate: xdr.ClaimPredicate{
				Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
			},
		},
	}

	balanceID1 := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	id, err := xdr.MarshalHex(balanceID1)
	tt.Assert.NoError(err)
	cBalance1 := ClaimableBalance{
		BalanceID:          id,
		Claimants:          claimants,
		Asset:              asset1,
		Sponsor:            null.StringFrom(sponsor1),
		LastModifiedLedger: 123,
		Amount:             10,
	}
	balanceInsertBuilder := q.NewClaimableBalanceBatchInsertBuilder()
	claimantsInsertBuilder := q.NewClaimableBalanceClaimantBatchInsertBuilder()

	tt.Assert.NoError(balanceInsertBuilder.Add(cBalance1))
	tt.Assert.NoError(insertClaimants(claimantsInsertBuilder, cBalance1))

	claimants2 := []Claimant{
		{
			Destination: dest2,
			Predicate: xdr.ClaimPredicate{
				Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
			},
		},
	}

	balanceID2 := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{4, 5, 6},
	}
	id, err = xdr.MarshalHex(balanceID2)
	tt.Assert.NoError(err)
	cBalance2 := ClaimableBalance{
		BalanceID: id,
		Claimants: claimants2,
		Asset:     asset2,
		Sponsor:   null.StringFrom(sponsor2),

		LastModifiedLedger: 456,
		Amount:             10,
	}

	tt.Assert.NoError(balanceInsertBuilder.Add(cBalance2))
	tt.Assert.NoError(insertClaimants(claimantsInsertBuilder, cBalance2))

	tt.Assert.NoError(claimantsInsertBuilder.Exec(tt.Ctx))
	tt.Assert.NoError(balanceInsertBuilder.Exec(tt.Ctx))

	pageQuery := db2.MustPageQuery("", false, "", 1)

	// no claimant parameter, no filters
	query := ClaimableBalancesQuery{
		PageQuery: pageQuery,
	}
	validateClaimableBalanceQuery(tt, q, query, []claimableBalanceQueryResult{
		{Claimants: []string{dest1, dest2}},
	})

	// invalid claimant parameter
	query = ClaimableBalancesQuery{
		PageQuery: pageQuery,
		Claimant:  xdr.MustAddressPtr("GA25GQLHJU3LPEJXEIAXK23AWEA5GWDUGRSHTQHDFT6HXHVMRULSQJUJ"),
		Asset:     &asset2,
		Sponsor:   xdr.MustAddressPtr(sponsor1),
	}
	validateClaimableBalanceQuery(tt, q, query, []claimableBalanceQueryResult{})

	// claimant parameter, no filters
	query = ClaimableBalancesQuery{
		PageQuery: pageQuery,
		Claimant:  xdr.MustAddressPtr(dest1),
	}
	validateClaimableBalanceQuery(tt, q, query, []claimableBalanceQueryResult{
		{Claimants: []string{dest1, dest2}},
	})

	// claimant parameter, asset filter
	query = ClaimableBalancesQuery{
		PageQuery: pageQuery,
		Claimant:  xdr.MustAddressPtr(dest2),
		Asset:     &asset1,
	}
	validateClaimableBalanceQuery(tt, q, query, []claimableBalanceQueryResult{
		{Claimants: []string{dest1, dest2}, Asset: asset1.String()},
	})

	// claimant parameter, sponsor filter
	query = ClaimableBalancesQuery{
		PageQuery: pageQuery,
		Claimant:  xdr.MustAddressPtr(dest2),
		Sponsor:   xdr.MustAddressPtr(sponsor1),
	}
	validateClaimableBalanceQuery(tt, q, query, []claimableBalanceQueryResult{
		{Claimants: []string{dest1, dest2}, Sponsor: sponsor1},
	})

	//claimant parameter, asset filter, sponsor filter
	query = ClaimableBalancesQuery{
		PageQuery: pageQuery,
		Claimant:  xdr.MustAddressPtr(dest2),
		Asset:     &asset2,
		Sponsor:   xdr.MustAddressPtr(sponsor2),
	}
	validateClaimableBalanceQuery(tt, q, query, []claimableBalanceQueryResult{
		{Claimants: []string{dest2}, Asset: asset2.String(), Sponsor: sponsor2},
	})
}

func TestUpdateClaimableBalance(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	accountID := "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
	lastModifiedLedgerSeq := xdr.Uint32(123)
	asset := xdr.MustNewCreditAsset("USD", accountID)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	id, err := xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance := ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: accountID,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 123,
		Amount:             10,
	}

	err = q.UpsertClaimableBalances(tt.Ctx, []ClaimableBalance{cBalance})
	tt.Assert.NoError(err)

	cBalancesClaimants, err := q.GetClaimantsByClaimableBalances(tt.Ctx, []string{cBalance.BalanceID})
	tt.Assert.NoError(err)
	tt.Assert.Len(cBalancesClaimants[cBalance.BalanceID], 1)
	tt.Assert.Equal(ClaimableBalanceClaimant{
		BalanceID:          cBalance.BalanceID,
		Destination:        accountID,
		LastModifiedLedger: cBalance.LastModifiedLedger,
	}, cBalancesClaimants[cBalance.BalanceID][0])

	// add sponsor
	cBalance2 := ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: accountID,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 123 + 1,
		Amount:             10,
		Sponsor:            null.StringFrom("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
	}

	err = q.UpsertClaimableBalances(tt.Ctx, []ClaimableBalance{cBalance2})
	tt.Assert.NoError(err)

	cbs := []ClaimableBalance{}
	err = q.Select(tt.Ctx, &cbs, selectClaimableBalances)
	tt.Assert.NoError(err)
	tt.Assert.Len(cbs, 1)
	tt.Assert.Equal("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", cbs[0].Sponsor.String)
	tt.Assert.Equal(uint32(lastModifiedLedgerSeq+1), cbs[0].LastModifiedLedger)

	cBalancesClaimants, err = q.GetClaimantsByClaimableBalances(tt.Ctx, []string{cBalance2.BalanceID})
	tt.Assert.NoError(err)
	tt.Assert.Len(cBalancesClaimants[cBalance2.BalanceID], 1)
	tt.Assert.Equal(ClaimableBalanceClaimant{
		BalanceID:          cBalance2.BalanceID,
		Destination:        accountID,
		LastModifiedLedger: cBalance2.LastModifiedLedger,
	}, cBalancesClaimants[cBalance2.BalanceID][0])
}

func TestFindClaimableBalance(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.BeginTx(tt.Ctx, &sql.TxOptions{}))
	defer func() {
		_ = q.Rollback()
	}()

	accountID := "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
	asset := xdr.MustNewCreditAsset("USD", accountID)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	id, err := xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance := ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: accountID,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 123,
		Amount:             10,
	}

	balanceInsertBuilder := q.NewClaimableBalanceBatchInsertBuilder()
	tt.Assert.NoError(balanceInsertBuilder.Add(cBalance))
	tt.Assert.NoError(balanceInsertBuilder.Exec(tt.Ctx))

	cb, err := q.FindClaimableBalanceByID(tt.Ctx, id)

	tt.Assert.Equal(cBalance.BalanceID, cb.BalanceID)
	tt.Assert.Equal(cBalance.Asset, cb.Asset)
	tt.Assert.Equal(cBalance.Amount, cb.Amount)

	for i, hClaimant := range cb.Claimants {
		tt.Assert.Equal(cBalance.Claimants[i].Destination, hClaimant.Destination)
		tt.Assert.Equal(cBalance.Claimants[i].Predicate, hClaimant.Predicate)
	}
}
func TestGetClaimableBalancesByID(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.BeginTx(tt.Ctx, &sql.TxOptions{}))
	defer func() {
		_ = q.Rollback()
	}()

	accountID := "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
	asset := xdr.MustNewCreditAsset("USD", accountID)
	balanceID := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{1, 2, 3},
	}
	id, err := xdr.MarshalHex(balanceID)
	tt.Assert.NoError(err)
	cBalance := ClaimableBalance{
		BalanceID: id,
		Claimants: []Claimant{
			{
				Destination: accountID,
				Predicate: xdr.ClaimPredicate{
					Type: xdr.ClaimPredicateTypeClaimPredicateUnconditional,
				},
			},
		},
		Asset:              asset,
		LastModifiedLedger: 123,
		Amount:             10,
	}

	balanceInsertBuilder := q.NewClaimableBalanceBatchInsertBuilder()
	tt.Assert.NoError(balanceInsertBuilder.Add(cBalance))
	tt.Assert.NoError(balanceInsertBuilder.Exec(tt.Ctx))

	r, err := q.GetClaimableBalancesByID(tt.Ctx, []string{id})
	tt.Assert.NoError(err)
	tt.Assert.Len(r, 1)

	removed, err := q.RemoveClaimableBalances(tt.Ctx, []string{id})
	tt.Assert.NoError(err)
	tt.Assert.Equal(int64(1), removed)

	r, err = q.GetClaimableBalancesByID(tt.Ctx, []string{id})
	tt.Assert.NoError(err)
	tt.Assert.Len(r, 0)
}

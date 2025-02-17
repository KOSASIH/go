package history

import (
	"bytes"
	"context"
	"database/sql"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"

	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/xdr"
)

func TestAssetStatContracts(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.Begin(context.Background()))

	assetStats := []ExpAssetStat{
		{
			AssetType: xdr.AssetTypeAssetTypeNative,
			Accounts: ExpAssetStatAccounts{
				Authorized:                      0,
				AuthorizedToMaintainLiabilities: 0,
				ClaimableBalances:               0,
				LiquidityPools:                  0,
				Unauthorized:                    0,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "0",
				AuthorizedToMaintainLiabilities: "0",
				ClaimableBalances:               "0",
				LiquidityPools:                  "0",
				Unauthorized:                    "0",
			},
		},
		{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum12,
			AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			AssetCode:   "ETHER",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      1,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "23",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "4",
				LiquidityPools:                  "5",
			},
		},
		{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
			AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			AssetCode:   "USD",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      2,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "1",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "4",
				LiquidityPools:                  "5",
			},
		},
	}
	var contractID [32]byte
	for i := 0; i < 2; i++ {
		assetStats[i].SetContractID(contractID)
		contractID[0]++
	}
	tt.Assert.NoError(q.InsertAssetStats(tt.Ctx, assetStats))
	tt.Assert.NoError(q.Commit())

	contractID[0] = 0
	for i := 0; i < 2; i++ {
		var assetStat ExpAssetStat
		assetStat, err := q.GetAssetStatByContract(tt.Ctx, contractID)
		tt.Assert.NoError(err)
		tt.Assert.True(assetStat.Equals(assetStats[i]))
		contractID[0]++
	}

	usd := assetStats[2]
	usd.SetContractID([32]byte{})
	_, err := q.UpdateAssetStat(tt.Ctx, usd)
	tt.Assert.EqualError(err, "exec failed: pq: duplicate key value violates unique constraint \"exp_asset_stats_contract_id_key\"")

	usd.SetContractID([32]byte{2})
	rowsUpdated, err := q.UpdateAssetStat(tt.Ctx, usd)
	tt.Assert.NoError(err)
	tt.Assert.Equal(int64(1), rowsUpdated)

	assetStats[2] = usd
	contractID = [32]byte{}
	for i := 0; i < 3; i++ {
		var assetStat ExpAssetStat
		assetStat, err = q.GetAssetStatByContract(tt.Ctx, contractID)
		tt.Assert.NoError(err)
		tt.Assert.True(assetStat.Equals(assetStats[i]))
		contractID[0]++
	}
}

func TestAssetContractStats(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.Begin(context.Background()))

	c1 := ContractAssetStatRow{
		ContractID: []byte{1},
		Stat: ContractStat{
			ActiveBalance:   "100",
			ActiveHolders:   2,
			ArchivedBalance: "0",
			ArchivedHolders: 0,
		},
	}
	c2 := ContractAssetStatRow{
		ContractID: []byte{2},
		Stat: ContractStat{
			ActiveBalance:   "40",
			ActiveHolders:   1,
			ArchivedBalance: "0",
			ArchivedHolders: 0,
		},
	}
	c3 := ContractAssetStatRow{
		ContractID: []byte{3},
		Stat: ContractStat{
			ActiveBalance:   "900",
			ActiveHolders:   12,
			ArchivedBalance: "23",
			ArchivedHolders: 3,
		},
	}

	rows := []ContractAssetStatRow{c1, c2, c3}
	tt.Assert.NoError(q.InsertContractAssetStats(tt.Ctx, rows))

	for _, row := range rows {
		result, err := q.GetContractAssetStat(tt.Ctx, row.ContractID)
		tt.Assert.NoError(err)
		tt.Assert.Equal(result, row)
	}

	c2.Stat.ActiveHolders = 3
	c2.Stat.ActiveBalance = "20"
	c3.Stat.ArchivedBalance = "900"
	c2.Stat.ActiveHolders = 5
	numRows, err := q.UpdateContractAssetStat(tt.Ctx, c2)
	tt.Assert.NoError(err)
	tt.Assert.Equal(int64(1), numRows)
	row, err := q.GetContractAssetStat(tt.Ctx, c2.ContractID)
	tt.Assert.NoError(err)
	tt.Assert.Equal(c2, row)

	numRows, err = q.RemoveAssetContractStat(tt.Ctx, c3.ContractID)
	tt.Assert.NoError(err)
	tt.Assert.Equal(int64(1), numRows)

	_, err = q.GetContractAssetStat(tt.Ctx, c3.ContractID)
	tt.Assert.Equal(sql.ErrNoRows, err)

	for _, row := range []ContractAssetStatRow{c1, c2} {
		result, err := q.GetContractAssetStat(tt.Ctx, row.ContractID)
		tt.Assert.NoError(err)
		tt.Assert.Equal(result, row)
	}

	tt.Assert.NoError(q.Rollback())
}

func TestInsertAssetStats(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.Begin(context.Background()))

	tt.Assert.NoError(q.InsertAssetStats(tt.Ctx, []ExpAssetStat{}))

	assetStats := []ExpAssetStat{
		{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
			AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			AssetCode:   "USD",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      2,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "1",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "4",
				LiquidityPools:                  "5",
			},
		},
		{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum12,
			AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			AssetCode:   "ETHER",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      1,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "23",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "4",
				LiquidityPools:                  "5",
			},
		},
	}
	tt.Assert.NoError(q.InsertAssetStats(tt.Ctx, assetStats))

	for _, assetStat := range assetStats {
		got, err := q.GetAssetStat(tt.Ctx, assetStat.AssetType, assetStat.AssetCode, assetStat.AssetIssuer)
		tt.Assert.NoError(err)
		tt.Assert.Equal(got, assetStat)
	}

	tt.Assert.NoError(q.Rollback())
}

func TestInsertAssetStat(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	assetStats := []ExpAssetStat{
		{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
			AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			AssetCode:   "USD",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      2,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "1",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "4",
				LiquidityPools:                  "5",
			},
		},
		{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum12,
			AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			AssetCode:   "ETHER",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      1,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "23",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "4",
				LiquidityPools:                  "5",
			},
		},
	}

	for _, assetStat := range assetStats {
		numChanged, err := q.InsertAssetStat(tt.Ctx, assetStat)
		tt.Assert.NoError(err)
		tt.Assert.Equal(numChanged, int64(1))

		got, err := q.GetAssetStat(tt.Ctx, assetStat.AssetType, assetStat.AssetCode, assetStat.AssetIssuer)
		tt.Assert.NoError(err)
		tt.Assert.Equal(got, assetStat)
	}
}

func TestInsertAssetStatAlreadyExistsError(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	assetStat := ExpAssetStat{
		AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
		AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		AssetCode:   "USD",
		Accounts: ExpAssetStatAccounts{
			Authorized:                      2,
			AuthorizedToMaintainLiabilities: 3,
			Unauthorized:                    4,
		},
		Balances: ExpAssetStatBalances{
			Authorized:                      "1",
			AuthorizedToMaintainLiabilities: "2",
			Unauthorized:                    "3",
			ClaimableBalances:               "4",
			LiquidityPools:                  "5",
		},
	}

	numChanged, err := q.InsertAssetStat(tt.Ctx, assetStat)
	tt.Assert.NoError(err)
	tt.Assert.Equal(numChanged, int64(1))

	numChanged, err = q.InsertAssetStat(tt.Ctx, assetStat)
	tt.Assert.Error(err)
	tt.Assert.Equal(numChanged, int64(0))
}

func TestUpdateAssetStatDoesNotExistsError(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	assetStat := ExpAssetStat{
		AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
		AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		AssetCode:   "USD",
		Accounts: ExpAssetStatAccounts{
			Authorized:                      2,
			AuthorizedToMaintainLiabilities: 3,
			Unauthorized:                    4,
		},
		Balances: ExpAssetStatBalances{
			Authorized:                      "1",
			AuthorizedToMaintainLiabilities: "2",
			Unauthorized:                    "3",
			ClaimableBalances:               "4",
			LiquidityPools:                  "5",
		},
	}

	numChanged, err := q.UpdateAssetStat(tt.Ctx, assetStat)
	tt.Assert.Nil(err)
	tt.Assert.Equal(numChanged, int64(0))

	_, err = q.GetAssetStat(tt.Ctx, assetStat.AssetType, assetStat.AssetCode, assetStat.AssetIssuer)
	tt.Assert.Equal(err, sql.ErrNoRows)
}

func TestUpdateStat(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}

	assetStat := ExpAssetStat{
		AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
		AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		AssetCode:   "USD",
		Accounts: ExpAssetStatAccounts{
			Authorized:                      2,
			AuthorizedToMaintainLiabilities: 3,
			Unauthorized:                    4,
		},
		Balances: ExpAssetStatBalances{
			Authorized:                      "1",
			AuthorizedToMaintainLiabilities: "2",
			Unauthorized:                    "3",
			ClaimableBalances:               "4",
			LiquidityPools:                  "5",
		},
	}

	numChanged, err := q.InsertAssetStat(tt.Ctx, assetStat)
	tt.Assert.NoError(err)
	tt.Assert.Equal(numChanged, int64(1))

	got, err := q.GetAssetStat(tt.Ctx, assetStat.AssetType, assetStat.AssetCode, assetStat.AssetIssuer)
	tt.Assert.NoError(err)
	tt.Assert.Equal(got, assetStat)

	assetStat.SetContractID([32]byte{23})

	numChanged, err = q.UpdateAssetStat(tt.Ctx, assetStat)
	tt.Assert.Nil(err)
	tt.Assert.Equal(numChanged, int64(1))

	got, err = q.GetAssetStat(tt.Ctx, assetStat.AssetType, assetStat.AssetCode, assetStat.AssetIssuer)
	tt.Assert.NoError(err)
	tt.Assert.True(got.Equals(assetStat))
}

func TestGetAssetStatDoesNotExist(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	assetStat := ExpAssetStat{
		AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
		AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		AssetCode:   "USD",
		Accounts: ExpAssetStatAccounts{
			Authorized:                      2,
			AuthorizedToMaintainLiabilities: 3,
			Unauthorized:                    4,
		},
		Balances: ExpAssetStatBalances{
			Authorized:                      "1",
			AuthorizedToMaintainLiabilities: "2",
			Unauthorized:                    "3",
			ClaimableBalances:               "4",
			LiquidityPools:                  "5",
		},
	}

	_, err := q.GetAssetStat(tt.Ctx, assetStat.AssetType, assetStat.AssetCode, assetStat.AssetIssuer)
	tt.Assert.Equal(err, sql.ErrNoRows)
}

func TestRemoveAssetStat(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}

	assetStat := ExpAssetStat{
		AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
		AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		AssetCode:   "USD",
		Accounts: ExpAssetStatAccounts{
			Authorized:                      2,
			AuthorizedToMaintainLiabilities: 3,
			Unauthorized:                    4,
		},
		Balances: ExpAssetStatBalances{
			Authorized:                      "1",
			AuthorizedToMaintainLiabilities: "2",
			Unauthorized:                    "3",
			ClaimableBalances:               "4",
			LiquidityPools:                  "5",
		},
	}

	numChanged, err := q.RemoveAssetStat(tt.Ctx,
		assetStat.AssetType,
		assetStat.AssetCode,
		assetStat.AssetIssuer,
	)
	tt.Assert.Nil(err)
	tt.Assert.Equal(numChanged, int64(0))

	numChanged, err = q.InsertAssetStat(tt.Ctx, assetStat)
	tt.Assert.NoError(err)
	tt.Assert.Equal(numChanged, int64(1))

	got, err := q.GetAssetStat(tt.Ctx, assetStat.AssetType, assetStat.AssetCode, assetStat.AssetIssuer)
	tt.Assert.NoError(err)
	tt.Assert.Equal(got, assetStat)

	numChanged, err = q.RemoveAssetStat(tt.Ctx,
		assetStat.AssetType,
		assetStat.AssetCode,
		assetStat.AssetIssuer,
	)
	tt.Assert.Nil(err)
	tt.Assert.Equal(numChanged, int64(1))

	_, err = q.GetAssetStat(tt.Ctx, assetStat.AssetType, assetStat.AssetCode, assetStat.AssetIssuer)
	tt.Assert.Equal(err, sql.ErrNoRows)
}

func TestGetAssetStatsCursorValidation(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}

	for _, testCase := range []struct {
		name          string
		cursor        string
		expectedError string
	}{
		{
			"cursor does not use underscore as serpator",
			"usdc-GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			"invalid asset stats cursor",
		},
		{
			"cursor has no underscore",
			"usdc",
			"invalid asset stats cursor",
		},
		{
			"cursor has too many underscores",
			"usdc_GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H_credit_alphanum4_",
			"invalid asset type in asset stats cursor",
		},
		{
			"issuer in cursor is invalid",
			"usd_abcdefghijklmnopqrstuv_credit_alphanum4",
			"invalid issuer in asset stats cursor",
		},
		{
			"asset type in cursor is invalid",
			"usd_GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H_credit_alphanum",
			"invalid asset type in asset stats cursor",
		},
		{
			"asset code in cursor is too long",
			"abcdefghijklmnopqrstuv_GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H_credit_alphanum12",
			"invalid asset stats cursor",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			page := db2.PageQuery{
				Cursor: testCase.cursor,
				Order:  "asc",
				Limit:  5,
			}
			results, err := q.GetAssetStats(tt.Ctx, "", "", page)
			tt.Assert.Empty(results)
			tt.Assert.NotNil(err)
			tt.Assert.Contains(err.Error(), testCase.expectedError)
		})
	}
}

func TestGetAssetStatsOrderValidation(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}

	page := db2.PageQuery{
		Order: "invalid",
		Limit: 5,
	}
	results, err := q.GetAssetStats(tt.Ctx, "", "", page)
	tt.Assert.Empty(results)
	tt.Assert.NotNil(err)
	tt.Assert.Contains(err.Error(), "invalid page order")
}

func TestGetAssetStatsFiltersAndCursor(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}
	zero := ContractStat{
		ActiveBalance:   "0",
		ActiveHolders:   0,
		ArchivedBalance: "0",
		ArchivedHolders: 0,
	}
	usdAssetStat := AssetAndContractStat{
		ExpAssetStat: ExpAssetStat{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
			AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			AssetCode:   "USD",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      2,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "1",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "0",
				LiquidityPools:                  "0",
			},
		},
		Contracts: zero,
	}
	etherAssetStat := AssetAndContractStat{
		ExpAssetStat: ExpAssetStat{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum12,
			AssetIssuer: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			AssetCode:   "ETHER",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      1,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "23",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "0",
				LiquidityPools:                  "0",
			},
		},
		Contracts: zero,
	}
	otherUSDAssetStat := AssetAndContractStat{
		ExpAssetStat: ExpAssetStat{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
			AssetIssuer: "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			AssetCode:   "USD",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      2,
				AuthorizedToMaintainLiabilities: 3,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "1",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "0",
				LiquidityPools:                  "0",
			},
		},
		Contracts: zero,
	}
	eurAssetStat := AssetAndContractStat{
		ExpAssetStat: ExpAssetStat{
			AssetType:   xdr.AssetTypeAssetTypeCreditAlphanum4,
			AssetIssuer: "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			AssetCode:   "EUR",
			Accounts: ExpAssetStatAccounts{
				Authorized:                      3,
				AuthorizedToMaintainLiabilities: 2,
				Unauthorized:                    4,
			},
			Balances: ExpAssetStatBalances{
				Authorized:                      "111",
				AuthorizedToMaintainLiabilities: "2",
				Unauthorized:                    "3",
				ClaimableBalances:               "1",
				LiquidityPools:                  "2",
			},
		},
		Contracts: ContractStat{
			ActiveBalance:   "120",
			ActiveHolders:   3,
			ArchivedBalance: "90",
			ArchivedHolders: 1,
		},
	}
	eurAssetStat.SetContractID([32]byte{})
	assetStats := []AssetAndContractStat{
		etherAssetStat,
		eurAssetStat,
		otherUSDAssetStat,
		usdAssetStat,
	}
	for _, assetStat := range assetStats {
		numChanged, err := q.InsertAssetStat(tt.Ctx, assetStat.ExpAssetStat)
		tt.Assert.NoError(err)
		tt.Assert.Equal(numChanged, int64(1))
		if assetStat.Contracts != zero {
			numChanged, err = q.InsertContractAssetStat(tt.Ctx, ContractAssetStatRow{
				ContractID: *assetStat.ContractID,
				Stat:       assetStat.Contracts,
			})
			tt.Assert.NoError(err)
			tt.Assert.Equal(numChanged, int64(1))
		}
	}

	// insert contract stat which has no corresponding asset stat row
	// to test that it isn't included in the results
	numChanged, err := q.InsertContractAssetStat(tt.Ctx, ContractAssetStatRow{
		ContractID: []byte{1},
		Stat: ContractStat{
			ActiveBalance:   "400",
			ActiveHolders:   30,
			ArchivedBalance: "0",
			ArchivedHolders: 0,
		},
	})
	tt.Assert.NoError(err)
	tt.Assert.Equal(numChanged, int64(1))

	for _, testCase := range []struct {
		name        string
		assetCode   string
		assetIssuer string
		cursor      string
		order       string
		expected    []AssetAndContractStat
	}{
		{
			"no filter without cursor",
			"",
			"",
			"",
			"asc",
			[]AssetAndContractStat{
				etherAssetStat,
				eurAssetStat,
				otherUSDAssetStat,
				usdAssetStat,
			},
		},
		{
			"no filter with cursor",
			"",
			"",
			"ABC_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"asc",
			[]AssetAndContractStat{
				etherAssetStat,
				eurAssetStat,
				otherUSDAssetStat,
				usdAssetStat,
			},
		},
		{
			"no filter with cursor descending",
			"",
			"",
			"ZZZ_GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H_credit_alphanum4",
			"desc",
			[]AssetAndContractStat{
				usdAssetStat,
				otherUSDAssetStat,
				eurAssetStat,
				etherAssetStat,
			},
		},
		{
			"no filter with cursor and offset",
			"",
			"",
			"ETHER_GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H_credit_alphanum12",
			"asc",
			[]AssetAndContractStat{
				eurAssetStat,
				otherUSDAssetStat,
				usdAssetStat,
			},
		},
		{
			"no filter with cursor and offset descending",
			"",
			"",
			"EUR_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"desc",
			[]AssetAndContractStat{
				etherAssetStat,
			},
		},
		{
			"no filter with cursor and offset descending including eur",
			"",
			"",
			"EUR_GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H_credit_alphanum4",
			"desc",
			[]AssetAndContractStat{
				eurAssetStat,
				etherAssetStat,
			},
		},
		{
			"filter on code without cursor",
			"USD",
			"",
			"",
			"asc",
			[]AssetAndContractStat{
				otherUSDAssetStat,
				usdAssetStat,
			},
		},
		{
			"filter on code with cursor",
			"USD",
			"",
			"USD_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"asc",
			[]AssetAndContractStat{
				usdAssetStat,
			},
		},
		{
			"filter on code with cursor descending",
			"USD",
			"",
			"USD_GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H_credit_alphanum4",
			"desc",
			[]AssetAndContractStat{
				otherUSDAssetStat,
			},
		},
		{
			"filter on issuer without cursor",
			"",
			"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			"",
			"asc",
			[]AssetAndContractStat{
				eurAssetStat,
				otherUSDAssetStat,
			},
		},
		{
			"filter on issuer with cursor",
			"",
			"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			"EUR_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"asc",
			[]AssetAndContractStat{
				otherUSDAssetStat,
			},
		},
		{
			"filter on issuer with cursor descending",
			"",
			"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			"USD_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"desc",
			[]AssetAndContractStat{
				eurAssetStat,
			},
		},
		{
			"filter on non existent code without cursor",
			"BTC",
			"",
			"",
			"asc",
			nil,
		},
		{
			"filter on non existent code with cursor",
			"BTC",
			"",
			"BTC_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"asc",
			nil,
		},
		{
			"filter on non existent issuer without cursor",
			"",
			"GAEIHD6U4WSBHJGA2HPWOQ3OQEFQ3Y7QZE2DR76YKZNKPW5YDLYW4UGF",
			"",
			"asc",
			nil,
		},
		{
			"filter on non existent issuer with cursor",
			"",
			"GAEIHD6U4WSBHJGA2HPWOQ3OQEFQ3Y7QZE2DR76YKZNKPW5YDLYW4UGF",
			"AAA_GAEIHD6U4WSBHJGA2HPWOQ3OQEFQ3Y7QZE2DR76YKZNKPW5YDLYW4UGF_credit_alphanum4",
			"asc",
			nil,
		},
		{
			"filter on non existent code and non existent issuer without cursor",
			"BTC",
			"GAEIHD6U4WSBHJGA2HPWOQ3OQEFQ3Y7QZE2DR76YKZNKPW5YDLYW4UGF",
			"",
			"asc",
			nil,
		},
		{
			"filter on non existent code and non existent issuer with cursor",
			"BTC",
			"GAEIHD6U4WSBHJGA2HPWOQ3OQEFQ3Y7QZE2DR76YKZNKPW5YDLYW4UGF",
			"AAA_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"asc",
			nil,
		},
		{
			"filter on both code and issuer without cursor",
			"USD",
			"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			"",
			"asc",
			[]AssetAndContractStat{
				otherUSDAssetStat,
			},
		},
		{
			"filter on both code and issuer with cursor",
			"USD",
			"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			"USC_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"asc",
			[]AssetAndContractStat{
				otherUSDAssetStat,
			},
		},
		{
			"filter on both code and issuer with cursor descending",
			"USD",
			"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			"USE_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"desc",
			[]AssetAndContractStat{
				otherUSDAssetStat,
			},
		},
		{
			"cursor negates filter",
			"USD",
			"GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2",
			"USD_GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2_credit_alphanum4",
			"asc",
			nil,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			page := db2.PageQuery{
				Order:  testCase.order,
				Cursor: testCase.cursor,
				Limit:  5,
			}
			results, err := q.GetAssetStats(tt.Ctx, testCase.assetCode, testCase.assetIssuer, page)
			tt.Assert.NoError(err)
			tt.Assert.Equal(testCase.expected, results)

			page.Limit = 1
			results, err = q.GetAssetStats(tt.Ctx, testCase.assetCode, testCase.assetIssuer, page)
			tt.Assert.NoError(err)
			if len(testCase.expected) == 0 {
				tt.Assert.Equal(testCase.expected, results)
			} else {
				tt.Assert.Equal(testCase.expected[:1], results)
			}

			if page.Cursor == "" {
				page = page.Invert()
				page.Limit = 5

				results, err = q.GetAssetStats(tt.Ctx, testCase.assetCode, testCase.assetIssuer, page)
				tt.Assert.NoError(err)
				slices.Reverse(results)
				tt.Assert.Equal(testCase.expected, results)
			}
		})
	}
}

func assertContractAssetBalancesEqual(t *testing.T, balances, otherBalances []ContractAssetBalance) {
	assert.Equal(t, len(balances), len(otherBalances))

	sort.Slice(balances, func(i, j int) bool {
		return bytes.Compare(balances[i].KeyHash, balances[j].KeyHash) < 0
	})
	sort.Slice(otherBalances, func(i, j int) bool {
		return bytes.Compare(otherBalances[i].KeyHash, otherBalances[j].KeyHash) < 0
	})

	for i, balance := range balances {
		other := otherBalances[i]
		assert.Equal(t, balance, other)
	}
}

func TestInsertContractAssetBalances(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}

	tt.Assert.NoError(q.Begin(context.Background()))

	keyHash := [32]byte{}
	contractID := [32]byte{1}
	balance := ContractAssetBalance{
		KeyHash:          keyHash[:],
		ContractID:       contractID[:],
		Amount:           "100",
		ExpirationLedger: 10,
	}

	otherKeyHash := [32]byte{2}
	otherContractID := [32]byte{3}
	otherBalance := ContractAssetBalance{
		KeyHash:          otherKeyHash[:],
		ContractID:       otherContractID[:],
		Amount:           "101",
		ExpirationLedger: 11,
	}

	tt.Assert.NoError(
		q.InsertContractAssetBalances(context.Background(), []ContractAssetBalance{balance, otherBalance}),
	)

	nonExistantKeyHash := [32]byte{4}
	balances, err := q.GetContractAssetBalances(context.Background(), []xdr.Hash{keyHash, otherKeyHash, nonExistantKeyHash})
	tt.Assert.NoError(err)

	assertContractAssetBalancesEqual(t, balances, []ContractAssetBalance{balance, otherBalance})

	tt.Assert.NoError(q.Rollback())
}

func TestRemoveContractAssetBalances(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}
	tt.Assert.NoError(q.Begin(context.Background()))

	keyHash := [32]byte{}
	contractID := [32]byte{1}
	balance := ContractAssetBalance{
		KeyHash:          keyHash[:],
		ContractID:       contractID[:],
		Amount:           "100",
		ExpirationLedger: 10,
	}

	otherKeyHash := [32]byte{2}
	otherContractID := [32]byte{3}
	otherBalance := ContractAssetBalance{
		KeyHash:          otherKeyHash[:],
		ContractID:       otherContractID[:],
		Amount:           "101",
		ExpirationLedger: 11,
	}

	tt.Assert.NoError(
		q.InsertContractAssetBalances(context.Background(), []ContractAssetBalance{balance, otherBalance}),
	)
	nonExistantKeyHash := xdr.Hash{4}

	tt.Assert.NoError(
		q.RemoveContractAssetBalances(context.Background(), []xdr.Hash{nonExistantKeyHash}),
	)
	balances, err := q.GetContractAssetBalances(context.Background(), []xdr.Hash{keyHash, otherKeyHash, nonExistantKeyHash})
	tt.Assert.NoError(err)

	assertContractAssetBalancesEqual(t, balances, []ContractAssetBalance{balance, otherBalance})

	tt.Assert.NoError(
		q.RemoveContractAssetBalances(context.Background(), []xdr.Hash{nonExistantKeyHash, otherKeyHash}),
	)

	balances, err = q.GetContractAssetBalances(context.Background(), []xdr.Hash{keyHash, otherKeyHash, nonExistantKeyHash})
	tt.Assert.NoError(err)

	assertContractAssetBalancesEqual(t, balances, []ContractAssetBalance{balance})

	tt.Assert.NoError(q.Rollback())
}

func TestUpdateContractAssetBalanceAmounts(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}
	tt.Assert.NoError(q.Begin(context.Background()))

	keyHash := [32]byte{}
	contractID := [32]byte{1}
	balance := ContractAssetBalance{
		KeyHash:          keyHash[:],
		ContractID:       contractID[:],
		Amount:           "100",
		ExpirationLedger: 10,
	}

	otherKeyHash := [32]byte{2}
	otherContractID := [32]byte{3}
	otherBalance := ContractAssetBalance{
		KeyHash:          otherKeyHash[:],
		ContractID:       otherContractID[:],
		Amount:           "101",
		ExpirationLedger: 11,
	}

	tt.Assert.NoError(
		q.InsertContractAssetBalances(context.Background(), []ContractAssetBalance{balance, otherBalance}),
	)

	nonExistantKeyHash := xdr.Hash{4}

	tt.Assert.NoError(
		q.UpdateContractAssetBalanceAmounts(
			context.Background(),
			[]xdr.Hash{otherKeyHash, keyHash, nonExistantKeyHash},
			[]string{"1", "2", "3"},
		),
	)

	balances, err := q.GetContractAssetBalances(context.Background(), []xdr.Hash{keyHash, otherKeyHash})
	tt.Assert.NoError(err)

	balance.Amount = "2"
	otherBalance.Amount = "1"
	assertContractAssetBalancesEqual(t, balances, []ContractAssetBalance{balance, otherBalance})

	tt.Assert.NoError(q.Rollback())
}

func TestUpdateContractAssetBalanceExpirations(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)

	q := &Q{tt.HorizonSession()}
	tt.Assert.NoError(q.Begin(context.Background()))

	keyHash := [32]byte{}
	contractID := [32]byte{1}
	balance := ContractAssetBalance{
		KeyHash:          keyHash[:],
		ContractID:       contractID[:],
		Amount:           "100",
		ExpirationLedger: 10,
	}

	otherKeyHash := [32]byte{2}
	otherContractID := [32]byte{3}
	otherBalance := ContractAssetBalance{
		KeyHash:          otherKeyHash[:],
		ContractID:       otherContractID[:],
		Amount:           "101",
		ExpirationLedger: 11,
	}

	tt.Assert.NoError(
		q.InsertContractAssetBalances(context.Background(), []ContractAssetBalance{balance, otherBalance}),
	)

	balances, err := q.GetContractAssetBalancesExpiringAt(context.Background(), 10)
	tt.Assert.NoError(err)
	assertContractAssetBalancesEqual(t, balances, []ContractAssetBalance{balance})

	balances, err = q.GetContractAssetBalancesExpiringAt(context.Background(), 11)
	tt.Assert.NoError(err)
	assertContractAssetBalancesEqual(t, balances, []ContractAssetBalance{otherBalance})

	nonExistantKeyHash := xdr.Hash{4}

	tt.Assert.NoError(
		q.UpdateContractAssetBalanceExpirations(
			context.Background(),
			[]xdr.Hash{otherKeyHash, keyHash, nonExistantKeyHash},
			[]uint32{200, 200, 500},
		),
	)

	balances, err = q.GetContractAssetBalances(context.Background(), []xdr.Hash{keyHash, otherKeyHash})
	tt.Assert.NoError(err)
	balance.ExpirationLedger = 200
	otherBalance.ExpirationLedger = 200
	assertContractAssetBalancesEqual(t, balances, []ContractAssetBalance{balance, otherBalance})

	balances, err = q.GetContractAssetBalancesExpiringAt(context.Background(), 10)
	tt.Assert.NoError(err)
	assert.Empty(t, balances)

	balances, err = q.GetContractAssetBalancesExpiringAt(context.Background(), 200)
	tt.Assert.NoError(err)
	assertContractAssetBalancesEqual(t, balances, []ContractAssetBalance{balance, otherBalance})

	tt.Assert.NoError(q.Rollback())
}

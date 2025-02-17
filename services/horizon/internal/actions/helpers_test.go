package actions

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"

	horizonContext "github.com/stellar/go/services/horizon/internal/context"
	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/services/horizon/internal/ledger"
	hProblem "github.com/stellar/go/services/horizon/internal/render/problem"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/render/problem"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

func TestGetTransactionID(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeFooBarTestActionRequest()

	txID, err := GetTransactionID(r, "valid_tx_id")
	tt.Assert.NoError(err)
	tt.Assert.Equal(
		"aa168f12124b7c196c0adaee7c73a64d37f99428cacb59a91ff389626845e7cf",
		txID,
	)

	txID, err = GetTransactionID(r, "invalid_uppercase_tx_id")
	tt.Assert.Error(err)

	txID, err = GetTransactionID(r, "invalid_too_short_tx_id")
	tt.Assert.Error(err)
}

func TestGetAssetType(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeFooBarTestActionRequest()

	ts, err := getAssetType(r, "native_asset_type")
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(xdr.AssetTypeAssetTypeNative, ts)
	}

	ts, err = getAssetType(r, "4_asset_type")
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(xdr.AssetTypeAssetTypeCreditAlphanum4, ts)
	}

	ts, err = getAssetType(r, "12_asset_type")
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(xdr.AssetTypeAssetTypeCreditAlphanum12, ts)
	}
}

func TestGetCursor(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()

	ledgerState := &ledger.State{}
	// now uses the ledger state
	r := makeTestActionRequest("/?cursor=now", nil)
	cursor, err := getCursor(ledgerState, r, "cursor")
	if tt.Assert.NoError(err) {
		expected := toid.AfterLedger(ledgerState.CurrentStatus().HistoryLatest).String()
		tt.Assert.Equal(expected, cursor)
	}

	//Last-Event-ID overrides cursor
	r = makeFooBarTestActionRequest()
	r.Header.Set("Last-Event-ID", "from_header")
	cursor, err = getCursor(ledgerState, r, "cursor")
	if tt.Assert.NoError(err) {
		tt.Assert.Equal("from_header", cursor)
	}
}

func TestValidateCursorWithinHistory(t *testing.T) {
	tt := assert.New(t)
	testCases := []struct {
		cursor string
		order  string
		valid  bool
	}{
		{
			cursor: "10",
			order:  "desc",
			valid:  true,
		},
		{
			cursor: "10-1234",
			order:  "desc",
			valid:  true,
		},
		{
			cursor: "0",
			order:  "desc",
			valid:  false,
		},
		{
			cursor: "0-1234",
			order:  "desc",
			valid:  false,
		},
		{
			cursor: "10",
			order:  "asc",
			valid:  true,
		},
		{
			cursor: "10-1234",
			order:  "asc",
			valid:  true,
		},
		{
			cursor: "0",
			order:  "asc",
			valid:  false,
		},
		{
			cursor: "0-1234",
			order:  "asc",
			valid:  false,
		},
		{
			cursor: "1",
			order:  "asc",
			valid:  true,
		},
		{
			cursor: "1-1234",
			order:  "asc",
			valid:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("cursor: %s", tc.cursor), func(t *testing.T) {
			pq, err := db2.NewPageQuery(tc.cursor, false, tc.order, 10)
			tt.NoError(err)
			err = validateCursorWithinHistory(&ledger.State{}, pq)

			if tc.valid {
				tt.NoError(err)
			} else {
				tt.True(strings.Contains(err.Error(), "problem: before_history"))
			}
		})
	}
}

func TestActionGetLimit(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()

	// happy path
	r := makeFooBarTestActionRequest()
	limit, err := getLimit(r, "limit", 5, 200)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(uint64(2), limit)
	}

	r = makeTestActionRequest("/?limit=200", nil)
	limit, err = getLimit(r, "limit", 5, 200)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(uint64(200), limit)
	}

	// defaults
	r = makeTestActionRequest("/", nil)
	limit, err = getLimit(r, "limit", 5, 200)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(uint64(5), limit)
	}

	r = makeTestActionRequest("/?limit=", nil)
	limit, err = getLimit(r, "limit", 5, 200)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(uint64(5), limit)
	}

	// invalids
	r = makeTestActionRequest("/?limit=0", nil)
	_, err = getLimit(r, "limit", 5, 200)
	tt.Assert.Error(err)

	r = makeTestActionRequest("/?limit=-1", nil)
	_, err = getLimit(r, "limit", 5, 200)
	tt.Assert.Error(err)

	r = makeTestActionRequest("/?limit=201", nil)
	_, err = getLimit(r, "limit", 5, 200)
	tt.Assert.Error(err)
}

func TestGetLimit(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()

	// happy path
	r := makeFooBarTestActionRequest()
	limit, err := getLimit(r, "limit", 5, 200)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(uint64(2), limit)
	}

	r = makeTestActionRequest("/?limit=200", nil)
	limit, err = getLimit(r, "limit", 5, 200)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(uint64(200), limit)
	}

	// defaults
	r = makeTestActionRequest("/", nil)
	limit, err = getLimit(r, "limit", 5, 200)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(uint64(5), limit)
	}

	r = makeTestActionRequest("/?limit=", nil)
	limit, err = getLimit(r, "limit", 5, 200)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(uint64(5), limit)
	}

	// invalids
	r = makeTestActionRequest("/?limit=0", nil)
	_, err = getLimit(r, "limit", 5, 200)
	tt.Assert.Error(err)

	r = makeTestActionRequest("/?limit=-1", nil)
	_, err = getLimit(r, "limit", 5, 200)
	tt.Assert.Error(err)

	r = makeTestActionRequest("/?limit=201", nil)
	_, err = getLimit(r, "limit", 5, 200)
	tt.Assert.Error(err)
}

func TestActionGetPageQuery(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeFooBarTestActionRequest()
	ledgerState := &ledger.State{}

	// happy path
	pq, err := GetPageQuery(ledgerState, r)
	tt.Assert.NoError(err)
	tt.Assert.Equal("123456", pq.Cursor)
	tt.Assert.Equal(uint64(2), pq.Limit)
	tt.Assert.Equal("asc", pq.Order)

	// regression: GetPagQuery does not overwrite err
	r = makeTestActionRequest("/?limit=foo", nil)
	_, err = getLimit(r, "limit", 1, 200)
	tt.Assert.Error(err)
	_, err = GetPageQuery(ledgerState, r)
	tt.Assert.Error(err)

	// regression: https://github.com/stellar/go/services/horizon/internal/issues/372
	// (limit of 0 turns into 10)
	makeTestActionRequest("/?limit=0", nil)
	_, err = GetPageQuery(ledgerState, r)
	tt.Assert.Error(err)
}

func TestGetPageQuery(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeFooBarTestActionRequest()
	ledgerState := &ledger.State{}

	// happy path
	pq, err := GetPageQuery(ledgerState, r)
	tt.Assert.NoError(err)
	tt.Assert.Equal("123456", pq.Cursor)
	tt.Assert.Equal(uint64(2), pq.Limit)
	tt.Assert.Equal("asc", pq.Order)

	// regression: GetPagQuery does not overwrite err
	r = makeTestActionRequest("/?limit=foo", nil)
	_, err = getLimit(r, "limit", 1, 200)
	tt.Assert.Error(err)
	_, err = GetPageQuery(ledgerState, r)
	tt.Assert.Error(err)

	// regression: https://github.com/stellar/go/services/horizon/internal/issues/372
	// (limit of 0 turns into 10)
	r = makeTestActionRequest("/?limit=0", nil)
	_, err = GetPageQuery(ledgerState, r)
	tt.Assert.Error(err)
}

func TestPageQueryCursorDefaultOrder(t *testing.T) {
	ledgerState := &ledger.State{}

	// truncated history
	ledgerState.SetHorizonStatus(ledger.HorizonStatus{
		HistoryLatest:         7000,
		HistoryLatestClosedAt: time.Now(),
		HistoryElder:          300,
		ExpHistoryLatest:      7000,
	})

	req := makeTestActionRequest("/foo-bar/blah?limit=2", testURLParams())

	// default asc, w/o cursor
	pq, err := GetPageQuery(ledgerState, req)
	assert.NoError(t, err)
	assert.Empty(t, pq.Cursor)
	assert.NoError(t, validateAndAdjustCursor(ledgerState, &pq))
	assert.Equal(t, toid.AfterLedger(299).String(), pq.Cursor)
	assert.Equal(t, uint64(2), pq.Limit)
	assert.Equal(t, "asc", pq.Order)

	cursor := toid.AfterLedger(200).String()
	reqWithCursor := makeTestActionRequest(fmt.Sprintf("/foo-bar/blah?cursor=%s&limit=2", cursor), testURLParams())

	// default asc, w/ cursor
	pq, err = GetPageQuery(ledgerState, reqWithCursor)
	assert.NoError(t, err)
	assert.Equal(t, cursor, pq.Cursor)
	assert.NoError(t, validateAndAdjustCursor(ledgerState, &pq))
	assert.Equal(t, toid.AfterLedger(299).String(), pq.Cursor)
	assert.Equal(t, uint64(2), pq.Limit)
	assert.Equal(t, "asc", pq.Order)

	// full history
	ledgerState.SetHorizonStatus(ledger.HorizonStatus{
		HistoryLatest:         7000,
		HistoryLatestClosedAt: time.Now(),
		HistoryElder:          0,
		ExpHistoryLatest:      7000,
	})

	// default asc, w/o cursor
	pq, err = GetPageQuery(ledgerState, req)
	assert.NoError(t, err)
	assert.Empty(t, pq.Cursor)
	assert.NoError(t, validateAndAdjustCursor(ledgerState, &pq))
	assert.Equal(t, toid.AfterLedger(0).String(), pq.Cursor)
	assert.Equal(t, uint64(2), pq.Limit)
	assert.Equal(t, "asc", pq.Order)

	// default asc, w/ cursor
	pq, err = GetPageQuery(ledgerState, reqWithCursor)
	assert.NoError(t, err)
	assert.Equal(t, cursor, pq.Cursor)
	assert.NoError(t, validateAndAdjustCursor(ledgerState, &pq))
	assert.Equal(t, cursor, pq.Cursor)
	assert.Equal(t, uint64(2), pq.Limit)
	assert.Equal(t, "asc", pq.Order)

}

func TestGetPageQueryWithoutCursor(t *testing.T) {
	ledgerState := &ledger.State{}

	validateCursor := func(limit uint64, order string, expectedCursor string) {
		req := makeTestActionRequest(fmt.Sprintf("/foo-bar/blah?limit=%d&order=%s", limit, order), testURLParams())
		pq, err := GetPageQuery(ledgerState, req)
		assert.NoError(t, err)
		assert.Empty(t, pq.Cursor)
		assert.NoError(t, validateAndAdjustCursor(ledgerState, &pq))
		assert.Equal(t, expectedCursor, pq.Cursor)
		assert.Equal(t, limit, pq.Limit)
		assert.Equal(t, order, pq.Order)
	}

	// truncated history
	ledgerState.SetHorizonStatus(ledger.HorizonStatus{
		HistoryLatest:         7000,
		HistoryLatestClosedAt: time.Now(),
		HistoryElder:          300,
		ExpHistoryLatest:      7000,
	})

	validateCursor(2, "asc", toid.AfterLedger(299).String())
	validateCursor(2, "desc", "")

	// full history
	ledgerState.SetHorizonStatus(ledger.HorizonStatus{
		HistoryLatest:         7000,
		HistoryLatestClosedAt: time.Now(),
		HistoryElder:          0,
		ExpHistoryLatest:      7000,
	})

	validateCursor(2, "asc", toid.AfterLedger(0).String())
	validateCursor(2, "desc", "")
}

func TestValidateAndAdjustCursor(t *testing.T) {
	ledgerState := &ledger.State{}

	validateCursor := func(cursor string, limit uint64, order string, expectedCursor string, expectedError error) {
		pq := db2.PageQuery{Cursor: cursor,
			Limit: limit,
			Order: order,
		}
		err := validateAndAdjustCursor(ledgerState, &pq)
		if expectedError != nil {
			assert.EqualError(t, expectedError, err.Error())
		} else {
			assert.NoError(t, err)
		}
		assert.Equal(t, expectedCursor, pq.Cursor)
		assert.Equal(t, limit, pq.Limit)
		assert.Equal(t, order, pq.Order)
	}

	// full history
	ledgerState.SetHorizonStatus(ledger.HorizonStatus{
		HistoryLatest:         7000,
		HistoryLatestClosedAt: time.Now(),
		HistoryElder:          0,
		ExpHistoryLatest:      7000,
	})

	// invalid cursor
	validateCursor("blah", 2, "asc", "blah", problem.BadRequest)
	validateCursor("blah", 2, "desc", "blah", problem.BadRequest)

	validateCursor(toid.AfterLedger(0).String(), 2, "asc", toid.AfterLedger(0).String(), nil)
	validateCursor(toid.AfterLedger(200).String(), 2, "asc", toid.AfterLedger(200).String(), nil)
	validateCursor(toid.AfterLedger(7001).String(), 2, "asc", toid.AfterLedger(7001).String(), nil)

	validateCursor(toid.AfterLedger(0).String(), 2, "desc", toid.AfterLedger(0).String(), nil)
	validateCursor(toid.AfterLedger(200).String(), 2, "desc", toid.AfterLedger(200).String(), nil)
	validateCursor(toid.AfterLedger(7001).String(), 2, "desc", toid.AfterLedger(7001).String(), nil)

	// truncated history
	ledgerState.SetHorizonStatus(ledger.HorizonStatus{
		HistoryLatest:         7000,
		HistoryLatestClosedAt: time.Now(),
		HistoryElder:          300,
		ExpHistoryLatest:      7000,
	})

	// invalid cursor
	validateCursor("blah", 2, "asc", "blah", problem.BadRequest)
	validateCursor("blah", 2, "desc", "blah", problem.BadRequest)

	// asc order
	validateCursor(toid.AfterLedger(0).String(), 2, "asc", toid.AfterLedger(299).String(), nil)
	validateCursor(toid.AfterLedger(200).String(), 2, "asc", toid.AfterLedger(299).String(), nil)
	validateCursor(toid.AfterLedger(298).String(), 2, "asc", toid.AfterLedger(299).String(), nil)
	validateCursor(toid.AfterLedger(299).String(), 2, "asc", toid.AfterLedger(299).String(), nil)
	validateCursor(toid.AfterLedger(300).String(), 2, "asc", toid.AfterLedger(300).String(), nil)
	validateCursor(toid.AfterLedger(301).String(), 2, "asc", toid.AfterLedger(301).String(), nil)
	validateCursor(toid.AfterLedger(7001).String(), 2, "asc", toid.AfterLedger(7001).String(), nil)

	// desc order
	validateCursor(toid.AfterLedger(0).String(), 2, "desc", toid.AfterLedger(0).String(), hProblem.BeforeHistory)
	validateCursor(toid.AfterLedger(200).String(), 2, "desc", toid.AfterLedger(200).String(), hProblem.BeforeHistory)
	validateCursor(toid.AfterLedger(299).String(), 2, "desc", toid.AfterLedger(299).String(), hProblem.BeforeHistory)
	validateCursor(toid.AfterLedger(300).String(), 2, "desc", toid.AfterLedger(300).String(), nil)
	validateCursor(toid.AfterLedger(320).String(), 2, "desc", toid.AfterLedger(320).String(), nil)
	validateCursor(toid.AfterLedger(7001).String(), 2, "desc", toid.AfterLedger(7001).String(), nil)
}

func TestGetString(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeFooBarTestActionRequest()
	cursor, err := getString(r, "cursor")
	tt.Assert.NoError(err)
	tt.Assert.Equal("123456", cursor)
	r.Form = url.Values{
		"cursor": {"goodbye"},
	}
	cursor, err = getString(r, "cursor")
	tt.Assert.NoError(err)
	tt.Assert.Equal("goodbye", cursor)
}

func TestPath(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeFooBarTestActionRequest()

	tt.Assert.Equal("/foo-bar/blah", r.URL.Path)
}

func TestBaseGetURLParam(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeFooBarTestActionRequest()

	val, ok := getURLParam(r, "two")
	tt.Assert.Equal("2", val)
	tt.Assert.Equal(true, ok)

	// valid empty string
	val, ok = getURLParam(r, "blank")
	tt.Assert.Equal("", val)
	tt.Assert.Equal(true, ok)

	// url param not found
	val, ok = getURLParam(r, "foobarcursor")
	tt.Assert.Equal("", val)
	tt.Assert.Equal(false, ok)
}

func TestGetURLParam(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeTestActionRequest("/accounts/{account_id}/operations?limit=100", nil)

	// simulates a request where the named param is not passed.
	// Regression for https://github.com/stellar/go/issues/1965
	rctx := chi.RouteContext(r.Context())
	rctx.URLParams.Keys = []string{
		"account_id",
	}

	val, ok := getURLParam(r, "account_id")
	tt.Assert.Empty(val)
	tt.Assert.False(ok)
}

func TestFullURL(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	r := makeFooBarTestActionRequest()

	url := FullURL(r.Context())
	tt.Assert.Equal("http:///foo-bar/blah?limit=2&cursor=123456", url.String())
}

func TestGetParams(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()

	type QueryParams struct {
		SellingBuyingAssetQueryParams `valid:"-"`
		Account                       string `schema:"account_id" valid:"accountID"`
	}

	account := "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"

	// Simulate chi's URL params. The following would be equivalent to having a
	// chi route like the following `/accounts/{account_id}`
	urlParams := map[string]string{
		"account_id":           account,
		"selling_asset_type":   "credit_alphanum4",
		"selling_asset_code":   "USD",
		"selling_asset_issuer": account,
	}

	r := makeTestActionRequest("/transactions?limit=2&cursor=123456&order=desc", urlParams)
	qp := QueryParams{}
	err := getParams(&qp, r)

	tt.Assert.NoError(err)
	tt.Assert.Equal(account, qp.Account)
	selling, err := qp.Selling()
	tt.Assert.NoError(err)
	tt.Assert.NotNil(selling)
	tt.Assert.True(usd.Equals(*selling))

	urlParams = map[string]string{
		"account_id":         account,
		"selling_asset_type": "native",
	}

	r = makeTestActionRequest("/transactions?limit=2&cursor=123456&order=desc", urlParams)
	qp = QueryParams{}
	err = getParams(&qp, r)

	tt.Assert.NoError(err)
	selling, err = qp.Selling()
	tt.Assert.NoError(err)
	tt.Assert.NotNil(selling)
	tt.Assert.True(native.Equals(*selling))

	urlParams = map[string]string{"account_id": "1"}
	r = makeTestActionRequest("/transactions?limit=2&cursor=123456&order=desc", urlParams)
	qp = QueryParams{}
	err = getParams(&qp, r)

	if tt.Assert.IsType(&problem.P{}, err) {
		p := err.(*problem.P)
		tt.Assert.Equal("bad_request", p.Type)
		tt.Assert.Equal("account_id", p.Extras["invalid_field"])
		tt.Assert.Equal(
			"Account ID must start with `G` and contain 56 alphanum characters",
			p.Extras["reason"],
		)
	}

	// Test that we get the URL parameter properly
	// when a query parameter with the same name is provided
	urlParams = map[string]string{
		"account_id": "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
	}
	r = makeTestActionRequest("/transactions?account_id=bar", urlParams)
	err = getParams(&qp, r)
	tt.Assert.NoError(err)
	tt.Assert.Equal("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H", qp.Account)

}

type ParamsValidator struct {
	Account string `schema:"account_id" valid:"required"`
}

func (pv ParamsValidator) Validate() error {
	return problem.MakeInvalidFieldProblem(
		"Name",
		errors.New("Invalid"),
	)
}

func TestGetParamsCustomValidator(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()

	urlParams := map[string]string{"account_id": "1"}
	r := makeTestActionRequest("/transactions", urlParams)
	qp := ParamsValidator{}
	err := getParams(&qp, r)

	if tt.Assert.IsType(&problem.P{}, err) {
		p := err.(*problem.P)
		tt.Assert.Equal("bad_request", p.Type)
		tt.Assert.Equal("Name", p.Extras["invalid_field"])
	}
}

func makeFooBarTestActionRequest() *http.Request {
	return makeTestActionRequest("/foo-bar/blah?limit=2&cursor=123456", testURLParams())
}

func makeTestActionRequest(path string, body map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range body {
		rctx.URLParams.Add(k, v)
	}

	r, _ := http.NewRequest("GET", path, nil)

	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(context.WithValue(ctx, &horizonContext.RequestContextKey, r))
}

func testURLParams() map[string]string {
	return map[string]string{
		"blank":                   "",
		"minus_one":               "-1",
		"zero":                    "0",
		"two":                     "2",
		"twenty":                  "20",
		"32min":                   fmt.Sprint(math.MinInt32),
		"32max":                   fmt.Sprint(math.MaxInt32),
		"64min":                   fmt.Sprint(math.MinInt64),
		"64max":                   fmt.Sprint(math.MaxInt64),
		"native_asset_type":       "native",
		"4_asset_type":            "credit_alphanum4",
		"4_asset_code":            "USD",
		"4_asset_issuer":          "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		"12_asset_type":           "credit_alphanum12",
		"12_asset_code":           "USD",
		"12_asset_issuer":         "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		"long_4_asset_type":       "credit_alphanum4",
		"long_4_asset_code":       "SPOOON",
		"long_4_asset_issuer":     "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		"long_12_asset_type":      "credit_alphanum12",
		"long_12_asset_code":      "OHMYGODITSSOLONG",
		"long_12_asset_issuer":    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		"valid_tx_id":             "aa168f12124b7c196c0adaee7c73a64d37f99428cacb59a91ff389626845e7cf",
		"invalid_uppercase_tx_id": "AA168F12124B7C196C0ADAEE7C73A64D37F99428CACB59A91FF389626845E7CF",
		"invalid_too_short_tx_id": "aa168f12124b7c196c0adaee7c73a64d37f99428cacb59a91ff389626845e7",
	}
}

func makeRequest(
	t *testing.T,
	queryParams map[string]string,
	routeParams map[string]string,
	session db.SessionInterface,
) *http.Request {
	request, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	query := url.Values{}
	for key, value := range queryParams {
		query.Set(key, value)
	}
	request.URL.RawQuery = query.Encode()

	chiRouteContext := chi.NewRouteContext()
	for key, value := range routeParams {
		chiRouteContext.URLParams.Add(key, value)
	}
	ctx := context.WithValue(
		context.WithValue(context.Background(), chi.RouteCtxKey, chiRouteContext),
		&horizonContext.SessionContextKey,
		session,
	)

	return request.WithContext(ctx)
}

func TestGetURIParams(t *testing.T) {
	tt := assert.New(t)
	type QueryParams struct {
		SellingBuyingAssetQueryParams `valid:"-"`
		Account                       string `schema:"account_id" valid:"accountID"`
	}

	expected := []string{
		"selling_asset_type",
		"selling_asset_issuer",
		"selling_asset_code",
		"buying_asset_type",
		"buying_asset_issuer",
		"buying_asset_code",
		"selling",
		"buying",
		"account_id",
	}

	qp := QueryParams{}
	tt.Equal(expected, getURIParams(&qp, false))
}

package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldDiscardAsset(t *testing.T) {
	testAsset := hProtocol.AssetStat{}
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{}
	testAsset.Balances.Authorized = ""
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{}
	testAsset.Balances.Authorized = "0"
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{}
	testAsset.Balances.Authorized = "12345.67"
	testAsset.Accounts.Authorized = 0
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{}
	testAsset.Balances.Authorized = "12345.67"
	testAsset.Accounts.Authorized = 12
	testAsset.Code = "REMOVE"
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{}
	testAsset.Balances.Authorized = "12345.67"
	testAsset.Accounts.Authorized = 100
	testAsset.Code = "SOMETHINGVALID"
	testAsset.Links.Toml.Href = ""
	assert.Equal(t, shouldDiscardAsset(testAsset, true), false)

	testAsset = hProtocol.AssetStat{}
	testAsset.Balances.Authorized = "12345.67"
	testAsset.Accounts.Authorized = 40
	testAsset.Code = "SOMETHINGVALID"
	testAsset.Links.Toml.Href = "http://www.stellar.org/.well-known/stellar.toml"
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{}
	testAsset.Balances.Authorized = "12345.67"
	testAsset.Accounts.Authorized = 40
	testAsset.Code = "SOMETHINGVALID"
	testAsset.Links.Toml.Href = ""
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{}
	testAsset.Balances.Authorized = "12345.67"
	testAsset.Accounts.Authorized = 40
	testAsset.Code = "SOMETHINGVALID"
	testAsset.Links.Toml.Href = "https://www.stellar.org/.well-known/stellar.toml"
	assert.Equal(t, shouldDiscardAsset(testAsset, true), false)
}

func TestDomainsMatch(t *testing.T) {
	tomlURL, _ := url.Parse("https://stellar.org/stellar.toml")
	orgURL, _ := url.Parse("https://stellar.org/")
	assert.True(t, domainsMatch(tomlURL, orgURL))

	tomlURL, _ = url.Parse("https://assets.stellar.org/stellar.toml")
	orgURL, _ = url.Parse("https://stellar.org/")
	assert.False(t, domainsMatch(tomlURL, orgURL))

	tomlURL, _ = url.Parse("https://stellar.org/stellar.toml")
	orgURL, _ = url.Parse("https://home.stellar.org/")
	assert.True(t, domainsMatch(tomlURL, orgURL))

	tomlURL, _ = url.Parse("https://stellar.org/stellar.toml")
	orgURL, _ = url.Parse("https://home.stellar.com/")
	assert.False(t, domainsMatch(tomlURL, orgURL))

	tomlURL, _ = url.Parse("https://stellar.org/stellar.toml")
	orgURL, _ = url.Parse("https://stellar.com/")
	assert.False(t, domainsMatch(tomlURL, orgURL))
}

func TestIsDomainVerified(t *testing.T) {
	tomlURL := "https://stellar.org/stellar.toml"
	orgURL := "https://stellar.org/"
	hasCurrency := true
	assert.True(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "https://stellar.org/stellar.toml"
	orgURL = ""
	hasCurrency = true
	assert.True(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = ""
	orgURL = ""
	hasCurrency = true
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "https://stellar.org/stellar.toml"
	orgURL = "https://stellar.org/"
	hasCurrency = false
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "http://stellar.org/stellar.toml"
	orgURL = "https://stellar.org/"
	hasCurrency = true
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "https://stellar.org/stellar.toml"
	orgURL = "http://stellar.org/"
	hasCurrency = true
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "https://stellar.org/stellar.toml"
	orgURL = "https://stellar.com/"
	hasCurrency = true
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))
}

func TestIgnoreInvalidTOMLUrls(t *testing.T) {
	invalidURL := "https:// there is something wrong here.com/stellar.toml"
	_, err := fetchTOMLData(invalidURL)

	urlErr, ok := errors.Cause(err).(*url.Error)
	if !ok {
		t.Fatalf("err expected to be a url.Error but was %#v", err)
	}
	assert.Equal(t, "parse", urlErr.Op)
	assert.Equal(t, "https:// there is something wrong here.com/stellar.toml", urlErr.URL)
	assert.EqualError(t, urlErr.Err, `invalid character " " in host name`)
}

func TestProcessAsset_notCached(t *testing.T) {
	logger := log.DefaultLogger
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `SIGNING_KEY="not cached signing key"`)
	}))
	asset := hProtocol.AssetStat{}
	asset.Code = "SOMETHINGVALID"
	asset.Accounts.Authorized = 1
	asset.Balances.Authorized = "123.4"
	asset.Links.Toml.Href = server.URL
	tomlCache := &TOMLCache{}
	finalAsset, err := processAsset(logger, asset, tomlCache, true)
	require.NoError(t, err)
	assert.NotZero(t, finalAsset)
	assert.Equal(t, "not cached signing key", finalAsset.IssuerDetails.SigningKey)
	cachedTOML, ok := tomlCache.Get(server.URL)
	assert.True(t, ok)
	assert.Equal(t, TOMLIssuer{SigningKey: "not cached signing key"}, cachedTOML)
}

func TestProcessAsset_cached(t *testing.T) {
	logger := log.DefaultLogger
	asset := hProtocol.AssetStat{}
	asset.Code = "SOMETHINGVALID"
	asset.Accounts.Authorized = 1
	asset.Balances.Authorized = "123.4"
	asset.Links.Toml.Href = "url"
	tomlCache := &TOMLCache{}
	tomlCache.Set("url", TOMLIssuer{SigningKey: "signing key"})
	finalAsset, err := processAsset(logger, asset, tomlCache, true)
	require.NoError(t, err)
	assert.NotZero(t, finalAsset)
	assert.Equal(t, "signing key", finalAsset.IssuerDetails.SigningKey)
}

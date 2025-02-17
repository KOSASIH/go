package scraper

import (
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	horizonclient "github.com/stellar/go/clients/horizonclient"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/ticker/internal/utils"
	"github.com/stellar/go/support/errors"
	hlog "github.com/stellar/go/support/log"
)

// shouldDiscardAsset maps the criteria for discarding an asset from the asset index
func shouldDiscardAsset(asset hProtocol.AssetStat, shouldValidateTOML bool) bool {

	if asset.Balances.Authorized == "" {
		return true
	}
	f, _ := strconv.ParseFloat(asset.Balances.Authorized, 64)
	if f == 0.0 {
		return true
	}
	// [StellarX Ticker]: assets need at least some adoption to show up
	if asset.Accounts.Authorized < 10 {
		return true
	}
	if asset.Code == "REMOVE" {
		return true
	}
	// [StellarX Ticker]: assets with at least 100 accounts get a pass,
	// even with toml issues
	if asset.Accounts.Authorized >= 100 {
		return false
	}

	if shouldValidateTOML {
		if asset.Links.Toml.Href == "" {
			return true
		}
		// [StellarX Ticker]: TOML files should be hosted on HTTPS
		if !strings.HasPrefix(asset.Links.Toml.Href, "https://") {
			return true
		}
	}

	return false
}

// decodeTOMLIssuer decodes retrieved TOML issuer data into a TOMLIssuer struct
func decodeTOMLIssuer(tomlData string) (issuer TOMLIssuer, err error) {
	_, err = toml.Decode(tomlData, &issuer)
	return
}

// fetchTOMLData fetches the TOML data from the URL.
func fetchTOMLData(tomlURL string) (data string, err error) {
	if tomlURL == "" {
		err = errors.New("Asset does not have a TOML URL")
		return
	}

	timeout := time.Duration(10 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest("GET", tomlURL, nil)
	if err != nil {
		err = errors.Wrap(err, "invalid URL or request")
		return
	}

	req.Header.Set("User-Agent", "Stellar Ticker v1.0")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	data = string(body)
	return
}

func domainsMatch(tomlURL *url.URL, orgURL *url.URL) bool {
	tomlDomainParts := strings.Split(tomlURL.Host, ".")
	orgDomainParts := strings.Split(orgURL.Host, ".")

	if len(orgDomainParts) < len(tomlDomainParts) {
		// Org can only be a subdomain if it has more (or equal)
		// pieces than the TOML domain
		return false
	}

	lenDiff := len(orgDomainParts) - len(tomlDomainParts)
	orgDomainParts = orgDomainParts[lenDiff:]
	orgRootDomain := strings.Join(orgDomainParts, ".")
	return tomlURL.Host == orgRootDomain
}

// isDomainVerified performs some checking to ensure we can trust the Asset's domain
func isDomainVerified(orgURL string, tomlURL string, hasCurrency bool) bool {
	if tomlURL == "" {
		return false
	}

	parsedTomlURL, err := url.Parse(tomlURL)
	if err != nil || parsedTomlURL.Scheme != "https" {
		return false
	}

	if !hasCurrency {
		return false
	}

	if orgURL == "" {
		// if no orgURL is provided, we'll simply use tomlURL, so no need
		// for domain verification
		return true
	}

	parsedOrgURL, err := url.Parse(orgURL)
	if err != nil || parsedOrgURL.Scheme != "https" {
		return false
	}

	if !domainsMatch(parsedTomlURL, parsedOrgURL) {
		return false
	}
	return true
}

// makeTomlAsset aggregates Horizon Data with TOML Data
func makeFinalAsset(
	asset hProtocol.AssetStat,
	issuer TOMLIssuer,
	errors []error,
) (t FinalAsset, err error) {
	amount, err := strconv.ParseFloat(asset.Balances.Authorized, 64)
	if err != nil {
		return
	}

	t = FinalAsset{
		Type:          asset.Type,
		Code:          asset.Code,
		Issuer:        asset.Issuer,
		NumAccounts:   asset.Accounts.Authorized,
		AuthRequired:  asset.Flags.AuthRequired,
		AuthRevocable: asset.Flags.AuthRevocable,
		Amount:        amount,
		IssuerDetails: issuer,
	}

	t.IssuerDetails.TOMLURL = asset.Links.Toml.Href

	hasCurrency := false
	for _, currency := range t.IssuerDetails.Currencies {
		if currency.Code == asset.Code && currency.Issuer == asset.Issuer {
			hasCurrency = true
			t.AnchorAsset = currency.AnchorAsset
			t.AnchorAssetType = currency.AnchorAssetType
			t.DisplayDecimals = currency.DisplayDecimals
			t.Name = currency.Name
			t.Desc = currency.Desc
			t.Conditions = currency.Conditions
			t.IsAssetAnchored = currency.IsAssetAnchored
			t.FixedNumber = currency.FixedNumber
			t.MaxNumber = currency.MaxNumber
			t.IsUnlimited = currency.IsUnlimited
			t.RedemptionInstructions = currency.RedemptionInstructions
			t.CollateralAddresses = currency.CollateralAddresses
			t.CollateralAddressSignatures = currency.CollateralAddressSignatures
			t.Status = currency.Status
			break
		}
	}
	t.AssetControlledByDomain = isDomainVerified(
		t.IssuerDetails.Documentation.OrgURL,
		asset.Links.Toml.Href,
		hasCurrency,
	)

	if !hasCurrency {
		t.AssetControlledByDomain = false
	}

	now := time.Now()
	if len(errors) > 0 {
		t.Error = fmt.Sprintf("%v", errors)
		t.IsValid = false
	} else {
		t.LastValid = now
		t.IsValid = true
	}
	t.LastChecked = now
	t.AnchorAssetType = strings.ToLower(t.AnchorAssetType)

	return
}

// processAsset merges data from an AssetStat with data retrieved from its corresponding TOML file
func processAsset(logger *hlog.Entry, asset hProtocol.AssetStat, tomlCache *TOMLCache, shouldValidateTOML bool) (FinalAsset, error) {
	var errors []error
	var issuer TOMLIssuer

	if shouldValidateTOML {
		tomlURL := asset.Links.Toml.Href
		logger = logger.WithField("asset_toml_url", tomlURL)
		logger.Info("Collecting TOML for asset")

		var ok bool
		issuer, ok = tomlCache.Get(tomlURL)
		if ok {
			logger.Info("Using cached TOML for asset")
		} else {
			logger.Info("Fetching TOML for asset")
			tomlData, err := fetchTOMLData(tomlURL)
			if err != nil {
				errors = append(errors, err)
			}

			issuer, err = decodeTOMLIssuer(tomlData)
			if err != nil {
				errors = append(errors, err)
			}

			tomlCache.Set(tomlURL, issuer)
		}
	}

	return makeFinalAsset(asset, issuer, errors)
}

// parallelProcessAssets filters the assets that don't match the shouldDiscardAsset criteria.
// non-trash assets are sent to the assetQueue.
// The TOML validation is performed in parallel to improve performance.
func (c *ScraperConfig) parallelProcessAssets(assets []hProtocol.AssetStat, parallelism int, assetQueue chan<- FinalAsset) (numNonTrash int, numTrash int) {
	shouldValidateTOML := c.Client != horizonclient.DefaultTestNetClient // TOMLs shouldn't be validated on TestNet
	var mutex = &sync.Mutex{}
	var wg sync.WaitGroup
	numAssets := len(assets)
	chunkSize := int(math.Ceil(float64(numAssets) / float64(parallelism)))
	wg.Add(parallelism)

	// Sort assets by their toml URL so that assets with the same toml URL are
	// grouped together. This is so that we can load each toml URL once, and
	// cache the result for the subsequent assets without needing to store more
	// than one toml in memory at a time.
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Links.Toml.Href < assets[j].Links.Toml.Href
	})

	// The assets are divided into chunks of chunkSize, and each goroutine is responsible
	// for cleaning up one chunk
	for i := 0; i < parallelism; i++ {
		go func(start int) {
			defer wg.Done()

			end := start + chunkSize
			if end > numAssets {
				end = numAssets
			}

			// Each routine running concurrently has a separate cache of TOMLs
			// loaded. A single shared cache would be better, but this is a
			// tradeoff for simplicity because a shared map mutated with HTTP
			// lookups would have a significant amount of contention.
			tomlCache := &TOMLCache{}

			for j := start; j < end; j++ {
				logger := c.Logger.
					WithField("asset_code", assets[j].Asset.Code).
					WithField("asset_issuer", assets[j].Asset.Issuer)
				if !shouldDiscardAsset(assets[j], shouldValidateTOML) {
					c.Logger.Info("Processing asset")
					finalAsset, err := processAsset(logger, assets[j], tomlCache, shouldValidateTOML)
					if err != nil {
						mutex.Lock()
						numTrash++
						mutex.Unlock()
						continue
					}
					assetQueue <- finalAsset
				} else {
					c.Logger.Info("Discarding asset")
					mutex.Lock()
					numTrash++
					mutex.Unlock()
				}
			}
		}(i * chunkSize)
	}

	wg.Wait()
	close(assetQueue)

	numNonTrash = len(assets) - numTrash
	return
}

// retrieveAssets retrieves existing assets from the Horizon API. If limit=0, will fetch all assets.
func (c *ScraperConfig) retrieveAssets(limit int) (assets []hProtocol.AssetStat, err error) {
	r := horizonclient.AssetRequest{Limit: 200}

	assetsPage, err := c.Client.Assets(r)
	if err != nil {
		return
	}

	c.Logger.Info("Fetching assets from Horizon")

	for assetsPage.Links.Next.Href != assetsPage.Links.Self.Href {
		err = utils.Retry(5, 5*time.Second, c.Logger, func() error {
			assetsPage, err = c.Client.Assets(r)
			if err != nil {
				c.Logger.Info("Horizon rate limit reached!")
			}
			return err
		})
		if err != nil {
			return
		}
		assets = append(assets, assetsPage.Embedded.Records...)

		if limit != 0 { // for performance reasons, only perform these additional checks when limit != 0
			numAssets := len(assets)
			if numAssets >= limit {
				diff := numAssets - limit
				assets = assets[0 : numAssets-diff]
				break
			}
		}

		nextURL := assetsPage.Links.Next.Href
		n, err := nextCursor(nextURL)
		if err != nil {
			return assets, err
		}
		c.Logger.Debug("Cursor currently at:", n)

		r = horizonclient.AssetRequest{Limit: 200, Cursor: n}
	}

	c.Logger.Infof("Fetched: %d assets\n", len(assets))
	return
}

package history

import (
	"strings"

	"github.com/stellar/go/support/collections/set"
	"github.com/stellar/go/xdr"
)

type AssetKey struct {
	Type   string
	Code   string
	Issuer string
}

func (key AssetKey) String() string {
	if key.Type == xdr.AssetTypeToString[xdr.AssetTypeAssetTypeNative] {
		return key.Type
	}
	return key.Type + "/" + key.Code + "/" + key.Issuer
}

// AssetKeyFromXDR constructs an AssetKey from an xdr asset
func AssetKeyFromXDR(asset xdr.Asset) AssetKey {
	return AssetKey{
		Type:   xdr.AssetTypeToString[asset.Type],
		Code:   strings.TrimRight(asset.GetCode(), "\x00"),
		Issuer: asset.GetIssuer(),
	}
}

// FutureAssetID represents a future history asset.
// A FutureAssetID is created by an AssetLoader and
// the asset id is available after calling Exec() on
// the AssetLoader.
type FutureAssetID = future[AssetKey, Asset]

// AssetLoader will map assets to their history
// asset ids. If there is no existing mapping for a given sset,
// the AssetLoader will insert into the history_assets table to
// establish a mapping.
type AssetLoader = loader[AssetKey, Asset]

// NewAssetLoader will construct a new AssetLoader instance.
func NewAssetLoader(concurrencyMode ConcurrencyMode) *AssetLoader {
	return &AssetLoader{
		sealed: false,
		set:    set.Set[AssetKey]{},
		ids:    map[AssetKey]int64{},
		stats:  LoaderStats{},
		name:   "AssetLoader",
		table:  "history_assets",
		columnsForKeys: func(keys []AssetKey) []columnValues {
			assetTypes := make([]string, 0, len(keys))
			assetCodes := make([]string, 0, len(keys))
			assetIssuers := make([]string, 0, len(keys))
			for _, key := range keys {
				assetTypes = append(assetTypes, key.Type)
				assetCodes = append(assetCodes, key.Code)
				assetIssuers = append(assetIssuers, key.Issuer)
			}

			return []columnValues{
				{
					name:    "asset_code",
					dbType:  "character varying(12)",
					objects: assetCodes,
				},
				{
					name:    "asset_type",
					dbType:  "character varying(64)",
					objects: assetTypes,
				},
				{
					name:    "asset_issuer",
					dbType:  "character varying(56)",
					objects: assetIssuers,
				},
			}
		},
		mappingFromRow: func(asset Asset) (AssetKey, int64) {
			return AssetKey{
				Type:   asset.Type,
				Code:   asset.Code,
				Issuer: asset.Issuer,
			}, asset.ID
		},
		less: func(a AssetKey, b AssetKey) bool {
			return a.String() < b.String()
		},
		concurrencyMode: concurrencyMode,
	}
}

// AssetLoaderStub is a stub wrapper around AssetLoader which allows
// you to manually configure the mapping of assets to history asset ids
type AssetLoaderStub struct {
	Loader *AssetLoader
}

// NewAssetLoaderStub returns a new AssetLoaderStub instance
func NewAssetLoaderStub() AssetLoaderStub {
	return AssetLoaderStub{Loader: NewAssetLoader(ConcurrentInserts)}
}

// Insert updates the wrapped AssetLoaderStub so that the given asset
// address is mapped to the provided history asset id
func (a AssetLoaderStub) Insert(asset AssetKey, id int64) {
	a.Loader.sealed = true
	a.Loader.ids[asset] = id
}

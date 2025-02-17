package asset

// NewNativeAsset creates an Asset representing the native token (XLM).
func NewNativeAsset() *Asset {
	return &Asset{AssetType: &Asset_Native{Native: true}}
}

// NewIssuedAsset creates an Asset with an asset code and issuer.
func NewIssuedAsset(assetCode, issuer string) *Asset {
	return &Asset{
		AssetType: &Asset_IssuedAsset{
			IssuedAsset: &IssuedAsset{
				AssetCode: assetCode,
				Issuer:    issuer,
			},
		},
	}
}

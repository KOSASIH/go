package asset

import (
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestNewNativeAsset(t *testing.T) {
	nativeAsset := NewNativeAsset()

	assert.NotNil(t, nativeAsset, "Native asset should not be nil")
	assert.IsType(t, &Asset{}, nativeAsset, "Native asset should be of type *Asset")
	assert.IsType(t, &Asset_Native{}, nativeAsset.AssetType, "AssetType should be *Asset_Native")
	assert.True(t, nativeAsset.GetNative(), "Native asset should have Native set to true")
}

func TestNewIssuedAsset(t *testing.T) {
	assetCode := "USDC"
	issuer := "GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN"

	issuedAsset := NewIssuedAsset(assetCode, issuer)

	assert.NotNil(t, issuedAsset, "Issued asset should not be nil")
	assert.IsType(t, &Asset{}, issuedAsset, "Issued asset should be of type *Asset")
	assert.IsType(t, &Asset_IssuedAsset{}, issuedAsset.AssetType, "AssetType should be *Asset_IssuedAsset")

	// Check issued asset fields
	assert.Equal(t, assetCode, issuedAsset.GetIssuedAsset().AssetCode, "Asset code should match")
	assert.Equal(t, issuer, issuedAsset.GetIssuedAsset().Issuer, "Issuer should match")
}

func TestAssetSerialization(t *testing.T) {
	assetCode := "USDC"
	issuer := "GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN"
	original := NewIssuedAsset(assetCode, issuer)

	serializedAsset, err := proto.Marshal(original)
	assert.NoError(t, err, "Failed to marshal asset")

	var deserializedAsset Asset
	err = proto.Unmarshal(serializedAsset, &deserializedAsset)
	assert.NoError(t, err, "Failed to unmarshal asset")

	assert.True(t, proto.Equal(original, &deserializedAsset), "Deserialized asset does not match the original")
}

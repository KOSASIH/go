package history

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
)

func TestAssetKeyToString(t *testing.T) {
	num4key := AssetKey{
		Type:   "credit_alphanum4",
		Code:   "USD",
		Issuer: "A1B2C3",
	}

	num12key := AssetKey{
		Type:   "credit_alphanum12",
		Code:   "USDABC",
		Issuer: "A1B2C3",
	}

	nativekey := AssetKey{
		Type: "native",
	}

	assert.Equal(t, num4key.String(), "credit_alphanum4/USD/A1B2C3")
	assert.Equal(t, num12key.String(), "credit_alphanum12/USDABC/A1B2C3")
	assert.Equal(t, nativekey.String(), "native")
}

func TestAssetLoader(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	session := tt.HorizonSession()

	testAssetLoader(t, session, ConcurrentInserts)
	test.ResetHorizonDB(t, tt.HorizonDB)
	testAssetLoader(t, session, ConcurrentDeletes)
}

func testAssetLoader(t *testing.T, session *db.Session, mode ConcurrencyMode) {
	var keys []AssetKey
	for i := 0; i < 100; i++ {
		var key AssetKey
		if i == 0 {
			key = AssetKeyFromXDR(xdr.Asset{Type: xdr.AssetTypeAssetTypeNative})
		} else if i%2 == 0 {
			code := [4]byte{0, 0, 0, 0}
			copy(code[:], fmt.Sprintf("ab%d", i))
			key = AssetKeyFromXDR(xdr.Asset{
				Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
				AlphaNum4: &xdr.AlphaNum4{
					AssetCode: code,
					Issuer:    xdr.MustAddress(keypair.MustRandom().Address())}})
		} else {
			code := [12]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
			copy(code[:], fmt.Sprintf("abcdef%d", i))
			key = AssetKeyFromXDR(xdr.Asset{
				Type: xdr.AssetTypeAssetTypeCreditAlphanum12,
				AlphaNum12: &xdr.AlphaNum12{
					AssetCode: code,
					Issuer:    xdr.MustAddress(keypair.MustRandom().Address())}})

		}
		keys = append(keys, key)
	}

	loader := NewAssetLoader(mode)
	for _, key := range keys {
		future := loader.GetFuture(key)
		_, err := future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid loader state,`)
		duplicateFuture := loader.GetFuture(key)
		assert.Equal(t, future, duplicateFuture)
	}

	err := loader.Exec(context.Background(), session)
	assert.NoError(t, err)
	assert.Equal(t, LoaderStats{
		Total:    100,
		Inserted: 100,
	}, loader.Stats())
	assert.Panics(t, func() {
		loader.GetFuture(AssetKey{Type: "invalid"})
	})

	q := &Q{session}
	for _, key := range keys {
		var internalID int64
		internalID, err = loader.GetNow(key)
		assert.NoError(t, err)
		var assetXDR xdr.Asset
		if key.Type == "native" {
			assetXDR = xdr.MustNewNativeAsset()
		} else {
			assetXDR = xdr.MustNewCreditAsset(key.Code, key.Issuer)
		}
		var assetID int64
		assetID, err = q.GetAssetID(context.Background(), assetXDR)
		assert.NoError(t, err)
		assert.Equal(t, assetID, internalID)
	}

	_, err = loader.GetNow(AssetKey{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `was not found`)

	// check that Loader works when all the previous values are already
	// present in the db and also add 10 more rows to insert
	loader = NewAssetLoader(mode)
	for i := 0; i < 10; i++ {
		var key AssetKey
		if i%2 == 0 {
			code := [4]byte{0, 0, 0, 0}
			copy(code[:], fmt.Sprintf("ab%d", i))
			key = AssetKeyFromXDR(xdr.Asset{
				Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
				AlphaNum4: &xdr.AlphaNum4{
					AssetCode: code,
					Issuer:    xdr.MustAddress(keypair.MustRandom().Address())}})
		} else {
			code := [12]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
			copy(code[:], fmt.Sprintf("abcdef%d", i))
			key = AssetKeyFromXDR(xdr.Asset{
				Type: xdr.AssetTypeAssetTypeCreditAlphanum12,
				AlphaNum12: &xdr.AlphaNum12{
					AssetCode: code,
					Issuer:    xdr.MustAddress(keypair.MustRandom().Address())}})

		}
		keys = append(keys, key)
	}

	for _, key := range keys {
		future := loader.GetFuture(key)
		_, err = future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid loader state,`)
	}
	assert.NoError(t, loader.Exec(context.Background(), session))
	assert.Equal(t, LoaderStats{
		Total:    110,
		Inserted: 10,
	}, loader.Stats())

	for _, key := range keys {
		var internalID int64
		internalID, err = loader.GetNow(key)
		assert.NoError(t, err)
		var assetXDR xdr.Asset
		if key.Type == "native" {
			assetXDR = xdr.MustNewNativeAsset()
		} else {
			assetXDR = xdr.MustNewCreditAsset(key.Code, key.Issuer)
		}
		var assetID int64
		assetID, err = q.GetAssetID(context.Background(), assetXDR)
		assert.NoError(t, err)
		assert.Equal(t, assetID, internalID)
	}
}

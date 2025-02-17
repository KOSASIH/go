package history

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/guregu/null"

	"github.com/stellar/go/protocols/horizon/effects"
	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/toid"
)

func TestEffectsForLiquidityPool(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}
	tt.Assert.NoError(q.Begin(tt.Ctx))

	// Insert Effect
	address := "GAQAA5L65LSYH7CQ3VTJ7F3HHLGCL3DSLAR2Y47263D56MNNGHSQSTVY"
	muxedAddres := "MAQAA5L65LSYH7CQ3VTJ7F3HHLGCL3DSLAR2Y47263D56MNNGHSQSAAAAAAAAAAE2LP26"
	accountLoader := NewAccountLoader(ConcurrentInserts)

	builder := q.NewEffectBatchInsertBuilder()
	sequence := int32(56)
	details, err := json.Marshal(map[string]string{
		"amount":     "1000.0000000",
		"asset_type": "native",
	})
	tt.Assert.NoError(err)
	opID := toid.New(sequence, 1, 1).ToInt64()
	tt.Assert.NoError(builder.Add(
		accountLoader.GetFuture(address),
		null.StringFrom(muxedAddres),
		opID,
		1,
		3,
		details,
	))

	tt.Assert.NoError(accountLoader.Exec(tt.Ctx, q))
	tt.Assert.NoError(builder.Exec(tt.Ctx, q))

	// Insert Liquidity Pool history
	liquidityPoolID := "abcde"
	lpLoader := NewLiquidityPoolLoader(ConcurrentInserts)

	operationBuilder := q.NewOperationLiquidityPoolBatchInsertBuilder()
	tt.Assert.NoError(operationBuilder.Add(opID, lpLoader.GetFuture(liquidityPoolID)))
	tt.Assert.NoError(lpLoader.Exec(tt.Ctx, q))
	tt.Assert.NoError(operationBuilder.Exec(tt.Ctx, q))

	tt.Assert.NoError(q.Commit())

	var effects []Effect
	effects, err = q.EffectsForLiquidityPool(tt.Ctx, liquidityPoolID, db2.PageQuery{
		Cursor: "0-0",
		Order:  "asc",
		Limit:  10,
	}, 0)
	tt.Assert.NoError(err)

	tt.Assert.Len(effects, 1)
	effect := effects[0]
	tt.Assert.Equal(effect.Account, address)

	effects, err = q.EffectsForLiquidityPool(tt.Ctx, liquidityPoolID, db2.PageQuery{
		Cursor: fmt.Sprintf("%d-0", toid.New(sequence+2, 0, 0).ToInt64()),
		Order:  "desc",
		Limit:  200,
	}, sequence-3)
	tt.Require.NoError(err)
	tt.Require.Len(effects, 1)
	tt.Require.Equal(effects[0], effect)

	effects, err = q.EffectsForLiquidityPool(tt.Ctx, liquidityPoolID, db2.PageQuery{
		Cursor: fmt.Sprintf("%d-0", toid.New(sequence+5, 0, 0).ToInt64()),
		Order:  "desc",
		Limit:  200,
	}, sequence+2)
	tt.Require.NoError(err)
	tt.Require.Empty(effects)
}

func TestEffectsForTrustlinesSponsorshipEmptyAssetType(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}
	tt.Assert.NoError(q.Begin(tt.Ctx))

	address := "GAQAA5L65LSYH7CQ3VTJ7F3HHLGCL3DSLAR2Y47263D56MNNGHSQSTVY"
	muxedAddres := "MAQAA5L65LSYH7CQ3VTJ7F3HHLGCL3DSLAR2Y47263D56MNNGHSQSAAAAAAAAAAE2LP26"
	accountLoader := NewAccountLoader(ConcurrentInserts)

	builder := q.NewEffectBatchInsertBuilder()
	sequence := int32(56)
	tests := []struct {
		effectType        EffectType
		details           map[string]string
		expectedAssetType string
	}{
		{
			EffectTrustlineSponsorshipCreated,
			map[string]string{
				"asset":   "USD:GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
				"sponsor": "GDMQUXK7ZUCWM5472ZU3YLDP4BMJLQQ76DEMNYDEY2ODEEGGRKLEWGW2",
			},
			"credit_alphanum4",
		},
		{
			EffectTrustlineSponsorshipCreated,
			map[string]string{
				"asset":   "USDCE:GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
				"sponsor": "GDMQUXK7ZUCWM5472ZU3YLDP4BMJLQQ76DEMNYDEY2ODEEGGRKLEWGW2",
			},
			"credit_alphanum12",
		},
		{
			EffectTrustlineSponsorshipUpdated,
			map[string]string{
				"asset":   "USD:GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
				"sponsor": "GDMQUXK7ZUCWM5472ZU3YLDP4BMJLQQ76DEMNYDEY2ODEEGGRKLEWGW2",
			},
			"credit_alphanum4",
		},
		{
			EffectTrustlineSponsorshipUpdated,
			map[string]string{
				"asset":   "USDCE:GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
				"sponsor": "GDMQUXK7ZUCWM5472ZU3YLDP4BMJLQQ76DEMNYDEY2ODEEGGRKLEWGW2",
			},
			"credit_alphanum12",
		},
		{
			EffectTrustlineSponsorshipRemoved,
			map[string]string{
				"asset":   "USD:GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
				"sponsor": "GDMQUXK7ZUCWM5472ZU3YLDP4BMJLQQ76DEMNYDEY2ODEEGGRKLEWGW2",
			},
			"credit_alphanum4",
		},
		{
			EffectTrustlineSponsorshipRemoved,
			map[string]string{
				"asset":   "USDCE:GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
				"sponsor": "GDMQUXK7ZUCWM5472ZU3YLDP4BMJLQQ76DEMNYDEY2ODEEGGRKLEWGW2",
			},
			"credit_alphanum12",
		},
	}
	opID := toid.New(sequence, 1, 1).ToInt64()

	for i, test := range tests {
		var bytes []byte
		bytes, err := json.Marshal(test.details)
		tt.Require.NoError(err)

		tt.Require.NoError(builder.Add(
			accountLoader.GetFuture(address),
			null.StringFrom(muxedAddres),
			opID,
			uint32(i),
			test.effectType,
			bytes,
		))
	}
	tt.Require.NoError(accountLoader.Exec(tt.Ctx, q))
	tt.Require.NoError(builder.Exec(tt.Ctx, q))
	tt.Assert.NoError(q.Commit())

	results, err := q.Effects(tt.Ctx, db2.PageQuery{
		Cursor: "0-0",
		Order:  "asc",
		Limit:  200,
	}, 0)
	tt.Require.NoError(err)
	tt.Require.Len(results, len(tests))

	for i, test := range tests {
		switch test.effectType {
		case EffectTrustlineSponsorshipCreated:
			var eff effects.TrustlineSponsorshipCreated
			err := results[i].UnmarshalDetails(&eff)
			tt.Require.NoError(err)
			tt.Assert.Equal(test.expectedAssetType, eff.Type)
		case EffectTrustlineSponsorshipUpdated:
			var eff effects.TrustlineSponsorshipUpdated
			err := results[i].UnmarshalDetails(&eff)
			tt.Require.NoError(err)
			tt.Assert.Equal(test.expectedAssetType, eff.Type)
		case EffectTrustlineSponsorshipRemoved:
			var eff effects.TrustlineSponsorshipRemoved
			err := results[i].UnmarshalDetails(&eff)
			tt.Require.NoError(err)
			tt.Assert.Equal(test.expectedAssetType, eff.Type)
		default:
			panic(fmt.Sprintf("Unknown type %v", test.effectType))
		}
	}
}

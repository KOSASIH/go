package xdr

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go/gxdr"
	"github.com/stellar/go/randxdr"
)

func TestScValEqualsCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 30000; i++ {
		scVal := ScVal{}

		shape := &gxdr.SCVal{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		require.NoError(t, gxdr.Convert(shape, &scVal))

		clonedScVal := ScVal{}
		require.NoError(t, gxdr.Convert(shape, &clonedScVal))
		require.True(t, scVal.Equals(clonedScVal), "scVal: %#v, clonedScVal: %#v", scVal, clonedScVal)
	}
}

func TestScValStringCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 30000; i++ {
		scVal := ScVal{}

		shape := &gxdr.SCVal{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		require.NoError(t, gxdr.Convert(shape, &scVal))

		var str string
		require.NotPanics(t, func() {
			str = scVal.String()
		})
		require.NotEqual(t, str, "unknown")
	}
}

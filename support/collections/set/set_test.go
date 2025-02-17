package set

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	s := NewSet[string](10)
	s.Add("sanity")
	require.True(t, s.Contains("sanity"))
	require.False(t, s.Contains("check"))

	s.AddSlice([]string{"a", "b", "c"})
	require.True(t, s.Contains("b"))
	require.ElementsMatch(t, []string{"sanity", "a", "b", "c"}, s.Slice())
}

func TestSafeSet(t *testing.T) {
	s := NewSafeSet[string](0)
	s.Add("sanity")
	require.True(t, s.Contains("sanity"))
	require.False(t, s.Contains("check"))
}

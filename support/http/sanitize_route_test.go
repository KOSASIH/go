package http

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSanitizesRoutesForPrometheus(t *testing.T) {
	for _, setup := range []struct {
		name     string
		route    string
		expected string
	}{
		{
			"normal routes",
			"/accounts",
			"/accounts",
		},
		{
			"non-regex params",
			"/claimable_balances/{id}",
			"/claimable_balances/{id}",
		},
		{
			"named regexes",
			"/accounts/{account_id:\\w+}/effects",
			"/accounts/{account_id}/effects",
		},
		{
			"unnamed regexes",
			"/accounts/{\\w+}/effects",
			"/accounts/{\\\\w+}/effects",
		},
		{
			// Not likely used in routes, but just safer for prom metrics anyway
			"quotes",
			"/{\"}",
			"/{\\\"}",
		},
	} {
		t.Run(setup.name, func(t *testing.T) {
			assert.Equal(t, setup.expected, sanitizeMetricRoute(setup.route))
		})
	}

}

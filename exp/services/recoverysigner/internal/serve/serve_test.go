package serve

import (
	"encoding/json"
	"testing"

	"github.com/stellar/go/keypair"
	supportlog "github.com/stellar/go/support/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2"
)

func TestGetHandlerDeps(t *testing.T) {
	signingKey := "SBWLXUTJR2CGVPGCZDIGGLQDPX7ZGGBHBFXBJ555MNIQ2PZCCLM643Z3"
	signingKeyFull := keypair.MustParseFull(signingKey)

	opts := Options{
		Logger:      supportlog.DefaultLogger,
		SigningKeys: signingKey,
		SEP10JWKS:   `{"keys":[{"kty":"EC","crv":"P-256","alg":"ES256","x":"i8chX_7Slm4VQ_Y6XBWVBnxIO5-XSWH1GJsXWNkal3E","y":"G22r0OgrcQnkfCAqsS6wvtHgR0SbfvXNJy6-jJfvc94"}]}`,
	}

	sep10JWKS := jose.JSONWebKeySet{}
	err := json.Unmarshal([]byte(opts.SEP10JWKS), &sep10JWKS)
	require.NoError(t, err)

	got, err := getHandlerDeps(opts)
	assert.NoError(t, err)

	assert.Equal(t, []*keypair.Full{signingKeyFull}, got.SigningKeys)
	assert.Equal(t, []*keypair.FromAddress{signingKeyFull.FromAddress()}, got.SigningAddresses)
	assert.Equal(t, sep10JWKS, got.SEP10JWKS)
	assert.Equal(t, []*keypair.FromAddress{}, got.AllowedSourceAccounts)
}

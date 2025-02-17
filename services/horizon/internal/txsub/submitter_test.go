package txsub

import (
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/services/horizon/internal/test"
)

const (
	TxXDR = "AAAAAAGUcmKO5465JxTSLQOQljwk2SfqAJmZSG6JH6wtqpwhAAABLAAAAAAAAAABAAAAAAAAAAEAAAALaGVsbG8gd29ybGQAAAAAAwAAAAAAAAAAAAAAABbxCy3mLg3hiTqX4VUEEp60pFOrJNxYM1JtxXTwXhY2AAAAAAvrwgAAAAAAAAAAAQAAAAAW8Qst5i4N4Yk6l+FVBBKetKRTqyTcWDNSbcV08F4WNgAAAAAN4Lazj4x61AAAAAAAAAAFAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABLaqcIQAAAEBKwqWy3TaOxoGnfm9eUjfTRBvPf34dvDA0Nf+B8z4zBob90UXtuCqmQqwMCyH+okOI3c05br3khkH0yP4kCwcE"
)

func TestDefaultSubmitter(t *testing.T) {
	ctx := test.Context()
	// submits to the configured stellar-core instance correctly
	server := test.NewStaticMockServer(`{
		"status": "PENDING",
		"error": null
		}`)
	defer server.Close()

	s := NewDefaultSubmitter(http.DefaultClient, server.URL, prometheus.NewRegistry())
	sr := s.Submit(ctx, TxXDR)
	assert.Nil(t, sr.Err)
	assert.True(t, sr.Duration > 0)
	assert.Equal(t, TxXDR, server.LastRequest.URL.Query().Get("blob"))

	// Succeeds when stellar-core gives the DUPLICATE response.
	server = test.NewStaticMockServer(`{
				"status": "DUPLICATE",
				"error": null
				}`)
	defer server.Close()

	s = NewDefaultSubmitter(http.DefaultClient, server.URL, prometheus.NewRegistry())
	sr = s.Submit(ctx, TxXDR)
	assert.Nil(t, sr.Err)

	// Errors when the stellar-core url is empty

	s = NewDefaultSubmitter(http.DefaultClient, "", prometheus.NewRegistry())
	sr = s.Submit(ctx, TxXDR)
	assert.NotNil(t, sr.Err)

	//errors when the stellar-core url is not parseable

	s = NewDefaultSubmitter(http.DefaultClient, "http://Not a url", prometheus.NewRegistry())
	sr = s.Submit(ctx, TxXDR)
	assert.NotNil(t, sr.Err)

	// errors when the stellar-core url is not reachable
	s = NewDefaultSubmitter(http.DefaultClient, "http://127.0.0.1:65535", prometheus.NewRegistry())
	sr = s.Submit(ctx, TxXDR)
	assert.NotNil(t, sr.Err)

	// errors when the stellar-core returns an unparseable response
	server = test.NewStaticMockServer(`{`)
	defer server.Close()

	s = NewDefaultSubmitter(http.DefaultClient, server.URL, prometheus.NewRegistry())
	sr = s.Submit(ctx, TxXDR)
	assert.NotNil(t, sr.Err)

	// errors when the stellar-core returns an exception response
	server = test.NewStaticMockServer(`{"exception": "Invalid XDR"}`)
	defer server.Close()

	s = NewDefaultSubmitter(http.DefaultClient, server.URL, prometheus.NewRegistry())
	sr = s.Submit(ctx, TxXDR)
	assert.NotNil(t, sr.Err)
	assert.Contains(t, sr.Err.Error(), "Invalid XDR")

	// errors when the stellar-core returns an unrecognized status
	server = test.NewStaticMockServer(`{"status": "NOTREAL"}`)
	defer server.Close()

	s = NewDefaultSubmitter(http.DefaultClient, server.URL, prometheus.NewRegistry())
	sr = s.Submit(ctx, TxXDR)
	assert.NotNil(t, sr.Err)
	assert.Contains(t, sr.Err.Error(), "NOTREAL")

	// errors when the stellar-core returns an error response
	server = test.NewStaticMockServer(`{"status": "ERROR", "error": "1234"}`)
	defer server.Close()

	s = NewDefaultSubmitter(http.DefaultClient, server.URL, prometheus.NewRegistry())
	sr = s.Submit(ctx, TxXDR)
	assert.IsType(t, &FailedTransactionError{}, sr.Err)
	ferr := sr.Err.(*FailedTransactionError)
	assert.Equal(t, "1234", ferr.ResultXDR)
}

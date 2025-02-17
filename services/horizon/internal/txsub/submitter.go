package txsub

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go/clients/stellarcore"
	proto "github.com/stellar/go/protocols/stellarcore"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/log"
)

// NewDefaultSubmitter returns a new, simple Submitter implementation
// that submits directly to the stellar-core at `url` using the http client
// `h`.
func NewDefaultSubmitter(h *http.Client, url string, registry *prometheus.Registry) Submitter {
	return &submitter{
		StellarCore: stellarcore.NewClientWithMetrics(stellarcore.Client{
			HTTP: h,
			URL:  url,
		}, registry, "txsub"),
		Log: log.DefaultLogger.WithField("service", "txsub.submitter"),
	}
}

// submitter is the default implementation for the Submitter interface.  It
// submits directly to the configured stellar-core instance using the
// configured http client.
type submitter struct {
	StellarCore stellarcore.ClientWithMetrics
	Log         *log.Entry
}

// Submit sends the provided envelope to stellar-core and parses the response into
// a SubmissionResult
func (sub *submitter) Submit(ctx context.Context, rawTx string) (result SubmissionResult) {
	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
		sub.Log.Ctx(ctx).WithFields(log.F{
			"err":      result.Err,
			"duration": result.Duration.Seconds(),
		}).Info("Submitter result")
	}()

	cresp, err := sub.StellarCore.SubmitTx(ctx, rawTx)
	if err != nil {
		result.Err = errors.Wrap(err, "failed to submit")
		return
	}

	// interpret response
	if cresp.IsException() {
		result.Err = errors.Errorf("stellar-core exception: %s", cresp.Exception)
		return
	}

	switch cresp.Status {
	case proto.TXStatusError:
		result.Err = &FailedTransactionError{cresp.Error, cresp.DiagnosticEvents}
	case proto.TXStatusPending, proto.TXStatusDuplicate, proto.TXStatusTryAgainLater:
		//noop.  A nil Err indicates success
	default:
		result.Err = errors.Errorf("Unrecognized stellar-core status response: %s", cresp.Status)
	}

	return
}

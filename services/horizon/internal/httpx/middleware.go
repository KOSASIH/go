package httpx

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go/services/horizon/internal/actions"
	horizonContext "github.com/stellar/go/services/horizon/internal/context"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/errors"
	"github.com/stellar/go/services/horizon/internal/hchi"
	"github.com/stellar/go/services/horizon/internal/ingest"
	"github.com/stellar/go/services/horizon/internal/ledger"
	"github.com/stellar/go/services/horizon/internal/render"
	hProblem "github.com/stellar/go/services/horizon/internal/render/problem"
	"github.com/stellar/go/support/db"
	supportErrors "github.com/stellar/go/support/errors"
	supportHttp "github.com/stellar/go/support/http"
	"github.com/stellar/go/support/log"
	"github.com/stellar/go/support/render/problem"
)

// requestCacheHeadersMiddleware adds caching headers to each response.
func requestCacheHeadersMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Before changing this read Stack Overflow answer about staled request
		// in older versions of Chrome:
		// https://stackoverflow.com/questions/27513994/chrome-stalls-when-making-multiple-requests-to-same-resource
		w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0")
		h.ServeHTTP(w, r)
	})
}

func contextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = hchi.WithChiRequestID(ctx)
		ctx = horizonContext.RequestContext(ctx, w, r)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

const (
	clientNameHeader    = "X-Client-Name"
	clientVersionHeader = "X-Client-Version"
	appNameHeader       = "X-App-Name"
	appVersionHeader    = "X-App-Version"
)

func newWrapResponseWriter(w http.ResponseWriter, r *http.Request) middleware.WrapResponseWriter {
	mw, ok := w.(middleware.WrapResponseWriter)
	if !ok {
		mw = middleware.NewWrapResponseWriter(w, r.ProtoMajor)
	}

	return mw
}

// loggerMiddleware logs http requests and resposnes to the logging subsytem of horizon.
func loggerMiddleware(serverMetrics *ServerMetrics) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			mw := newWrapResponseWriter(w, r)

			logger := log.WithField("req", middleware.GetReqID(ctx))
			ctx = log.Set(ctx, logger)

			// Checking `Accept` header from user request because if the streaming connection
			// is reset before sending the first event no Content-Type header is sent in a response.
			acceptHeader := r.Header.Get("Accept")
			streaming := strings.Contains(acceptHeader, render.MimeEventStream)
			route := supportHttp.GetChiRoutePattern(r)

			requestLabels := prometheus.Labels{
				"route":     route,
				"streaming": strconv.FormatBool(streaming),
				"method":    r.Method,
			}
			serverMetrics.RequestsInFlightGauge.With(requestLabels).Inc()
			defer serverMetrics.RequestsInFlightGauge.With(requestLabels).Dec()
			serverMetrics.RequestsReceivedCounter.With(requestLabels).Inc()

			then := time.Now()
			next.ServeHTTP(mw, r.WithContext(ctx))
			duration := time.Since(then)
			logEndOfRequest(ctx, r, route, serverMetrics.RequestDurationSummary, duration, mw, streaming)
		})
	}
}

// timeoutMiddleware ensures the request is terminated after the given timeout
func timeoutMiddleware(timeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			mw := newWrapResponseWriter(w, r)
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer func() {
				cancel()
				if ctx.Err() == context.DeadlineExceeded {
					if mw.Status() == 0 {
						// only write the header if it hasn't been written yet
						mw.WriteHeader(http.StatusGatewayTimeout)
					}
				}
			}()

			// txsub has a custom timeout
			if r.Method != http.MethodPost {
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(mw, r)
		}
		return http.HandlerFunc(fn)
	}
}

// getClientData gets client data (name or version) from header or GET parameter
// (useful when not possible to set headers, like in EventStream).
func getClientData(r *http.Request, headerName string) string {
	value := r.Header.Get(headerName)
	if value != "" {
		return value
	}

	value = r.URL.Query().Get(headerName)
	if value == "" {
		value = "undefined"
	}

	return value
}

func logEndOfRequest(ctx context.Context, r *http.Request, route string, requestDurationSummary *prometheus.SummaryVec, duration time.Duration, mw middleware.WrapResponseWriter, streaming bool) {

	referer := r.Referer()
	if referer == "" {
		referer = r.Header.Get("Origin")
	}
	if referer == "" {
		referer = "undefined"
	}

	log.Ctx(ctx).WithFields(log.F{
		"bytes":           mw.BytesWritten(),
		"client_name":     getClientData(r, clientNameHeader),
		"client_version":  getClientData(r, clientVersionHeader),
		"app_name":        getClientData(r, appNameHeader),
		"app_version":     getClientData(r, appVersionHeader),
		"duration":        duration.Seconds(),
		"x_forwarder_for": r.Header.Get("X-Forwarded-For"),
		"host":            r.Host,
		"ip":              remoteAddrIP(r),
		"ip_port":         r.RemoteAddr,
		"method":          r.Method,
		"path":            r.URL.String(),
		"route":           route,
		"status":          mw.Status(),
		"streaming":       streaming,
		"referer":         referer,
	}).Info("Finished request")

	requestDurationSummary.With(prometheus.Labels{
		"status":    strconv.FormatInt(int64(mw.Status()), 10),
		"route":     route,
		"streaming": strconv.FormatBool(streaming),
		"method":    r.Method,
	}).Observe(float64(duration.Seconds()))
}

// recoverMiddleware helps the server recover from panics. It ensures that
// no request can fully bring down the horizon server, and it also logs the
// panics to the logging subsystem.
func recoverMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		defer func() {
			if rec := recover(); rec != nil {
				err := errors.FromPanic(rec)
				errors.ReportToSentry(err, r)
				problem.Render(ctx, w, err)
			}
		}()

		h.ServeHTTP(w, r)
	})
}

// NewHistoryMiddleware adds session to the request context and ensures Horizon
// is not in a stale state, which is when the difference between latest core
// ledger and latest history ledger is higher than the given threshold
func NewHistoryMiddleware(ledgerState *ledger.State, staleThreshold int32, session db.SessionInterface, contextDBTimeout time.Duration) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if routePattern := supportHttp.GetChiRoutePattern(r); routePattern != "" {
				ctx = context.WithValue(ctx, &db.RouteContextKey, routePattern)
			}
			ctx = setContextDBTimeout(contextDBTimeout, ctx)
			if staleThreshold > 0 {
				ls := ledgerState.CurrentStatus()
				isStale := (ls.CoreLatest - ls.HistoryLatest) > int32(staleThreshold)
				if isStale {
					err := hProblem.StaleHistory
					err.Extras = map[string]interface{}{
						"history_latest_ledger": ls.HistoryLatest,
						"core_latest_ledger":    ls.CoreLatest,
					}
					problem.Render(ctx, w, err)
					return
				}
			}

			requestSession := session.Clone()
			h.ServeHTTP(w, r.WithContext(
				context.WithValue(
					ctx,
					&horizonContext.SessionContextKey,
					requestSession,
				),
			))
		})
	}
}

// StateMiddleware is a middleware which enables a state handler if the state
// has been initialized.
// Unless NoStateVerification is set, it ensures that the state (ledger entries)
// has been verified and is correct (Otherwise returns `500 Internal Server Error` to prevent
// returning invalid data to the user)
type StateMiddleware struct {
	HorizonSession      db.SessionInterface
	ClientQueryTimeout  time.Duration
	NoStateVerification bool
}

func ingestionStatus(ctx context.Context, q *history.Q) (uint32, bool, error) {
	version, err := q.GetIngestVersion(ctx)
	if err != nil {
		return 0, false, supportErrors.Wrap(
			err, "Error running GetIngestVersion",
		)
	}

	lastIngestedLedger, err := q.GetLastLedgerIngestNonBlocking(ctx)
	if err != nil {
		return 0, false, supportErrors.Wrap(
			err, "Error running GetLastLedgerIngestNonBlocking",
		)
	}

	var lastHistoryLedger uint32
	err = q.LatestLedger(ctx, &lastHistoryLedger)
	if err != nil {
		return 0, false, supportErrors.Wrap(err, "Error running LatestLedger")
	}

	ready := version == ingest.CurrentVersion &&
		lastIngestedLedger > 0 &&
		lastIngestedLedger == lastHistoryLedger

	return lastIngestedLedger, ready, nil
}

// WrapFunc executes the middleware on a given HTTP handler function
func (m *StateMiddleware) WrapFunc(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if routePattern := supportHttp.GetChiRoutePattern(r); routePattern != "" {
			ctx = context.WithValue(ctx, &db.RouteContextKey, routePattern)
		}
		ctx = setContextDBTimeout(m.ClientQueryTimeout, ctx)
		session := m.HorizonSession.Clone()
		q := &history.Q{session}
		sseRequest := render.Negotiate(r) == render.MimeEventStream

		// We want to start a repeatable read session to ensure that the data we
		// fetch from the db belong to the same ledger.
		// Otherwise, because the ingestion system is running concurrently with this request,
		// it is possible to have one read fetch data from ledger N and another read
		// fetch data from ledger N+1 .
		err := session.BeginTx(ctx, &sql.TxOptions{
			Isolation: sql.LevelRepeatableRead,
			ReadOnly:  true,
		})
		if err != nil {
			err = supportErrors.Wrap(err, "Error starting ingestion read transaction")
			problem.Render(ctx, w, err)
			return
		}
		defer session.Rollback()

		if !m.NoStateVerification {
			stateInvalid, invalidErr := q.GetExpStateInvalid(ctx)
			if invalidErr != nil {
				invalidErr = supportErrors.Wrap(invalidErr, "Error running GetExpStateInvalid")
				problem.Render(ctx, w, invalidErr)
				return
			}
			if stateInvalid {
				problem.Render(ctx, w, problem.ServerError)
				return
			}
		}

		lastIngestedLedger, ready, err := ingestionStatus(ctx, q)
		if err != nil {
			problem.Render(ctx, w, err)
			return
		}
		if !m.NoStateVerification && !ready {
			problem.Render(ctx, w, hProblem.StillIngesting)
			return
		}

		// for SSE requests we need to discard the repeatable read transaction
		// otherwise, the stream will not pick up updates occurring in future
		// ledgers
		if sseRequest {
			if err = session.Rollback(); err != nil {
				problem.Render(
					ctx,
					w,
					supportErrors.Wrap(
						err,
						"Could not roll back repeatable read session for SSE request",
					),
				)
				return
			}
		} else {
			actions.SetLastLedgerHeader(w, lastIngestedLedger)
		}

		h.ServeHTTP(w, r.WithContext(
			context.WithValue(ctx, &horizonContext.SessionContextKey, session),
		))
	}
}

func setContextDBTimeout(timeout time.Duration, ctx context.Context) context.Context {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	return context.WithValue(ctx, &db.DeadlineCtxKey, deadline)
}

// WrapFunc executes the middleware on a given HTTP handler function
func (m *StateMiddleware) Wrap(h http.Handler) http.Handler {
	return m.WrapFunc(h.ServeHTTP)
}

type ReplicaSyncCheckMiddleware struct {
	PrimaryHistoryQ *history.Q
	ReplicaHistoryQ *history.Q
	ServerMetrics   *ServerMetrics
}

// WrapFunc executes the middleware on a given HTTP handler function
func (m *ReplicaSyncCheckMiddleware) WrapFunc(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for attempt := 1; attempt <= 4; attempt++ {
			primaryIngestLedger, err := m.PrimaryHistoryQ.GetLastLedgerIngestNonBlocking(r.Context())
			if err != nil {
				problem.Render(r.Context(), w, err)
				return
			}

			replicaIngestLedger, err := m.ReplicaHistoryQ.GetLastLedgerIngestNonBlocking(r.Context())
			if err != nil {
				problem.Render(r.Context(), w, err)
				return
			}

			if replicaIngestLedger >= primaryIngestLedger {
				break
			}

			switch attempt {
			case 1:
				time.Sleep(20 * time.Millisecond)
			case 2:
				time.Sleep(40 * time.Millisecond)
			case 3:
				time.Sleep(80 * time.Millisecond)
			case 4:
				problem.Render(r.Context(), w, hProblem.StaleHistory)
				m.ServerMetrics.ReplicaLagErrorsCounter.Inc()
				return
			}
		}

		h.ServeHTTP(w, r)
	}
}

func (m *ReplicaSyncCheckMiddleware) Wrap(h http.Handler) http.Handler {
	return m.WrapFunc(h.ServeHTTP)
}

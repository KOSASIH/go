package horizon

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go/clients/stellarcore"
	"github.com/stellar/go/services/horizon/internal/corestate"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/httpx"
	"github.com/stellar/go/services/horizon/internal/ingest"
	"github.com/stellar/go/services/horizon/internal/ledger"
	"github.com/stellar/go/services/horizon/internal/operationfeestats"
	"github.com/stellar/go/services/horizon/internal/paths"
	"github.com/stellar/go/services/horizon/internal/txsub"
	"github.com/stellar/go/support/app"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/log"
	"github.com/stellar/go/support/logmetrics"
)

// App represents the root of the state of a horizon instance.
type App struct {
	done            chan struct{}
	doneOnce        sync.Once
	config          Config
	webServer       *httpx.Server
	historyQ        *history.Q
	primaryHistoryQ *history.Q
	ctx             context.Context
	cancel          func()
	horizonVersion  string
	coreState       corestate.Store
	orderBookStream *ingest.OrderBookStream
	submitter       *txsub.System
	paths           paths.Finder
	ingester        ingest.System
	ticks           *time.Ticker
	ledgerState     *ledger.State

	// metrics
	prometheusRegistry *prometheus.Registry
	buildInfoGauge     *prometheus.GaugeVec
	ingestingGauge     prometheus.Gauge
}

func (a *App) GetCoreState() corestate.State {
	return a.coreState.Get()
}

const tickerMaxFrequency = 1 * time.Second
const tickerMaxDuration = 5 * time.Second

// NewApp constructs an new App instance from the provided config.
func NewApp(config Config) (*App, error) {
	a := &App{
		config:         config,
		ledgerState:    &ledger.State{},
		horizonVersion: app.Version(),
		ticks:          time.NewTicker(tickerMaxFrequency),
		done:           make(chan struct{}),
	}

	if err := a.init(); err != nil {
		return nil, err
	}
	return a, nil
}

// Serve starts the horizon web server, binding it to a socket, setting up
// the shutdown signals.
func (a *App) Serve() error {

	log.Infof("Starting horizon on :%d (ingest: %v)", a.config.Port, a.config.Ingest)

	if a.config.AdminPort != 0 {
		log.Infof("Starting internal server on :%d", a.config.AdminPort)
	}

	go a.run()
	if !a.config.DisablePathFinding {
		go a.orderBookStream.Run(a.ctx)
	}

	// WaitGroup for all go routines. Makes sure that DB is closed when
	// all services gracefully shutdown.
	var wg sync.WaitGroup

	if a.ingester != nil {
		wg.Add(1)
		go func() {
			a.ingester.Run()
			wg.Done()
		}()
	}

	// configure shutdown signal handler
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-signalChan:
			a.Close()
		case <-a.done:
			return
		}
	}()

	wg.Add(1)
	go func() {
		a.waitForDone()
		wg.Done()
	}()

	log.Infof("Starting to serve requests")
	err := a.webServer.Serve()
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	wg.Wait()
	a.CloseDB()

	log.Info("stopped")
	return nil
}

// Close cancels the app. It does not close DB connections - use App.CloseDB().
func (a *App) Close() {
	a.doneOnce.Do(func() {
		close(a.done)
	})
}

func (a *App) waitForDone() {
	<-a.done
	a.Shutdown()
}

func (a *App) Shutdown() {
	if a.webServer != nil {
		webShutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		a.webServer.Shutdown(webShutdownCtx)
	}
	a.cancel()
	if a.ingester != nil {
		a.ingester.Shutdown()
	}
	a.ticks.Stop()
}

// CloseDB closes DB connections. When using during web server shut down make
// sure all requests are first properly finished to avoid "sql: database is
// closed" errors.
func (a *App) CloseDB() {
	a.historyQ.SessionInterface.Close()
}

// HistoryQ returns a helper object for performing sql queries against the
// history portion of horizon's database.
func (a *App) HistoryQ() *history.Q {
	return a.historyQ
}

// HorizonSession returns a new session that loads data from the horizon
// database.
func (a *App) HorizonSession() db.SessionInterface {
	return a.historyQ.SessionInterface.Clone()
}

func (a *App) Config() Config {
	return a.config
}

// Paths returns the paths.Finder instance used by horizon
func (a *App) Paths() paths.Finder {
	return a.paths
}

func isLocalAddress(url string, port uint) bool {
	localHostURL := fmt.Sprintf("http://localhost:%d", port)
	localIPURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	return strings.HasPrefix(url, localHostURL) || strings.HasPrefix(url, localIPURL)
}

// UpdateCoreLedgerState triggers a refresh of Stellar-Core ledger state.
// This is done separately from Horizon ledger state update to prevent issues
// in case Stellar-Core query timeout.
func (a *App) UpdateCoreLedgerState(ctx context.Context) {
	var next ledger.CoreStatus

	if a.config.StellarCoreURL == "" {
		return
	}
	// #4446 If the ingestion state machine is in the build state, the query can time out
	// because the captive-core buffer may be full. In this case, skip the check.
	if a.config.CaptiveCoreToml != nil &&
		isLocalAddress(a.config.StellarCoreURL, a.config.CaptiveCoreToml.HTTPPort) &&
		a.ingester != nil && a.ingester.GetCurrentState() == ingest.Build {
		return
	}

	logErr := func(err error, msg string) {
		log.WithStack(err).WithField("err", err.Error()).Error(msg)
	}

	coreClient := &stellarcore.Client{
		HTTP: http.DefaultClient,
		URL:  a.config.StellarCoreURL,
	}

	coreInfo, err := coreClient.Info(ctx)
	if err != nil {
		logErr(err, "failed to load the stellar-core info")
		return
	}
	next.CoreLatest = int32(coreInfo.Info.Ledger.Num)
	a.ledgerState.SetCoreStatus(next)
}

// UpdateHorizonLedgerState triggers a refresh of Horizon ledger state.
// This is done separately from Core ledger state update to prevent issues
// in case Stellar-Core query timeout.
func (a *App) UpdateHorizonLedgerState(ctx context.Context) {
	var next ledger.HorizonStatus

	logErr := func(err error, msg string) {
		log.WithStack(err).WithField("err", err.Error()).Error(msg)
	}

	var err error
	next.HistoryLatest, next.HistoryLatestClosedAt, err =
		a.HistoryQ().LatestLedgerSequenceClosedAt(ctx)
	if err != nil {
		logErr(err, "failed to load the latest known ledger state from history DB")
		return
	}

	err = a.HistoryQ().ElderLedger(ctx, &next.HistoryElder)
	if err != nil {
		logErr(err, "failed to load the oldest known ledger state from history DB")
		return
	}

	next.ExpHistoryLatest, err = a.HistoryQ().GetLastLedgerIngestNonBlocking(ctx)
	if err != nil {
		logErr(err, "failed to load the oldest known exp ledger state from history DB")
		return
	}

	a.ledgerState.SetHorizonStatus(next)
}

// UpdateFeeStatsState triggers a refresh of several operation fee metrics.
func (a *App) UpdateFeeStatsState(ctx context.Context) {
	var (
		next          operationfeestats.State
		latest        history.LatestLedger
		feeStats      history.FeeStats
		capacityStats history.LedgerCapacityUsageStats
	)

	logErr := func(err error, msg string) {
		// If DB is empty ignore the error
		if errors.Cause(err) == sql.ErrNoRows {
			return
		}

		log.WithStack(err).WithField("err", err.Error()).Error(msg)
	}

	cur, ok := operationfeestats.CurrentState()

	err := a.HistoryQ().LatestLedgerBaseFeeAndSequence(ctx, &latest)
	if err != nil {
		logErr(err, "failed to load the latest known ledger's base fee and sequence number")
		return
	}

	// finish early if no new ledgers
	if ok && cur.LastLedger == uint32(latest.Sequence) {
		return
	}

	next.LastBaseFee = int64(latest.BaseFee)
	next.LastLedger = uint32(latest.Sequence)

	err = a.HistoryQ().FeeStats(ctx, latest.Sequence, &feeStats)
	if err != nil {
		logErr(err, "failed to load operation fee stats")
		return
	}

	err = a.HistoryQ().LedgerCapacityUsageStats(ctx, latest.Sequence, &capacityStats)
	if err != nil {
		logErr(err, "failed to load ledger capacity usage stats")
		return
	}

	next.LedgerCapacityUsage = capacityStats.CapacityUsage.String

	// if no transactions in last 5 ledgers, return
	// latest ledger's base fee for all
	if !feeStats.MaxFeeMode.Valid && !feeStats.MaxFeeMin.Valid {
		// MaxFee
		next.MaxFeeMax = next.LastBaseFee
		next.MaxFeeMin = next.LastBaseFee
		next.MaxFeeMode = next.LastBaseFee
		next.MaxFeeP10 = next.LastBaseFee
		next.MaxFeeP20 = next.LastBaseFee
		next.MaxFeeP30 = next.LastBaseFee
		next.MaxFeeP40 = next.LastBaseFee
		next.MaxFeeP50 = next.LastBaseFee
		next.MaxFeeP60 = next.LastBaseFee
		next.MaxFeeP70 = next.LastBaseFee
		next.MaxFeeP80 = next.LastBaseFee
		next.MaxFeeP90 = next.LastBaseFee
		next.MaxFeeP95 = next.LastBaseFee
		next.MaxFeeP99 = next.LastBaseFee

		// FeeCharged
		next.FeeChargedMax = next.LastBaseFee
		next.FeeChargedMin = next.LastBaseFee
		next.FeeChargedMode = next.LastBaseFee
		next.FeeChargedP10 = next.LastBaseFee
		next.FeeChargedP20 = next.LastBaseFee
		next.FeeChargedP30 = next.LastBaseFee
		next.FeeChargedP40 = next.LastBaseFee
		next.FeeChargedP50 = next.LastBaseFee
		next.FeeChargedP60 = next.LastBaseFee
		next.FeeChargedP70 = next.LastBaseFee
		next.FeeChargedP80 = next.LastBaseFee
		next.FeeChargedP90 = next.LastBaseFee
		next.FeeChargedP95 = next.LastBaseFee
		next.FeeChargedP99 = next.LastBaseFee

	} else {
		// MaxFee
		next.MaxFeeMax = feeStats.MaxFeeMax.Int64
		next.MaxFeeMin = feeStats.MaxFeeMin.Int64
		next.MaxFeeMode = feeStats.MaxFeeMode.Int64
		next.MaxFeeP10 = feeStats.MaxFeeP10.Int64
		next.MaxFeeP20 = feeStats.MaxFeeP20.Int64
		next.MaxFeeP30 = feeStats.MaxFeeP30.Int64
		next.MaxFeeP40 = feeStats.MaxFeeP40.Int64
		next.MaxFeeP50 = feeStats.MaxFeeP50.Int64
		next.MaxFeeP60 = feeStats.MaxFeeP60.Int64
		next.MaxFeeP70 = feeStats.MaxFeeP70.Int64
		next.MaxFeeP80 = feeStats.MaxFeeP80.Int64
		next.MaxFeeP90 = feeStats.MaxFeeP90.Int64
		next.MaxFeeP95 = feeStats.MaxFeeP95.Int64
		next.MaxFeeP99 = feeStats.MaxFeeP99.Int64

		// FeeCharged
		next.FeeChargedMax = feeStats.FeeChargedMax.Int64
		next.FeeChargedMin = feeStats.FeeChargedMin.Int64
		next.FeeChargedMode = feeStats.FeeChargedMode.Int64
		next.FeeChargedP10 = feeStats.FeeChargedP10.Int64
		next.FeeChargedP20 = feeStats.FeeChargedP20.Int64
		next.FeeChargedP30 = feeStats.FeeChargedP30.Int64
		next.FeeChargedP40 = feeStats.FeeChargedP40.Int64
		next.FeeChargedP50 = feeStats.FeeChargedP50.Int64
		next.FeeChargedP60 = feeStats.FeeChargedP60.Int64
		next.FeeChargedP70 = feeStats.FeeChargedP70.Int64
		next.FeeChargedP80 = feeStats.FeeChargedP80.Int64
		next.FeeChargedP90 = feeStats.FeeChargedP90.Int64
		next.FeeChargedP95 = feeStats.FeeChargedP95.Int64
		next.FeeChargedP99 = feeStats.FeeChargedP99.Int64
	}

	operationfeestats.SetState(next)
}

// UpdateStellarCoreInfo updates the value of CoreVersion,
// CurrentProtocolVersion, and CoreSupportedProtocolVersion from the Stellar
// core API.
//
// Warning: This method should only return an error if it is fatal. See usage
// in `App.Tick`
func (a *App) UpdateStellarCoreInfo(ctx context.Context) error {
	if a.config.StellarCoreURL == "" {
		return nil
	}

	// #4446 If the ingestion state machine is in the build state, the query can time out
	// because the captive-core buffer may be full. In this case, skip the check.
	if a.config.CaptiveCoreToml != nil &&
		isLocalAddress(a.config.StellarCoreURL, a.config.CaptiveCoreToml.HTTPPort) &&
		a.ingester != nil && a.ingester.GetCurrentState() == ingest.Build {
		return nil
	}

	core := &stellarcore.Client{
		URL: a.config.StellarCoreURL,
	}

	resp, err := core.Info(ctx)
	if err != nil {
		log.Warnf("could not load stellar-core info: %s", err)
		return nil
	}

	// Check if NetworkPassphrase is different, if so exit Horizon as it can break the
	// state of the application.
	if resp.Info.Network != a.config.NetworkPassphrase {
		return fmt.Errorf(
			"Network passphrase of stellar-core (%s) does not match Horizon configuration (%s). Exiting...",
			resp.Info.Network,
			a.config.NetworkPassphrase,
		)
	}

	a.coreState.Set(resp)
	return nil
}

// Tick triggers horizon to update all of it's background processes such as
// transaction submission, metrics, ingestion and reaping.
func (a *App) Tick(ctx context.Context) error {
	var wg sync.WaitGroup
	log.Debug("ticking app")

	// update ledger state, operation fee state, and stellar-core info in parallel
	wg.Add(4)
	var err error
	go func() { a.UpdateCoreLedgerState(ctx); wg.Done() }()
	go func() { a.UpdateHorizonLedgerState(ctx); wg.Done() }()
	go func() { a.UpdateFeeStatsState(ctx); wg.Done() }()
	go func() { err = a.UpdateStellarCoreInfo(ctx); wg.Done() }()
	wg.Wait()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() { a.submitter.Tick(ctx); wg.Done() }()
	wg.Wait()

	log.Debug("finished ticking app")
	return ctx.Err()
}

// Init initializes app, using the config to populate db connections and
// whatnot.
func (a *App) init() error {
	// app-context
	a.ctx, a.cancel = context.WithCancel(context.Background())

	// log
	log.DefaultLogger.SetLevel(a.config.LogLevel)
	logMetrics := logmetrics.New("horizon")
	log.DefaultLogger.AddHook(logMetrics)

	// sentry
	initSentry(a)

	// loggly
	initLogglyLog(a)

	// metrics and log.metrics
	a.prometheusRegistry = prometheus.NewRegistry()
	for _, counter := range logMetrics {
		a.prometheusRegistry.MustRegister(counter)
	}

	// stellarCoreInfo
	a.UpdateStellarCoreInfo(a.ctx)

	// horizon-db and core-db
	mustInitHorizonDB(a)

	if a.config.Ingest {
		// ingester
		initIngester(a)
	}
	initPathFinder(a)

	// txsub
	initSubmissionSystem(a)

	// go metrics
	initGoMetrics(a)

	// process metrics
	initProcessMetrics(a)

	// db-metrics
	initDbMetrics(a)

	// ingest.metrics
	initIngestMetrics(a)

	// txsub.metrics
	initTxSubMetrics(a)

	routerConfig := httpx.RouterConfig{
		DBSession:               a.historyQ.SessionInterface,
		TxSubmitter:             a.submitter,
		RateQuota:               a.config.RateQuota,
		BehindCloudflare:        a.config.BehindCloudflare,
		BehindAWSLoadBalancer:   a.config.BehindAWSLoadBalancer,
		SSEUpdateFrequency:      a.config.SSEUpdateFrequency,
		StaleThreshold:          a.config.StaleThreshold,
		ConnectionTimeout:       a.config.ConnectionTimeout,
		ClientQueryTimeout:      a.config.ClientQueryTimeout,
		MaxConcurrentRequests:   a.config.MaxConcurrentRequests,
		MaxHTTPRequestSize:      a.config.MaxHTTPRequestSize,
		NetworkPassphrase:       a.config.NetworkPassphrase,
		MaxPathLength:           a.config.MaxPathLength,
		MaxAssetsPerPathRequest: a.config.MaxAssetsPerPathRequest,
		PathFinder:              a.paths,
		PrometheusRegistry:      a.prometheusRegistry,
		CoreGetter:              a,
		HorizonVersion:          a.horizonVersion,
		FriendbotURL:            a.config.FriendbotURL,
		DisableTxSub:            a.config.DisableTxSub,
		StellarCoreURL:          a.config.StellarCoreURL,
		HealthCheck: healthCheck{
			session: a.historyQ.SessionInterface,
			ctx:     a.ctx,
			core: &stellarcore.Client{
				HTTP: &http.Client{Timeout: infoRequestTimeout},
				URL:  a.config.StellarCoreURL,
			},
			cache: newHealthCache(healthCacheTTL),
		},
		SkipTxMeta: a.config.SkipTxmeta,
	}

	if a.primaryHistoryQ != nil {
		routerConfig.PrimaryDBSession = a.primaryHistoryQ.SessionInterface
	}

	var err error
	config := httpx.ServerConfig{
		Port:      uint16(a.config.Port),
		AdminPort: uint16(a.config.AdminPort),
	}
	if a.config.TLSCert != "" && a.config.TLSKey != "" {
		config.TLSConfig = &httpx.TLSConfig{
			CertPath: a.config.TLSCert,
			KeyPath:  a.config.TLSKey,
		}
	}
	a.webServer, err = httpx.NewServer(config, routerConfig, a.ledgerState)
	if err != nil {
		return err
	}

	// web.metrics
	initWebMetrics(a)

	return nil
}

// run is the function that runs in the background that triggers Tick each
// second
func (a *App) run() {
	for {
		select {
		case <-a.ticks.C:
			ctx, cancel := context.WithTimeout(a.ctx, tickerMaxDuration)
			err := a.Tick(ctx)
			if err != nil {
				log.Warnf("error ticking app: %s", err)
			}
			cancel() // Release timer
		case <-a.ctx.Done():
			log.Info("finished background ticker")
			return
		}
	}
}

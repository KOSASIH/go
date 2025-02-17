package horizon

import (
	"net/url"
	"time"

	"github.com/stellar/go/ingest/ledgerbackend"

	"github.com/sirupsen/logrus"
	"github.com/stellar/throttled"
)

// Config is the configuration for horizon.  It gets populated by the
// app's main function and is provided to NewApp.
type Config struct {
	DatabaseURL        string
	RoDatabaseURL      string
	HistoryArchiveURLs []string
	Port               uint
	AdminPort          uint

	CaptiveCoreBinaryPath       string
	CaptiveCoreConfigPath       string
	CaptiveCoreTomlParams       ledgerbackend.CaptiveCoreTomlParams
	CaptiveCoreToml             *ledgerbackend.CaptiveCoreToml
	CaptiveCoreStoragePath      string
	CaptiveCoreReuseStoragePath bool
	CaptiveCoreConfigUseDB      bool
	HistoryArchiveCaching       bool

	StellarCoreURL string

	// MaxDBConnections has a priority over all 4 values below.
	MaxDBConnections            int
	HorizonDBMaxOpenConnections int
	HorizonDBMaxIdleConnections int

	SSEUpdateFrequency time.Duration
	ConnectionTimeout  time.Duration
	ClientQueryTimeout time.Duration
	// MaxHTTPRequestSize is the maximum allowed request payload size
	MaxHTTPRequestSize    uint
	RateQuota             *throttled.RateQuota
	MaxConcurrentRequests uint
	FriendbotURL          *url.URL
	LogLevel              logrus.Level
	LogFile               string

	// MaxPathLength is the maximum length of the path returned by `/paths` endpoint.
	MaxPathLength uint
	// MaxAssetsPerPathRequest is the maximum number of assets considered for `/paths/strict-send` and `/paths/strict-receive`
	MaxAssetsPerPathRequest int
	// DisablePoolPathFinding configures horizon to run path finding without including liquidity pools
	// in the path finding search.
	DisablePoolPathFinding bool
	// DisablePathFinding configures horizon without the path finding endpoint.
	DisablePathFinding bool
	// MaxPathFindingRequests is the maximum number of path finding requests horizon will allow
	// in a 1-second period. A value of 0 disables the limit.
	MaxPathFindingRequests uint

	NetworkPassphrase string
	SentryDSN         string
	LogglyToken       string
	LogglyTag         string
	// TLSCert is a path to a certificate file to use for horizon's TLS config
	TLSCert string
	// TLSKey is the path to a private key file to use for horizon's TLS config
	TLSKey string
	// Ingest toggles whether this horizon instance should run the data ingestion subsystem.
	Ingest bool
	// HistoryRetentionCount represents the minimum number of ledgers worth of
	// history data to retain in the horizon database. For the purposes of
	// determining a "retention duration", each ledger roughly corresponds to 10
	// seconds of real time.
	HistoryRetentionCount uint
	// HistoryRetentionReapCount is the number of ledgers worth of history data
	// to remove per second from the Horizon database. It is intended to allow
	// control over the amount of CPU and database load caused by reaping,
	// especially if enabling reaping for the first time or in times of
	// increased ledger load.
	HistoryRetentionReapCount uint
	// ReapFrequency configures how often (in units of ledgers) history is reaped.
	// If ReapFrequency is set to 1 history is reaped after ingesting every ledger.
	// If ReapFrequency is set to 2 history is reaped after ingesting every two ledgers.
	// etc...
	ReapFrequency uint
	// ReapLookupTables enables the reaping of history lookup tables
	ReapLookupTables bool
	// StaleThreshold represents the number of ledgers a history database may be
	// out-of-date by before horizon begins to respond with an error to history
	// requests.
	StaleThreshold uint
	// IngestDisableStateVerification disables state verification
	// `System.verifyState()` when set to `true`.
	IngestDisableStateVerification bool
	// IngestStateVerificationCheckpointFrequency configures how often state verification is performed.
	// If IngestStateVerificationCheckpointFrequency is set to 1 state verification is run on every checkpoint,
	// If IngestStateVerificationCheckpointFrequency is set to 2 state verification is run on every second checkpoint,
	// etc...
	IngestStateVerificationCheckpointFrequency uint
	// IngestStateVerificationTimeout configures a timeout on the state verification routine.
	// If IngestStateVerificationTimeout is set to 0 the timeout is disabled.
	IngestStateVerificationTimeout time.Duration
	// IngestEnableExtendedLogLedgerStats enables extended ledger stats in
	// logging.
	IngestEnableExtendedLogLedgerStats bool
	// ApplyMigrations will apply pending migrations to the horizon database
	// before starting the horizon service
	ApplyMigrations bool
	// CheckpointFrequency establishes how many ledgers exist between checkpoints
	CheckpointFrequency uint32
	// BehindCloudflare determines if Horizon instance is behind Cloudflare. In
	// such case http.Request.RemoteAddr will be replaced with Cloudflare header.
	BehindCloudflare bool
	// BehindAWSLoadBalancer determines if Horizon instance is behind AWS load
	// balances like ELB or ALB. In such case http.Request.RemoteAddr will be
	// replaced with the last IP in X-Forwarded-For header.
	BehindAWSLoadBalancer bool
	// RoundingSlippageFilter excludes trades from /trade_aggregations with rounding slippage >x bps
	RoundingSlippageFilter int
	// Stellar network: 'testnet' or 'pubnet'
	Network string
	// DisableTxSub disables transaction submission functionality for Horizon.
	DisableTxSub bool
	// SkipTxmeta, when enabled, will not store meta xdr in history transaction table
	SkipTxmeta bool
}

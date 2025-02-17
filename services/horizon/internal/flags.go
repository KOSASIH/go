package horizon

import (
	_ "embed"
	"fmt"
	"go/types"
	stdLog "log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/stellar/throttled"

	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/network"
	"github.com/stellar/go/services/horizon/internal/db2/schema"
	apkg "github.com/stellar/go/support/app"
	support "github.com/stellar/go/support/config"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/log"
)

const (
	// DatabaseURLFlagName is the command line flag for configuring the Horizon postgres URL
	DatabaseURLFlagName = "db-url"
	// IngestFlagName is the command line flag for enabling ingestion on the Horizon instance
	IngestFlagName = "ingest"
	// StellarCoreDBURLFlagName is the command line flag for configuring the postgres Stellar Core URL
	StellarCoreDBURLFlagName = "stellar-core-db-url"
	// StellarCoreURLFlagName is the command line flag for configuring the URL fore Stellar Core HTTP endpoint
	StellarCoreURLFlagName = "stellar-core-url"
	// StellarCoreBinaryPathName is the command line flag for configuring the path to the stellar core binary
	StellarCoreBinaryPathName = "stellar-core-binary-path"
	// captiveCoreConfigAppendPathName is the command line flag for configuring the path to the captive core additional configuration
	// Note captiveCoreConfigAppendPathName is deprecated in favor of CaptiveCoreConfigPathName
	captiveCoreConfigAppendPathName = "captive-core-config-append-path"
	// CaptiveCoreConfigPathName is the command line flag for configuring the path to the captive core configuration file
	CaptiveCoreConfigPathName = "captive-core-config-path"
	// CaptiveCoreConfigUseDB is the command line flag for enabling captive core runtime to use an external db url
	// connection rather than RAM for ledger states
	CaptiveCoreConfigUseDB = "captive-core-use-db"
	// CaptiveCoreHTTPPortFlagName is the commandline flag for specifying captive core HTTP port
	CaptiveCoreHTTPPortFlagName = "captive-core-http-port"
	// EnableCaptiveCoreIngestionFlagName is the commandline flag for enabling captive core ingestion
	EnableCaptiveCoreIngestionFlagName = "enable-captive-core-ingestion"
	// NetworkPassphraseFlagName is the command line flag for specifying the network passphrase
	NetworkPassphraseFlagName = "network-passphrase"
	// HistoryArchiveURLsFlagName is the command line flag for specifying the history archive URLs
	HistoryArchiveURLsFlagName = "history-archive-urls"
	// HistoryArchiveCaching is the flag for controlling whether or not there's
	// an on-disk cache for history archive downloads
	HistoryArchiveCachingFlagName = "history-archive-caching"
	// NetworkFlagName is the command line flag for specifying the "network"
	NetworkFlagName = "network"
	// EnableIngestionFilteringFlagName is the command line flag for enabling the experimental ingestion filtering feature (now enabled by default)
	EnableIngestionFilteringFlagName = "exp-enable-ingestion-filtering"
	// DisableTxSubFlagName is the command line flag for disabling transaction submission feature of Horizon
	DisableTxSubFlagName = "disable-tx-sub"
	// SkipTxmeta is the command line flag for disabling persistence of tx meta in history transaction table
	SkipTxmeta = "skip-txmeta"

	// StellarPubnet is a constant representing the Stellar public network
	StellarPubnet = "pubnet"
	// StellarTestnet is a constant representing the Stellar test network
	StellarTestnet = "testnet"

	defaultMaxConcurrentRequests = uint(1000)
	defaultMaxHTTPRequestSize    = uint(200 * 1024)
	clientQueryTimeoutNotSet     = -1
)

var (
	IngestCmd        = "ingest"
	RecordMetricsCmd = "record-metrics"
	DbCmd            = "db"
	ServeCmd         = "serve"
	HorizonCmd       = "horizon"

	DbFillGapsCmd             = "fill-gaps"
	DbReingestCmd             = "reingest"
	IngestTriggerStateRebuild = "trigger-state-rebuild"
	IngestBuildStateCmd       = "build-state"
	IngestStressTestCmd       = "stress-test"
	IngestVerifyRangeCmd      = "verify-range"

	ApiServerCommands = []string{HorizonCmd, ServeCmd}
	IngestionCommands = append(ApiServerCommands,
		IngestBuildStateCmd,
		IngestStressTestCmd,
		IngestVerifyRangeCmd,
		DbFillGapsCmd,
		DbReingestCmd)
	DatabaseBoundCommands = append(ApiServerCommands, DbCmd, IngestCmd)
)

// validateBothOrNeither ensures that both options are provided, if either is provided.
func validateBothOrNeither(option1, option2 string) error {
	arg1, arg2 := viper.GetString(option1), viper.GetString(option2)
	if arg1 != "" && arg2 == "" {
		return fmt.Errorf("Invalid config: %s = %s, but corresponding option %s is not configured", option1, arg1, option2)
	}
	if arg1 == "" && arg2 != "" {
		return fmt.Errorf("Invalid config: %s = %s, but corresponding option %s is not configured", option2, arg2, option1)
	}
	return nil
}

func applyMigrations(config Config) error {
	dbConn, err := db.Open("postgres", config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("could not connect to horizon db: %v", err)
	}
	defer dbConn.Close()

	numMigrations, err := schema.Migrate(dbConn.DB.DB, schema.MigrateUp, 0)
	if err != nil {
		return fmt.Errorf("could not apply migrations: %v", err)
	}
	if numMigrations > 0 {
		stdLog.Printf("successfully applied %v horizon migrations\n", numMigrations)
	}
	return nil
}

// checkMigrations looks for necessary database migrations and fails with a descriptive error if migrations are needed.
func checkMigrations(config Config) error {
	migrationsToApplyUp := schema.GetMigrationsUp(config.DatabaseURL)
	if len(migrationsToApplyUp) > 0 {
		return fmt.Errorf(
			`There are %v migrations to apply in the "up" direction.
The necessary migrations are: %v
A database migration is required to run this version (%v) of Horizon. Run "horizon db migrate up" to update your DB. Consult the Changelog (https://github.com/stellar/go/blob/master/services/horizon/CHANGELOG.md) for more information.`,
			len(migrationsToApplyUp),
			migrationsToApplyUp,
			apkg.Version(),
		)
	}

	nMigrationsDown := schema.GetNumMigrationsDown(config.DatabaseURL)
	if nMigrationsDown > 0 {
		return fmt.Errorf(
			`A database migration DOWN to an earlier version of the schema is required to run this version (%v) of Horizon. Consult the Changelog (https://github.com/stellar/go/blob/master/services/horizon/CHANGELOG.md) for more information.
In order to migrate the database DOWN, using the HIGHEST version number of Horizon you have installed (not this binary), run "horizon db migrate down %v".`,
			apkg.Version(),
			nMigrationsDown,
		)
	}
	return nil
}

// Flags returns a Config instance and a list of commandline flags which modify the Config instance
func Flags() (*Config, support.ConfigOptions) {
	config := &Config{}

	// flags defines the complete flag configuration for horizon.
	// Add a new entry here to connect a new field in the horizon.Config struct
	var flags = support.ConfigOptions{
		&support.ConfigOption{
			Name:           DatabaseURLFlagName,
			EnvVar:         "DATABASE_URL",
			ConfigKey:      &config.DatabaseURL,
			OptType:        types.String,
			Required:       true,
			Usage:          "horizon postgres database to connect with",
			UsedInCommands: DatabaseBoundCommands,
		},
		&support.ConfigOption{
			Name:           "ro-database-url",
			ConfigKey:      &config.RoDatabaseURL,
			OptType:        types.String,
			Required:       false,
			Usage:          "horizon postgres read-replica to connect with, when set it will return stale history error when replica is behind primary",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           StellarCoreBinaryPathName,
			OptType:        types.String,
			FlagDefault:    "",
			Required:       false,
			Usage:          "path to stellar core binary, look for the stellar-core binary in $PATH by default.",
			ConfigKey:      &config.CaptiveCoreBinaryPath,
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           DisableTxSubFlagName,
			OptType:        types.Bool,
			FlagDefault:    false,
			Required:       false,
			Usage:          "disables the transaction submission functionality of Horizon.",
			ConfigKey:      &config.DisableTxSub,
			Hidden:         false,
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:        captiveCoreConfigAppendPathName,
			OptType:     types.String,
			FlagDefault: "",
			Required:    false,
			Usage:       "DEPRECATED in favor of " + CaptiveCoreConfigPathName,
			CustomSetValue: func(opt *support.ConfigOption) error {
				if val := viper.GetString(opt.Name); val != "" {
					if viper.GetString(CaptiveCoreConfigPathName) != "" {
						stdLog.Printf(
							"both --%s and --%s are set. %s is deprecated so %s will be used instead",
							captiveCoreConfigAppendPathName,
							CaptiveCoreConfigPathName,
							captiveCoreConfigAppendPathName,
							CaptiveCoreConfigPathName,
						)
					} else {
						config.CaptiveCoreConfigPath = val
					}
				}
				return nil
			},
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:        CaptiveCoreConfigPathName,
			OptType:     types.String,
			FlagDefault: "",
			Required:    false,
			Usage:       "path to the configuration file used by captive core. It must, at least, include enough details to define a quorum set. Any fields in the configuration file which are not supported by captive core will be rejected.",
			CustomSetValue: func(opt *support.ConfigOption) error {
				if val := viper.GetString(opt.Name); val != "" {
					config.CaptiveCoreConfigPath = val
					config.CaptiveCoreTomlParams.Strict = true
				}
				return nil
			},
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:        CaptiveCoreConfigUseDB,
			OptType:     types.Bool,
			FlagDefault: true,
			Required:    false,
			Usage:       `when enabled, Horizon ingestion will instruct the captive core invocation to use an external db url for ledger states rather than in memory(RAM). Will result in several GB of space shifting out of RAM and to the external db persistence. The external db url is determined by the presence of DATABASE parameter in the captive-core-config-path or if absent, the db will default to sqlite and the db file will be stored at location derived from captive-core-storage-path parameter.`,
			CustomSetValue: func(opt *support.ConfigOption) error {
				if val := viper.GetBool(opt.Name); val {
					stdLog.Printf("The usage of the flag --captive-core-use-db has been deprecated. " +
						"Setting it to false to achieve in-memory functionality on captive core will be removed in " +
						"future releases. We recommend removing usage of this flag now in preparation.")
					config.CaptiveCoreConfigUseDB = val
					config.CaptiveCoreTomlParams.UseDB = val
				}
				return nil
			},
			ConfigKey:      &config.CaptiveCoreConfigUseDB,
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:        EnableCaptiveCoreIngestionFlagName,
			OptType:     types.String,
			FlagDefault: "",
			Required:    false,
			Hidden:      true,
			CustomSetValue: func(opt *support.ConfigOption) error {
				if val := viper.GetString(opt.Name); val != "" {
					stdLog.Printf(
						"DEPRECATED - The usage of the flag --enable-captive-core-ingestion has been deprecated. " +
							"Horizon now uses Captive-Core ingestion by default and this flag will soon be removed in " +
							"the future.",
					)
				}
				return nil
			},
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:        EnableIngestionFilteringFlagName,
			OptType:     types.String,
			FlagDefault: "",
			Required:    false,
			CustomSetValue: func(opt *support.ConfigOption) error {
				// ingestion filtering is always enabled, it has no rules by default.
				if val := viper.GetString(opt.Name); val != "" {
					stdLog.Printf(
						"DEPRECATED - No ingestion filter rules are defined by default, which equates to " +
							"no filtering of historical data. If you have never added filter rules to this deployment, then no further action is needed. " +
							"If you have defined ingestion filter rules previously but disabled filtering overall by setting the env variable EXP_ENABLE_INGESTION_FILTERING=false, " +
							"then you should now delete the filter rules using the Horizon Admin API to achieve the same no-filtering result. Remove usage of this variable in all cases.",
					)
				}
				return nil
			},
			Hidden: true,
		},
		&support.ConfigOption{
			Name:           "captive-core-http-port",
			OptType:        types.Uint,
			CustomSetValue: support.SetOptionalUint,
			Required:       false,
			FlagDefault:    uint(0),
			Usage:          "HTTP port for Captive Core to listen on (0 disables the HTTP server)",
			ConfigKey:      &config.CaptiveCoreTomlParams.HTTPPort,
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:    "captive-core-storage-path",
			OptType: types.String,
			CustomSetValue: func(opt *support.ConfigOption) error {
				existingValue := viper.GetString(opt.Name)
				if existingValue == "" || existingValue == "." {
					cwd, err := os.Getwd()
					if err != nil {
						return fmt.Errorf("unable to determine the current directory: %s", err)
					}
					existingValue = cwd
				}
				*opt.ConfigKey.(*string) = existingValue
				return nil
			},
			Required:       false,
			Usage:          "Storage location for Captive Core bucket data. If not set, the current working directory is used as the default location.",
			ConfigKey:      &config.CaptiveCoreStoragePath,
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "captive-core-peer-port",
			OptType:        types.Uint,
			FlagDefault:    uint(0),
			CustomSetValue: support.SetOptionalUint,
			Required:       false,
			Usage:          "port for Captive Core to bind to for connecting to the Stellar swarm (0 uses Stellar Core's default)",
			ConfigKey:      &config.CaptiveCoreTomlParams.PeerPort,
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:     StellarCoreDBURLFlagName,
			EnvVar:   "STELLAR_CORE_DATABASE_URL",
			OptType:  types.String,
			Required: false,
			Hidden:   true,
			CustomSetValue: func(opt *support.ConfigOption) error {
				if val := viper.GetString(opt.Name); val != "" {
					return fmt.Errorf("flag --stellar-core-db-url and environment variable STELLAR_CORE_DATABASE_URL have been removed and no longer valid, must use captive core configuration for ingestion")
				}
				return nil
			},
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           StellarCoreURLFlagName,
			ConfigKey:      &config.StellarCoreURL,
			OptType:        types.String,
			Usage:          "stellar-core to connect with (for http commands). If unset and the local Captive core is enabled, it will use http://localhost:<stellar_captive_core_http_port>",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:      HistoryArchiveURLsFlagName,
			ConfigKey: &config.HistoryArchiveURLs,
			OptType:   types.String,
			Required:  false,
			CustomSetValue: func(co *support.ConfigOption) error {
				stringOfUrls := viper.GetString(co.Name)
				urlStrings := strings.Split(stringOfUrls, ",")
				//urlStrings contains a single empty value when stringOfUrls is empty
				if len(urlStrings) == 1 && urlStrings[0] == "" {
					*(co.ConfigKey.(*[]string)) = []string{}
				} else {
					*(co.ConfigKey.(*[]string)) = urlStrings
				}
				return nil
			},
			Usage:          "comma-separated list of stellar history archives to connect with",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           HistoryArchiveCachingFlagName,
			ConfigKey:      &config.HistoryArchiveCaching,
			OptType:        types.Bool,
			FlagDefault:    true,
			Usage:          "adds caching for history archive downloads (requires an add'l 10GB of disk space on mainnet)",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "port",
			ConfigKey:      &config.Port,
			OptType:        types.Uint,
			FlagDefault:    uint(8000),
			Usage:          "tcp port to listen on for http requests",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "admin-port",
			ConfigKey:      &config.AdminPort,
			OptType:        types.Uint,
			FlagDefault:    uint(0),
			Usage:          "WARNING: this should not be accessible from the Internet and does not use TLS, tcp port to listen on for admin http requests, 0 (default) disables the admin server",
			UsedInCommands: append(ApiServerCommands, RecordMetricsCmd),
		},
		&support.ConfigOption{
			Name:           "max-db-connections",
			ConfigKey:      &config.MaxDBConnections,
			OptType:        types.Int,
			FlagDefault:    0,
			Usage:          "when set has a priority over horizon-db-max-open-connections, horizon-db-max-idle-connections. max horizon database open connections may need to be increased when responses are slow but DB CPU is normal",
			UsedInCommands: DatabaseBoundCommands,
		},
		&support.ConfigOption{
			Name:           "horizon-db-max-open-connections",
			ConfigKey:      &config.HorizonDBMaxOpenConnections,
			OptType:        types.Int,
			FlagDefault:    20,
			Usage:          "max horizon database open connections. may need to be increased when responses are slow but DB CPU is normal",
			UsedInCommands: DatabaseBoundCommands,
		},
		&support.ConfigOption{
			Name:           "horizon-db-max-idle-connections",
			ConfigKey:      &config.HorizonDBMaxIdleConnections,
			OptType:        types.Int,
			FlagDefault:    20,
			Usage:          "max horizon database idle connections. may need to be set to the same value as horizon-db-max-open-connections when responses are slow and DB CPU is normal, because it may indicate that a lot of time is spent closing/opening idle connections. This can happen in case of high variance in number of requests. must be equal or lower than max open connections",
			UsedInCommands: DatabaseBoundCommands,
		},
		&support.ConfigOption{
			Name:           "sse-update-frequency",
			ConfigKey:      &config.SSEUpdateFrequency,
			OptType:        types.Int,
			FlagDefault:    5,
			CustomSetValue: support.SetDuration,
			Usage:          "defines how often streams should check if there's a new ledger (in seconds), may need to increase in case of big number of streams",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "connection-timeout",
			ConfigKey:      &config.ConnectionTimeout,
			OptType:        types.Int,
			FlagDefault:    55,
			CustomSetValue: support.SetDuration,
			Usage:          "defines the timeout of connection after which 504 response will be sent or stream will be closed, if Horizon is behind a load balancer with idle connection timeout, this should be set to a few seconds less that idle timeout, does not apply to POST /transactions",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:        "client-query-timeout",
			ConfigKey:   &config.ClientQueryTimeout,
			OptType:     types.Int,
			FlagDefault: clientQueryTimeoutNotSet,
			CustomSetValue: func(co *support.ConfigOption) error {
				if !support.IsExplicitlySet(co) {
					*(co.ConfigKey.(*time.Duration)) = time.Duration(co.FlagDefault.(int))
					return nil
				}
				duration := viper.GetInt(co.Name)
				if duration < 0 {
					return fmt.Errorf("%s cannot be negative", co.Name)
				}
				*(co.ConfigKey.(*time.Duration)) = time.Duration(duration) * time.Second
				return nil
			},
			Usage: "defines the timeout for when horizon will cancel all postgres queries connected to an HTTP request. The timeout is measured in seconds since the start of the HTTP request. Note, this timeout does not apply to POST /transactions. " +
				"The difference between client-query-timeout and connection-timeout is that connection-timeout applies a postgres statement timeout whereas client-query-timeout will send an additional request to postgres to cancel the ongoing query. " +
				"Generally, client-query-timeout should be configured to be higher than connection-timeout to allow the postgres statement timeout to kill long running queries without having to send the additional cancel request to postgres. " +
				"By default, client-query-timeout will be set to twice the connection-timeout. Setting client-query-timeout to 0 will disable the timeout which means that Horizon will never kill long running queries using the cancel request, however, " +
				"long running queries can still be killed through the postgres statement timeout which is configured via the connection-timeout flag.",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "max-http-request-size",
			ConfigKey:      &config.MaxHTTPRequestSize,
			OptType:        types.Uint,
			FlagDefault:    defaultMaxHTTPRequestSize,
			Usage:          "sets the limit on the maximum allowed http request payload size, default is 200kb, to disable the limit check, set to 0, only do so if you acknowledge the implications of accepting unbounded http request payload sizes.",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:        "max-concurrent-requests",
			ConfigKey:   &config.MaxConcurrentRequests,
			OptType:     types.Uint,
			FlagDefault: defaultMaxConcurrentRequests,
			Usage: "sets the limit on the maximum number of concurrent http requests, default is 1000, to disable the limit set to 0. " +
				"If Horizon receives a request which would exceed the limit of concurrent http requests, Horizon will respond with a 503 status code.",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:        "per-hour-rate-limit",
			ConfigKey:   &config.RateQuota,
			OptType:     types.Int,
			FlagDefault: 3600,
			CustomSetValue: func(co *support.ConfigOption) error {
				var rateLimit *throttled.RateQuota = nil
				perHourRateLimit := viper.GetInt(co.Name)
				if perHourRateLimit != 0 {
					rateLimit = &throttled.RateQuota{
						MaxRate:  throttled.PerHour(perHourRateLimit),
						MaxBurst: 100,
					}
					*(co.ConfigKey.(**throttled.RateQuota)) = rateLimit
				}
				return nil
			},
			Usage:          "max count of requests allowed in a one hour period, by remote ip address",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "friendbot-url",
			ConfigKey:      &config.FriendbotURL,
			OptType:        types.String,
			CustomSetValue: support.SetURL,
			Usage:          "friendbot service to redirect to",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:        "log-level",
			ConfigKey:   &config.LogLevel,
			OptType:     types.String,
			FlagDefault: "info",
			CustomSetValue: func(co *support.ConfigOption) error {
				ll, err := logrus.ParseLevel(viper.GetString(co.Name))
				if err != nil {
					return fmt.Errorf("could not parse log-level: %v", viper.GetString(co.Name))
				}
				*(co.ConfigKey.(*logrus.Level)) = ll
				return nil
			},
			Usage: "minimum log severity (debug, info, warn, error) to log",
		},
		&support.ConfigOption{
			Name:      "log-file",
			ConfigKey: &config.LogFile,
			OptType:   types.String,
			Usage:     "name of the file where logs will be saved (leave empty to send logs to stdout)",
		},
		&support.ConfigOption{
			Name:           "captive-core-log-path",
			ConfigKey:      &config.CaptiveCoreTomlParams.LogPath,
			OptType:        types.String,
			CustomSetValue: support.SetOptionalString,
			Required:       false,
			Usage:          "name of the path for Core logs (leave empty to log w/ Horizon only)",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "max-path-length",
			ConfigKey:      &config.MaxPathLength,
			OptType:        types.Uint,
			FlagDefault:    uint(3),
			Usage:          "the maximum number of assets on the path in `/paths` endpoint, warning: increasing this value will increase /paths response time",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "max-assets-per-path-request",
			ConfigKey:      &config.MaxAssetsPerPathRequest,
			OptType:        types.Int,
			FlagDefault:    int(15),
			Usage:          "the maximum number of assets in '/paths/strict-send' and '/paths/strict-receive' endpoints",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "disable-pool-path-finding",
			ConfigKey:      &config.DisablePoolPathFinding,
			OptType:        types.Bool,
			FlagDefault:    false,
			Required:       false,
			Usage:          "excludes liquidity pools from consideration in the `/paths` endpoint",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "disable-path-finding",
			ConfigKey:      &config.DisablePathFinding,
			OptType:        types.Bool,
			FlagDefault:    false,
			Required:       false,
			Usage:          "disables the path finding endpoints",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:        "max-path-finding-requests",
			ConfigKey:   &config.MaxPathFindingRequests,
			OptType:     types.Uint,
			FlagDefault: uint(0),
			Required:    false,
			Usage: "The maximum number of path finding requests per second horizon will allow." +
				" A value of zero (the default) disables the limit.",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           NetworkPassphraseFlagName,
			ConfigKey:      &config.NetworkPassphrase,
			OptType:        types.String,
			Required:       false,
			Usage:          "Override the network passphrase",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "sentry-dsn",
			ConfigKey:      &config.SentryDSN,
			OptType:        types.String,
			Usage:          "Sentry URL to which panics and errors should be reported",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:      "loggly-token",
			ConfigKey: &config.LogglyToken,
			OptType:   types.String,
			Usage:     "Loggly token, used to configure log forwarding to loggly",
		},
		&support.ConfigOption{
			Name:        "loggly-tag",
			ConfigKey:   &config.LogglyTag,
			OptType:     types.String,
			FlagDefault: "horizon",
			Usage:       "Tag to be added to every loggly log event",
		},
		&support.ConfigOption{
			Name:           "tls-cert",
			ConfigKey:      &config.TLSCert,
			OptType:        types.String,
			Usage:          "TLS certificate file to use for securing connections to horizon",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "tls-key",
			ConfigKey:      &config.TLSKey,
			OptType:        types.String,
			Usage:          "TLS private key file to use for securing connections to horizon",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           IngestFlagName,
			ConfigKey:      &config.Ingest,
			OptType:        types.Bool,
			FlagDefault:    true,
			Usage:          "causes this horizon process to ingest data from stellar-core into horizon's db",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "cursor-name",
			EnvVar:         "CURSOR_NAME",
			OptType:        types.String,
			Hidden:         true,
			UsedInCommands: IngestionCommands,
			CustomSetValue: func(opt *support.ConfigOption) error {
				if val := viper.GetString(opt.Name); val != "" {
					return fmt.Errorf("flag --cursor-name has been removed and no longer valid, must use captive core configuration for ingestion")
				}
				return nil
			},
		},
		&support.ConfigOption{
			Name:           "history-retention-count",
			ConfigKey:      &config.HistoryRetentionCount,
			OptType:        types.Uint,
			FlagDefault:    uint(0),
			Usage:          "the minimum number of ledgers to maintain within Horizon's history tables (0 = retain an unlimited number of ledgers)",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "history-retention-reap-count",
			ConfigKey:      &config.HistoryRetentionReapCount,
			OptType:        types.Uint,
			FlagDefault:    uint(50_000),
			Usage:          "the batch size (in ledgers) to remove per reap from the Horizon database",
			UsedInCommands: IngestionCommands,
			CustomSetValue: func(opt *support.ConfigOption) error {
				val := viper.GetUint(opt.Name)
				if val <= 0 || val > 500_000 {
					return fmt.Errorf("flag --history-retention-reap-count must be in range [1, 500,000]")
				}
				*(opt.ConfigKey.(*uint)) = val
				return nil
			},
		},
		&support.ConfigOption{
			Name:        "reap-frequency",
			ConfigKey:   &config.ReapFrequency,
			OptType:     types.Uint,
			FlagDefault: uint(720),
			Usage: "the frequency in units of ledgers for how often history is reaped. " +
				"A value of 1 implies history is trimmed after every ledger. " +
				"A value of 2 implies history is trimmed on every second ledger.",
			UsedInCommands: IngestionCommands,
			CustomSetValue: func(opt *support.ConfigOption) error {
				val := viper.GetUint(opt.Name)
				if val <= 0 {
					return fmt.Errorf("flag --reap-frequency must be positive")
				}
				*(opt.ConfigKey.(*uint)) = val
				return nil
			},
		},
		{
			Name:           "reap-lookup-tables",
			ConfigKey:      &config.ReapLookupTables,
			OptType:        types.Bool,
			FlagDefault:    true,
			Usage:          "enables the reaping of history lookup tables.",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "history-stale-threshold",
			ConfigKey:      &config.StaleThreshold,
			OptType:        types.Uint,
			FlagDefault:    uint(0),
			Usage:          "the maximum number of ledgers the history db is allowed to be out of date from the connected stellar-core db before horizon considers history stale",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "skip-cursor-update",
			OptType:        types.String,
			Hidden:         true,
			UsedInCommands: IngestionCommands,
			CustomSetValue: func(opt *support.ConfigOption) error {
				if val := viper.GetString(opt.Name); val != "" {
					return fmt.Errorf("flag --skip-cursor-update has been removed and no longer valid, must use captive core configuration for ingestion")
				}
				return nil
			},
		},
		&support.ConfigOption{
			Name:           "ingest-disable-state-verification",
			ConfigKey:      &config.IngestDisableStateVerification,
			OptType:        types.Bool,
			FlagDefault:    false,
			Usage:          "disable periodic verification of ledger state in horizon db (not recommended)",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:        "ingest-state-verification-checkpoint-frequency",
			ConfigKey:   &config.IngestStateVerificationCheckpointFrequency,
			OptType:     types.Uint,
			FlagDefault: uint(1),
			Usage: "the frequency in units per checkpoint for how often state verification is executed. " +
				"A value of 1 implies running state verification on every checkpoint. " +
				"A value of 2 implies running state verification on every second checkpoint.",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "ingest-state-verification-timeout",
			ConfigKey:      &config.IngestStateVerificationTimeout,
			OptType:        types.Int,
			FlagDefault:    0,
			CustomSetValue: support.SetDurationMinutes,
			Usage: "defines an upper bound in minutes for on how long state verification is allowed to run. " +
				"A value of 0 disables the timeout.",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "ingest-enable-extended-log-ledger-stats",
			ConfigKey:      &config.IngestEnableExtendedLogLedgerStats,
			OptType:        types.Bool,
			FlagDefault:    false,
			Usage:          "enables extended ledger stats in the log (ledger entry changes and operations stats)",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "apply-migrations",
			ConfigKey:      &config.ApplyMigrations,
			OptType:        types.Bool,
			FlagDefault:    false,
			Required:       false,
			Usage:          "applies pending migrations before starting horizon",
			UsedInCommands: DatabaseBoundCommands,
		},
		&support.ConfigOption{
			Name:           "checkpoint-frequency",
			ConfigKey:      &config.CheckpointFrequency,
			OptType:        types.Uint32,
			FlagDefault:    uint32(64),
			Required:       false,
			Usage:          "establishes how many ledgers exist between checkpoints, do NOT change this unless you really know what you are doing",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           "behind-cloudflare",
			ConfigKey:      &config.BehindCloudflare,
			OptType:        types.Bool,
			FlagDefault:    false,
			Required:       false,
			Usage:          "determines if Horizon instance is behind Cloudflare, in such case client IP in the logs will be replaced with Cloudflare header (cannot be used with --behind-aws-load-balancer)",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "behind-aws-load-balancer",
			ConfigKey:      &config.BehindAWSLoadBalancer,
			OptType:        types.Bool,
			FlagDefault:    false,
			Required:       false,
			Usage:          "determines if Horizon instance is behind AWS load balances like ELB or ALB, in such case client IP in the logs will be replaced with the last IP in X-Forwarded-For header (cannot be used with --behind-cloudflare)",
			UsedInCommands: ApiServerCommands,
		},
		&support.ConfigOption{
			Name:           "rounding-slippage-filter",
			ConfigKey:      &config.RoundingSlippageFilter,
			OptType:        types.Int,
			FlagDefault:    1000,
			Required:       false,
			Usage:          "excludes trades from /trade_aggregations unless their rounding slippage is <x bps",
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:      NetworkFlagName,
			ConfigKey: &config.Network,
			OptType:   types.String,
			Required:  false,
			CustomSetValue: func(co *support.ConfigOption) error {
				val := viper.GetString(co.Name)
				if val != "" && val != StellarPubnet && val != StellarTestnet {
					return fmt.Errorf("invalid network %s. Use '%s' or '%s'",
						val, StellarPubnet, StellarTestnet)
				}
				*co.ConfigKey.(*string) = val
				return nil
			},
			Usage: fmt.Sprintf("stellar public network, either '%s' or '%s'."+
				" It automatically configures network settings, including %s, %s, and %s.",
				StellarPubnet, StellarTestnet, NetworkPassphraseFlagName,
				HistoryArchiveURLsFlagName, CaptiveCoreConfigPathName),
			UsedInCommands: IngestionCommands,
		},
		&support.ConfigOption{
			Name:           SkipTxmeta,
			ConfigKey:      &config.SkipTxmeta,
			OptType:        types.Bool,
			FlagDefault:    false,
			Required:       false,
			Usage:          "excludes tx meta from persistence on transaction history",
			UsedInCommands: IngestionCommands,
		},
	}

	return config, flags
}

// NewAppFromFlags constructs a new Horizon App from the given command line flags
func NewAppFromFlags(config *Config, flags support.ConfigOptions) (*App, error) {
	err := ApplyFlags(config, flags, ApplyOptions{RequireCaptiveCoreFullConfig: true})
	if err != nil {
		return nil, err
	}
	// Validate app-specific arguments
	if (!config.DisableTxSub || config.Ingest) && config.StellarCoreURL == "" {
		return nil, fmt.Errorf("flag --%s cannot be empty", StellarCoreURLFlagName)
	}

	log.Infof("Initializing horizon...")
	app, err := NewApp(*config)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize app: %s", err)
	}
	return app, nil
}

type ApplyOptions struct {
	RequireCaptiveCoreFullConfig bool
	NoCaptiveCore                bool
}

// getCaptiveCoreBinaryPath retrieves the path of the Captive Core binary
// Returns the path or an error if the binary is not found
func getCaptiveCoreBinaryPath() (string, error) {
	result, err := exec.LookPath("stellar-core")
	if err != nil {
		return "", err
	}
	return result, nil
}

// setCaptiveCoreConfiguration prepares configuration for the Captive Core
func setCaptiveCoreConfiguration(config *Config, options ApplyOptions) error {
	stdLog.Println("Preparing captive core...")

	var err error
	// If the user didn't specify a Stellar Core binary, we can check the
	// $PATH and possibly fill it in for them.
	if config.CaptiveCoreBinaryPath == "" {
		if config.CaptiveCoreBinaryPath, err = getCaptiveCoreBinaryPath(); err != nil {
			return fmt.Errorf("captive core requires %s", StellarCoreBinaryPathName)
		}
	}

	var defaultCaptiveCoreConfig []byte
	switch config.Network {
	case StellarPubnet:
		defaultCaptiveCoreConfig = ledgerbackend.PubnetDefaultConfig
	case StellarTestnet:

		defaultCaptiveCoreConfig = ledgerbackend.TestnetDefaultConfig
	}

	config.CaptiveCoreTomlParams.CoreBinaryPath = config.CaptiveCoreBinaryPath
	config.CaptiveCoreTomlParams.HistoryArchiveURLs = config.HistoryArchiveURLs
	config.CaptiveCoreTomlParams.NetworkPassphrase = config.NetworkPassphrase

	if config.CaptiveCoreConfigPath != "" {
		config.CaptiveCoreToml, err = ledgerbackend.NewCaptiveCoreTomlFromFile(config.CaptiveCoreConfigPath,
			config.CaptiveCoreTomlParams)
		if err != nil {
			return errors.Wrap(err, "invalid captive core toml file")
		}
	} else if !options.RequireCaptiveCoreFullConfig {
		// Creates a minimal captive-core config (without quorum information), just enough to run captive core.
		// This is used by certain database commands, such as `reingest and fill-gaps, to reingest historical data.
		config.CaptiveCoreToml, err = ledgerbackend.NewCaptiveCoreToml(config.CaptiveCoreTomlParams)
		if err != nil {
			return errors.Wrap(err, "invalid captive core toml file")
		}
	} else if len(defaultCaptiveCoreConfig) != 0 {
		config.CaptiveCoreToml, err = ledgerbackend.NewCaptiveCoreTomlFromData(defaultCaptiveCoreConfig,
			config.CaptiveCoreTomlParams)
		if err != nil {
			return errors.Wrap(err, "invalid captive core toml file")
		}
	} else {
		return fmt.Errorf("invalid config: captive core requires that --%s is set or you can set the --%s "+
			"parameter to use the default captive core config", CaptiveCoreConfigPathName, NetworkFlagName)
	}

	// If we don't supply an explicit core URL and running captive core process with the http port enabled,
	// point to it.
	if config.StellarCoreURL == "" && config.CaptiveCoreToml.HTTPPort != 0 {
		config.StellarCoreURL = fmt.Sprintf("http://localhost:%d", config.CaptiveCoreToml.HTTPPort)
	}

	return nil
}

// ApplyFlags applies the command line flags on the given Config instance
func ApplyFlags(config *Config, flags support.ConfigOptions, options ApplyOptions) error {
	// Check if the user has passed any flags and if so, print a DEPRECATED warning message.
	flagsPassedByUser := flags.GetCommandLineFlagsPassedByUser()
	if len(flagsPassedByUser) > 0 {
		result := fmt.Sprintf("DEPRECATED - the use of command-line flags: [%s], has been deprecated in favor of environment variables. "+
			"Please consult our Configuring section in the developer documentation on how to use them - https://developers.stellar.org/docs/run-api-server/configuring", "--"+strings.Join(flagsPassedByUser, ",--"))
		stdLog.Println(result)
	}

	// Verify required options and load the config struct
	if err := flags.RequireE(); err != nil {
		return err
	}

	if err := flags.SetValues(); err != nil {
		return err
	}

	// Validate options that should be provided together
	if err := validateBothOrNeither("tls-cert", "tls-key"); err != nil {
		return err
	}

	if config.Ingest {
		// Migrations should be checked as early as possible. Apply and check
		// only on ingesting instances which are required to have write-access
		// to the DB.
		if config.ApplyMigrations {
			stdLog.Println("Applying DB migrations...")
			if err := applyMigrations(*config); err != nil {
				return err
			}
		}
		stdLog.Println("Checking DB migrations...")
		if err := checkMigrations(*config); err != nil {
			return err
		}

		if err := setNetworkConfiguration(config); err != nil {
			return err
		}

		if !options.NoCaptiveCore {
			err := setCaptiveCoreConfiguration(config, options)
			if err != nil {
				return errors.Wrap(err, "error generating captive core configuration")
			}
		}
	}

	// Configure log file
	if config.LogFile != "" {
		logFile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			log.DefaultLogger.SetOutput(logFile)
		} else {
			return fmt.Errorf("failed to open file to log: %s", err)
		}
	}

	// Configure log level
	log.DefaultLogger.SetLevel(config.LogLevel)

	// Configure DB params. When config.MaxDBConnections is set, set other
	// DB params to that value for backward compatibility.
	if config.MaxDBConnections != 0 {
		config.HorizonDBMaxOpenConnections = config.MaxDBConnections
		config.HorizonDBMaxIdleConnections = config.MaxDBConnections
	}

	if config.BehindCloudflare && config.BehindAWSLoadBalancer {
		return fmt.Errorf("invalid config: Only one option of --behind-cloudflare and --behind-aws-load-balancer is allowed." +
			" If Horizon is behind both, use --behind-cloudflare only")
	}

	if config.ClientQueryTimeout == clientQueryTimeoutNotSet {
		// the default value for cancel-db-query-timeout is twice the connection-timeout
		config.ClientQueryTimeout = config.ConnectionTimeout * 2
	}

	return nil
}

func setNetworkConfiguration(config *Config) error {
	if config.Network != "" {
		if config.NetworkPassphrase != "" {
			return fmt.Errorf("invalid config: %s parameter not allowed with the %s parameter",
				NetworkPassphraseFlagName, NetworkFlagName)
		}

		if len(config.HistoryArchiveURLs) > 0 {
			return fmt.Errorf("invalid config: %s parameter not allowed with the %s parameter",
				HistoryArchiveURLsFlagName, NetworkFlagName)
		}

		switch config.Network {
		case StellarPubnet:
			config.NetworkPassphrase = network.PublicNetworkPassphrase
			config.HistoryArchiveURLs = network.PublicNetworkhistoryArchiveURLs
		case StellarTestnet:
			config.NetworkPassphrase = network.TestNetworkPassphrase
			config.HistoryArchiveURLs = network.TestNetworkhistoryArchiveURLs
		default:
			return fmt.Errorf("no default configuration found for network %s", config.Network)
		}
	}

	if config.NetworkPassphrase == "" {
		return fmt.Errorf("%s must be set", NetworkPassphraseFlagName)
	}

	if len(config.HistoryArchiveURLs) == 0 {
		return fmt.Errorf("%s must be set", HistoryArchiveURLsFlagName)
	}
	return nil
}

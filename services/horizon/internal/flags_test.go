package horizon

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/stellar/go/network"
	"github.com/stellar/go/services/horizon/internal/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_createCaptiveCoreDefaultConfig(t *testing.T) {

	var errorMsgDefaultConfig = "invalid config: %s parameter not allowed with the network parameter"
	tests := []struct {
		name               string
		config             Config
		networkPassphrase  string
		historyArchiveURLs []string
		errStr             string
	}{
		{
			name: "testnet default config",
			config: Config{Network: StellarTestnet,
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			networkPassphrase:  network.TestNetworkPassphrase,
			historyArchiveURLs: network.TestNetworkhistoryArchiveURLs,
		},
		{
			name: "pubnet default config",
			config: Config{Network: StellarPubnet,
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			networkPassphrase:  network.PublicNetworkPassphrase,
			historyArchiveURLs: network.PublicNetworkhistoryArchiveURLs,
		},
		{
			name: "testnet validation; history archive urls supplied",
			config: Config{Network: StellarTestnet,
				HistoryArchiveURLs:    []string{"network history archive urls supplied"},
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: fmt.Sprintf(errorMsgDefaultConfig, HistoryArchiveURLsFlagName),
		},
		{
			name: "pubnet validation; history archive urls supplied",
			config: Config{Network: StellarPubnet,
				HistoryArchiveURLs:    []string{"network history archive urls supplied"},
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: fmt.Sprintf(errorMsgDefaultConfig, HistoryArchiveURLsFlagName),
		},
		{
			name: "testnet validation; network passphrase supplied",
			config: Config{Network: StellarTestnet,
				NetworkPassphrase:     "network passphrase supplied",
				HistoryArchiveURLs:    []string{},
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: fmt.Sprintf(errorMsgDefaultConfig, NetworkPassphraseFlagName),
		},
		{
			name: "pubnet validation; network passphrase supplied",
			config: Config{Network: StellarPubnet,
				NetworkPassphrase:     "pubnet network passphrase supplied",
				HistoryArchiveURLs:    []string{},
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: fmt.Sprintf(errorMsgDefaultConfig, NetworkPassphraseFlagName),
		},
		{
			name: "unknown network specified",
			config: Config{Network: "unknown",
				NetworkPassphrase:     "",
				HistoryArchiveURLs:    []string{},
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: "no default configuration found for network unknown",
		},
		{
			name: "no network specified; passphrase not supplied",
			config: Config{
				HistoryArchiveURLs:    []string{"HistoryArchiveURLs"},
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: fmt.Sprintf("%s must be set", NetworkPassphraseFlagName),
		},
		{
			name: "no network specified; history archive urls not supplied",
			config: Config{
				NetworkPassphrase:     "NetworkPassphrase",
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: fmt.Sprintf("%s must be set", HistoryArchiveURLsFlagName),
		},

		{
			name: "unknown network specified",
			config: Config{Network: "unknown",
				NetworkPassphrase:     "",
				HistoryArchiveURLs:    []string{},
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: "no default configuration found for network unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.CaptiveCoreTomlParams.UseDB = true
			e := setNetworkConfiguration(&tt.config)
			if tt.errStr == "" {
				assert.NoError(t, e)
				assert.Equal(t, tt.networkPassphrase, tt.config.NetworkPassphrase)
				assert.Equal(t, tt.historyArchiveURLs, tt.config.HistoryArchiveURLs)
			} else {
				assert.Equal(t, tt.errStr, e.Error())
			}
		})
	}
}

func TestSetCaptiveCoreConfig(t *testing.T) {
	tests := []struct {
		name                     string
		requireCaptiveCoreConfig bool
		config                   Config
		errStr                   string
	}{
		{
			name:                     "testnet default config",
			requireCaptiveCoreConfig: true,
			config: Config{
				Network:               StellarTestnet,
				NetworkPassphrase:     network.TestNetworkPassphrase,
				HistoryArchiveURLs:    network.TestNetworkhistoryArchiveURLs,
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
		},
		{
			name:                     "pubnet default config",
			requireCaptiveCoreConfig: true,
			config: Config{
				Network:               StellarPubnet,
				NetworkPassphrase:     network.PublicNetworkPassphrase,
				HistoryArchiveURLs:    network.PublicNetworkhistoryArchiveURLs,
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
		},
		{
			name:                     "no network specified; valid parameters",
			requireCaptiveCoreConfig: true,
			config: Config{
				NetworkPassphrase:     network.PublicNetworkPassphrase,
				HistoryArchiveURLs:    network.PublicNetworkhistoryArchiveURLs,
				CaptiveCoreConfigPath: "../../../ingest/ledgerbackend/configs/captive-core-pubnet.cfg",
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
		},

		{
			name:                     "no network specified; captive-core-config-path not supplied",
			requireCaptiveCoreConfig: true,
			config: Config{
				NetworkPassphrase:     network.PublicNetworkPassphrase,
				HistoryArchiveURLs:    network.PublicNetworkhistoryArchiveURLs,
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: fmt.Sprintf("invalid config: captive core requires that --%s is set or "+
				"you can set the --%s parameter to use the default captive core config", CaptiveCoreConfigPathName, NetworkFlagName),
		},
		{
			name:                     "no network specified; captive-core-config-path invalid file",
			requireCaptiveCoreConfig: true,
			config: Config{
				NetworkPassphrase:     network.PublicNetworkPassphrase,
				HistoryArchiveURLs:    network.PublicNetworkhistoryArchiveURLs,
				CaptiveCoreConfigPath: "xyz.cfg",
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: "invalid captive core toml file: could not load toml path:" +
				" open xyz.cfg: no such file or directory",
		},
		{
			name:                     "no network specified; captive-core-config-path incorrect config",
			requireCaptiveCoreConfig: true,
			config: Config{
				NetworkPassphrase:     network.PublicNetworkPassphrase,
				HistoryArchiveURLs:    network.PublicNetworkhistoryArchiveURLs,
				CaptiveCoreConfigPath: "../../../ingest/ledgerbackend/configs/captive-core-testnet.cfg",
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
			errStr: fmt.Sprintf("invalid captive core toml file: invalid captive core toml: "+
				"NETWORK_PASSPHRASE in captive core config file: %s does not match passed configuration (%s)",
				network.TestNetworkPassphrase, network.PublicNetworkPassphrase),
		},
		{
			name:                     "no network specified; full captive-core-config not required",
			requireCaptiveCoreConfig: false,
			config: Config{
				CaptiveCoreBinaryPath: "/path/to/captive-core/binary",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.CaptiveCoreTomlParams.UseDB = true
			e := setCaptiveCoreConfiguration(&tt.config,
				ApplyOptions{RequireCaptiveCoreFullConfig: tt.requireCaptiveCoreConfig})
			if tt.errStr == "" {
				assert.NoError(t, e)
			} else {
				require.Error(t, e)
				assert.Equal(t, tt.errStr, e.Error())
			}
		})
	}
}

func TestClientQueryTimeoutFlag(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		flag   string
		parsed time.Duration
		err    string
	}{
		{
			"negative value",
			"-1",
			0,
			"client-query-timeout cannot be negative",
		},
		{
			"default value",
			"",
			time.Second * 110,
			"",
		},
		{
			"custom value",
			"20",
			time.Second * 20,
			"",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			environmentVars := horizonEnvVars()
			if testCase.flag != "" {
				environmentVars["CLIENT_QUERY_TIMEOUT"] = testCase.flag
			}

			envManager := test.NewEnvironmentManager()
			defer func() {
				envManager.Restore()
			}()
			if err := envManager.InitializeEnvironmentVariables(environmentVars); err != nil {
				require.NoError(t, err)
			}

			config, flags := Flags()
			horizonCmd := &cobra.Command{
				Use:           "horizon",
				Short:         "Client-facing api server for the Stellar network",
				SilenceErrors: true,
				SilenceUsage:  true,
				Long:          "Client-facing API server for the Stellar network.",
			}
			if err := flags.Init(horizonCmd); err != nil {
				require.NoError(t, err)
			}
			if err := ApplyFlags(config, flags, ApplyOptions{RequireCaptiveCoreFullConfig: true}); err != nil {
				require.EqualError(t, err, testCase.err)
			} else {
				require.Empty(t, testCase.err)
			}
			require.Equal(t, testCase.parsed, config.ClientQueryTimeout)
		})
	}
}

func TestEnvironmentVariables(t *testing.T) {
	environmentVars := horizonEnvVars()

	envManager := test.NewEnvironmentManager()
	defer func() {
		envManager.Restore()
	}()
	if err := envManager.InitializeEnvironmentVariables(environmentVars); err != nil {
		fmt.Println(err)
	}

	config, flags := Flags()
	horizonCmd := &cobra.Command{
		Use:           "horizon",
		Short:         "Client-facing api server for the Stellar network",
		SilenceErrors: true,
		SilenceUsage:  true,
		Long:          "Client-facing API server for the Stellar network.",
	}
	if err := flags.Init(horizonCmd); err != nil {
		fmt.Println(err)
	}
	if err := ApplyFlags(config, flags, ApplyOptions{RequireCaptiveCoreFullConfig: true}); err != nil {
		fmt.Println(err)
	}
	assert.Equal(t, config.Ingest, false)
	assert.Equal(t, config.HistoryArchiveURLs, []string{"http://localhost:1570"})
	assert.Equal(t, config.DatabaseURL, "postgres://postgres@localhost/test_332cb65e6b00?sslmode=disable&timezone=UTC")
	assert.Equal(t, config.StellarCoreURL, "http://localhost:11626")
	assert.Equal(t, config.NetworkPassphrase, "Standalone Network ; February 2017")
	assert.Equal(t, config.ApplyMigrations, true)
	assert.Equal(t, config.CheckpointFrequency, uint32(8))
	assert.Equal(t, config.MaxDBConnections, 50)
	assert.Equal(t, config.AdminPort, uint(6060))
	assert.Equal(t, config.Port, uint(8001))
	assert.Equal(t, config.CaptiveCoreBinaryPath, os.Getenv("HORIZON_INTEGRATION_TESTS_CAPTIVE_CORE_BIN"))
	assert.Equal(t, config.CaptiveCoreConfigPath, "../docker/captive-core-integration-tests.cfg")
	assert.Equal(t, config.CaptiveCoreConfigUseDB, true)
}

func horizonEnvVars() map[string]string {
	return map[string]string{
		"INGEST":                   "false",
		"HISTORY_ARCHIVE_URLS":     "http://localhost:1570",
		"DATABASE_URL":             "postgres://postgres@localhost/test_332cb65e6b00?sslmode=disable&timezone=UTC",
		"STELLAR_CORE_URL":         "http://localhost:11626",
		"NETWORK_PASSPHRASE":       "Standalone Network ; February 2017",
		"APPLY_MIGRATIONS":         "true",
		"CHECKPOINT_FREQUENCY":     "8",
		"MAX_DB_CONNECTIONS":       "50",
		"ADMIN_PORT":               "6060",
		"PORT":                     "8001",
		"CAPTIVE_CORE_BINARY_PATH": os.Getenv("HORIZON_INTEGRATION_TESTS_CAPTIVE_CORE_BIN"),
		"CAPTIVE_CORE_CONFIG_PATH": "../docker/captive-core-integration-tests.cfg",
		"CAPTIVE_CORE_USE_DB":      "true",
	}
}

func TestRemovedFlags(t *testing.T) {
	tests := []struct {
		name            string
		environmentVars map[string]string
		errStr          string
		cmdArgs         []string
	}{
		{
			name: "STELLAR_CORE_DATABASE_URL removed",
			environmentVars: map[string]string{
				"INGEST":                    "false",
				"STELLAR_CORE_DATABASE_URL": "coredb",
				"DATABASE_URL":              "dburl",
			},
			errStr: "flag --stellar-core-db-url and environment variable STELLAR_CORE_DATABASE_URL have been removed and no longer valid, must use captive core configuration for ingestion",
		},
		{
			name: "--stellar-core-db-url  removed",
			environmentVars: map[string]string{
				"INGEST":       "false",
				"DATABASE_URL": "dburl",
			},
			errStr:  "flag --stellar-core-db-url and environment variable STELLAR_CORE_DATABASE_URL have been removed and no longer valid, must use captive core configuration for ingestion",
			cmdArgs: []string{"--stellar-core-db-url=coredb"},
		},
		{
			name: "CURSOR_NAME removed",
			environmentVars: map[string]string{
				"INGEST":       "false",
				"CURSOR_NAME":  "cursor",
				"DATABASE_URL": "dburl",
			},
			errStr: "flag --cursor-name has been removed and no longer valid, must use captive core configuration for ingestion",
		},
		{
			name: "SKIP_CURSOR_UPDATE removed",
			environmentVars: map[string]string{
				"INGEST":             "false",
				"SKIP_CURSOR_UPDATE": "true",
				"DATABASE_URL":       "dburl",
			},
			errStr: "flag --skip-cursor-update has been removed and no longer valid, must use captive core configuration for ingestion",
		},
	}

	envManager := test.NewEnvironmentManager()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				envManager.Restore()
			}()
			err := envManager.InitializeEnvironmentVariables(tt.environmentVars)
			require.NoError(t, err)

			config, flags := Flags()
			testCmd := &cobra.Command{
				Use: "test",
			}

			require.NoError(t, flags.Init(testCmd))
			require.NoError(t, testCmd.ParseFlags(tt.cmdArgs))

			err = ApplyFlags(config, flags, ApplyOptions{})
			require.Error(t, err)
			assert.Equal(t, tt.errStr, err.Error())
		})
	}
}

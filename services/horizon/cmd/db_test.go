package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	horizon "github.com/stellar/go/services/horizon/internal"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest"
	"github.com/stellar/go/support/db/dbtest"
)

func TestDBCommandsTestSuite(t *testing.T) {
	dbCmdSuite := &DBCommandsTestSuite{}
	suite.Run(t, dbCmdSuite)
}

type DBCommandsTestSuite struct {
	suite.Suite
	db      *dbtest.DB
	rootCmd *cobra.Command
}

func (s *DBCommandsTestSuite) SetupSuite() {
	runDBReingestRangeFn = func([]history.LedgerRange, bool, uint, uint, uint,
		horizon.Config, ingest.StorageBackendConfig) error {
		return nil
	}

	s.db = dbtest.Postgres(s.T())

	RootCmd.SetArgs([]string{
		"db", "migrate", "up", "--db-url", s.db.DSN})
	require.NoError(s.T(), RootCmd.Execute())
}

func (s *DBCommandsTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *DBCommandsTestSuite) BeforeTest(suiteName string, testName string) {
	s.rootCmd = NewRootCmd()
}

func (s *DBCommandsTestSuite) TestInvalidParameterParallelJobSize() {
	s.rootCmd.SetArgs([]string{
		"db", "reingest", "range",
		"--db-url", s.db.DSN,
		"--network", "testnet",
		"--parallel-workers", "2",
		"--parallel-job-size", "10",
		"--ledgerbackend", "datastore",
		"--datastore-config", "../internal/ingest/testdata/config.storagebackend.toml",
		"2",
		"10"})

	require.Equal(s.T(), "unknown flag: --parallel-job-size", s.rootCmd.Execute().Error())
}

func (s *DBCommandsTestSuite) TestDbReingestAndFillGapsCmds() {
	tests := []struct {
		name          string
		args          []string
		ledgerBackend ingest.LedgerBackendType
		expectError   bool
		errorMessage  string
	}{
		{
			name: "default; w/ individual network flags",
			args: []string{
				"1", "100",
				"--network-passphrase", "passphrase",
				"--history-archive-urls", "[]",
			},
			expectError: false,
		},
		{
			name: "default; w/o individual network flags",
			args: []string{
				"1", "100",
			},
			expectError:  true,
			errorMessage: "network-passphrase must be set",
		},
		{
			name: "default; no history-archive-urls flag",
			args: []string{
				"1", "100",
				"--network-passphrase", "passphrase",
			},
			expectError:  true,
			errorMessage: "history-archive-urls must be set",
		},
		{
			name: "default; w/ network parameter",
			args: []string{
				"1", "100",
				"--network", "testnet",
			},
			expectError: false,
		},
		{
			name: "datastore; w/ individual network flags",
			args: []string{
				"1", "100",
				"--ledgerbackend", "datastore",
				"--datastore-config", "../internal/ingest/testdata/config.storagebackend.toml",
				"--network-passphrase", "passphrase",
				"--history-archive-urls", "[]",
			},
			expectError: false,
		},
		{
			name: "datastore; w/o individual network flags",
			args: []string{
				"1", "100",
				"--ledgerbackend", "datastore",
				"--datastore-config", "../internal/ingest/testdata/config.storagebackend.toml",
			},
			expectError:  true,
			errorMessage: "network-passphrase must be set",
		},
		{
			name: "datastore; no history-archive-urls flag",
			args: []string{
				"1", "100",
				"--ledgerbackend", "datastore",
				"--datastore-config", "../internal/ingest/testdata/config.storagebackend.toml",
				"--network-passphrase", "passphrase",
			},
			expectError:  true,
			errorMessage: "history-archive-urls must be set",
		},
		{
			name: "captive-core; valid",
			args: []string{
				"1", "100",
				"--network", "testnet",
				"--ledgerbackend", "captive-core",
			},
			expectError: false,
		},
		{
			name: "invalid datastore",
			args: []string{
				"1", "100",
				"--network", "testnet",
				"--ledgerbackend", "unknown",
			},
			expectError:  true,
			errorMessage: "invalid ledger backend: unknown, must be 'captive-core' or 'datastore'",
		},
		{
			name: "datastore; missing config file",
			args: []string{
				"1", "100",
				"--network", "testnet",
				"--ledgerbackend", "datastore",
				"--datastore-config", "invalid.config.toml",
			},
			expectError:  true,
			errorMessage: "failed to load datastore ledgerbackend config file",
		},
		{
			name: "datastore; w/ config",
			args: []string{
				"1", "100",
				"--network", "testnet",
				"--ledgerbackend", "datastore",
				"--datastore-config", "../internal/ingest/testdata/config.storagebackend.toml",
			},
			expectError: false,
		},
		{
			name: "datastore; w/o config",
			args: []string{
				"1", "100",
				"--network", "testnet",
				"--ledgerbackend", "datastore",
			},
			expectError:  true,
			errorMessage: "datastore config file is required for datastore ledgerbackend type",
		},
	}

	commands := []struct {
		cmd  []string
		name string
	}{
		{[]string{"db", "reingest", "range"}, "TestDbReingestRangeCmd"},
		{[]string{"db", "fill-gaps"}, "TestDbFillGapsCmd"},
	}

	for _, command := range commands {
		for _, tt := range tests {
			s.T().Run(tt.name+"_"+command.name, func(t *testing.T) {

				rootCmd := NewRootCmd()
				var args []string
				args = append(command.cmd, tt.args...)
				rootCmd.SetArgs(append([]string{
					"--db-url", s.db.DSN,
					"--stellar-core-binary-path", "/test/core/bin/path",
				}, args...))

				if tt.expectError {
					err := rootCmd.Execute()
					require.Error(t, err)
					require.Contains(t, err.Error(), tt.errorMessage)
				} else {
					require.NoError(t, rootCmd.Execute())
				}
			})
		}
	}
}

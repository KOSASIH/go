package galexie

import (
	"context"
	"fmt"
	"testing"

	"github.com/stellar/go/historyarchive"
	"github.com/stellar/go/network"

	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	ctx := context.Background()

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.On("GetLatestLedgerSequence").Return(uint32(5), nil).Once()
	mockArchive.On("GetCheckpointManager").
		Return(historyarchive.NewCheckpointManager(
			historyarchive.DefaultCheckpointFrequency)).Once()

	config, err := NewConfig(
		RuntimeSettings{StartLedger: 2, EndLedger: 3, ConfigFilePath: "test/test.toml", Mode: Append}, nil)

	require.NoError(t, err)
	err = config.ValidateAndSetLedgerRange(ctx, mockArchive)
	require.NoError(t, err)
	require.Equal(t, config.StellarCoreConfig.Network, "pubnet")
	require.Equal(t, config.DataStoreConfig.Type, "ABC")
	require.Equal(t, config.DataStoreConfig.Schema.FilesPerPartition, uint32(1))
	require.Equal(t, config.DataStoreConfig.Schema.LedgersPerFile, uint32(3))
	require.Equal(t, config.UserAgent, "galexie")
	require.True(t, config.Resumable())
	url, ok := config.DataStoreConfig.Params["destination_bucket_path"]
	require.True(t, ok)
	require.Equal(t, url, "your-bucket-name/subpath/testnet")
	mockArchive.AssertExpectations(t)
}

func TestGenerateHistoryArchiveFromPreconfiguredNetwork(t *testing.T) {
	ctx := context.Background()
	config, err := NewConfig(
		RuntimeSettings{StartLedger: 2, EndLedger: 3, ConfigFilePath: "test/valid_captive_core_preconfigured.toml", Mode: Append}, nil)
	require.NoError(t, err)

	_, err = config.GenerateHistoryArchive(ctx, nil)
	require.NoError(t, err)
}

func TestGenerateHistoryArchiveFromManulConfiguredNetwork(t *testing.T) {
	ctx := context.Background()
	config, err := NewConfig(
		RuntimeSettings{StartLedger: 2, EndLedger: 3, ConfigFilePath: "test/valid_captive_core_manual.toml", Mode: Append}, nil)
	require.NoError(t, err)

	_, err = config.GenerateHistoryArchive(ctx, nil)
	require.NoError(t, err)
}

func TestNewConfigUserAgent(t *testing.T) {
	config, err := NewConfig(
		RuntimeSettings{StartLedger: 2, EndLedger: 3, ConfigFilePath: "test/useragent.toml"}, nil)
	require.NoError(t, err)
	require.Equal(t, config.UserAgent, "useragent_x")
}

func TestResumeDisabled(t *testing.T) {
	// resumable is only enabled when mode is Append
	config, err := NewConfig(
		RuntimeSettings{StartLedger: 2, EndLedger: 3, ConfigFilePath: "test/test.toml", Mode: ScanFill}, nil)
	require.NoError(t, err)
	require.False(t, config.Resumable())
}

func TestInvalidConfigFilePath(t *testing.T) {
	_, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/notfound.toml"}, nil)
	require.ErrorContains(t, err, "config file test/notfound.toml was not found")
}

func TestNoCaptiveCoreBin(t *testing.T) {
	cfg, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/no_core_bin.toml"}, nil)
	require.NoError(t, err)

	_, err = cfg.GenerateCaptiveCoreConfig("")
	require.ErrorContains(t, err, "Invalid captive core config, no stellar-core binary path was provided.")
}

func TestDefaultCaptiveCoreBin(t *testing.T) {
	cfg, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/no_core_bin.toml"},
		func(string) (string, error) { return "v20.2.0-2-g6e73c0a88", nil })
	require.NoError(t, err)

	ccConfig, err := cfg.GenerateCaptiveCoreConfig("/test/default/stellar-core")
	require.NoError(t, err)
	require.Equal(t, ccConfig.BinaryPath, "/test/default/stellar-core")
}

func TestInvalidCaptiveCorePreconfiguredNetwork(t *testing.T) {
	_, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/invalid_preconfigured_network.toml"}, nil)

	require.ErrorContains(t, err, "invalid captive core config")
}

func TestValidCaptiveCorePreconfiguredNetwork(t *testing.T) {
	cfg, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/valid_captive_core_preconfigured.toml"},
		func(string) (string, error) { return "v20.2.0-2-g6e73c0a88", nil })
	require.NoError(t, err)

	require.Equal(t, cfg.StellarCoreConfig.NetworkPassphrase, network.PublicNetworkPassphrase)
	require.Equal(t, cfg.StellarCoreConfig.HistoryArchiveUrls, network.PublicNetworkhistoryArchiveURLs)

	ccConfig, err := cfg.GenerateCaptiveCoreConfig("")
	require.NoError(t, err)

	// validates that ingest/ledgerbackend/configs/captive-core-pubnet.cfg was loaded
	require.Equal(t, ccConfig.BinaryPath, "test/stellar-core")
	require.Equal(t, ccConfig.NetworkPassphrase, network.PublicNetworkPassphrase)
	require.Equal(t, ccConfig.HistoryArchiveURLs, network.PublicNetworkhistoryArchiveURLs)
	require.Empty(t, ccConfig.Toml.HistoryEntries)
	require.Len(t, ccConfig.Toml.Validators, 23)
	require.Equal(t, ccConfig.Toml.Validators[0].Name, "bootes")
}

func TestValidCaptiveCoreManualNetwork(t *testing.T) {
	cfg, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/valid_captive_core_manual.toml"},
		func(string) (string, error) { return "v20.2.0-2-g6e73c0a88", nil })
	require.NoError(t, err)
	require.Equal(t, cfg.CoreVersion, "")
	require.Equal(t, cfg.StellarCoreConfig.NetworkPassphrase, "test")
	require.Equal(t, cfg.StellarCoreConfig.HistoryArchiveUrls, []string{"http://testarchive"})

	ccConfig, err := cfg.GenerateCaptiveCoreConfig("")
	require.NoError(t, err)

	require.Equal(t, ccConfig.BinaryPath, "test/stellar-core")
	require.Equal(t, ccConfig.NetworkPassphrase, "test")
	require.Equal(t, ccConfig.HistoryArchiveURLs, []string{"http://testarchive"})
	require.Empty(t, ccConfig.Toml.HistoryEntries)
	require.Len(t, ccConfig.Toml.Validators, 1)
	require.Equal(t, ccConfig.Toml.Validators[0].Name, "local_core")
	require.Equal(t, cfg.CoreVersion, "v20.2.0-2-g6e73c0a88")
}

func TestValidCaptiveCoreOverridenToml(t *testing.T) {
	cfg, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/valid_captive_core_override.toml"},
		func(string) (string, error) { return "v20.2.0-2-g6e73c0a88", nil })
	require.NoError(t, err)
	require.Equal(t, cfg.StellarCoreConfig.NetworkPassphrase, network.PublicNetworkPassphrase)
	require.Equal(t, cfg.StellarCoreConfig.HistoryArchiveUrls, network.PublicNetworkhistoryArchiveURLs)

	ccConfig, err := cfg.GenerateCaptiveCoreConfig("")
	require.NoError(t, err)

	// the external core cfg file should have applied over the preconf'd network config
	require.Equal(t, ccConfig.BinaryPath, "test/stellar-core")
	require.Equal(t, ccConfig.NetworkPassphrase, network.PublicNetworkPassphrase)
	require.Equal(t, ccConfig.HistoryArchiveURLs, network.PublicNetworkhistoryArchiveURLs)
	require.Empty(t, ccConfig.Toml.HistoryEntries)
	require.Len(t, ccConfig.Toml.Validators, 1)
	require.Equal(t, ccConfig.Toml.Validators[0].Name, "local_core")
	require.Equal(t, cfg.CoreVersion, "v20.2.0-2-g6e73c0a88")
}

func TestValidCaptiveCoreOverridenArchiveUrls(t *testing.T) {
	cfg, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/valid_captive_core_override_archives.toml"},
		func(string) (string, error) { return "v20.2.0-2-g6e73c0a88\n", nil })
	require.NoError(t, err)

	require.Equal(t, cfg.StellarCoreConfig.NetworkPassphrase, network.PublicNetworkPassphrase)
	require.Equal(t, cfg.StellarCoreConfig.HistoryArchiveUrls, []string{"http://testarchive"})

	ccConfig, err := cfg.GenerateCaptiveCoreConfig("")
	require.NoError(t, err)

	// validates that ingest/ledgerbackend/configs/captive-core-pubnet.cfg was loaded
	require.Equal(t, ccConfig.BinaryPath, "test/stellar-core")
	require.Equal(t, ccConfig.NetworkPassphrase, network.PublicNetworkPassphrase)
	require.Equal(t, ccConfig.HistoryArchiveURLs, []string{"http://testarchive"})
	require.Empty(t, ccConfig.Toml.HistoryEntries)
	require.Len(t, ccConfig.Toml.Validators, 23)
	require.Equal(t, ccConfig.Toml.Validators[0].Name, "bootes")
}

func TestInvalidCaptiveCoreTomlPath(t *testing.T) {
	_, err := NewConfig(
		RuntimeSettings{ConfigFilePath: "test/invalid_captive_core_toml_path.toml"},
		nil)
	require.ErrorContains(t, err, "Failed to load captive-core-toml-path file")
}

func TestValidateStartAndEndLedger(t *testing.T) {
	latestNetworkLedger := uint32(20000)
	latestNetworkLedgerPadding := historyarchive.DefaultCheckpointFrequency * 2

	tests := []struct {
		name        string
		startLedger uint32
		endLedger   uint32
		errMsg      string
		mode        Mode
		mockHas     bool
	}{
		{
			name:        "End ledger same as latest ledger",
			startLedger: 512,
			endLedger:   512,
			mode:        ScanFill,
			errMsg:      "invalid end value, must be greater than start",
			mockHas:     false,
		},
		{
			name:        "End ledger greater than start ledger",
			startLedger: 512,
			endLedger:   600,
			mode:        ScanFill,
			errMsg:      "",
			mockHas:     true,
		},
		{
			name:        "No end ledger provided, append mode, no error",
			startLedger: 512,
			endLedger:   0,
			mode:        Append,
			errMsg:      "",
			mockHas:     true,
		},
		{
			name:        "No end ledger provided, scan-and-fill error",
			startLedger: 512,
			endLedger:   0,
			mode:        ScanFill,
			errMsg:      "invalid end value, unbounded mode not supported, end must be greater than start.",
		},
		{
			name:        "End ledger before start ledger",
			startLedger: 512,
			endLedger:   2,
			mode:        ScanFill,
			errMsg:      "invalid end value, must be greater than start",
		},
		{
			name:        "End ledger exceeds latest ledger",
			startLedger: 512,
			endLedger:   latestNetworkLedger + latestNetworkLedgerPadding + 1,
			mode:        ScanFill,
			mockHas:     true,
			errMsg: fmt.Sprintf("end %d exceeds latest network ledger %d",
				latestNetworkLedger+latestNetworkLedgerPadding+1, latestNetworkLedger+latestNetworkLedgerPadding),
		},
		{
			name:        "Start ledger 0",
			startLedger: 0,
			endLedger:   2,
			mode:        ScanFill,
			errMsg:      "invalid start value, must be greater than one.",
		},
		{
			name:        "Start ledger 1",
			startLedger: 1,
			endLedger:   2,
			mode:        ScanFill,
			errMsg:      "invalid start value, must be greater than one.",
		},
		{
			name:        "Start ledger exceeds latest ledger",
			startLedger: latestNetworkLedger + latestNetworkLedgerPadding + 1,
			endLedger:   latestNetworkLedger + latestNetworkLedgerPadding + 2,
			mode:        ScanFill,
			mockHas:     true,
			errMsg: fmt.Sprintf("start %d exceeds latest network ledger %d",
				latestNetworkLedger+latestNetworkLedgerPadding+1, latestNetworkLedger+latestNetworkLedgerPadding),
		},
	}

	ctx := context.Background()
	mockArchive := &historyarchive.MockArchive{}
	mockArchive.On("GetLatestLedgerSequence").Return(latestNetworkLedger, nil)
	mockArchive.On("GetCheckpointManager").
		Return(historyarchive.NewCheckpointManager(
			historyarchive.DefaultCheckpointFrequency))

	mockedHasCtr := 0
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockHas {
				mockedHasCtr++
			}
			config, err := NewConfig(
				RuntimeSettings{StartLedger: tt.startLedger, EndLedger: tt.endLedger, ConfigFilePath: "test/validate_start_end.toml", Mode: tt.mode}, nil)
			require.NoError(t, err)
			err = config.ValidateAndSetLedgerRange(ctx, mockArchive)
			if tt.errMsg != "" {
				require.Error(t, err)
				require.Equal(t, tt.errMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
	mockArchive.AssertExpectations(t)
}

func TestAdjustedLedgerRangeBoundedMode(t *testing.T) {
	tests := []struct {
		name          string
		configFile    string
		start         uint32
		end           uint32
		expectedStart uint32
		expectedEnd   uint32
	}{
		{
			name:          "No change, 1 ledger per file",
			configFile:    "test/1perfile.toml",
			start:         2,
			end:           3,
			expectedStart: 2,
			expectedEnd:   3,
		},
		{
			name:          "Min start ledger2, round up end ledger, 10 ledgers per file",
			configFile:    "test/10perfile.toml",
			start:         2,
			end:           3,
			expectedStart: 2,
			expectedEnd:   9,
		},
		{
			name:          "Round down start ledger and round up end ledger, 15 ledgers per file ",
			configFile:    "test/15perfile.toml",
			start:         4,
			end:           10,
			expectedStart: 2,
			expectedEnd:   14,
		},
		{
			name:          "Round down start ledger and round up end ledger, 64 ledgers per file ",
			configFile:    "test/64perfile.toml",
			start:         400,
			end:           500,
			expectedStart: 384,
			expectedEnd:   511,
		},
		{
			name:          "No change, 64 ledger per file",
			configFile:    "test/64perfile.toml",
			start:         64,
			end:           128,
			expectedStart: 64,
			expectedEnd:   191,
		},
	}

	ctx := context.Background()
	mockArchive := &historyarchive.MockArchive{}
	mockArchive.On("GetLatestLedgerSequence").Return(uint32(500), nil)
	mockArchive.On("GetCheckpointManager").
		Return(historyarchive.NewCheckpointManager(
			historyarchive.DefaultCheckpointFrequency))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewConfig(
				RuntimeSettings{StartLedger: tt.start, EndLedger: tt.end, ConfigFilePath: tt.configFile, Mode: ScanFill}, nil)

			require.NoError(t, err)
			err = config.ValidateAndSetLedgerRange(ctx, mockArchive)
			require.NoError(t, err)
			require.EqualValues(t, tt.expectedStart, config.StartLedger)
			require.EqualValues(t, tt.expectedEnd, config.EndLedger)
		})
	}
	mockArchive.AssertExpectations(t)
}

func TestAdjustedLedgerRangeUnBoundedMode(t *testing.T) {
	tests := []struct {
		name          string
		configFile    string
		start         uint32
		end           uint32
		expectedStart uint32
		expectedEnd   uint32
	}{
		{
			name:          "No change, 1 ledger per file",
			configFile:    "test/1perfile.toml",
			start:         2,
			end:           0,
			expectedStart: 2,
			expectedEnd:   0,
		},
		{
			name:          "Round down start ledger, 15 ledgers per file ",
			configFile:    "test/15perfile.toml",
			start:         4,
			end:           0,
			expectedStart: 2,
			expectedEnd:   0,
		},
		{
			name:          "Round down start ledger, 64 ledgers per file ",
			configFile:    "test/64perfile.toml",
			start:         400,
			end:           0,
			expectedStart: 384,
			expectedEnd:   0,
		},
		{
			name:          "No change, 64 ledger per file",
			configFile:    "test/64perfile.toml",
			start:         64,
			end:           0,
			expectedStart: 64,
			expectedEnd:   0,
		},
	}

	ctx := context.Background()

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.On("GetLatestLedgerSequence").Return(uint32(500), nil)
	mockArchive.On("GetCheckpointManager").
		Return(historyarchive.NewCheckpointManager(
			historyarchive.DefaultCheckpointFrequency))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewConfig(
				RuntimeSettings{StartLedger: tt.start, EndLedger: tt.end, ConfigFilePath: tt.configFile, Mode: Append}, nil)
			require.NoError(t, err)
			err = config.ValidateAndSetLedgerRange(ctx, mockArchive)
			require.NoError(t, err)
			require.EqualValues(t, tt.expectedStart, config.StartLedger)
			require.EqualValues(t, tt.expectedEnd, config.EndLedger)
		})
	}
	mockArchive.AssertExpectations(t)
}

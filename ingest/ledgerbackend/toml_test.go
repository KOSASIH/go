package ledgerbackend

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUint(v uint) *uint {
	return &v
}

func newString(s string) *string {
	return &s
}

func TestCaptiveCoreTomlValidation(t *testing.T) {
	for _, testCase := range []struct {
		name              string
		networkPassphrase string
		appendPath        string
		httpPort          *uint
		peerPort          *uint
		logPath           *string
		expectedError     string
		inMemory          bool
	}{
		{
			name:              "mismatching NETWORK_PASSPHRASE",
			networkPassphrase: "bogus passphrase",
			appendPath:        filepath.Join("testdata", "appendix-with-fields.cfg"),
			httpPort:          newUint(6789),
			peerPort:          newUint(12345),
			logPath:           nil,
			expectedError: "invalid captive core toml: NETWORK_PASSPHRASE in captive core config file: " +
				"Public Global Stellar Network ; September 2015 does not match passed configuration (bogus passphrase)",
		},
		{
			name:              "mismatching HTTP_PORT",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "appendix-with-fields.cfg"),
			httpPort:          newUint(1161),
			peerPort:          newUint(12345),
			logPath:           nil,
			expectedError: "invalid captive core toml: HTTP_PORT in captive core config file: 6789 " +
				"does not match passed configuration (1161)",
		},
		{
			name:              "mismatching PEER_PORT",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "appendix-with-fields.cfg"),
			httpPort:          newUint(6789),
			peerPort:          newUint(2346),
			logPath:           nil,
			expectedError: "invalid captive core toml: PEER_PORT in captive core config file: 12345 " +
				"does not match passed configuration (2346)",
		},
		{
			name:              "mismatching LOG_FILE_PATH",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "appendix-with-fields.cfg"),
			httpPort:          newUint(6789),
			peerPort:          newUint(12345),
			logPath:           newString("/my/test/path"),
			expectedError: "invalid captive core toml: LOG_FILE_PATH in captive core config file:  " +
				"does not match passed configuration (/my/test/path)",
		},
		{
			name:              "duplicate HOME_DOMAIN entry",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "duplicate-home-domain.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found duplicate home domain in captive " +
				"core configuration: testnet.stellar.org",
		},
		{
			name:              "empty HOME_DOMAIN entry",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "empty-home-domain.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid home domain entry which is " +
				"missing a HOME_DOMAIN value",
		},
		{
			name:              "HOME_DOMAIN with empty QUALITY",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "empty-home-domain-quality.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid home domain entry which is " +
				"missing a QUALITY value: testnet.stellar.org",
		},
		{
			name:              "HOME_DOMAIN with invalid QUALITY",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "invalid-home-domain-quality.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid home domain entry which has an " +
				"invalid QUALITY value: testnet.stellar.org",
		},
		{
			name:              "duplicate VALIDATOR entry",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "duplicate-validator.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found duplicate validator in captive core " +
				"configuration: sdf_testnet_1",
		},
		{
			name:              "VALIDATOR with invalid public key",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "validator-has-invalid-public-key.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid validator entry which has an invalid " +
				"PUBLIC_KEY : sdf_testnet_2",
		},
		{
			name:              "VALIDATOR with invalid quality",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "validator-has-invalid-quality.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid validator entry which has an invalid " +
				"QUALITY value: sdf_testnet_2",
		},
		{
			name:              "VALIDATOR without home domain",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "validator-missing-home-domain.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid validator entry which is missing a " +
				"HOME_DOMAIN value: sdf_testnet_1",
		},
		{
			name:              "VALIDATOR without name",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "validator-missing-name.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid validator entry which is missing " +
				"a NAME value: ",
		},
		{
			name:              "VALIDATOR without public key",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "validator-missing-public-key.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid validator entry which is missing " +
				"a PUBLIC_KEY value: sdf_testnet_1",
		},
		{
			name:              "VALIDATOR without quality",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "validator-missing-quality.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError: "invalid captive core toml: found invalid validator entry which is missing " +
				"a QUALITY value: sdf_testnet_2",
		},
		{
			name:              "field not supported by captive core",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "invalid-captive-core-field.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError:     "could not unmarshal captive core toml: these fields are not supported by captive core: [\"CATCHUP_RECENT\"]",
		},
		{
			name:              "database field was invalid for captive core",
			networkPassphrase: "Public Global Stellar Network ; September 2015",
			appendPath:        filepath.Join("testdata", "invalid-captive-core-database-field.cfg"),
			httpPort:          nil,
			peerPort:          nil,
			logPath:           nil,
			expectedError:     `invalid captive core toml: invalid DATABASE parameter: postgres://mydb, for captive core config, must be valid sqlite3 db url`,
		},
		{
			name:          "unexpected BUCKET_DIR_PATH",
			appendPath:    filepath.Join("testdata", "appendix-with-bucket-dir-path.cfg"),
			expectedError: "could not unmarshal captive core toml: setting BUCKET_DIR_PATH is disallowed for Captive Core, use CAPTIVE_CORE_STORAGE_PATH instead",
		},
		{
			name:       "invalid DEPRECATED_SQL_LEDGER_STATE on-disk",
			appendPath: filepath.Join("testdata", "sample-appendix-on-disk.cfg"),
			expectedError: "invalid captive core toml: CAPTIVE_CORE_USE_DB parameter is set to true, indicating " +
				"stellar-core on-disk mode, in which DEPRECATED_SQL_LEDGER_STATE must be set to false",
		},
		{
			name:       "invalid DEPRECATED_SQL_LEDGER_STATE in-memory",
			appendPath: filepath.Join("testdata", "sample-appendix-in-memory.cfg"),
			expectedError: "invalid captive core toml: CAPTIVE_CORE_USE_DB parameter is set to false, indicating " +
				"stellar-core in-memory mode, in which DEPRECATED_SQL_LEDGER_STATE must be set to true",
			inMemory: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			params := CaptiveCoreTomlParams{
				NetworkPassphrase:  testCase.networkPassphrase,
				HistoryArchiveURLs: []string{"http://localhost:1170"},
				HTTPPort:           testCase.httpPort,
				PeerPort:           testCase.peerPort,
				LogPath:            testCase.logPath,
				Strict:             true,
				UseDB:              !testCase.inMemory,
			}
			_, err := NewCaptiveCoreTomlFromFile(testCase.appendPath, params)
			assert.EqualError(t, err, testCase.expectedError)
		})
	}
}

func TestGenerateConfig(t *testing.T) {
	for _, testCase := range []struct {
		name                           string
		appendPath                     string
		mode                           stellarCoreRunnerMode
		expectedPath                   string
		httpPort                       *uint
		peerPort                       *uint
		logPath                        *string
		useDB                          bool
		enforceSorobanDiagnosticEvents bool
		enforceEmitMetaV1              bool
	}{
		{
			name:         "offline config with no appendix",
			mode:         stellarCoreRunnerModeOffline,
			appendPath:   "",
			expectedPath: filepath.Join("testdata", "expected-offline-core.cfg"),
			httpPort:     newUint(6789),
			peerPort:     newUint(12345),
			logPath:      nil,
			useDB:        true,
		},
		{
			name:         "offline config with no peer port",
			mode:         stellarCoreRunnerModeOffline,
			appendPath:   "",
			expectedPath: filepath.Join("testdata", "expected-offline-with-no-peer-port.cfg"),
			httpPort:     newUint(6789),
			peerPort:     nil,
			logPath:      newString("/var/stellar-core/test.log"),
		},
		{
			name:         "online config with appendix",
			mode:         stellarCoreRunnerModeOnline,
			appendPath:   filepath.Join("testdata", "sample-appendix.cfg"),
			expectedPath: filepath.Join("testdata", "expected-online-core.cfg"),
			httpPort:     newUint(6789),
			peerPort:     newUint(12345),
			logPath:      nil,
		},
		{
			name:         "online config with unsupported field in appendix",
			mode:         stellarCoreRunnerModeOnline,
			appendPath:   filepath.Join("testdata", "invalid-captive-core-field.cfg"),
			expectedPath: filepath.Join("testdata", "expected-online-core.cfg"),
			httpPort:     newUint(6789),
			peerPort:     newUint(12345),
			logPath:      nil,
		},
		{
			name:         "online config with no peer port",
			mode:         stellarCoreRunnerModeOnline,
			appendPath:   filepath.Join("testdata", "sample-appendix.cfg"),
			expectedPath: filepath.Join("testdata", "expected-online-with-no-peer-port.cfg"),
			httpPort:     newUint(6789),
			peerPort:     nil,
			logPath:      newString("/var/stellar-core/test.log"),
		},
		{
			name:         "online config with no http port",
			mode:         stellarCoreRunnerModeOnline,
			appendPath:   filepath.Join("testdata", "sample-appendix.cfg"),
			expectedPath: filepath.Join("testdata", "expected-online-with-no-http-port.cfg"),
			httpPort:     nil,
			peerPort:     newUint(12345),
			logPath:      nil,
		},
		{
			name:         "offline config with appendix",
			mode:         stellarCoreRunnerModeOffline,
			appendPath:   filepath.Join("testdata", "sample-appendix.cfg"),
			expectedPath: filepath.Join("testdata", "expected-offline-with-appendix-core.cfg"),
			httpPort:     newUint(6789),
			peerPort:     newUint(12345),
			logPath:      nil,
		},
		{
			name:         "offline config with extra fields in appendix",
			mode:         stellarCoreRunnerModeOffline,
			appendPath:   filepath.Join("testdata", "appendix-with-fields.cfg"),
			expectedPath: filepath.Join("testdata", "expected-offline-with-extra-fields.cfg"),
			httpPort:     newUint(6789),
			peerPort:     newUint(12345),
			logPath:      nil,
		},
		{
			name:                           "offline config with enforce diagnostic events and metav1",
			mode:                           stellarCoreRunnerModeOffline,
			expectedPath:                   filepath.Join("testdata", "expected-offline-enforce-diag-events-and-metav1.cfg"),
			logPath:                        nil,
			enforceSorobanDiagnosticEvents: true,
			enforceEmitMetaV1:              true,
		},
		{
			name:                           "offline config disabling enforced diagnostic events and metav1",
			mode:                           stellarCoreRunnerModeOffline,
			expectedPath:                   filepath.Join("testdata", "expected-offline-enforce-disabled-diagnostic-events.cfg"),
			appendPath:                     filepath.Join("testdata", "appendix-disable-diagnostic-events-and-metav1.cfg"),
			logPath:                        nil,
			enforceSorobanDiagnosticEvents: true,
			enforceEmitMetaV1:              true,
		},
		{
			name:                           "online config with enforce diagnostic events and meta v1",
			mode:                           stellarCoreRunnerModeOnline,
			appendPath:                     filepath.Join("testdata", "sample-appendix.cfg"),
			expectedPath:                   filepath.Join("testdata", "expected-online-with-no-http-port-diag-events-metav1.cfg"),
			httpPort:                       nil,
			peerPort:                       newUint(12345),
			logPath:                        nil,
			enforceSorobanDiagnosticEvents: true,
			enforceEmitMetaV1:              true,
		},
		{
			name:         "offline config with minimum persistent entry in appendix",
			mode:         stellarCoreRunnerModeOnline,
			appendPath:   filepath.Join("testdata", "appendix-with-minimum-persistent-entry.cfg"),
			expectedPath: filepath.Join("testdata", "expected-online-with-appendix-minimum-persistent-entry.cfg"),
			logPath:      nil,
		},
		{
			name:         "default BucketlistDB config",
			mode:         stellarCoreRunnerModeOnline,
			appendPath:   filepath.Join("testdata", "sample-appendix.cfg"),
			expectedPath: filepath.Join("testdata", "expected-default-bucketlistdb-core.cfg"),
			useDB:        true,
			logPath:      nil,
		},
		{
			name:         "BucketlistDB config in appendix",
			mode:         stellarCoreRunnerModeOnline,
			appendPath:   filepath.Join("testdata", "sample-appendix-bucketlistdb.cfg"),
			expectedPath: filepath.Join("testdata", "expected-bucketlistdb-core.cfg"),
			useDB:        true,
			logPath:      nil,
		},
		{
			name:         "Query parameters in appendix",
			mode:         stellarCoreRunnerModeOnline,
			appendPath:   filepath.Join("testdata", "sample-appendix-query-params.cfg"),
			expectedPath: filepath.Join("testdata", "expected-query-params.cfg"),
			useDB:        true,
			logPath:      nil,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var err error
			var captiveCoreToml *CaptiveCoreToml
			params := CaptiveCoreTomlParams{
				NetworkPassphrase:                  "Public Global Stellar Network ; September 2015",
				HistoryArchiveURLs:                 []string{"http://localhost:1170"},
				HTTPPort:                           testCase.httpPort,
				PeerPort:                           testCase.peerPort,
				LogPath:                            testCase.logPath,
				Strict:                             false,
				UseDB:                              testCase.useDB,
				EnforceSorobanDiagnosticEvents:     testCase.enforceSorobanDiagnosticEvents,
				EnforceSorobanTransactionMetaExtV1: testCase.enforceEmitMetaV1,
			}
			if testCase.appendPath != "" {
				captiveCoreToml, err = NewCaptiveCoreTomlFromFile(testCase.appendPath, params)
			} else {
				captiveCoreToml, err = NewCaptiveCoreToml(params)
			}
			assert.NoError(t, err)

			configBytes, err := generateConfig(captiveCoreToml, testCase.mode)
			assert.NoError(t, err)

			expectedByte, err := ioutil.ReadFile(testCase.expectedPath)
			assert.NoError(t, err)

			assert.Equal(t, string(expectedByte), string(configBytes))
		})
	}
}

func TestGenerateCoreConfigInMemory(t *testing.T) {
	appendPath := filepath.Join("testdata", "sample-appendix.cfg")
	expectedPath := filepath.Join("testdata", "expected-in-mem-core.cfg")
	var err error
	var captiveCoreToml *CaptiveCoreToml
	params := CaptiveCoreTomlParams{
		NetworkPassphrase:  "Public Global Stellar Network ; September 2015",
		HistoryArchiveURLs: []string{"http://localhost:1170"},
		Strict:             false,
		UseDB:              false,
	}
	captiveCoreToml, err = NewCaptiveCoreTomlFromFile(appendPath, params)
	assert.NoError(t, err)

	configBytes, err := generateConfig(captiveCoreToml, stellarCoreRunnerModeOnline)
	assert.NoError(t, err)

	expectedByte, err := ioutil.ReadFile(expectedPath)
	assert.NoError(t, err)

	assert.Equal(t, string(expectedByte), string(configBytes))
}

func TestHistoryArchiveURLTrailingSlash(t *testing.T) {
	httpPort := uint(8000)
	peerPort := uint(8000)
	logPath := "logPath"

	params := CaptiveCoreTomlParams{
		NetworkPassphrase:  "Public Global Stellar Network ; September 2015",
		HistoryArchiveURLs: []string{"http://localhost:1170/"},
		HTTPPort:           &httpPort,
		PeerPort:           &peerPort,
		LogPath:            &logPath,
		Strict:             false,
	}

	captiveCoreToml, err := NewCaptiveCoreToml(params)
	assert.NoError(t, err)
	assert.Len(t, captiveCoreToml.HistoryEntries, 1)
	for _, entry := range captiveCoreToml.HistoryEntries {
		assert.Equal(t, "curl -sf http://localhost:1170/{0} -o {1}", entry.Get)
	}
}

func TestExternalStorageConfigUsesDatabaseToml(t *testing.T) {
	var err error
	var captiveCoreToml *CaptiveCoreToml
	httpPort := uint(8000)
	peerPort := uint(8000)
	logPath := "logPath"

	params := CaptiveCoreTomlParams{
		NetworkPassphrase:  "Public Global Stellar Network ; September 2015",
		HistoryArchiveURLs: []string{"http://localhost:1170"},
		HTTPPort:           &httpPort,
		PeerPort:           &peerPort,
		LogPath:            &logPath,
		Strict:             false,
	}

	captiveCoreToml, err = NewCaptiveCoreToml(params)
	assert.NoError(t, err)
	captiveCoreToml.Database = "sqlite3:///etc/defaults/stellar.db"

	configBytes, err := generateConfig(captiveCoreToml, stellarCoreRunnerModeOffline)

	assert.NoError(t, err)
	toml := CaptiveCoreToml{}
	require.NoError(t, toml.unmarshal(configBytes, true))
	assert.Equal(t, toml.Database, "sqlite3:///etc/defaults/stellar.db")
}

func TestDBConfigDefaultsToSqlite(t *testing.T) {
	var err error
	var captiveCoreToml *CaptiveCoreToml
	httpPort := uint(8000)
	peerPort := uint(8000)
	logPath := "logPath"

	params := CaptiveCoreTomlParams{
		NetworkPassphrase:  "Public Global Stellar Network ; September 2015",
		HistoryArchiveURLs: []string{"http://localhost:1170"},
		HTTPPort:           &httpPort,
		PeerPort:           &peerPort,
		LogPath:            &logPath,
		Strict:             false,
		UseDB:              true,
	}

	captiveCoreToml, err = NewCaptiveCoreToml(params)
	assert.NoError(t, err)

	configBytes, err := generateConfig(captiveCoreToml, stellarCoreRunnerModeOffline)

	assert.NoError(t, err)
	toml := CaptiveCoreToml{}
	require.NoError(t, toml.unmarshal(configBytes, true))
	assert.Equal(t, toml.Database, "sqlite3://stellar.db")
	assert.Equal(t, *toml.DeprecatedSqlLedgerState, false)
	assert.Equal(t, *toml.BucketListDBPageSizeExp, defaultBucketListDBPageSize)
	assert.Equal(t, toml.BucketListDBCutoff, (*uint)(nil))
}

func TestNonDBConfigDoesNotUpdateDatabase(t *testing.T) {
	var err error
	var captiveCoreToml *CaptiveCoreToml
	httpPort := uint(8000)
	peerPort := uint(8000)
	logPath := "logPath"

	// UseDB not set, which means it's false
	params := CaptiveCoreTomlParams{
		NetworkPassphrase:  "Public Global Stellar Network ; September 2015",
		HistoryArchiveURLs: []string{"http://localhost:1170"},
		HTTPPort:           &httpPort,
		PeerPort:           &peerPort,
		LogPath:            &logPath,
		Strict:             false,
	}

	captiveCoreToml, err = NewCaptiveCoreToml(params)
	assert.NoError(t, err)

	configBytes, err := generateConfig(captiveCoreToml, stellarCoreRunnerModeOffline)

	assert.NoError(t, err)
	toml := CaptiveCoreToml{}
	require.NoError(t, toml.unmarshal(configBytes, true))
	assert.Equal(t, toml.Database, "")
}

func TestHTTPQueryParameters(t *testing.T) {
	var err error
	var captiveCoreToml *CaptiveCoreToml
	httpPort := uint(8000)
	peerPort := uint(8000)
	logPath := "logPath"

	params := CaptiveCoreTomlParams{
		NetworkPassphrase:  "Public Global Stellar Network ; September 2015",
		HistoryArchiveURLs: []string{"http://localhost:1170"},
		HTTPPort:           &httpPort,
		PeerPort:           &peerPort,
		LogPath:            &logPath,
		Strict:             false,
		UseDB:              true,
		HTTPQueryServerParams: &HTTPQueryServerParams{
			Port:            100,
			ThreadPoolSize:  200,
			SnapshotLedgers: 300,
		},
	}

	captiveCoreToml, err = NewCaptiveCoreToml(params)
	assert.NoError(t, err)

	configBytes, err := generateConfig(captiveCoreToml, stellarCoreRunnerModeOffline)

	assert.NoError(t, err)
	toml := CaptiveCoreToml{}
	require.NoError(t, toml.unmarshal(configBytes, true))
	require.NotNil(t, *toml.HTTPQueryPort)
	assert.Equal(t, *toml.HTTPQueryPort, uint(100))
	require.NotNil(t, *toml.QueryThreadPoolSize)
	assert.Equal(t, *toml.QueryThreadPoolSize, uint(200))
	require.NotNil(t, *toml.QuerySnapshotLedgers)
	assert.Equal(t, *toml.QuerySnapshotLedgers, uint(300))
}

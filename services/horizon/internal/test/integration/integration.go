//lint:file-ignore U1001 Ignore all unused code, this is only used in tests.
package integration

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/stellar/go/historyarchive"
	"github.com/stellar/go/ingest/ledgerbackend"

	"github.com/stellar/go/services/horizon/internal/test"

	"github.com/2opremio/pretty"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/clients/stellarcore"
	"github.com/stellar/go/keypair"
	proto "github.com/stellar/go/protocols/horizon"
	horizoncmd "github.com/stellar/go/services/horizon/cmd"
	horizon "github.com/stellar/go/services/horizon/internal"
	"github.com/stellar/go/services/horizon/internal/ingest"
	"github.com/stellar/go/support/config"
	"github.com/stellar/go/support/db/dbtest"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

const (
	StandaloneNetworkPassphrase = "Standalone Network ; February 2017"
	HorizonDefaultPort          = "8000"
	AdminPort                   = 6060
	StellarCorePort             = 11626
	HistoryArchivePort          = 1570
	StellarRPCPort              = 8080
	HistoryArchiveUrl           = "http://localhost:1570"
	CheckpointFrequency         = 8
)

const (
	StellarCoreURL = "http://localhost:11626"
)

const HorizonInitErrStr = "cannot initialize Horizon"

type Config struct {
	ProtocolVersion           uint32
	EnableStellarRPC          bool
	LogContainers             bool
	SkipCoreContainerCreation bool
	CoreDockerImage           string
	StellarRPCDockerImage     string
	SkipProtocolUpgrade       bool
	QuickExpiration           bool
	NetworkPassphrase         string

	// Weird naming here because bools default to false, but we want to start
	// Horizon by default.
	SkipHorizonStart bool

	// If you want to override the default parameters passed to Horizon, you can
	// set this map accordingly. All of them are passed along as --k=v, but if
	// you pass an empty value, the parameter will be dropped. (Note that you
	// should exclude the prepending `--` from keys; this is for compatibility
	// with the constant names in flags.go)
	//
	// You can also control the environmental variables in a similar way, but
	// note that CLI args take precedence over envvars, so set the corresponding
	// CLI arg empty.
	HorizonWebParameters    map[string]string
	HorizonIngestParameters map[string]string
	HorizonEnvironment      map[string]string
}

type CaptiveConfig struct {
	binaryPath  string
	configPath  string
	storagePath string
}

type Test struct {
	t *testing.T

	composePath       string
	validatorConfPath string
	rpcCoreConfPath   string

	config              Config
	coreConfig          CaptiveConfig
	horizonIngestConfig horizon.Config
	horizonWebConfig    horizon.Config
	testDB              *dbtest.DB
	environment         *test.EnvironmentManager

	horizonClient      *sdk.Client
	horizonAdminClient *sdk.AdminClient
	coreClient         *stellarcore.Client

	webNode          *horizon.App
	ingestNode       *horizon.App
	appStopped       *sync.WaitGroup
	shutdownOnce     sync.Once
	shutdownCalls    []func()
	masterKey        *keypair.Full
	passPhrase       string
	coreUpgradeState *CoreUpgradeState
}

type CoreUpgradeState struct {
	maxUpgradeLedger uint32
}

// GetTestConfig returns the default test Config required to run NewTest.
func GetTestConfig() *Config {
	return &Config{
		ProtocolVersion:           17,
		SkipHorizonStart:          true,
		SkipCoreContainerCreation: false,
		HorizonIngestParameters:   map[string]string{},
		HorizonEnvironment:        map[string]string{},
	}
}

// NewTest starts a new environment for integration test at a given
// protocol version and blocks until Horizon starts ingesting.
//
// Skips the test if HORIZON_INTEGRATION_TESTS env variable is not set.
//
// WARNING: This requires Docker Compose installed.
func NewTest(t *testing.T, config Config) *Test {
	if os.Getenv("HORIZON_INTEGRATION_TESTS_ENABLED") == "" {
		t.Skip("skipping integration test: HORIZON_INTEGRATION_TESTS_ENABLED not set")
	}

	if config.ProtocolVersion == 0 {
		// Default to the maximum supported protocol version
		config.ProtocolVersion = ingest.MaxSupportedProtocolVersion
		// If the environment tells us that Core only supports up to certain version,
		// use that.
		maxSupportedCoreProtocolFromEnv := GetCoreMaxSupportedProtocol()
		if maxSupportedCoreProtocolFromEnv != 0 && maxSupportedCoreProtocolFromEnv < ingest.MaxSupportedProtocolVersion {
			config.ProtocolVersion = maxSupportedCoreProtocolFromEnv
		}
	}
	if config.NetworkPassphrase == "" {
		config.NetworkPassphrase = StandaloneNetworkPassphrase
	}
	validatorParams := validatorCoreConfigTemplatePrams{
		Accelerate:                            CheckpointFrequency < historyarchive.DefaultCheckpointFrequency,
		NetworkPassphrase:                     config.NetworkPassphrase,
		TestingMinimumPersistentEntryLifetime: 65536,
		TestingSorobanHighLimitOverride:       false,
	}
	if config.QuickExpiration {
		validatorParams.TestingSorobanHighLimitOverride = true
		validatorParams.TestingMinimumPersistentEntryLifetime = 10
	}
	var i *Test
	if !config.SkipCoreContainerCreation {
		composePath := findDockerComposePath()
		i = &Test{
			t:           t,
			config:      config,
			composePath: composePath,
			passPhrase:  config.NetworkPassphrase,
			environment: test.NewEnvironmentManager(),
		}
		i.validatorConfPath = i.createCoreValidatorConf(validatorParams)
		i.rpcCoreConfPath = i.createCaptiveCoreConf(captiveCoreConfigTemplatePrams{
			validatorCoreConfigTemplatePrams: validatorParams,
			ValidatorAddress:                 "core",
		})
		// Only run Stellar Core container and its dependencies.
		i.startCoreValidator()
	} else {
		i = &Test{
			t:           t,
			config:      config,
			environment: test.NewEnvironmentManager(),
		}
	}
	i.configureCaptiveCore(validatorParams)

	i.prepareShutdownHandlers()
	i.coreClient = &stellarcore.Client{URL: "http://localhost:" + strconv.Itoa(StellarCorePort)}
	if !config.SkipCoreContainerCreation {
		i.waitForCore()
		if i.config.EnableStellarRPC {
			i.startRPC()
			i.waitForStellarRPC()
		}
	}

	if !config.SkipHorizonStart {
		if innerErr := i.StartHorizon(true); innerErr != nil {
			t.Fatalf("Failed to start Horizon: %v", innerErr)
		}

		i.WaitForHorizonIngest()
	}

	return i
}

func (i *Test) configureCaptiveCore(validatorParams validatorCoreConfigTemplatePrams) {
	i.coreConfig.binaryPath = os.Getenv("HORIZON_INTEGRATION_TESTS_CAPTIVE_CORE_BIN")
	i.coreConfig.configPath = i.createCaptiveCoreConf(captiveCoreConfigTemplatePrams{
		validatorCoreConfigTemplatePrams: validatorParams,
		ValidatorAddress:                 "localhost",
	})
	i.coreConfig.storagePath = i.CurrentTest().TempDir()

	if value := i.getIngestParameter(
		horizon.StellarCoreBinaryPathName,
		"STELLAR_CORE_BINARY_PATH",
	); value != "" {
		i.coreConfig.binaryPath = value
	}
	if value := i.getIngestParameter(
		horizon.CaptiveCoreConfigPathName,
		"CAPTIVE_CORE_CONFIG_PATH",
	); value != "" {
		i.coreConfig.configPath = value
	}
}

func (i *Test) getIngestParameter(argName, envName string) string {
	if value, ok := i.config.HorizonEnvironment[envName]; ok {
		return value
	}
	if value, ok := i.config.HorizonIngestParameters[argName]; ok {
		return value
	}
	return ""
}

func (i *Test) createCoreValidatorConf(params validatorCoreConfigTemplatePrams) string {
	tomlFile, err := os.CreateTemp("", "stellar-core-integration-test-*.toml")
	require.NoError(i.t, err)

	tmpl, err := template.New("core-validator").Parse(validatorCoreConfigTemplate)
	require.NoError(i.t, err)
	err = tmpl.Execute(tomlFile, params)
	require.NoError(i.t, err)

	require.NoError(i.t, tomlFile.Close())

	i.t.Cleanup(func() {
		require.NoError(i.t, os.Remove(tomlFile.Name()))
	})
	return tomlFile.Name()
}

func (i *Test) createCaptiveCoreConf(params captiveCoreConfigTemplatePrams) string {
	tomlFile, err := os.CreateTemp("", "captive-core-integration-test-*.toml")
	require.NoError(i.t, err)

	tmpl, err := template.New("captive-core").Parse(captiveCoreConfigTemplate)
	require.NoError(i.t, err)
	err = tmpl.Execute(tomlFile, params)
	require.NoError(i.t, err)

	require.NoError(i.t, tomlFile.Close())

	i.t.Cleanup(func() {
		require.NoError(i.t, os.Remove(tomlFile.Name()))
	})
	return tomlFile.Name()
}

// Runs a docker-compose command applied to the above configs
func (i *Test) runComposeCommand(envVars []string, args ...string) {
	cmdline := append(
		[]string{
			"compose",
			"-f",
			filepath.Join(i.composePath, "docker-compose.integration-tests.yml"),
		},
		args...,
	)
	cmd := exec.Command("docker", cmdline...)

	cmd.Env = append(
		envVars,
		fmt.Sprintf("CAPTIVE_CORE_CONFIG_FILE=%s", i.rpcCoreConfPath),
		fmt.Sprintf("CORE_CONFIG_FILE=%s", i.validatorConfPath),
	)

	i.t.Log("Running", cmd.Args)
	out, innerErr := cmd.Output()
	if len(out) > 0 {
		fmt.Printf("stdout:\n%s\n", string(out))
	}
	if exitErr, ok := innerErr.(*exec.ExitError); ok {
		fmt.Printf("stderr:\n%s\n", string(exitErr.Stderr))
	}

	if innerErr != nil {
		i.t.Fatalf("Compose command failed: %v", innerErr)
	}
}

func (i *Test) startCoreValidator() {
	var envVars []string
	var coreImageOverride string

	if i.config.CoreDockerImage != "" {
		coreImageOverride = i.config.CoreDockerImage
	} else if img := os.Getenv("HORIZON_INTEGRATION_TESTS_DOCKER_IMG"); img != "" {
		coreImageOverride = img
	}
	if coreImageOverride != "" {
		envVars = append(
			envVars,
			fmt.Sprintf("CORE_IMAGE=%s", coreImageOverride),
		)
	}

	i.runComposeCommand(envVars, "up", "--detach", "--quiet-pull", "--no-color", "core")
}

func (i *Test) startRPC() {
	var envVars []string
	var stellarRPCOverride string

	if i.config.StellarRPCDockerImage != "" {
		stellarRPCOverride = i.config.CoreDockerImage
	} else if img := os.Getenv("HORIZON_INTEGRATION_TESTS_STELLAR_RPC_DOCKER_IMG"); img != "" {
		stellarRPCOverride = img
	}
	if stellarRPCOverride != "" {
		envVars = append(
			envVars,
			fmt.Sprintf("STELLAR_RPC_IMAGE=%s", stellarRPCOverride),
			fmt.Sprintf("NETWORK_PASSPHRASE=%s", i.config.NetworkPassphrase),
		)
	}

	i.runComposeCommand(envVars, "up", "--detach", "--quiet-pull", "--no-color", "stellar-rpc")
}

func (i *Test) removeContainers(containers ...string) {
	i.runComposeCommand(
		nil,
		append(
			[]string{
				"rm",
				"-fvs",
			},
			containers...,
		)...,
	)
}

func (i *Test) prepareShutdownHandlers() {
	i.shutdownCalls = append(i.shutdownCalls,
		func() {
			if i.webNode != nil {
				i.webNode.Close()
			}
			if i.ingestNode != nil {
				i.ingestNode.Close()
			}
			if !i.config.SkipCoreContainerCreation {
				if i.config.LogContainers {
					i.runComposeCommand(nil, "logs", "core")
				}
				i.removeContainers("core")
				if i.config.EnableStellarRPC {
					if i.config.LogContainers {
						i.runComposeCommand(nil, "logs", "stellar-rpc")
					}
					i.removeContainers("stellar-rpc")
				}
			}
		},
		i.environment.Restore,
	)

	// Register cleanup handlers (on panic and ctrl+c) so the containers are
	// stopped even if ingestion or testing fails.
	i.t.Cleanup(i.Shutdown)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		i.Shutdown()
		os.Exit(int(syscall.SIGTERM))
	}()
}

// if startIngestProcess=true, will restart the ingest sub process also
func (i *Test) RestartHorizon(restartIngestProcess bool) error {
	i.StopHorizon()

	if err := i.StartHorizon(restartIngestProcess); err != nil {
		return err
	}

	i.WaitForHorizonIngest()
	return nil
}

func (i *Test) GetHorizonIngestConfig() horizon.Config {
	return i.horizonIngestConfig
}

func (i *Test) GetHorizonWebConfig() horizon.Config {
	return i.horizonWebConfig
}

func (i *Test) GetTestDB() *dbtest.DB {
	return i.testDB
}

// Shutdown stops the integration tests and destroys all its associated
// resources. It will be implicitly called when the calling test (i.e. the
// `testing.Test` passed to `New()`) is finished if it hasn't been explicitly
// called before.
func (i *Test) Shutdown() {
	i.shutdownOnce.Do(func() {
		// run them in the opposite order in which they where added
		for callI := len(i.shutdownCalls) - 1; callI >= 0; callI-- {
			i.shutdownCalls[callI]()
		}
	})
}

// StartHorizon initializes and starts the Horizon client-facing API server.
// When startIngestProcess=true, start a second process for ingest server
func (i *Test) StartHorizon(startIngestProcess bool) error {
	i.t.Logf("Starting horizon.....")
	i.testDB = dbtest.Postgres(i.t)
	i.shutdownCalls = append(i.shutdownCalls, func() {
		if i.appStopped == nil {
			// appStopped is nil when the horizon cmd.Execute creates an App, but gets an intentional error and StartHorizon
			// never gets to point of running App.Serve() which would have closed the db conn eventually
			// since it wires up listener to App.Close() invocation, so, we must manually detect this edge case and
			// close the app's db here to clean up
			if i.webNode != nil {
				i.webNode.CloseDB()
			}
			if i.ingestNode != nil {
				i.ingestNode.CloseDB()
			}
		}
		i.StopHorizon()
		i.testDB.Close()
	})
	var err error

	// To facilitate custom runs of Horizon, we merge a default set of
	// parameters with the tester-supplied ones (if any).
	mergedWebArgs := MergeMaps(i.getDefaultWebArgs(), i.config.HorizonWebParameters)
	mergedIngestArgs := MergeMaps(i.getDefaultIngestArgs(), i.config.HorizonIngestParameters)

	// Set up Horizon clients
	i.setupHorizonClient(mergedWebArgs)
	if err = i.setupHorizonAdminClient(mergedIngestArgs); err != nil {
		return err
	}

	if err = i.initializeEnvironmentVariables(); err != nil {
		return err
	}

	// setup Horizon web process
	webArgs := mapToFlags(mergedWebArgs)
	i.t.Log("Horizon command line webArgs:", webArgs)
	webConfig, webConfigOpts := horizon.Flags()
	webCmd := i.createWebCommand(webConfig, webConfigOpts)
	webCmd.SetArgs(webArgs)
	if err = webConfigOpts.Init(webCmd); err != nil {
		return errors.Wrap(err, "cannot initialize params")
	}
	if err = webCmd.Execute(); err != nil {
		return errors.Wrap(err, HorizonInitErrStr)
	}
	i.horizonWebConfig = *webConfig

	// setup horizon ingest process
	if startIngestProcess {
		ingestArgs := mapToFlags(mergedIngestArgs)
		i.t.Log("Horizon command line ingestArgs:", ingestArgs)
		// setup Horizon ingest command
		ingestConfig, ingestConfigOpts := horizon.Flags()
		ingestCmd := i.createIngestCommand(ingestConfig, ingestConfigOpts)
		ingestCmd.SetArgs(ingestArgs)
		if err = ingestConfigOpts.Init(ingestCmd); err != nil {
			return errors.Wrap(err, "cannot initialize params")
		}
		if err = ingestCmd.Execute(); err != nil {
			return errors.Wrap(err, HorizonInitErrStr)
		}
		i.horizonIngestConfig = *ingestConfig
	} else {
		// not running ingestion, normally that process would do migration through --apply-migrations
		// so migrage the empty in any case directly
		var rootCmd = horizoncmd.NewRootCmd()
		rootCmd.SetArgs([]string{
			"db", "migrate", "up", "--db-url", i.testDB.DSN})
		require.NoError(i.t, rootCmd.Execute())
	}

	i.appStopped = &sync.WaitGroup{}
	if i.ingestNode != nil {
		i.appStopped.Add(1)
		go func() {
			_ = i.ingestNode.Serve()
			i.appStopped.Done()
		}()
	}

	i.appStopped.Add(1)
	go func() {
		_ = i.webNode.Serve()
		i.appStopped.Done()
	}()

	return nil
}

func (i *Test) getDefaultArgs() map[string]string {
	// TODO: Ideally, we'd be pulling host/port information from the Docker
	//       Compose YAML file itself rather than hardcoding it.
	return map[string]string{
		"ingest":               "false",
		"history-archive-urls": HistoryArchiveUrl,
		"db-url":               i.testDB.RO_DSN,
		"stellar-core-url":     i.coreClient.URL,
		"network-passphrase":   i.passPhrase,
		"apply-migrations":     "true",
		"port":                 HorizonDefaultPort,
		// due to ARTIFICIALLY_ACCELERATE_TIME_FOR_TESTING
		"checkpoint-frequency": strconv.Itoa(CheckpointFrequency),
		"per-hour-rate-limit":  "0",  // disable rate limiting
		"max-db-connections":   "50", // the postgres container supports 100 connections, be conservative
	}
}

func (i *Test) getDefaultWebArgs() map[string]string {
	return MergeMaps(i.getDefaultArgs(), map[string]string{"admin-port": "0"})
}

func (i *Test) getDefaultIngestArgs() map[string]string {
	return MergeMaps(i.getDefaultArgs(), map[string]string{
		"admin-port":                strconv.Itoa(i.AdminPort()),
		"port":                      "8001",
		"db-url":                    i.testDB.DSN,
		"stellar-core-binary-path":  i.coreConfig.binaryPath,
		"captive-core-config-path":  i.coreConfig.configPath,
		"captive-core-http-port":    "21626",
		"captive-core-use-db":       "true",
		"captive-core-storage-path": i.coreConfig.storagePath,
		"ingest":                    "true"})
}

func (i *Test) createWebCommand(webConfig *horizon.Config, webConfigOpts config.ConfigOptions) *cobra.Command {
	webCmd := &cobra.Command{
		Use:   "horizon",
		Short: "Client-facing API server for the Stellar network",
		Long:  "Client-facing API server for the Stellar network.",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			i.webNode, err = horizon.NewAppFromFlags(webConfig, webConfigOpts)
			if err != nil {
				// Explicitly exit here as that's how these tests are structured for now.
				fmt.Println(err)
				os.Exit(1)
			}
		},
	}
	return webCmd
}

func (i *Test) createIngestCommand(ingestConfig *horizon.Config, ingestConfigOpts config.ConfigOptions) *cobra.Command {
	ingestCmd := &cobra.Command{
		Use:   "horizon",
		Short: "Ingest of Stellar network",
		Long:  "Ingest of Stellar network.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			i.ingestNode, err = horizon.NewAppFromFlags(ingestConfig, ingestConfigOpts)
			if err != nil {
				fmt.Println(err)
			}
			return err
		},
	}
	return ingestCmd
}

func (i *Test) initializeEnvironmentVariables() error {
	var env strings.Builder
	for key, value := range i.config.HorizonEnvironment {
		env.WriteString(fmt.Sprintf("%s=%s ", key, value))
	}
	i.t.Logf("Horizon environmental variables: %s\n", env.String())

	// prepare env
	for key, value := range i.config.HorizonEnvironment {
		innerErr := i.environment.Add(key, value)
		if innerErr != nil {
			return errors.Wrap(innerErr, fmt.Sprintf(
				"failed to set envvar (%s=%s)", key, value))
		}
	}
	return nil
}

func (i *Test) setupHorizonAdminClient(ingestArgs map[string]string) error {
	adminPort := uint16(i.AdminPort())
	if port, ok := ingestArgs["admin-port"]; ok {
		if cmdAdminPort, parseErr := strconv.ParseInt(port, 0, 16); parseErr == nil {
			adminPort = uint16(cmdAdminPort)
		}
	}

	var err error
	i.horizonAdminClient, err = sdk.NewAdminClient(adminPort, "", 0)
	if err != nil {
		return errors.Wrap(err, "cannot initialize Horizon admin client")
	}
	return nil
}

func (i *Test) setupHorizonClient(webArgs map[string]string) {
	hostname := "localhost"
	horizonPort := HorizonDefaultPort
	if port, ok := webArgs["port"]; ok {
		horizonPort = port
	}

	i.horizonClient = &sdk.Client{
		HorizonURL: fmt.Sprintf("http://%s:%s", hostname, horizonPort),
	}
}

func (i *Test) CreateCaptiveCoreConfig() (ledgerbackend.CaptiveCoreConfig, error) {
	captiveCoreConfig := ledgerbackend.CaptiveCoreConfig{
		BinaryPath:          i.CoreBinaryPath(),
		HistoryArchiveURLs:  []string{HistoryArchiveUrl},
		NetworkPassphrase:   i.config.NetworkPassphrase,
		CheckpointFrequency: CheckpointFrequency, // This is required for accelerated archive creation for integration test
		UseDB:               true,
		StoragePath:         i.CurrentTest().TempDir(),
	}

	tomlParams := ledgerbackend.CaptiveCoreTomlParams{
		NetworkPassphrase:  i.config.NetworkPassphrase,
		HistoryArchiveURLs: []string{HistoryArchiveUrl},
		UseDB:              true,
	}

	toml, err := ledgerbackend.NewCaptiveCoreTomlFromFile(i.coreConfig.configPath, tomlParams)
	if err != nil {
		return ledgerbackend.CaptiveCoreConfig{}, err
	}

	captiveCoreConfig.Toml = toml
	return captiveCoreConfig, nil
}

const maxWaitForCoreStartup = 30 * time.Second
const maxWaitForCoreUpgrade = 5 * time.Second
const coreStartupPingInterval = time.Second

// Wait for core to be up and manually close the first ledger
func (i *Test) waitForCore() {
	i.t.Log("Waiting for core to be up...")
	startTime := time.Now()
	for time.Since(startTime) < maxWaitForCoreStartup {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		infoTime := time.Now()
		_, err := i.coreClient.Info(ctx)
		cancel()
		if err != nil {
			i.t.Logf("could not obtain info response: %v", err)
			// sleep up to a second between consecutive calls.
			if durationSince := time.Since(infoTime); durationSince < coreStartupPingInterval {
				time.Sleep(coreStartupPingInterval - durationSince)
			}
			continue
		}
		break
	}

	i.UpgradeProtocol(i.config.ProtocolVersion)

	startTime = time.Now()
	for time.Since(startTime) < maxWaitForCoreUpgrade {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		infoTime := time.Now()
		info, err := i.coreClient.Info(ctx)
		cancel()
		if err != nil || !info.IsSynced() {
			i.t.Logf("Core is still not synced: %v %v", err, info)
			// sleep up to a second between consecutive calls.
			if durationSince := time.Since(infoTime); durationSince < coreStartupPingInterval {
				time.Sleep(coreStartupPingInterval - durationSince)
			}
			continue
		}
		i.t.Log("Core is up.")
		return
	}
	i.t.Fatalf("Core could not sync after %v + %v", maxWaitForCoreStartup, maxWaitForCoreUpgrade)
}

const stellarRPCInitTime = 60 * 6 * time.Second
const stellarRPCHealthCheckInterval = time.Second

// Wait for stellar rpc to be up
func (i *Test) waitForStellarRPC() {
	i.t.Log("Waiting for Stellar RPC to be up...")

	start := time.Now()
	for time.Since(start) < stellarRPCInitTime {
		ctx, cancel := context.WithTimeout(context.Background(), stellarRPCHealthCheckInterval)
		// TODO: soroban-tools should be exporting a proper Go client
		ch := jhttp.NewChannel("http://localhost:"+strconv.Itoa(StellarRPCPort), nil)
		stellarRPCClient := jrpc2.NewClient(ch, nil)
		callTime := time.Now()
		_, err := stellarRPCClient.Call(ctx, "getHealth", nil)
		cancel()
		if err != nil {
			i.t.Logf("stellar rpc is unhealthy: %v", err)
			// sleep up to a second between consecutive calls.
			if durationSince := time.Since(callTime); durationSince < stellarRPCHealthCheckInterval {
				time.Sleep(stellarRPCHealthCheckInterval - durationSince)
			}
			continue
		}
		i.t.Log("stellar rpc is up.")
		return
	}

	i.t.Fatalf("stellar rpc unhealthy after %v", time.Since(start))
}

type RPCSimulateHostFunctionResult struct {
	Auth []string `json:"auth"`
	XDR  string   `json:"xdr"`
}

type RPCSimulateTxResponse struct {
	Error           string                          `json:"error,omitempty"`
	TransactionData string                          `json:"transactionData"`
	Results         []RPCSimulateHostFunctionResult `json:"results"`
	MinResourceFee  int64                           `json:"minResourceFee,string"`
}

func (i *Test) PreflightHostFunctions(
	sourceAccount txnbuild.Account, function txnbuild.InvokeHostFunction,
) (txnbuild.InvokeHostFunction, int64) {
	if function.HostFunction.Type == xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
		fmt.Printf("Preflighting function call to: %s\n", string(function.HostFunction.InvokeContract.FunctionName))
	}
	result, transactionData := i.simulateTransaction(sourceAccount, &function)
	function.Ext = xdr.TransactionExt{
		V:           1,
		SorobanData: &transactionData,
	}
	var funAuth []xdr.SorobanAuthorizationEntry
	for _, res := range result.Results {
		var decodedRes xdr.ScVal
		err := xdr.SafeUnmarshalBase64(res.XDR, &decodedRes)
		assert.NoError(i.t, err)
		fmt.Printf("Result:\n\n%# +v\n\n", pretty.Formatter(decodedRes))
		for _, authBase64 := range res.Auth {
			var authEntry xdr.SorobanAuthorizationEntry
			err = xdr.SafeUnmarshalBase64(authBase64, &authEntry)
			assert.NoError(i.t, err)
			fmt.Printf("Auth:\n\n%# +v\n\n", pretty.Formatter(authEntry))
			funAuth = append(funAuth, authEntry)
		}
	}
	function.Auth = funAuth

	return function, result.MinResourceFee
}

func (i *Test) simulateTransaction(
	sourceAccount txnbuild.Account, op txnbuild.Operation,
) (RPCSimulateTxResponse, xdr.SorobanTransactionData) {
	// Before preflighting, make sure stellar-rpc is in sync with Horizon
	root, err := i.horizonClient.Root()
	assert.NoError(i.t, err)
	i.syncWithStellarRPC(uint32(root.HorizonSequence))

	// TODO: soroban-tools should be exporting a proper Go client
	ch := jhttp.NewChannel("http://localhost:"+strconv.Itoa(StellarRPCPort), nil)
	stellarRPCClient := jrpc2.NewClient(ch, nil)
	txParams := GetBaseTransactionParamsWithFee(sourceAccount, txnbuild.MinBaseFee, op)
	txParams.IncrementSequenceNum = false
	tx, err := txnbuild.NewTransaction(txParams)
	assert.NoError(i.t, err)
	base64, err := tx.Base64()
	assert.NoError(i.t, err)
	result := RPCSimulateTxResponse{}
	fmt.Printf("Preflight TX:\n\n%v \n\n", base64)
	err = stellarRPCClient.CallResult(context.Background(), "simulateTransaction", struct {
		Transaction string `json:"transaction"`
	}{base64}, &result)
	assert.NoError(i.t, err)
	assert.Empty(i.t, result.Error)
	var transactionData xdr.SorobanTransactionData
	err = xdr.SafeUnmarshalBase64(result.TransactionData, &transactionData)
	assert.NoError(i.t, err)
	fmt.Printf("Transaction Data:\n\n%# +v\n\n", pretty.Formatter(transactionData))
	return result, transactionData
}

func (i *Test) syncWithStellarRPC(ledgerToWaitFor uint32) {
	for j := 0; j < 20; j++ {
		result := struct {
			Sequence uint32 `json:"sequence"`
		}{}
		ch := jhttp.NewChannel("http://localhost:"+strconv.Itoa(StellarRPCPort), nil)
		stellarRPCClient := jrpc2.NewClient(ch, nil)
		err := stellarRPCClient.CallResult(context.Background(), "getLatestLedger", nil, &result)
		assert.NoError(i.t, err)
		if result.Sequence >= ledgerToWaitFor {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	i.t.Fatal("Time out waiting for stellar-rpc to sync")
}

func (i *Test) WaitUntilLedgerEntryTTL(ledgerKey xdr.LedgerKey) {
	ch := jhttp.NewChannel("http://localhost:"+strconv.Itoa(StellarRPCPort), nil)
	client := jrpc2.NewClient(ch, nil)

	keyB64, err := xdr.MarshalBase64(ledgerKey)
	assert.NoError(i.t, err)
	request := struct {
		Keys []string `json:"keys"`
	}{
		Keys: []string{keyB64},
	}
	ttled := false
	for attempt := 0; attempt < 50; attempt++ {
		var result struct {
			Entries []struct {
				LiveUntilLedgerSeq *uint32 `json:"liveUntilLedgerSeq,omitempty"`
			} `json:"entries"`
		}
		err := client.CallResult(context.Background(), "getLedgerEntries", request, &result)
		assert.NoError(i.t, err)
		if len(result.Entries) > 0 {
			liveUntilLedgerSeq := *result.Entries[0].LiveUntilLedgerSeq

			root, err := i.horizonClient.Root()
			assert.NoError(i.t, err)
			if uint32(root.HorizonSequence) > liveUntilLedgerSeq {
				ttled = true
				i.t.Logf("ledger entry ttl'ed")
				break
			}
			i.t.Log("waiting for ledger entry to ttl at ledger", liveUntilLedgerSeq)
		} else {
			i.t.Log("waiting for stellar-rpc to ingest the ledger entries")
		}
		time.Sleep(time.Second)
	}
	assert.True(i.t, ttled)
}

func (i *Test) PreflightExtendExpiration(
	account string, ledgerKeys []xdr.LedgerKey, extendAmt uint32,
) (proto.Account, txnbuild.ExtendFootprintTtl, int64) {
	sourceAccount, err := i.Client().AccountDetail(sdk.AccountRequest{
		AccountID: account,
	})
	assert.NoError(i.t, err)

	bumpFootprint := txnbuild.ExtendFootprintTtl{
		ExtendTo:      extendAmt,
		SourceAccount: "",
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Ext: xdr.ExtensionPoint{},
				Resources: xdr.SorobanResources{
					Footprint: xdr.LedgerFootprint{
						ReadOnly:  ledgerKeys,
						ReadWrite: nil,
					},
				},
				ResourceFee: 0,
			},
		},
	}
	result, transactionData := i.simulateTransaction(&sourceAccount, &bumpFootprint)
	bumpFootprint.Ext = xdr.TransactionExt{
		V:           1,
		SorobanData: &transactionData,
	}

	return sourceAccount, bumpFootprint, result.MinResourceFee
}

func (i *Test) RestoreFootprint(
	account string, ledgerKey xdr.LedgerKey,
) (proto.Account, txnbuild.RestoreFootprint, int64) {
	sourceAccount, err := i.Client().AccountDetail(sdk.AccountRequest{
		AccountID: account,
	})
	assert.NoError(i.t, err)

	restoreFootprint := txnbuild.RestoreFootprint{
		SourceAccount: "",
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Ext: xdr.ExtensionPoint{},
				Resources: xdr.SorobanResources{
					Footprint: xdr.LedgerFootprint{
						ReadWrite: []xdr.LedgerKey{ledgerKey},
					},
				},
				ResourceFee: 0,
			},
		},
	}
	result, transactionData := i.simulateTransaction(&sourceAccount, &restoreFootprint)
	restoreFootprint.Ext = xdr.TransactionExt{
		V:           1,
		SorobanData: &transactionData,
	}

	return sourceAccount, restoreFootprint, result.MinResourceFee
}

// UpgradeProtocol arms Core with upgrade and blocks until protocol is upgraded.
func (i *Test) UpgradeProtocol(version uint32) {
	i.t.Logf("Attempting Core Protocol upgade to version: %v", version)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err := i.coreClient.Upgrade(ctx, int(version))
	cancel()
	if err != nil {
		i.t.Fatalf("could not upgrade protocol: %v", err)
	}

	for t := 0; t < 10; t++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		info, err := i.coreClient.Info(ctx)
		cancel()
		if err != nil {
			i.t.Logf("could not obtain info response: %v", err)
			time.Sleep(time.Second)
			continue
		}

		ledgerSeq := info.Info.Ledger.Num
		if info.Info.Ledger.Version == int(version) {
			i.t.Logf("Protocol upgraded to: %d, in ledger sequence number: %v, hash: %v",
				info.Info.Ledger.Version, ledgerSeq, info.Info.Ledger.Hash)
			// Mark the fact that the core has been upgraded as of this ledger sequence
			// It could have been earlier than this, but certainly no later.
			// The core upgrade could have happened in any ledger since the coreClient.Upgrade was issued
			i.coreUpgradeState = &CoreUpgradeState{maxUpgradeLedger: uint32(ledgerSeq)}
			return
		}
		time.Sleep(time.Second)
	}

	i.t.Fatalf("could not upgrade protocol in 10s")
}

func (i *Test) WaitForHorizonWeb() {
	// wait until the web server is up before continuing to test requests
	require.Eventually(i.t, func() bool {
		if _, horizonErr := i.Client().Root(); horizonErr != nil {
			return false
		}
		return true
	}, time.Second*15, time.Millisecond*100)
}

func (i *Test) WaitForHorizonIngest() {
	for t := 60; t >= 0; t -= 1 {
		time.Sleep(time.Second)

		i.t.Log("Waiting for ingestion and protocol upgrade...")
		root, err := i.horizonClient.Root()
		if err != nil {
			i.t.Logf("could not obtain root response %v", err)
			continue
		}

		if root.HorizonSequence < 3 ||
			int(root.HorizonSequence) != int(root.IngestSequence) {
			i.t.Logf("Horizon ingesting... %v", root)
			continue
		}

		if uint32(root.CurrentProtocolVersion) == i.config.ProtocolVersion {
			i.t.Logf("Horizon protocol version matches %d: %+v",
				root.CurrentProtocolVersion, root)
			return
		}
	}

	i.t.Fatal("Horizon not ingesting...")
}

// Config returns the testing configuration for the current integration test run.
func (i *Test) Config() Config {
	return i.config
}

// CoreClient returns a stellar core client connected to the Stellar Core instance.
func (i *Test) CoreClient() *stellarcore.Client {
	return i.coreClient
}

func (i *Test) CoreBinaryPath() string {
	if i.coreConfig.binaryPath != "" {
		return i.coreConfig.binaryPath
	}
	corePath, err := exec.LookPath("stellar-core")
	require.NoError(i.t, err)
	return corePath
}

func (i *Test) CaptiveCoreStoragePath() string {
	return i.coreConfig.storagePath
}

// Client returns horizon.Client connected to started Horizon instance.
func (i *Test) Client() *sdk.Client {
	return i.horizonClient
}

// AdminClient returns horizon.Client connected to started Horizon instance.
func (i *Test) AdminClient() *sdk.AdminClient {
	return i.horizonAdminClient
}

// HorizonWeb returns the horizon.App instance for the current integration test
func (i *Test) HorizonWeb() *horizon.App {
	return i.webNode
}

func (i *Test) HorizonIngest() *horizon.App {
	return i.ingestNode
}

// StopHorizon shuts down the running Horizon process
func (i *Test) StopHorizon() {
	if i.webNode != nil {
		i.webNode.Close()
	}
	if i.ingestNode != nil {
		i.ingestNode.Close()
	}

	// Wait for Horizon to shut down completely.
	if i.appStopped != nil {
		i.appStopped.Wait()
	}
	i.webNode = nil
	i.ingestNode = nil
}

// AdminPort returns Horizon admin port.
func (i *Test) AdminPort() int {
	return AdminPort
}

// Metrics URL returns Horizon metrics URL.
func (i *Test) MetricsURL() string {
	return fmt.Sprintf("http://localhost:%d/metrics", i.AdminPort())
}

// AsyncTxSubOpenAPISpecURL returns the URL for getting the openAPI spec yaml for async-txsub endpoint.
func (i *Test) AsyncTxSubOpenAPISpecURL() string {
	return fmt.Sprintf("http://localhost:%d/transactions_async", i.AdminPort())
}

// Master returns a keypair of the network masterKey account.
func (i *Test) Master() *keypair.Full {
	if i.masterKey != nil {
		return i.masterKey
	}
	return keypair.Master(i.passPhrase).(*keypair.Full)
}

func (i *Test) MasterAccount() txnbuild.Account {
	account := i.MasterAccountDetails()
	return &account
}

func (i *Test) MasterAccountDetails() proto.Account {
	return i.MustGetAccount(i.Master())
}

func (i *Test) CurrentTest() *testing.T {
	return i.t
}

/* Utility functions for easier test case creation. */

// Creates new accounts via the master account.
//
// It funds each account with the given balance and then queries the API to
// find the randomized sequence number for future operations.
//
// Returns: The slice of created keypairs and account objects.
//
// Note: panics on any errors, since we assume that tests cannot proceed without
// this method succeeding.
func (i *Test) CreateAccounts(count int, initialBalance string) ([]*keypair.Full, []txnbuild.Account) {
	client := i.Client()
	master := i.Master()

	pairs := make([]*keypair.Full, count)
	ops := make([]txnbuild.Operation, count)

	// Two paths here: either caller already did some stuff with the master
	// account so we should retrieve the sequence number, or caller hasn't and
	// we start from scratch.
	request := sdk.AccountRequest{AccountID: master.Address()}
	account, err := client.AccountDetail(request)
	panicIf(err)

	masterAccount := txnbuild.SimpleAccount{
		AccountID: master.Address(),
		Sequence:  account.Sequence,
	}

	for i := 0; i < count; i++ {
		pair, _ := keypair.Random()
		pairs[i] = pair

		ops[i] = &txnbuild.CreateAccount{
			SourceAccount: masterAccount.AccountID,
			Destination:   pair.Address(),
			Amount:        initialBalance,
		}
	}

	// Submit transaction, then retrieve new account details.
	_ = i.MustSubmitOperations(&masterAccount, master, ops...)

	accounts := make([]txnbuild.Account, count)
	for i, kp := range pairs {
		request := sdk.AccountRequest{AccountID: kp.Address()}
		account, err := client.AccountDetail(request)
		panicIf(err)

		accounts[i] = &account
	}

	for _, keys := range pairs {
		i.t.Logf("Funded %s (%s) with %s XLM.\n",
			keys.Seed(), keys.Address(), initialBalance)
	}

	return pairs, accounts
}

// CreateAccount creates a new account via the master account.
func (i *Test) CreateAccount(initialBalance string) (*keypair.Full, txnbuild.Account) {
	kps, accts := i.CreateAccounts(1, initialBalance)
	return kps[0], accts[0]
}

// Panics on any error establishing a trustline.
func (i *Test) MustEstablishTrustline(
	truster *keypair.Full, account txnbuild.Account, asset txnbuild.Asset,
) (resp proto.Transaction) {
	txResp, err := i.EstablishTrustline(truster, account, asset)
	panicIf(err)
	return txResp
}

// EstablishTrustline works on a given asset for a particular account.
func (i *Test) EstablishTrustline(
	truster *keypair.Full, account txnbuild.Account, asset txnbuild.Asset,
) (proto.Transaction, error) {
	if asset.IsNative() {
		return proto.Transaction{}, nil
	}
	line, err := asset.ToChangeTrustAsset()
	if err != nil {
		return proto.Transaction{}, err
	}
	return i.SubmitOperations(account, truster, &txnbuild.ChangeTrust{
		Line:  line,
		Limit: "2000",
	})
}

// MustCreateClaimableBalance panics on any error creating a claimable balance.
func (i *Test) MustCreateClaimableBalance(
	source *keypair.Full, asset txnbuild.Asset, amount string,
	claimants ...txnbuild.Claimant,
) (claim proto.ClaimableBalance) {
	account := i.MustGetAccount(source)
	_ = i.MustSubmitOperations(&account, source,
		&txnbuild.CreateClaimableBalance{
			Destinations: claimants,
			Asset:        asset,
			Amount:       amount,
		},
	)

	// Ensure it exists in the global list
	balances, err := i.Client().ClaimableBalances(sdk.ClaimableBalanceRequest{})
	panicIf(err)

	claims := balances.Embedded.Records
	if len(claims) == 0 {
		panic(-1)
	}

	claim = claims[len(claims)-1] // latest one
	i.t.Logf("Created claimable balance w/ id=%s", claim.BalanceID)
	return
}

// MustGetAccount panics on any error retrieves an account's details from its
// key. This means it must have previously been funded.
func (i *Test) MustGetAccount(source *keypair.Full) proto.Account {
	client := i.Client()
	account, err := client.AccountDetail(sdk.AccountRequest{AccountID: source.Address()})
	panicIf(err)
	return account
}

// MustSubmitOperations submits a signed transaction from an account with
// standard options.
//
// Namely, we set the standard fee, time bounds, etc. to "non-production"
// defaults that work well for tests.
//
// Most transactions only need one signer, so see the more verbose
// `MustSubmitOperationsWithSigners` below for multi-sig transactions.
//
// Note: We assume that transaction will be successful here so we panic in case
// of all errors. To allow failures, use `SubmitOperations`.
func (i *Test) MustSubmitOperations(
	source txnbuild.Account, signer *keypair.Full, ops ...txnbuild.Operation,
) proto.Transaction {
	return i.MustSubmitOperationsWithFee(source, signer, txnbuild.MinBaseFee, ops...)
}

func (i *Test) MustSubmitOperationsWithFee(
	source txnbuild.Account, signer *keypair.Full, fee int64, ops ...txnbuild.Operation,
) proto.Transaction {
	tx, err := i.SubmitOperationsWithFee(source, signer, fee, ops...)
	panicIf(err)
	return tx
}

func (i *Test) SubmitOperations(
	source txnbuild.Account, signer *keypair.Full, ops ...txnbuild.Operation,
) (proto.Transaction, error) {
	return i.SubmitMultiSigOperations(source, []*keypair.Full{signer}, ops...)
}

func (i *Test) SubmitMultiSigOperations(
	source txnbuild.Account, signers []*keypair.Full, ops ...txnbuild.Operation,
) (proto.Transaction, error) {
	return i.SubmitMultiSigOperationsWithFee(source, signers, txnbuild.MinBaseFee, ops...)
}

func (i *Test) SubmitOperationsWithFee(
	source txnbuild.Account, signer *keypair.Full, fee int64, ops ...txnbuild.Operation,
) (proto.Transaction, error) {
	return i.SubmitMultiSigOperationsWithFee(source, []*keypair.Full{signer}, fee, ops...)
}

func (i *Test) SubmitMultiSigOperationsWithFee(
	source txnbuild.Account, signers []*keypair.Full, fee int64, ops ...txnbuild.Operation,
) (proto.Transaction, error) {
	tx, err := i.CreateSignedTransactionFromOpsWithFee(source, signers, fee, ops...)
	if err != nil {
		return proto.Transaction{}, err
	}
	return i.Client().SubmitTransaction(tx)
}

func (i *Test) MustSubmitMultiSigOperations(
	source txnbuild.Account, signers []*keypair.Full, ops ...txnbuild.Operation,
) proto.Transaction {
	tx, err := i.SubmitMultiSigOperations(source, signers, ops...)
	panicIf(err)
	return tx
}

func (i *Test) MustSubmitTransaction(signer *keypair.Full, txParams txnbuild.TransactionParams,
) proto.Transaction {
	tx, err := i.SubmitTransaction(signer, txParams)
	panicIf(err)
	return tx
}

func (i *Test) SubmitTransaction(
	signer *keypair.Full, txParams txnbuild.TransactionParams,
) (proto.Transaction, error) {
	return i.SubmitMultiSigTransaction([]*keypair.Full{signer}, txParams)
}

func (i *Test) SubmitMultiSigTransaction(
	signers []*keypair.Full, txParams txnbuild.TransactionParams,
) (proto.Transaction, error) {
	tx, err := i.CreateSignedTransaction(signers, txParams)
	if err != nil {
		return proto.Transaction{}, err
	}
	return i.Client().SubmitTransaction(tx)
}

func (i *Test) AsyncSubmitTransaction(
	signer *keypair.Full, txParams txnbuild.TransactionParams,
) (proto.AsyncTransactionSubmissionResponse, error) {
	tx, err := i.CreateSignedTransaction([]*keypair.Full{signer}, txParams)
	if err != nil {
		return proto.AsyncTransactionSubmissionResponse{}, err
	}
	return i.Client().AsyncSubmitTransaction(tx)
}

func (i *Test) MustSubmitMultiSigTransaction(
	signers []*keypair.Full, txParams txnbuild.TransactionParams,
) proto.Transaction {
	tx, err := i.SubmitMultiSigTransaction(signers, txParams)
	panicIf(err)
	return tx
}

func (i *Test) CreateSignedTransaction(signers []*keypair.Full, txParams txnbuild.TransactionParams,
) (*txnbuild.Transaction, error) {
	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}

	for _, signer := range signers {
		tx, err = tx.Sign(i.passPhrase, signer)
		if err != nil {
			return nil, err
		}
	}

	return tx, nil
}

func (i *Test) CreateSignedTransactionFromOps(
	source txnbuild.Account, signers []*keypair.Full, ops ...txnbuild.Operation,
) (*txnbuild.Transaction, error) {
	return i.CreateSignedTransactionFromOpsWithFee(source, signers, txnbuild.MinBaseFee, ops...)
}

func (i *Test) CreateSignedTransactionFromOpsWithFee(
	source txnbuild.Account, signers []*keypair.Full, fee int64, ops ...txnbuild.Operation,
) (*txnbuild.Transaction, error) {
	txParams := GetBaseTransactionParamsWithFee(source, fee, ops...)
	return i.CreateSignedTransaction(signers, txParams)
}

func GetBaseTransactionParamsWithFee(source txnbuild.Account, fee int64, ops ...txnbuild.Operation) txnbuild.TransactionParams {
	return txnbuild.TransactionParams{
		SourceAccount:        source,
		Operations:           ops,
		BaseFee:              fee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		IncrementSequenceNum: true,
	}
}

func (i *Test) CreateUnsignedTransaction(
	source txnbuild.Account, ops ...txnbuild.Operation,
) (*txnbuild.Transaction, error) {
	txParams := GetBaseTransactionParamsWithFee(source, txnbuild.MinBaseFee, ops...)
	return txnbuild.NewTransaction(txParams)
}

func (i *Test) GetCurrentCoreLedgerSequence() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	info, err := i.coreClient.Info(ctx)
	if err != nil {
		return 0, err
	}
	return info.Info.Ledger.Num, nil
}

// LogFailedTx is a convenience function to provide verbose information about a
// failing transaction to the test output log, if it's expected to succeed.
func (i *Test) LogFailedTx(txResponse proto.Transaction, horizonResult error) {
	t := i.CurrentTest()
	assert.NoErrorf(t, horizonResult, "Submitting the transaction failed")
	if prob := sdk.GetError(horizonResult); prob != nil {
		t.Logf("  problem: %s\n", prob.Problem.Detail)
		t.Logf("  extras: %s\n", prob.Problem.Extras["result_codes"])
		return
	}

	var txResult xdr.TransactionResult
	err := xdr.SafeUnmarshalBase64(txResponse.ResultXdr, &txResult)
	assert.NoErrorf(t, err, "Unmarshaling transaction failed.")
	assert.Equalf(t, xdr.TransactionResultCodeTxSuccess, txResult.Result.Code,
		"Transaction did not succeed: %d", txResult.Result.Code)
}

func (i *Test) GetPassPhrase() string {
	return i.passPhrase
}

// Cluttering code with if err != nil is absolute nonsense.
func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

// findDockerComposePath performs a best-effort attempt to find the project's
// Docker Compose files.
func findDockerComposePath() string {
	// Lets you check if a particular directory contains a file.
	directoryContainsFilename := func(dir string, filename string) bool {
		files, innerErr := ioutil.ReadDir(dir)
		panicIf(innerErr)

		for _, file := range files {
			if file.Name() == filename {
				return true
			}
		}

		return false
	}

	current, err := os.Getwd()
	panicIf(err)

	//
	// We have a primary and backup attempt for finding the necessary docker
	// files: via $GOPATH and via local directory traversal.
	//

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		monorepo := filepath.Join(gopath, "src", "github.com", "stellar", "go")
		if _, err = os.Stat(monorepo); !os.IsNotExist(err) {
			current = monorepo
		}
	}

	// In either case, we try to walk up the tree until we find "go.mod",
	// which we hope is the root directory of the project.
	for !directoryContainsFilename(current, "go.mod") {
		current, err = filepath.Abs(filepath.Join(current, ".."))

		// FIXME: This only works on *nix-like systems.
		if err != nil || filepath.Base(current)[0] == filepath.Separator {
			fmt.Println("Failed to establish project root directory.")
			panic(err)
		}
	}

	// Directly jump down to the folder that should contain the configs
	return filepath.Join(current, "services", "horizon", "docker")
}

// MergeMaps returns a new map which contains the keys and values of *all* input
// maps, overwriting earlier values with later values on duplicate keys.
func MergeMaps(maps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

// mapToFlags will convert a map of parameters into an array of CLI args (i.e.
// in the form --key=value). Note that an empty value for a key means to drop
// the parameter.
func mapToFlags(params map[string]string) []string {
	args := make([]string, 0, len(params))
	for key, value := range params {
		if value == "" {
			continue
		}

		args = append(args, fmt.Sprintf("--%s=%s", key, value))
	}
	return args
}

func GetCoreMaxSupportedProtocol() uint32 {
	str := os.Getenv("HORIZON_INTEGRATION_TESTS_CORE_MAX_SUPPORTED_PROTOCOL")
	if str == "" {
		return 0
	}
	version, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(version)
}

func (i *Test) GetEffectiveProtocolVersion() uint32 {
	return i.config.ProtocolVersion
}

func (i *Test) GetHistoryArchive() (*historyarchive.Archive, error) {
	return historyarchive.Connect(
		HistoryArchiveUrl,
		historyarchive.ArchiveOptions{
			NetworkPassphrase:   i.config.NetworkPassphrase,
			CheckpointFrequency: CheckpointFrequency,
		})
}

// This is approximate becuase the upgrade could have happened at a leger before this as well.
func (i *Test) GetUpgradedLedgerSeqAppx() (uint32, error) {
	if i.coreUpgradeState == nil {
		return 0, errors.Errorf("Core has not been upgraded yet")
	}
	return i.coreUpgradeState.maxUpgradeLedger, nil
}

func (i *Test) WaitForLedgerInArchive(waitTime time.Duration, ledgerSeq uint32) {
	archive, err := i.GetHistoryArchive()
	if err != nil {
		i.t.Fatalf("could not get history archive: %v", err)
	}

	var latestCheckpoint uint32

	assert.Eventually(i.t,
		func() bool {
			has, requestErr := archive.GetRootHAS()
			if requestErr != nil {
				i.t.Logf("Request to fetch checkpoint failed: %v", requestErr)
				return false
			}
			latestCheckpoint = has.CurrentLedger
			return latestCheckpoint >= ledgerSeq

		},
		waitTime,
		1*time.Second)
}

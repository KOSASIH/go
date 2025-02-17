package galexie

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stellar/go/historyarchive"
	"github.com/stellar/go/support/datastore"
	"github.com/stellar/go/support/storage"
)

const (
	maxWaitForCoreStartup   = (180 * time.Second)
	coreStartupPingInterval = time.Second
	// set the max ledger we want the standalone network to emit
	// tests then refer to ledger sequences only up to this, therefore
	// don't have to do complex waiting within test for a sequence to exist.
	waitForCoreLedgerSequence = 16
	configTemplate            = "test/integration_config_template.toml"
)

func TestGalexieTestSuite(t *testing.T) {
	if os.Getenv("GALEXIE_INTEGRATION_TESTS_ENABLED") != "true" {
		t.Skip("skipping integration test: GALEXIE_INTEGRATION_TESTS_ENABLED not true")
	}

	galexieSuite := &GalexieTestSuite{}
	suite.Run(t, galexieSuite)
}

type GalexieTestSuite struct {
	suite.Suite
	tempConfigFile  string
	ctx             context.Context
	ctxStop         context.CancelFunc
	coreContainerID string
	dockerCli       *client.Client
	gcsServer       *fakestorage.Server
	finishedSetup   bool
	config          Config
}

func (s *GalexieTestSuite) TestScanAndFill() {
	require := s.Require()

	rootCmd := defineCommands()

	rootCmd.SetArgs([]string{"scan-and-fill", "--start", "4", "--end", "5", "--config-file", s.tempConfigFile})
	var errWriter bytes.Buffer
	var outWriter bytes.Buffer
	rootCmd.SetErr(&errWriter)
	rootCmd.SetOut(&outWriter)
	err := rootCmd.ExecuteContext(s.ctx)
	require.NoError(err)

	output := outWriter.String()
	errOutput := errWriter.String()
	s.T().Log(output)
	s.T().Log(errOutput)

	datastore, err := datastore.NewDataStore(s.ctx, s.config.DataStoreConfig)
	require.NoError(err)

	_, err = datastore.GetFile(s.ctx, "FFFFFFFF--0-9/FFFFFFFA--5.xdr.zstd")
	require.NoError(err)
}

func (s *GalexieTestSuite) TestAppend() {
	require := s.Require()

	// first populate ledgers 4-5
	rootCmd := defineCommands()
	rootCmd.SetArgs([]string{"scan-and-fill", "--start", "6", "--end", "7", "--config-file", s.tempConfigFile})
	err := rootCmd.ExecuteContext(s.ctx)
	require.NoError(err)

	// now run an append of overalapping range, it will resume past existing ledgers
	rootCmd.SetArgs([]string{"append", "--start", "6", "--end", "9", "--config-file", s.tempConfigFile})
	var errWriter bytes.Buffer
	var outWriter bytes.Buffer
	rootCmd.SetErr(&errWriter)
	rootCmd.SetOut(&outWriter)
	err = rootCmd.ExecuteContext(s.ctx)
	require.NoError(err)

	output := outWriter.String()
	errOutput := errWriter.String()
	s.T().Log(output)
	s.T().Log(errOutput)

	datastore, err := datastore.NewDataStore(s.ctx, s.config.DataStoreConfig)
	require.NoError(err)

	_, err = datastore.GetFile(s.ctx, "FFFFFFFF--0-9/FFFFFFF6--9.xdr.zstd")
	require.NoError(err)
}

func (s *GalexieTestSuite) TestAppendUnbounded() {
	require := s.Require()

	rootCmd := defineCommands()
	rootCmd.SetArgs([]string{"append", "--start", "10", "--config-file", s.tempConfigFile})
	var errWriter bytes.Buffer
	var outWriter bytes.Buffer
	rootCmd.SetErr(&errWriter)
	rootCmd.SetOut(&outWriter)

	appendCtx, cancel := context.WithCancel(s.ctx)
	syn := make(chan struct{})
	defer func() { <-syn }()
	defer cancel()
	go func() {
		defer close(syn)
		require.NoError(rootCmd.ExecuteContext(appendCtx))
		output := outWriter.String()
		errOutput := errWriter.String()
		s.T().Log(output)
		s.T().Log(errOutput)
	}()

	datastore, err := datastore.NewDataStore(s.ctx, s.config.DataStoreConfig)
	require.NoError(err)

	require.EventuallyWithT(func(c *assert.CollectT) {
		// this checks every 50ms up to 180s total
		assert := assert.New(c)
		_, err = datastore.GetFile(s.ctx, "FFFFFFF5--10-19/FFFFFFF0--15.xdr.zstd")
		assert.NoError(err)
	}, 180*time.Second, 50*time.Millisecond, "append unbounded did not work")
}

func (s *GalexieTestSuite) SetupSuite() {
	var err error
	t := s.T()

	s.ctx, s.ctxStop = signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)

	defer func() {
		if !s.finishedSetup {
			s.TearDownSuite()
		}
	}()
	testTempDir := t.TempDir()

	galexieConfigTemplate, err := toml.LoadFile(configTemplate)
	if err != nil {
		t.Fatalf("unable to load config template file %v, %v", configTemplate, err)
	}

	// if GALEXIE_INTEGRATION_TESTS_CAPTIVE_CORE_BIN not specified,
	// galexie will attempt resolve core bin using 'stellar-core' from OS path
	galexieConfigTemplate.Set("stellar_core_config.stellar_core_binary_path",
		os.Getenv("GALEXIE_INTEGRATION_TESTS_CAPTIVE_CORE_BIN"))

	galexieConfigTemplate.Set("stellar_core_config.storage_path", filepath.Join(testTempDir, "captive-core"))

	tomlBytes, err := toml.Marshal(galexieConfigTemplate)
	if err != nil {
		t.Fatalf("unable to parse config file toml %v, %v", configTemplate, err)
	}
	if err = toml.Unmarshal(tomlBytes, &s.config); err != nil {
		t.Fatalf("unable to marshal config file toml into struct, %v", err)
	}

	tempSeedDataPath := filepath.Join(testTempDir, "data")
	if err = os.MkdirAll(filepath.Join(tempSeedDataPath, "integration-test"), 0777); err != nil {
		t.Fatalf("unable to create seed data in temp path, %v", err)
	}

	s.tempConfigFile = filepath.Join(testTempDir, "config.toml")
	err = os.WriteFile(s.tempConfigFile, tomlBytes, 0777)
	if err != nil {
		t.Fatalf("unable to write temp config file %v, %v", s.tempConfigFile, err)
	}

	testWriter := &testWriter{test: t}
	opts := fakestorage.Options{
		Scheme:      "http",
		Host:        "127.0.0.1",
		Port:        uint16(0),
		Writer:      testWriter,
		Seed:        tempSeedDataPath,
		StorageRoot: filepath.Join(testTempDir, "bucket"),
		PublicHost:  "127.0.0.1",
	}

	s.gcsServer, err = fakestorage.NewServerWithOptions(opts)

	if err != nil {
		t.Fatalf("couldn't start the fake gcs http server %v", err)
	}

	t.Logf("fake gcs server started at %v", s.gcsServer.URL())
	t.Setenv("STORAGE_EMULATOR_HOST", s.gcsServer.URL())

	quickstartImage := os.Getenv("GALEXIE_INTEGRATION_TESTS_QUICKSTART_IMAGE")
	if quickstartImage == "" {
		quickstartImage = "stellar/quickstart:testing"
	}
	pullQuickStartImage := true
	if os.Getenv("GALEXIE_INTEGRATION_TESTS_QUICKSTART_IMAGE_PULL") == "false" {
		pullQuickStartImage = false
	}

	s.mustStartCore(t, quickstartImage, pullQuickStartImage)
	s.mustWaitForCore(t, galexieConfigTemplate.GetArray("stellar_core_config.history_archive_urls").([]string),
		galexieConfigTemplate.Get("stellar_core_config.network_passphrase").(string))
	s.finishedSetup = true
}

func (s *GalexieTestSuite) TearDownSuite() {
	if s.coreContainerID != "" {
		s.T().Logf("Stopping the quickstart container %v", s.coreContainerID)
		containerLogs, err := s.dockerCli.ContainerLogs(s.ctx, s.coreContainerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})

		if err == nil {
			var errWriter bytes.Buffer
			var outWriter bytes.Buffer
			stdcopy.StdCopy(&outWriter, &errWriter, containerLogs)
			s.T().Log(outWriter.String())
			s.T().Log(errWriter.String())
		}
		if err := s.dockerCli.ContainerStop(context.Background(), s.coreContainerID, container.StopOptions{}); err != nil {
			s.T().Logf("unable to stop core container, %v, %v", s.coreContainerID, err)
		}
	}
	if s.dockerCli != nil {
		s.dockerCli.Close()
	}
	if s.gcsServer != nil {
		s.gcsServer.Stop()
	}
	s.ctxStop()
}

func (s *GalexieTestSuite) mustStartCore(t *testing.T, quickstartImage string, pullImage bool) {
	var err error
	s.dockerCli, err = client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("could not create docker client, %v", err)
	}

	if pullImage {
		imgReader, imgErr := s.dockerCli.ImagePull(s.ctx, quickstartImage, image.PullOptions{})
		if imgErr != nil {
			t.Fatalf("could not pull docker image, %v, %v", quickstartImage, imgErr)
		}
		// ImagePull is asynchronous.
		// The reader needs to be read completely for the pull operation to complete.
		_, err = io.Copy(io.Discard, imgReader)
		if err != nil {
			t.Fatalf("could not pull docker image, %v, %v", quickstartImage, err)
		}

		err = imgReader.Close()
		if err != nil {
			t.Fatalf("could not download all of docker image bytes after pull, %v, %v", quickstartImage, err)
		}
	}

	resp, err := s.dockerCli.ContainerCreate(s.ctx,
		&container.Config{
			Image: quickstartImage,
			// only run tge core service(no horizon, rpc, etc) and don't spend any time upgrading
			// the core with newer soroban limits
			Cmd: []string{"--enable", "core,,", "--limits", "default", "--local"},
			ExposedPorts: nat.PortSet{
				nat.Port("1570/tcp"):  {},
				nat.Port("11625/tcp"): {},
			},
		},

		&container.HostConfig{
			PortBindings: nat.PortMap{
				nat.Port("1570/tcp"):  {nat.PortBinding{HostIP: "127.0.0.1", HostPort: "1570"}},
				nat.Port("11625/tcp"): {nat.PortBinding{HostIP: "127.0.0.1", HostPort: "11625"}},
			},
			AutoRemove: true,
		},
		nil, nil, "")

	if err != nil {
		t.Fatalf("could not create quickstart docker container, %v, error %v", quickstartImage, err)
	}
	s.coreContainerID = resp.ID

	if err := s.dockerCli.ContainerStart(s.ctx, resp.ID, container.StartOptions{}); err != nil {
		t.Fatalf("could not run quickstart docker container, %v, error %v", quickstartImage, err)
	}
	t.Logf("Started quickstart container %v", s.coreContainerID)
}

func (s *GalexieTestSuite) mustWaitForCore(t *testing.T, archiveUrls []string, passphrase string) {
	t.Log("Waiting for core to be up...")
	startTime := time.Now()
	infoTime := startTime
	archive, err := historyarchive.NewArchivePool(archiveUrls, historyarchive.ArchiveOptions{
		NetworkPassphrase: passphrase,
		// due to ARTIFICIALLY_ACCELERATE_TIME_FOR_TESTING that is done by quickstart's local network
		CheckpointFrequency: 8,
		ConnectOptions: storage.ConnectOptions{
			Context: s.ctx,
		},
	})
	if err != nil {
		t.Fatalf("unable to create archive pool against core, %v", err)
	}
	for time.Since(startTime) < maxWaitForCoreStartup {
		if durationSince := time.Since(infoTime); durationSince < coreStartupPingInterval {
			time.Sleep(coreStartupPingInterval - durationSince)
		}
		infoTime = time.Now()
		has, requestErr := archive.GetRootHAS()
		if errors.Is(requestErr, context.Canceled) {
			break
		}
		if requestErr != nil {
			t.Logf("request to fetch checkpoint failed: %v", requestErr)
			continue
		}
		latestCheckpoint := has.CurrentLedger
		if latestCheckpoint >= waitForCoreLedgerSequence {
			return
		}
	}
	t.Fatalf("core did not progress ledgers within %v seconds", maxWaitForCoreStartup)
}

type testWriter struct {
	test *testing.T
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.test.Log(string(p))
	return len(p), nil
}

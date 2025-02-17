package galexie

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestFlagsOutput(t *testing.T) {
	var testResultSettings RuntimeSettings
	appRunnerSuccess := func(runtimeSettings RuntimeSettings) error {
		testResultSettings = runtimeSettings
		return nil
	}

	appRunnerError := func(runtimeSettings RuntimeSettings) error {
		return errors.New("test error")
	}

	ctx := context.Background()

	testCases := []struct {
		name              string
		commandArgs       []string
		expectedErrOutput string
		appRunner         func(runtimeSettings RuntimeSettings) error
		expectedSettings  RuntimeSettings
	}{
		{
			name:              "no sub-command",
			commandArgs:       []string{"--start", "4", "--end", "5", "--config-file", "myfile"},
			expectedErrOutput: "Error: ",
		},
		{
			name:              "append sub-command with start and end present",
			commandArgs:       []string{"append", "--start", "4", "--end", "5", "--config-file", "myfile"},
			expectedErrOutput: "",
			appRunner:         appRunnerSuccess,
			expectedSettings: RuntimeSettings{
				StartLedger:    4,
				EndLedger:      5,
				ConfigFilePath: "myfile",
				Mode:           Append,
				Ctx:            ctx,
			},
		},
		{
			name:              "append sub-command with start and end absent",
			commandArgs:       []string{"append", "--config-file", "myfile"},
			expectedErrOutput: "",
			appRunner:         appRunnerSuccess,
			expectedSettings: RuntimeSettings{
				StartLedger:    0,
				EndLedger:      0,
				ConfigFilePath: "myfile",
				Mode:           Append,
				Ctx:            ctx,
			},
		},
		{
			name:              "append sub-command prints app error",
			commandArgs:       []string{"append", "--start", "4", "--end", "5", "--config-file", "myfile"},
			expectedErrOutput: "test error",
			appRunner:         appRunnerError,
		},
		{
			name:              "scanfill sub-command with start and end present",
			commandArgs:       []string{"scan-and-fill", "--start", "4", "--end", "5", "--config-file", "myfile"},
			expectedErrOutput: "",
			appRunner:         appRunnerSuccess,
			expectedSettings: RuntimeSettings{
				StartLedger:    4,
				EndLedger:      5,
				ConfigFilePath: "myfile",
				Mode:           ScanFill,
				Ctx:            ctx,
			},
		},
		{
			name:              "scanfill sub-command with start and end absent",
			commandArgs:       []string{"scan-and-fill", "--config-file", "myfile"},
			expectedErrOutput: "",
			appRunner:         appRunnerSuccess,
			expectedSettings: RuntimeSettings{
				StartLedger:    0,
				EndLedger:      0,
				ConfigFilePath: "myfile",
				Mode:           ScanFill,
				Ctx:            ctx,
			},
		},
		{
			name:              "scanfill sub-command prints app error",
			commandArgs:       []string{"scan-and-fill", "--start", "4", "--end", "5", "--config-file", "myfile"},
			expectedErrOutput: "test error",
			appRunner:         appRunnerError,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// mock galexie's cmd runner to be this test's mock routine instead of real app
			galexieCmdRunner = testCase.appRunner
			rootCmd := defineCommands()
			rootCmd.SetArgs(testCase.commandArgs)
			var errWriter io.Writer = &bytes.Buffer{}
			var outWriter io.Writer = &bytes.Buffer{}
			rootCmd.SetErr(errWriter)
			rootCmd.SetOut(outWriter)
			rootCmd.ExecuteContext(ctx)

			errOutput := errWriter.(*bytes.Buffer).String()
			if testCase.expectedErrOutput != "" {
				assert.Contains(t, errOutput, testCase.expectedErrOutput)
			} else {
				assert.Equal(t, testCase.expectedSettings, testResultSettings)
			}
		})
	}
}

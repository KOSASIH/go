package ledgerbackend

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

var fakeExecCmdOut = ""

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := append([]string{"-test.run=TestExecCmdHelperProcess", "--", command}, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_EXEC_CMD_HELPER_PROCESS=1", "CMD_OUT="+fakeExecCmdOut)
	return cmd
}

func init() {
	execCommand = fakeExecCommand
}

func TestExecCmdHelperProcess(t *testing.T) {
	if os.Getenv("GO_EXEC_CMD_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, os.Getenv("CMD_OUT"))
	os.Exit(0)
}

func TestGetCoreBuildVersion(t *testing.T) {
	tests := []struct {
		name            string
		commandOutput   string
		expectedError   error
		expectedCoreVer string
	}{
		{
			name: "core build version found",
			commandOutput: "v20.2.0-2-g6e73c0a88\n" +
				"rust version: rustc 1.74.1 (a28077b28 2023-12-04)\n" +
				"soroban-env-host: \n" +
				"    curr:\n" +
				"       package version: 20.2.0\n" +
				"       git version: 1bfc0f2a2ee134efc1e1b0d5270281d0cba61c2e\n" +
				"       ledger protocol version: 20\n" +
				"       pre-release version: 0\n" +
				"       rs-stellar-xdr:\n" +
				"           package version: 20.1.0\n" +
				"           git version: 8b9d623ef40423a8462442b86997155f2c04d3a1\n" +
				"           base XDR git version: b96148cd4acc372cc9af17b909ffe4b12c43ecb6\n",
			expectedError:   nil,
			expectedCoreVer: "v20.2.0-2-g6e73c0a88",
		},
		{
			name:            "core build version not found",
			commandOutput:   "",
			expectedError:   fmt.Errorf("stellar-core version not found"),
			expectedCoreVer: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeExecCmdOut = tt.commandOutput
			coreVersion, err := CoreBuildVersion("")

			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedCoreVer, coreVersion)
			}
		})
	}
}

func TestGetCoreProtcolVersion(t *testing.T) {
	tests := []struct {
		name                    string
		commandOutput           string
		expectedError           error
		expectedProtocolVersion uint
	}{
		{
			name: "core protocol version found",
			commandOutput: "v20.2.0-2-g6e73c0a88\n" +
				"rust version: rustc 1.74.1 (a28077b28 2023-12-04)\n" +
				"soroban-env-host: \n" +
				"    curr:\n" +
				"       package version: 20.2.0\n" +
				"       git version: 1bfc0f2a2ee134efc1e1b0d5270281d0cba61c2e\n" +
				"       ledger protocol version: 21\n" +
				"       pre-release version: 0\n" +
				"       rs-stellar-xdr:\n" +
				"           package version: 20.1.0\n" +
				"           git version: 8b9d623ef40423a8462442b86997155f2c04d3a1\n" +
				"           base XDR git version: b96148cd4acc372cc9af17b909ffe4b12c43ecb6\n",
			expectedError:           nil,
			expectedProtocolVersion: 21,
		},
		{
			name:                    "core protocol version not found",
			commandOutput:           "",
			expectedError:           fmt.Errorf("error parsing protocol version from stellar-core output"),
			expectedProtocolVersion: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeExecCmdOut = tt.commandOutput
			coreVersion, err := CoreProtocolVersion("/usr/bin/stellar-core")

			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedProtocolVersion, coreVersion)
			}
		})
	}
}

package ledgerbackend

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// By default, it points to exec.Command, overridden for testing purpose
var execCommand = exec.Command

type CoreBuildVersionFunc func(coreBinaryPath string) (string, error)
type CoreProtocolVersionFunc func(coreBinaryPath string) (uint, error)

// CoreBuildVersion executes the "stellar-core version" command and parses its output to extract
// the core version
// The output of the "version" command is expected to be a multi-line string where the
// first line is the core version in format "vX.Y.Z-*".
func CoreBuildVersion(coreBinaryPath string) (string, error) {
	versionCmd := execCommand(coreBinaryPath, "version")
	versionOutput, err := versionCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute stellar-core version command: %w", err)
	}

	// Split the output into lines
	rows := strings.Split(string(versionOutput), "\n")
	if len(rows) == 0 || len(rows[0]) == 0 {
		return "", fmt.Errorf("stellar-core version not found")
	}

	return rows[0], nil
}

// CoreProtocolVersion retrieves the ledger protocol version from the specified stellar-core binary.
// It executes the "stellar-core version" command and parses the output to extract the protocol version.
func CoreProtocolVersion(coreBinaryPath string) (uint, error) {
	if coreBinaryPath == "" {
		return 0, fmt.Errorf("stellar-core binary path is empty")
	}

	versionBytes, err := execCommand(coreBinaryPath, "version").Output()
	if err != nil {
		return 0, fmt.Errorf("error executing stellar-core version command (%s): %w", coreBinaryPath, err)
	}

	versionRows := strings.Split(string(versionBytes), "\n")
	re := regexp.MustCompile(`^\s*ledger protocol version: (\d*)`)
	var ledgerProtocolStrings []string
	for _, line := range versionRows {
		ledgerProtocolStrings = re.FindStringSubmatch(line)
		if len(ledgerProtocolStrings) == 2 {
			val, err := strconv.Atoi(ledgerProtocolStrings[1])
			if err != nil {
				return 0, fmt.Errorf("error parsing protocol version from stellar-core output: %w", err)
			}
			return uint(val), nil
		}
	}

	return 0, fmt.Errorf("error parsing protocol version from stellar-core output")
}

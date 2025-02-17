package test

import (
	"fmt"
	"os"
	"strings"

	"github.com/stellar/go/support/errors"
)

type EnvironmentManager struct {
	oldEnvironment, newEnvironment map[string]string
}

func NewEnvironmentManager() *EnvironmentManager {
	env := &EnvironmentManager{}
	env.oldEnvironment = make(map[string]string)
	env.newEnvironment = make(map[string]string)
	return env
}

func (envManager *EnvironmentManager) InitializeEnvironmentVariables(environmentVars map[string]string) error {
	var env strings.Builder
	for key, value := range environmentVars {
		env.WriteString(fmt.Sprintf("%s=%s ", key, value))
	}

	// prepare env
	for key, value := range environmentVars {
		innerErr := envManager.Add(key, value)
		if innerErr != nil {
			return errors.Wrap(innerErr, fmt.Sprintf(
				"failed to set envvar (%s=%s)", key, value))
		}
	}
	return nil
}

// Add sets a new environment variable, saving the original value (if any).
func (envManager *EnvironmentManager) Add(key, value string) error {
	// If someone pushes an environmental variable more than once, we don't want
	// to lose the *original* value, so we're being careful here.
	if _, ok := envManager.newEnvironment[key]; !ok {
		if oldValue, ok := os.LookupEnv(key); ok {
			envManager.oldEnvironment[key] = oldValue
		}
	}

	envManager.newEnvironment[key] = value
	return os.Setenv(key, value)
}

// Restore restores the environment prior to any modifications.
//
// You should probably use this alongside `defer` to ensure the global
// environment isn't modified for longer than you intend.
func (envManager *EnvironmentManager) Restore() {
	for key := range envManager.newEnvironment {
		if oldValue, ok := envManager.oldEnvironment[key]; ok {
			os.Setenv(key, oldValue)
		} else {
			os.Unsetenv(key)
		}
	}
}

package cmd

import (
	"fmt"
	stdLog "log"

	"github.com/spf13/cobra"
	horizon "github.com/stellar/go/services/horizon/internal"
	"github.com/stellar/go/support/config"
)

var (
	globalConfig, globalFlags = horizon.Flags()

	RootCmd           = createRootCmd(globalConfig, globalFlags)
	originalHelpFunc  = RootCmd.HelpFunc()
	originalUsageFunc = RootCmd.UsageFunc()
)

func createRootCmd(horizonConfig *horizon.Config, configOptions config.ConfigOptions) *cobra.Command {
	return &cobra.Command{
		Use:           "horizon",
		Short:         "client-facing api server for the Stellar network",
		SilenceErrors: true,
		SilenceUsage:  true,
		Long: "Client-facing API server for the Stellar network. It acts as the interface between Stellar Core " +
			"and applications that want to access the Stellar network. It allows you to submit transactions to the " +
			"network, check the status of accounts, subscribe to event streams and more.\n" +
			"DEPRECATED - the use of command-line flags has been deprecated in favor of environment variables. Please" +
			"consult our Configuring section in the developer documentation on how to use them - https://developers.stellar.org/docs/run-api-server/configuring",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := horizon.NewAppFromFlags(horizonConfig, configOptions)
			if err != nil {
				return err
			}
			return app.Serve()
		},
	}
}

func initRootCmd(cmd *cobra.Command,
	originalHelpFn func(*cobra.Command, []string),
	originalUsageFn func(*cobra.Command) error,
	horizonGlobalFlags config.ConfigOptions) {
	// override the default help output, apply further filtering on which global flags
	// will be shown on the help outout dependent on the command help was issued upon.
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		enableGlobalOptionsInHelp(c, horizonGlobalFlags)
		originalHelpFn(c, args)
	})

	cmd.SetUsageFunc(func(c *cobra.Command) error {
		enableGlobalOptionsInHelp(c, horizonGlobalFlags)
		return originalUsageFn(c)
	})

	err := horizonGlobalFlags.Init(cmd)
	if err != nil {
		stdLog.Fatal(err.Error())
	}
}

func NewRootCmd() *cobra.Command {
	horizonGlobalConfig, horizonGlobalFlags := horizon.Flags()
	cmd := createRootCmd(horizonGlobalConfig, horizonGlobalFlags)
	initRootCmd(cmd, cmd.HelpFunc(), cmd.UsageFunc(), horizonGlobalFlags)
	DefineDBCommands(cmd, horizonGlobalConfig, horizonGlobalFlags)
	return cmd
}

// ErrUsage indicates we should print the usage string and exit with code 1
type ErrUsage struct {
	cmd *cobra.Command
}

func (e ErrUsage) Error() string {
	return e.cmd.UsageString()
}

// ErrExitCode Indicates we want to exit with a specific error code without printing an error.
type ErrExitCode int

func (e ErrExitCode) Error() string {
	return fmt.Sprintf("exit code: %d", e)
}

func init() {
	initRootCmd(RootCmd, originalHelpFunc, originalUsageFunc, globalFlags)
}

func Execute() error {
	return RootCmd.Execute()
}

func enableGlobalOptionsInHelp(cmd *cobra.Command, cos config.ConfigOptions) {
	for _, co := range cos {
		if co.Hidden {
			// this options was configured statically to be hidden
			// Init() has already set that once, leave it as-is.
			continue
		}

		// we don't want' to display global flags in help output
		// if no sub-command context given yet, i.e. just '-h' was used
		// or there are subcomands required to use.
		if cmd.Parent() == nil || cmd.HasAvailableSubCommands() {
			co.ToggleHidden(true)
			continue
		}

		if len(co.UsedInCommands) > 0 &&
			!contains(co.UsedInCommands, cmd) {
			co.ToggleHidden(true)
		} else {
			co.ToggleHidden(false)
		}
	}
}

// check if this command or any of it's sub-level parents match
// supportedCommands
func contains(supportedCommands []string, cmd *cobra.Command) bool {
	for _, supportedCommand := range supportedCommands {
		if supportedCommand == cmd.Name() {
			return true
		}
	}

	// don't do inheritance matching on the top most sub-commands.
	// they are second level deep, the horizon itself is top level.
	if cmd.Parent() != nil && cmd.Parent().Parent() != nil {
		return contains(supportedCommands, cmd.Parent())
	}
	return false
}

package galexie

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stellar/go/support/strutils"
)

var (
	galexieCmdRunner = func(runtimeSettings RuntimeSettings) error {
		app := NewApp()
		return app.Run(runtimeSettings)
	}
)

func Execute() error {
	rootCmd := defineCommands()
	return rootCmd.Execute()
}

func defineCommands() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "galexie",
		Short: "Export Stellar network ledger data to a remote data store",
		Long:  "Converts ledger meta data from Stellar network into static data and exports it remote data storage.",

		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("please specify one of the availble sub-commands to initiate export")
		},
	}
	var scanAndFillCmd = &cobra.Command{
		Use:   "scan-and-fill",
		Short: "scans the entire bounded requested range between 'start' and 'end' flags and exports only the ledgers which are missing from the data lake.",
		Long:  "scans the entire bounded requested range between 'start' and 'end' flags and exports only the ledgers which are missing from the data lake.",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings := bindCliParameters(cmd.PersistentFlags().Lookup("start"),
				cmd.PersistentFlags().Lookup("end"),
				cmd.PersistentFlags().Lookup("config-file"),
			)
			settings.Mode = ScanFill
			settings.Ctx = cmd.Context()
			if settings.Ctx == nil {
				settings.Ctx = context.Background()
			}
			return galexieCmdRunner(settings)
		},
	}
	var appendCmd = &cobra.Command{
		Use:   "append",
		Short: "export ledgers beginning with the first missing ledger after the specified 'start' ledger and resumes exporting from there",
		Long:  "export ledgers beginning with the first missing ledger after the specified 'start' ledger and resumes exporting from there",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings := bindCliParameters(cmd.PersistentFlags().Lookup("start"),
				cmd.PersistentFlags().Lookup("end"),
				cmd.PersistentFlags().Lookup("config-file"),
			)
			settings.Mode = Append
			settings.Ctx = cmd.Context()
			if settings.Ctx == nil {
				settings.Ctx = context.Background()
			}
			return galexieCmdRunner(settings)
		},
	}

	rootCmd.AddCommand(scanAndFillCmd)
	rootCmd.AddCommand(appendCmd)

	scanAndFillCmd.PersistentFlags().Uint32P("start", "s", 0, "Starting ledger (inclusive), must be set to a value greater than 1")
	scanAndFillCmd.PersistentFlags().Uint32P("end", "e", 0, "Ending ledger (inclusive), must be set to value greater than 'start' and less than the network's current ledger")
	scanAndFillCmd.PersistentFlags().String("config-file", "config.toml", "Path to the TOML config file. Defaults to 'config.toml' on runtime working directory path.")
	viper.BindPFlags(scanAndFillCmd.PersistentFlags())

	appendCmd.PersistentFlags().Uint32P("start", "s", 0, "Starting ledger (inclusive), must be set to a value greater than 1")
	appendCmd.PersistentFlags().Uint32P("end", "e", 0, "Ending ledger (inclusive), optional, setting to non-zero means bounded mode, "+
		"only export ledgers from 'start' up to 'end' value which must be greater than 'start' and less than the network's current ledger. "+
		"If 'end' is absent or '0' means unbounded mode, exporter will continue to run indefintely and export the latest closed ledgers from network as they are generated in real time.")
	appendCmd.PersistentFlags().String("config-file", "config.toml", "Path to the TOML config file. Defaults to 'config.toml' on runtime working directory path.")
	viper.BindPFlags(appendCmd.PersistentFlags())

	return rootCmd
}

func bindCliParameters(startFlag *pflag.Flag, endFlag *pflag.Flag, configFileFlag *pflag.Flag) RuntimeSettings {
	settings := RuntimeSettings{}

	viper.BindPFlag(startFlag.Name, startFlag)
	viper.BindEnv(startFlag.Name, strutils.KebabToConstantCase(startFlag.Name))
	settings.StartLedger = viper.GetUint32(startFlag.Name)

	viper.BindPFlag(endFlag.Name, endFlag)
	viper.BindEnv(endFlag.Name, strutils.KebabToConstantCase(endFlag.Name))
	settings.EndLedger = viper.GetUint32(endFlag.Name)

	viper.BindPFlag(configFileFlag.Name, configFileFlag)
	viper.BindEnv(configFileFlag.Name, strutils.KebabToConstantCase(configFileFlag.Name))
	settings.ConfigFilePath = viper.GetString(configFileFlag.Name)

	return settings
}

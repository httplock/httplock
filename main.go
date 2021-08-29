package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	log *logrus.Logger
)

var rootOpts struct {
	verbosity string
	confFile  string
}

var rootCmd = &cobra.Command{
	Use:           "httplock <cmd>",
	Short:         "HTTP proxy for enabling reproducible builds",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	log = &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.WarnLevel,
	}

	rootCmd.PersistentFlags().StringVarP(
		&rootOpts.verbosity,
		"verbosity", "v",
		logrus.InfoLevel.String(),
		"Log level (debug, info, warn, error, fatal, panic)",
	)
	rootCmd.RegisterFlagCompletionFunc(
		"verbosity",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"debug", "info", "warn", "error", "fatal", "panic"}, cobra.ShellCompDirectiveNoFileComp
		},
	)

	rootCmd.PersistentFlags().StringVarP(
		&rootOpts.confFile,
		"config", "c", "",
		"Config file",
	)

	rootCmd.PersistentPreRunE = rootPreRun
}

func rootPreRun(cmd *cobra.Command, args []string) error {
	lvl, err := logrus.ParseLevel(rootOpts.verbosity)
	if err != nil {
		return err
	}
	log.SetLevel(lvl)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

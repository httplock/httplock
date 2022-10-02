package main

import (
	"os"

	"github.com/httplock/httplock/internal/template"
	"github.com/httplock/httplock/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Output the current version",
	Args:  cobra.ExactArgs(0),
	RunE:  runVersion,
}

func init() {
	versionCmd.Flags().StringVarP(&rootOpts.format, "format", "", "{{printPretty .}}", "Format output with go template syntax")
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) error {
	info := version.GetInfo()
	return template.Writer(os.Stdout, rootOpts.format, info)
}

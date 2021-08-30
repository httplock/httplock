package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Output the current version",
	Args:  cobra.ExactArgs(0),
	RunE:  runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

//go:embed embed/*
var embedFS embed.FS

// //go:embed version.json
// var verB []byte

func runVersion(cmd *cobra.Command, args []string) error {
	verS := struct {
		VCSRef string
		VCSTag string
	}{}

	verB, err := embedFS.ReadFile("embed/version.json")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if len(verB) > 0 {
		err = json.Unmarshal(verB, &verS)
		if err != nil {
			return err
		}
	}

	verJ, err := json.MarshalIndent(verS, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", verJ)
	return nil
}

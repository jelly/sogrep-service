package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// RootCmd provides the commandline parser root.
var RootCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(2)
	},
}

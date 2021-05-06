package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// RootCmd provides the commandline parser root.
var RootCmd = &cobra.Command{
	Use: "sogrep-service",
	Short: "Sogrep service is a sogrep HTTP API",
	Long: `Sogrep service parses Arch Linux's links databases and
provides a simple HTTP JSON API to find out which packages require a given soname.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(2)
	},
}

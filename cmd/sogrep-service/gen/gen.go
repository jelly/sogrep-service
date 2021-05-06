package gen

import (
	"github.com/spf13/cobra"
)

// DefaultRootUse defines the root command Use value to use for generators.
var DefaultRootUse = "sogrep-service"

func CommandGen() *cobra.Command {
	genCmd := &cobra.Command{
		Use:   "gen [...args]",
		Short: "A collection of useful generators",
	}

	genCmd.AddCommand(CommandMan())
	genCmd.AddCommand(CommandAutoComplete())

	return genCmd
}

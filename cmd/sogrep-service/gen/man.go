package gen

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var (
	DefaultManDir = "man/"
)

func CommandMan() *cobra.Command {
	manCmd := &cobra.Command{
		Use:   "man [...args]",
		Short: "Generate man pages for the sogrep-service",
		Run: func(cmd *cobra.Command, args []string) {
			if err := man(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	manCmd.Flags().StringVar(&DefaultManDir, "dir", DefaultManDir, "Full path to directory to write the man pages")

	return manCmd
}

func man(cmd *cobra.Command, args []string) error {
	header := &doc.GenManHeader{
		Title:   "sogrep service for Arch Linux",
		Section: "1",
	}

	root := cmd.Root()
	root.DisableAutoGenTag = true
	root.Use = DefaultRootUse // Use the name of bin script for man pages.

	return doc.GenManTree(root, header, DefaultManDir)
}

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jelly/sogrep-service/server"
)

var (
	listenaddress = "localhost:8080"
)


func commandServe() *cobra.Command {
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "sogrep web service",
		Run: func(cmd *cobra.Command, args []string) {
			if err := serve(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	serveCmd.Flags().StringVar(&listenaddress, "listen-address", listenaddress, "Address on which the sogrep API is available.")

	serveCmd.Flags().Bool("log-timestamp", true, "Prefix each log line with timestamp")
	serveCmd.Flags().String("log-level", "info", "Log level (one of panic, fatal, error, warn, info or debug)")

	return serveCmd
}

func serve(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logTimestamp, _ := cmd.Flags().GetBool("log-timestamp")
	logLevel, _ := cmd.Flags().GetString("log-level")

	logger, err := newLogger(!logTimestamp, logLevel)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	logger.Debugln("starting sogrep web service")

	cfg := &server.Config{
		ListenAddress: listenaddress,

		Logger: logger,
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		return err
	}

	return srv.Serve(ctx)
}

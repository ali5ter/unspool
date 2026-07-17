// Package cmd implements the unspool CLI: flag parsing and mode dispatch
// (TUI, pipeline, sync-only).
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ali5ter/unspool/config"
)

// version is set at build time via -ldflags.
var version = "dev"

var (
	flagLogin  bool
	flagLogout bool
	flagSync   bool
	flagJSON   bool
	flagNoTTY  bool
)

var rootCmd = &cobra.Command{
	Use:     "unspool",
	Short:   "A subscription-first, Shorts-free YouTube TUI",
	Version: version,
	RunE:    run,
}

func init() {
	rootCmd.Flags().BoolVar(&flagLogin, "login", false, "authenticate with YouTube (OAuth loopback flow) and store the refresh token in the system keychain")
	rootCmd.Flags().BoolVar(&flagLogout, "logout", false, "remove stored credentials from the system keychain")
	rootCmd.Flags().BoolVar(&flagSync, "sync", false, "refresh the local cache and exit (cron-friendly)")
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "dump the feed as a JSON array to stdout (no TUI)")
	rootCmd.Flags().BoolVar(&flagNoTTY, "no-tty", false, "force pipeline mode")
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	switch {
	case flagLogin:
		return runLogin(cfg)
	case flagLogout:
		return runLogout()
	case flagSync:
		return runSync(cfg)
	case flagJSON, flagNoTTY, !isTTY():
		return runPipeline(cfg)
	default:
		return runTUI(cfg)
	}
}

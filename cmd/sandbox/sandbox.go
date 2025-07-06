package sandbox

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the sandbox command with subcommands
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Local development environment for Flowctl pipelines",
		Long: `Sandbox provides a local-first developer experience for running Flowctl pipelines
and their supporting infrastructure in an isolated containerized environment.

This command reduces friction in developing, testing, and debugging pipelines by
eliminating manual service startup and configuration.`,
	}

	// Add subcommands
	cmd.AddCommand(newStartCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newLogsCommand())
	cmd.AddCommand(newStopCommand())
	cmd.AddCommand(newUpgradeCommand())

	return cmd
}
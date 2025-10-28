package quickstart

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the quickstart command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quickstart",
		Short: "Run the Asset Balance Indexer quickstart pipeline",
		Long: `Run the Asset Balance Indexer quickstart pipeline.

This command starts all three components needed for the quickstart:
  - stellar-live-source-datalake: Streams ledger data from GCS
  - account-balance-processor: Extracts balance changes
  - duckdb-consumer: Writes results to DuckDB

The components are configured using a single YAML configuration file.`,
	}

	cmd.AddCommand(newRunCommand())
	return cmd
}

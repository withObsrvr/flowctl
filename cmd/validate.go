package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/runner"
	"github.com/withobsrvr/flowctl/internal/validator"
)

var validateCmd = &cobra.Command{
	Use:   "validate [pipeline-file]",
	Short: "Validate a pipeline configuration file",
	Long: `Validate a pipeline YAML file before running it.

This command checks for common configuration errors including:
- YAML structure and required fields
- Duplicate component IDs
- Port conflicts
- Command file existence
- Processor chain length warnings
- Consumer fan-out warnings
- Event type compatibility

Examples:
  # Validate a pipeline file
  flowctl validate my-pipeline.yaml

  # Validate default pipeline
  flowctl validate`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default pipeline file
		pipelineFile := "flow.yml"
		if len(args) > 0 {
			pipelineFile = args[0]
		}

		// Check if file exists
		if _, err := os.Stat(pipelineFile); err != nil {
			return fmt.Errorf("pipeline file not found: %s", pipelineFile)
		}

		// Load pipeline configuration
		pipeline, err := runner.LoadPipelineFromFile(pipelineFile)
		if err != nil {
			return fmt.Errorf("failed to load pipeline: %w", err)
		}

		// Create validator
		v := validator.NewValidator(pipeline)

		// Run validation
		result := v.Validate()

		// Print results
		fmt.Println(result.Format())

		// Exit with error code if validation failed
		if !result.Valid {
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

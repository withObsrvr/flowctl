package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	templateRepo string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [workspace-name]",
	Short: "Create a new Flow workspace",
	Long: `Create a new Flow workspace with the following structure:
  - flowctl.yaml (workspace configuration)
  - pipelines/ (directory for pipeline definitions)
  - sources/ (directory for source definitions)
  - processors/ (directory for processor definitions)
  - sinks/ (directory for sink definitions)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceName := args[0]

		// Create workspace directory
		if err := os.MkdirAll(workspaceName, 0755); err != nil {
			return fmt.Errorf("failed to create workspace directory: %w", err)
		}

		// Create subdirectories
		dirs := []string{"pipelines", "sources", "processors", "sinks"}
		for _, dir := range dirs {
			path := filepath.Join(workspaceName, dir)
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("failed to create %s directory: %w", dir, err)
			}
		}

		// Create flowctl.yaml
		configPath := filepath.Join(workspaceName, "flowctl.yaml")
		configContent := fmt.Sprintf(`# Flow workspace configuration
workspace: %s
version: v1

# Runtime configuration
runtime:
  context: default
  namespace: default

# Pipeline configuration
pipelines:
  directory: ./pipelines

# Component directories
sources:
  directory: ./sources
processors:
  directory: ./processors
sinks:
  directory: ./sinks
`, workspaceName)

		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			return fmt.Errorf("failed to create flowctl.yaml: %w", err)
		}

		fmt.Printf("Created Flow workspace '%s'\n", workspaceName)
		fmt.Printf("  - Configuration: %s\n", configPath)
		fmt.Printf("  - Pipeline definitions: %s/pipelines/\n", workspaceName)
		fmt.Printf("  - Source definitions: %s/sources/\n", workspaceName)
		fmt.Printf("  - Processor definitions: %s/processors/\n", workspaceName)
		fmt.Printf("  - Sink definitions: %s/sinks/\n", workspaceName)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&templateRepo, "template", "", "template repository to use for initialization")
}

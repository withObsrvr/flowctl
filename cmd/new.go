package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	templateName string
)

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new [TYPE] [NAME]",
	Short: "Scaffold new resources",
	Long: `Scaffold new resources like pipelines, sources, processors, or sinks.
This command creates a new resource definition with a template structure.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceType := args[0]
		resourceName := args[1]

		// Validate resource type
		validTypes := map[string]string{
			"pipeline":  "pipelines",
			"source":    "sources",
			"processor": "processors",
			"sink":      "sinks",
		}

		dirName, ok := validTypes[resourceType]
		if !ok {
			return fmt.Errorf("invalid resource type: %s. Must be one of: pipeline, source, processor, sink", resourceType)
		}

		// Get workspace root
		workspaceRoot := "."
		if viper.ConfigFileUsed() != "" {
			workspaceRoot = filepath.Dir(viper.ConfigFileUsed())
		}

		// Create resource directory if it doesn't exist
		resourceDir := filepath.Join(workspaceRoot, dirName)
		if err := os.MkdirAll(resourceDir, 0755); err != nil {
			return fmt.Errorf("failed to create resource directory: %w", err)
		}

		// Create resource file
		resourcePath := filepath.Join(resourceDir, resourceName+".yaml")

		// Generate template content based on resource type
		var templateContent string
		switch resourceType {
		case "pipeline":
			templateContent = fmt.Sprintf(`# Pipeline: %s
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: %s
spec:
  source:
    name: stellar-ledger
    config:
      network: public
      cursor: latest
  processors:
    - name: transform
      config:
        # Add processor configuration here
  sink:
    name: kafka
    config:
      brokers: localhost:9092
      topic: stellar-events
`, resourceName, resourceName)
		case "source":
			templateContent = fmt.Sprintf(`# Source: %s
apiVersion: flowctl/v1
kind: Source
metadata:
  name: %s
spec:
  type: stellar-ledger
  config:
    network: public
    cursor: latest
`, resourceName, resourceName)
		case "processor":
			templateContent = fmt.Sprintf(`# Processor: %s
apiVersion: flowctl/v1
kind: Processor
metadata:
  name: %s
spec:
  type: transform
  config:
    # Add processor configuration here
`, resourceName, resourceName)
		case "sink":
			templateContent = fmt.Sprintf(`# Sink: %s
apiVersion: flowctl/v1
kind: Sink
metadata:
  name: %s
spec:
  type: kafka
  config:
    brokers: localhost:9092
    topic: stellar-events
`, resourceName, resourceName)
		}

		if err := os.WriteFile(resourcePath, []byte(templateContent), 0644); err != nil {
			return fmt.Errorf("failed to create resource file: %w", err)
		}

		fmt.Printf("Created new %s '%s' at %s\n", resourceType, resourceName, resourcePath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringVar(&templateName, "template", "", "template to use for the new resource")
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	wideOutput bool
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get [TYPE]",
	Short: "List resources",
	Long: `List resources of a specific type. The output format can be controlled
using the --output flag. Available types: pipelines, sources, processors, sinks.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceType := args[0]

		// Validate resource type
		validTypes := map[string]string{
			"pipelines":  "Pipeline",
			"sources":    "Source",
			"processors": "Processor",
			"sinks":      "Sink",
		}

		kind, ok := validTypes[resourceType]
		if !ok {
			return fmt.Errorf("invalid resource type: %s. Must be one of: pipelines, sources, processors, sinks", resourceType)
		}

		// TODO: Fetch resources from runtime
		// For now, just print a placeholder message
		fmt.Printf("Listing %s resources...\n", kind)
		if wideOutput {
			fmt.Println("NAME\tSTATUS\tAGE\tCONFIG")
		} else {
			fmt.Println("NAME\tSTATUS\tAGE")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().BoolVar(&wideOutput, "wide", false, "show additional details")
}

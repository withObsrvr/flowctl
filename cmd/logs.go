package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	since  string
	follow bool
	tail   int
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs [TYPE/NAME]",
	Short: "View resource logs",
	Long: `View logs for a specific resource. The logs can be filtered by time range
and can be followed in real-time.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resource := args[0]

		// Parse resource type and name
		var resourceType, resourceName string
		if _, err := fmt.Sscanf(resource, "%s/%s", &resourceType, &resourceName); err != nil {
			return fmt.Errorf("invalid resource format. Use TYPE/NAME (e.g., pipeline/my-pipeline)")
		}

		// Validate resource type
		validTypes := map[string]bool{
			"pipeline":  true,
			"source":    true,
			"processor": true,
			"sink":      true,
		}

		if !validTypes[resourceType] {
			return fmt.Errorf("invalid resource type: %s. Must be one of: pipeline, source, processor, sink", resourceType)
		}

		// Parse since duration
		var sinceTime time.Time
		if since != "" {
			duration, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid duration format: %w", err)
			}
			sinceTime = time.Now().Add(-duration)
		}

		// TODO: Fetch logs from runtime
		// For now, just print a placeholder message
		fmt.Printf("Fetching logs for %s/%s...\n", resourceType, resourceName)
		if sinceTime != (time.Time{}) {
			fmt.Printf("Since: %s\n", sinceTime.Format(time.RFC3339))
		}
		if follow {
			fmt.Println("Following logs...")
		}
		if tail > 0 {
			fmt.Printf("Showing last %d lines\n", tail)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().StringVar(&since, "since", "", "show logs since duration (e.g., 1h, 30m)")
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow logs in real-time")
	logsCmd.Flags().IntVar(&tail, "tail", 0, "show last N lines")
}

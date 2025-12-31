package processors

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered processors",
		Long: `List all processor components registered with the control plane.

Processors are components that transform events from one type to another.
This command shows their input types, output types, health status, and endpoints.`,
		Example: `  # List all healthy processors
  flowctl processors list

  # List all processors including unhealthy ones
  flowctl processors list --include-unhealthy

  # List processors on a different control plane
  flowctl processors list --endpoint remote-host:8080`,
		RunE: runList,
	}
}

func runList(cmd *cobra.Command, args []string) error {
	// Connect to control plane
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to control plane at %s: %w\nHint: Make sure 'flowctl run' is running or the control plane is available", endpoint, err)
	}
	defer conn.Close()

	// Create client
	client := flowctlv1.NewControlPlaneServiceClient(conn)

	// List components, filtering for processors only
	resp, err := client.ListComponents(ctx, &flowctlv1.ListComponentsRequest{
		TypeFilter:       flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR,
		IncludeUnhealthy: includeUnhealthy,
	})
	if err != nil {
		return fmt.Errorf("failed to list processors: %w", err)
	}

	// Check if any processors found
	if len(resp.Components) == 0 {
		if includeUnhealthy {
			fmt.Println("No processors registered with the control plane.")
		} else {
			fmt.Println("No healthy processors registered with the control plane.")
			fmt.Println("Hint: Use --include-unhealthy to see unhealthy processors")
		}
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tINPUT TYPES\tOUTPUT TYPES\tSTATUS\tENDPOINT")
	fmt.Fprintln(w, "──\t───────────\t────────────\t──────\t────────")

	for _, comp := range resp.Components {
		if comp.Component == nil {
			continue
		}

		// Format input types
		inputTypes := formatEventTypes(comp.Component.InputEventTypes)
		outputTypes := formatEventTypes(comp.Component.OutputEventTypes)

		// Format status
		status := formatHealthStatus(comp.Status)

		// Get endpoint
		endpoint := comp.Component.Endpoint
		if endpoint == "" {
			endpoint = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			comp.Component.Id,
			inputTypes,
			outputTypes,
			status,
			endpoint,
		)
	}

	w.Flush()
	fmt.Printf("\nTotal: %d processor(s)\n", len(resp.Components))

	return nil
}

// formatEventTypes formats a list of event types for display
func formatEventTypes(types []string) string {
	if len(types) == 0 {
		return "-"
	}
	if len(types) == 1 {
		return types[0]
	}
	// Show first type + count if multiple
	return fmt.Sprintf("%s (+%d)", types[0], len(types)-1)
}

// formatHealthStatus formats health status with color indicators
func formatHealthStatus(status flowctlv1.HealthStatus) string {
	switch status {
	case flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY:
		return "✓ healthy"
	case flowctlv1.HealthStatus_HEALTH_STATUS_DEGRADED:
		return "⚠ degraded"
	case flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY:
		return "✗ unhealthy"
	case flowctlv1.HealthStatus_HEALTH_STATUS_STARTING:
		return "⟳ starting"
	case flowctlv1.HealthStatus_HEALTH_STATUS_STOPPING:
		return "⊗ stopping"
	default:
		return "? unknown"
	}
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// joinWithLimit joins strings with separator, truncating if needed
func joinWithLimit(items []string, sep string, maxLen int) string {
	result := strings.Join(items, sep)
	return truncateString(result, maxLen)
}

package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/runner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	statusControlPlaneAddr string
	statusIncludeUnhealthy bool
	statusWatch            bool
	statusWatchInterval    int
)

var statusCmd = &cobra.Command{
	Use:   "status [pipeline-file]",
	Short: "Show pipeline component status",
	Long: `Query the control plane to show real-time status of pipeline components.

This command connects to a running control plane and displays the current
state of all registered components including:
- Component health status
- Last heartbeat time
- Metrics (if available)
- Event types

Examples:
  # Show status for currently running pipeline
  flowctl status

  # Show status from specific control plane
  flowctl status --control-plane-addr localhost:9090

  # Include unhealthy components
  flowctl status --include-unhealthy

  # Watch status (refresh every 5 seconds)
  flowctl status --watch --interval 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Optional: Load pipeline file to get metadata
		var pipelineName string
		if len(args) > 0 {
			pipeline, err := runner.LoadPipelineFromFile(args[0])
			if err == nil {
				pipelineName = pipeline.Metadata.Name
			}
		}

		// Connect to control plane
		conn, err := grpc.NewClient(
			statusControlPlaneAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to control plane: %w", err)
		}
		defer conn.Close()

		client := flowctlv1.NewControlPlaneServiceClient(conn)
		ctx := context.Background()

		// If watch mode, loop
		if statusWatch {
			for {
				if err := displayStatus(ctx, client, pipelineName); err != nil {
					return err
				}
				time.Sleep(time.Duration(statusWatchInterval) * time.Second)
				fmt.Print("\033[H\033[2J") // Clear screen
			}
		}

		// Single status display
		return displayStatus(ctx, client, pipelineName)
	},
}

func displayStatus(ctx context.Context, client flowctlv1.ControlPlaneServiceClient, pipelineName string) error {
	// Query all components
	resp, err := client.ListComponents(ctx, &flowctlv1.ListComponentsRequest{
		IncludeUnhealthy: statusIncludeUnhealthy,
	})
	if err != nil {
		return fmt.Errorf("failed to list components: %w", err)
	}

	if len(resp.Components) == 0 {
		fmt.Println("No components registered with control plane")
		fmt.Println("Start a pipeline with: flowctl run pipeline.yaml")
		return nil
	}

	// Group components by type
	sources := []*flowctlv1.ComponentStatusResponse{}
	processors := []*flowctlv1.ComponentStatusResponse{}
	sinks := []*flowctlv1.ComponentStatusResponse{}

	for _, comp := range resp.Components {
		switch comp.Component.Type {
		case flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE:
			sources = append(sources, comp)
		case flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR:
			processors = append(processors, comp)
		case flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER:
			sinks = append(sinks, comp)
		}
	}

	// Count health statuses
	healthy := 0
	degraded := 0
	unhealthy := 0
	starting := 0
	for _, comp := range resp.Components {
		switch comp.Status {
		case flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY:
			healthy++
		case flowctlv1.HealthStatus_HEALTH_STATUS_DEGRADED:
			degraded++
		case flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY:
			unhealthy++
		case flowctlv1.HealthStatus_HEALTH_STATUS_STARTING:
			starting++
		}
	}

	// Print header
	if pipelineName != "" {
		fmt.Printf("Pipeline: %s\n", pipelineName)
	} else {
		fmt.Println("Pipeline Status")
	}

	status := "RUNNING"
	if unhealthy > 0 {
		status = "UNHEALTHY"
	} else if degraded > 0 {
		status = "DEGRADED"
	} else if starting > 0 {
		status = "STARTING"
	}

	fmt.Printf("Status: %s\n", colorizeStatus(status))
	fmt.Printf("Components: %d total (%d healthy", len(resp.Components), healthy)
	if starting > 0 {
		fmt.Printf(", %d starting", starting)
	}
	if degraded > 0 {
		fmt.Printf(", %d degraded", degraded)
	}
	if unhealthy > 0 {
		fmt.Printf(", %d unhealthy", unhealthy)
	}
	fmt.Println(")")
	fmt.Println()

	// Print sources
	if len(sources) > 0 {
		fmt.Printf("SOURCES (%d)\n", len(sources))
		for _, comp := range sources {
			printComponentStatus(comp)
		}
		fmt.Println()
	}

	// Print processors
	if len(processors) > 0 {
		fmt.Printf("PROCESSORS (%d)\n", len(processors))
		for _, comp := range processors {
			printComponentStatus(comp)
		}
		fmt.Println()
	}

	// Print sinks
	if len(sinks) > 0 {
		fmt.Printf("CONSUMERS (%d)\n", len(sinks))
		for _, comp := range sinks {
			printComponentStatus(comp)
		}
	}

	return nil
}

func printComponentStatus(comp *flowctlv1.ComponentStatusResponse) {
	statusStr := colorizeHealth(comp.Status.String())

	// Calculate uptime
	var uptime string
	if comp.RegisteredAt != nil {
		duration := time.Since(comp.RegisteredAt.AsTime())
		uptime = formatDuration(duration)
	}

	// Get last heartbeat
	var lastHeartbeat string
	if comp.LastHeartbeat != nil {
		duration := time.Since(comp.LastHeartbeat.AsTime())
		if duration < 5*time.Second {
			lastHeartbeat = "just now"
		} else {
			lastHeartbeat = fmt.Sprintf("%s ago", formatDuration(duration))
		}
	} else {
		lastHeartbeat = "never"
	}

	fmt.Printf("  %s (%s)\n", comp.Component.Id, statusStr)
	fmt.Printf("    Endpoint: %s\n", comp.Component.Endpoint)
	if uptime != "" {
		fmt.Printf("    Uptime: %s\n", uptime)
	}
	fmt.Printf("    Last heartbeat: %s\n", lastHeartbeat)

	// Print event types
	if len(comp.Component.InputEventTypes) > 0 {
		fmt.Printf("    Input types: %s\n", strings.Join(comp.Component.InputEventTypes, ", "))
	}
	if len(comp.Component.OutputEventTypes) > 0 {
		fmt.Printf("    Output types: %s\n", strings.Join(comp.Component.OutputEventTypes, ", "))
	}

	// Print metrics if available
	if len(comp.Metrics) > 0 {
		fmt.Println("    Metrics:")

		// Sort metric keys for consistent output
		keys := make([]string, 0, len(comp.Metrics))
		for k := range comp.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := comp.Metrics[key]
			fmt.Printf("      %s: %s\n", key, value)
		}
	}
}

func colorizeStatus(status string) string {
	switch status {
	case "RUNNING":
		return fmt.Sprintf("\033[32m%s\033[0m", status) // Green
	case "STARTING":
		return fmt.Sprintf("\033[36m%s\033[0m", status) // Cyan
	case "DEGRADED":
		return fmt.Sprintf("\033[33m%s\033[0m", status) // Yellow
	case "UNHEALTHY":
		return fmt.Sprintf("\033[31m%s\033[0m", status) // Red
	default:
		return status
	}
}

func colorizeHealth(health string) string {
	if strings.Contains(health, "HEALTHY") && !strings.Contains(health, "UNHEALTHY") {
		return fmt.Sprintf("\033[32m%s\033[0m", health) // Green
	} else if strings.Contains(health, "DEGRADED") {
		return fmt.Sprintf("\033[33m%s\033[0m", health) // Yellow
	} else if strings.Contains(health, "UNHEALTHY") {
		return fmt.Sprintf("\033[31m%s\033[0m", health) // Red
	} else if strings.Contains(health, "STARTING") {
		return fmt.Sprintf("\033[36m%s\033[0m", health) // Cyan
	}
	return health
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVar(&statusControlPlaneAddr, "control-plane-addr", "127.0.0.1:8080", "control plane address")
	statusCmd.Flags().BoolVar(&statusIncludeUnhealthy, "include-unhealthy", true, "include unhealthy components")
	statusCmd.Flags().BoolVar(&statusWatch, "watch", false, "watch mode (refresh periodically)")
	statusCmd.Flags().IntVar(&statusWatchInterval, "interval", 5, "watch interval in seconds")
}

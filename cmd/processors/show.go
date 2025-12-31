package processors

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func newShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <processor-id>",
		Short: "Show detailed information about a processor",
		Long: `Show detailed information about a specific processor component.

This command displays full details including:
- Component metadata (name, version, description)
- Input and output event types
- Health status and metrics
- Endpoint information
- Registration timestamp`,
		Example: `  # Show details for a specific processor
  flowctl processors show ttp-processor-v1

  # Show processor from remote control plane
  flowctl processors show ttp-processor-v1 --endpoint remote-host:8080`,
		Args: cobra.ExactArgs(1),
		RunE: runShow,
	}
}

func runShow(cmd *cobra.Command, args []string) error {
	processorID := args[0]

	// Connect to control plane
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to control plane at %s: %w", endpoint, err)
	}
	defer conn.Close()

	// Create client
	client := flowctlv1.NewControlPlaneServiceClient(conn)

	// Get component status
	resp, err := client.GetComponentStatus(ctx, &flowctlv1.ComponentStatusRequest{
		ServiceId: processorID,
	})
	if err != nil {
		return fmt.Errorf("processor '%s' not found: %w\nHint: Use 'flowctl processors list' to see available processors", processorID, err)
	}

	// Verify it's a processor
	if resp.Component == nil {
		return fmt.Errorf("component '%s' has no information available", processorID)
	}

	if resp.Component.Type != flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR {
		return fmt.Errorf("component '%s' is not a processor (type: %s)", processorID, resp.Component.Type)
	}

	// Print detailed information
	printProcessorDetails(resp)

	return nil
}

func printProcessorDetails(status *flowctlv1.ComponentStatusResponse) {
	comp := status.Component

	fmt.Printf("Processor: %s\n", comp.Id)
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	// Basic info
	fmt.Println("Basic Information:")
	if comp.Name != "" {
		fmt.Printf("  Name:        %s\n", comp.Name)
	}
	if comp.Version != "" {
		fmt.Printf("  Version:     %s\n", comp.Version)
	}
	if comp.Description != "" {
		fmt.Printf("  Description: %s\n", comp.Description)
	}
	fmt.Println()

	// Event types
	fmt.Println("Event Types:")
	fmt.Printf("  Input Types:  ")
	if len(comp.InputEventTypes) == 0 {
		fmt.Println("(none)")
	} else {
		fmt.Println()
		for _, t := range comp.InputEventTypes {
			fmt.Printf("    - %s\n", t)
		}
	}

	fmt.Printf("  Output Types: ")
	if len(comp.OutputEventTypes) == 0 {
		fmt.Println("(none)")
	} else {
		fmt.Println()
		for _, t := range comp.OutputEventTypes {
			fmt.Printf("    - %s\n", t)
		}
	}
	fmt.Println()

	// Connection info
	fmt.Println("Connection:")
	if comp.Endpoint != "" {
		fmt.Printf("  Endpoint:    %s\n", comp.Endpoint)
	} else {
		fmt.Printf("  Endpoint:    (not specified)\n")
	}
	fmt.Println()

	// Health status
	fmt.Println("Health:")
	fmt.Printf("  Status:       %s\n", formatHealthStatus(status.Status))
	if status.LastHeartbeat != nil {
		lastSeen := status.LastHeartbeat.AsTime()
		fmt.Printf("  Last Heartbeat: %s (%s ago)\n",
			lastSeen.Format("2006-01-02 15:04:05"),
			time.Since(lastSeen).Round(time.Second))
	}
	if status.RegisteredAt != nil {
		fmt.Printf("  Registered At:  %s\n", status.RegisteredAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	// Metrics
	if len(status.Metrics) > 0 {
		fmt.Println("Metrics:")
		for key, value := range status.Metrics {
			fmt.Printf("  %s: %s\n", key, value)
		}
		fmt.Println()
	}

	// Metadata
	if len(comp.Metadata) > 0 {
		fmt.Println("Metadata:")
		for key, value := range comp.Metadata {
			fmt.Printf("  %s: %s\n", key, value)
		}
		fmt.Println()
	}
}

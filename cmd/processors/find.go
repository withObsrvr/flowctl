package processors

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

var (
	inputType  string
	outputType string
	metadata   map[string]string
)

func newFindCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Find processors by event types or metadata",
		Long: `Find processors that match specific criteria.

This command helps discover processors based on:
- Input event types they accept
- Output event types they produce
- Metadata filters (network, blockchain, etc.)

This is the foundation for automatic topology discovery.`,
		Example: `  # Find processors that accept stellar.ledger.v1 events
  flowctl processors find --input stellar.ledger.v1

  # Find processors that produce token.transfer events
  flowctl processors find --output stellar.token.transfer.v1

  # Find processors for a specific network
  flowctl processors find --metadata network=testnet

  # Combine filters
  flowctl processors find --input stellar.ledger.v1 --metadata network=mainnet`,
		RunE: runFind,
	}

	cmd.Flags().StringVar(&inputType, "input", "", "Filter by input event type")
	cmd.Flags().StringVar(&outputType, "output", "", "Filter by output event type")
	cmd.Flags().StringToStringVar(&metadata, "metadata", nil, "Filter by metadata (key=value)")

	return cmd
}

func runFind(cmd *cobra.Command, args []string) error {
	// Validate that at least one filter is provided
	if inputType == "" && outputType == "" && len(metadata) == 0 {
		return fmt.Errorf("at least one filter must be specified (--input, --output, or --metadata)")
	}

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

	// Build discovery request
	req := &flowctlv1.DiscoveryRequest{
		Type:            flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR,
		MetadataFilters: metadata,
	}

	// Add event type filters
	var eventTypes []string
	if inputType != "" {
		eventTypes = append(eventTypes, inputType)
	}
	if outputType != "" {
		eventTypes = append(eventTypes, outputType)
	}
	req.EventTypes = eventTypes

	// Execute discovery
	resp, err := client.DiscoverComponents(ctx, req)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	// Filter results based on input/output specificity
	// (DiscoverComponents does OR logic on event types, we need to be more specific)
	var matches []*flowctlv1.ComponentInfo
	for _, comp := range resp.Components {
		if matchesFilters(comp, inputType, outputType) {
			matches = append(matches, comp)
		}
	}

	// Print results
	if len(matches) == 0 {
		fmt.Println("No processors found matching the specified criteria.")
		fmt.Println()
		fmt.Println("Search criteria:")
		if inputType != "" {
			fmt.Printf("  Input type:  %s\n", inputType)
		}
		if outputType != "" {
			fmt.Printf("  Output type: %s\n", outputType)
		}
		if len(metadata) > 0 {
			fmt.Println("  Metadata:")
			for k, v := range metadata {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
		return nil
	}

	// Print table header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tINPUT TYPES\tOUTPUT TYPES\tENDPOINT")
	fmt.Fprintln(w, "──\t───────────\t────────────\t────────")

	for _, comp := range matches {
		inputTypes := formatEventTypes(comp.InputEventTypes)
		outputTypes := formatEventTypes(comp.OutputEventTypes)

		endpoint := comp.Endpoint
		if endpoint == "" {
			endpoint = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			comp.Id,
			inputTypes,
			outputTypes,
			endpoint,
		)
	}

	w.Flush()
	fmt.Printf("\nFound: %d processor(s)\n", len(matches))

	return nil
}

// matchesFilters checks if a component matches the input/output type filters
func matchesFilters(comp *flowctlv1.ComponentInfo, inputType, outputType string) bool {
	// Check input type
	if inputType != "" {
		found := false
		for _, t := range comp.InputEventTypes {
			if t == inputType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check output type
	if outputType != "" {
		found := false
		for _, t := range comp.OutputEventTypes {
			if t == outputType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

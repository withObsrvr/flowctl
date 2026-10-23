package processors

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
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
  flowctl processors find --output-type stellar.token.transfer.v1

  # Find processors for a specific network
  flowctl processors find --metadata network=testnet

  # Combine filters
  flowctl processors find --input stellar.ledger.v1 --metadata network=mainnet`,
		RunE: runFind,
	}

	cmd.Flags().StringVar(&inputType, "input", "", "Filter by input event type")
	// Renamed from --output, which CLI-3 reserves for selecting an output format.
	// No deprecated alias: a local --output would shadow the global one, making
	// `--output json` filter by the event type "json" instead of emitting JSON.
	cmd.Flags().StringVar(&outputType, "output-type", "", "Filter by output event type")
	cmd.Flags().StringToStringVar(&metadata, "metadata", nil, "Filter by metadata (key=value)")

	return cmd
}

func runFind(cmd *cobra.Command, args []string) error {
	p := cliout.FromContext(cmd.Context())

	if inputType == "" && outputType == "" && len(metadata) == 0 {
		return exitcode.Errorf(exitcode.Usage,
			"at least one filter must be specified (--input, --output-type, or --metadata)")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		cliout.UnreachableInterceptor(endpoint),
	)
	if err != nil {
		return exitcode.Errorf(exitcode.Unavailable,
			"cannot reach the control plane at %s: %v", endpoint, err)
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

	resp, err := client.DiscoverComponents(ctx, req)
	if err != nil {
		return err
	}

	// No sanitize.Message here: cliout strips control characters as it formats
	// each cell, and machine output must keep the original bytes (CLI-22).

	// DiscoverComponents ORs the event-type filters, so narrow to components
	// matching every filter the user gave.
	rows := make([]cliout.Row, 0, len(resp.Components))
	for _, comp := range resp.Components {
		if matchesFilters(comp, inputType, outputType) {
			rows = append(rows, componentInfoRow(comp))
		}
	}

	if err := p.List(cliout.List{
		Schema:     processorListSchema,
		ItemSchema: processorSchema,
		Columns:    discoveryColumns(),
		Rows:       rows,
		Empty:      "No processors found matching " + describeFilters() + ".",
	}); err != nil {
		return err
	}

	if !p.Machine() && len(rows) > 0 {
		p.Info("\nFound: %d processor(s)", len(rows))
	}
	return nil
}

// describeFilters renders the active search criteria for the empty-result
// message, so a person can see what was actually asked for.
func describeFilters() string {
	var parts []string
	if inputType != "" {
		parts = append(parts, "input type "+inputType)
	}
	if outputType != "" {
		parts = append(parts, "output type "+outputType)
	}
	for k, v := range metadata {
		parts = append(parts, "metadata "+k+"="+v)
	}
	return strings.Join(parts, ", ")
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

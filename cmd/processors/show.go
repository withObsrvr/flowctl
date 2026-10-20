package processors

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
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
	p := cliout.FromContext(cmd.Context())
	processorID := args[0]

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return exitcode.Errorf(exitcode.Unavailable,
			"cannot reach the control plane at %s: %v", endpoint, err)
	}
	defer conn.Close()

	client := flowctlv1.NewControlPlaneServiceClient(conn)

	resp, err := client.GetComponentStatus(ctx, &flowctlv1.ComponentStatusRequest{
		ServiceId: processorID,
	})
	if err != nil {
		return exitcode.Errorf(exitcode.NotFound,
			"processor %q not found: %v\nHint: run 'flowctl processors list' to see what is registered", processorID, err)
	}

	// No sanitize.Message here: cliout strips control characters as it formats
	// each cell, and machine output must keep the original bytes (CLI-22).

	if resp.GetComponent() == nil {
		return exitcode.Errorf(exitcode.NotFound,
			"component %q has no information available", processorID)
	}
	if resp.Component.Type != flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR {
		return exitcode.Errorf(exitcode.Conflict,
			"component %q is not a processor (type: %s)", processorID, componentTypeName(resp.Component.Type))
	}

	row := componentRow(resp)
	if len(resp.GetMetrics()) > 0 {
		row["metrics"] = resp.GetMetrics()
	}
	if ts := resp.GetLastHeartbeat(); ts != nil {
		row["last_heartbeat_age"] = time.Since(ts.AsTime()).Round(time.Second).String()
	}

	return p.Object(cliout.Object{
		Schema: processorSchema,
		Fields: []cliout.Field{
			{Label: "Processor", Key: "id"},
			{Label: "Name", Key: "name"},
			{Label: "Version", Key: "version"},
			{Label: "Description", Key: "description"},
			{Label: "Input types", Key: "input_event_types", Format: listLines},
			{Label: "Output types", Key: "output_event_types", Format: listLines},
			{Label: "Endpoint", Key: "endpoint"},
			{Label: "Status", Key: "status"},
			{Label: "Last heartbeat", Key: "last_heartbeat"},
			{Label: "Registered at", Key: "registered_at"},
			{Label: "Metrics", Key: "metrics", Format: mapLines},
			{Label: "Metadata", Key: "metadata", Format: mapLines},
		},
		Data: row,
	})
}

// listLines renders a string slice for a detail view: all values, comma
// separated. The record keeps the slice itself.
func listLines(v any) string {
	items, ok := v.([]string)
	if !ok || len(items) == 0 {
		return "-"
	}
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = cliout.Value(s)
	}
	return strings.Join(out, ", ")
}

// mapLines renders a string map for a detail view, with keys sorted so the
// output is stable between runs.
func mapLines(v any) string {
	m, ok := v.(map[string]string)
	if !ok || len(m) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, cliout.Value(k)+"="+cliout.Value(m[k]))
	}
	return strings.Join(parts, ", ")
}

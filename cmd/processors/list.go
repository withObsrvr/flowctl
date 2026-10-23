package processors

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
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
  flowctl processors list --endpoint remote-host:8080

  # Every processor that accepts ledgers, as JSON
  flowctl processors list --output json |
    jq '.items[] | select(.input_event_types[] == "stellar.ledger.v1")'`,
		Args: cobra.NoArgs,
		RunE: runList,
	}
}

func runList(cmd *cobra.Command, args []string) error {
	p := cliout.FromContext(cmd.Context())

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		cliout.UnreachableInterceptor(endpoint),
	)
	if err != nil {
		return exitcode.Errorf(exitcode.Unavailable,
			"cannot reach the control plane at %s: %v\nHint: start 'flowctl run' or 'flowctl server' first", endpoint, err)
	}
	defer conn.Close()

	client := flowctlv1.NewControlPlaneServiceClient(conn)

	resp, err := client.ListComponents(ctx, &flowctlv1.ListComponentsRequest{
		TypeFilter:       flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR,
		IncludeUnhealthy: includeUnhealthy,
	})
	if err != nil {
		return err
	}

	// No sanitize.Message here: cliout strips control characters as it formats
	// each cell, and machine output must keep the original bytes (CLI-22).

	rows := make([]cliout.Row, 0, len(resp.Components))
	for _, comp := range resp.Components {
		if comp.GetComponent() == nil {
			continue
		}
		rows = append(rows, componentRow(comp))
	}

	empty := "No healthy processors registered with the control plane.\nHint: use --include-unhealthy to see unhealthy processors."
	if includeUnhealthy {
		empty = "No processors registered with the control plane."
	}

	if err := p.List(cliout.List{
		Schema:     processorListSchema,
		ItemSchema: processorSchema,
		Columns:    processorColumns(),
		Rows:       rows,
		Empty:      empty,
	}); err != nil {
		return err
	}

	// A count is a convenience for a person reading a terminal, not a result,
	// so it goes to stderr and is omitted when a program is reading (CLI-5).
	if !p.Machine() && len(rows) > 0 {
		p.Info("\nTotal: %d processor(s)", len(rows))
	}
	return nil
}

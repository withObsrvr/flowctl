package list

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
	pb "github.com/withobsrvr/flowctl/proto"
)

var (
	endpoint      string
	waitForServer bool
	timeout       time.Duration
)

// formatServiceType converts a ServiceType to a user-friendly string
func formatServiceType(serviceType pb.ServiceType) string {
	// Get the string representation from the enum
	typeStr := serviceType.String()

	// Handle the case where the enum value might not be in the generated map yet
	if typeStr == "" || typeStr == string(serviceType) {
		// Manual handling for known types including PIPELINE
		switch serviceType {
		case pb.ServiceType_SERVICE_TYPE_SOURCE:
			return "SOURCE"
		case pb.ServiceType_SERVICE_TYPE_PROCESSOR:
			return "PROCESSOR"
		case pb.ServiceType_SERVICE_TYPE_SINK:
			return "SINK"
		case pb.ServiceType_SERVICE_TYPE_PIPELINE: // SERVICE_TYPE_PIPELINE
			return "PIPELINE"
		default:
			return "UNKNOWN"
		}
	}

	// Remove the SERVICE_TYPE_ prefix if present
	if strings.HasPrefix(typeStr, "SERVICE_TYPE_") {
		return strings.TrimPrefix(typeStr, "SERVICE_TYPE_")
	}

	return typeStr
}

// NewCommand creates the list command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered services",
		Long: `List all services registered with the control plane.
If a pipeline is running, it will connect to the embedded control plane automatically.

Examples:
  # Human-readable table
  flowctl list

  # Every unhealthy service, for a monitoring check
  flowctl list --output json | jq '.items[] | select(.healthy | not)'

  # Stream records as they are produced
  flowctl list --output ndjson`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := cliout.FromContext(cmd.Context())

			// Only wait for the embedded control plane when a person is watching.
			// A script that asked for JSON wants an answer or an error, not a
			// two-second sleep (CLI-8).
			if endpoint == "localhost:8080" && waitForServer && !p.Machine() {
				p.Info("Checking for embedded control plane...")
				time.Sleep(2 * time.Second)
			}

			conn, err := grpc.NewClient(endpoint,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				cliout.UnreachableInterceptor(endpoint),
			)
			if err != nil {
				return exitcode.Errorf(exitcode.Unavailable,
					"cannot reach the control plane at %s: %v\nHint: start 'flowctl run' or 'flowctl server' first", endpoint, err)
			}
			defer conn.Close()

			client := pb.NewControlPlaneClient(conn)

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			resp, err := client.ListServices(ctx, &emptypb.Empty{})
			if err != nil {
				// exitcode.Of maps the gRPC status, so a dead control plane
				// exits 7 and a deadline exits 6 without classifying here.
				return err
			}

			rows := make([]cliout.Row, 0, len(resp.Services))
			for _, service := range resp.Services {
				row := cliout.Row{
					"id":      service.ServiceId,
					"type":    formatServiceType(service.ServiceType),
					"healthy": service.IsHealthy,
				}
				if service.LastHeartbeat != nil {
					row["last_heartbeat"] = service.LastHeartbeat.AsTime()
				}
				if len(service.Metrics) > 0 {
					row["metrics"] = service.Metrics
				}
				rows = append(rows, row)
			}

			return p.List(cliout.List{
				Schema:     "flowctl.service_list.v1",
				ItemSchema: "flowctl.service.v1",
				Columns: []cliout.Column{
					{Header: "ID", Key: "id"},
					{Header: "Type", Key: "type"},
					{Header: "Status", Key: "healthy", Format: health},
					{Header: "Last Heartbeat", Key: "last_heartbeat"},
				},
				Rows:  rows,
				Empty: "No services registered with the control plane.",
			})
		},
	}

	cmd.Flags().StringVarP(&endpoint, "endpoint", "e", "localhost:8080", "Control plane endpoint")
	cmd.Flags().BoolVar(&waitForServer, "wait", true, "Wait briefly for an embedded control plane")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "Timeout for the control plane call")

	return cmd
}

// health renders the boolean health field for human output.
func health(v any) string {
	if healthy, ok := v.(bool); ok && healthy {
		return "HEALTHY"
	}
	return "UNHEALTHY"
}

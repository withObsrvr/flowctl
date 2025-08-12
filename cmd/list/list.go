package list

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
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/withobsrvr/flowctl/proto"
)

var (
	endpoint     string
	waitForServer bool
	tableFormat  bool
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
		case pb.ServiceType(4): // SERVICE_TYPE_PIPELINE
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
If a pipeline is running, it will connect to the embedded control plane automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If endpoint is default, check if embedded control plane is running
			if endpoint == "localhost:8080" && waitForServer {
				fmt.Println("Checking for embedded control plane...")
				time.Sleep(2 * time.Second)
			}

			// Connect to control plane
			conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return fmt.Errorf("failed to connect to control plane at %s: %w\nHint: Make sure 'flowctl run' is running or start 'flowctl server'", endpoint, err)
			}
			defer conn.Close()

			// Create client
			client := pb.NewControlPlaneClient(conn)

			// Set timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// List services
			resp, err := client.ListServices(ctx, &emptypb.Empty{})
			if err != nil {
				return fmt.Errorf("failed to list services: %w", err)
			}

			// Print services
			if len(resp.Services) == 0 {
				fmt.Println("No services registered with the control plane.")
				return nil
			}

			if tableFormat {
				// Table format output
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tLAST HEARTBEAT")
				fmt.Fprintln(w, "----\t----\t------\t--------------")
				
				for _, service := range resp.Services {
					status := "UNHEALTHY"
					if service.IsHealthy {
						status = "HEALTHY"
					}
					
					lastHeartbeat := "Never"
					if service.LastHeartbeat != nil {
						lastHeartbeat = service.LastHeartbeat.AsTime().Format("2006-01-02 15:04:05")
					}
					
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						service.ServiceId,
						formatServiceType(service.ServiceType),
						status,
						lastHeartbeat,
					)
				}
				w.Flush()
			} else {
				// Detailed format output
				fmt.Printf("Registered Services (%d):\n", len(resp.Services))
				fmt.Println("========================")
				for _, service := range resp.Services {
					status := "🔴 UNHEALTHY"
					if service.IsHealthy {
						status = "🟢 HEALTHY"
					}

					fmt.Printf("ID: %s\n", service.ServiceId)
					fmt.Printf("Type: %s\n", formatServiceType(service.ServiceType))
					fmt.Printf("Status: %s\n", status)
					if service.LastHeartbeat != nil {
						fmt.Printf("Last Heartbeat: %s\n", service.LastHeartbeat.AsTime().Format(time.RFC3339))
					}
					
					if len(service.Metrics) > 0 {
						fmt.Println("Metrics:")
						for k, v := range service.Metrics {
							fmt.Printf("  %s: %.2f\n", k, v)
						}
					}
					fmt.Println("------------------------")
				}
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&endpoint, "endpoint", "e", "localhost:8080", "Control plane endpoint")
	cmd.Flags().BoolVar(&waitForServer, "wait", true, "Wait briefly for embedded control plane")
	cmd.Flags().BoolVarP(&tableFormat, "table", "t", false, "Display output in table format")

	return cmd
}
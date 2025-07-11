package list

import (
	"context"
	"fmt"
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
)

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

			fmt.Printf("Registered Services (%d):\n", len(resp.Services))
			fmt.Println("========================")
			for _, service := range resp.Services {
				status := "🔴 UNHEALTHY"
				if service.IsHealthy {
					status = "🟢 HEALTHY"
				}

				fmt.Printf("ID: %s\n", service.ServiceId)
				fmt.Printf("Type: %s\n", service.ServiceType)
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

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&endpoint, "endpoint", "e", "localhost:8080", "Control plane endpoint")
	cmd.Flags().BoolVar(&waitForServer, "wait", true, "Wait briefly for embedded control plane")

	return cmd
}
package cmd

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
	endpoint string
	listCmd  = &cobra.Command{
		Use:   "list",
		Short: "List registered services",
		Long:  `List all services registered with the control plane.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Connect to control plane
			conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return fmt.Errorf("failed to connect: %w", err)
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
			fmt.Println("Registered Services:")
			fmt.Println("------------------")
			for _, service := range resp.Services {
				fmt.Printf("ID: %s\n", service.ServiceId)
				fmt.Printf("Type: %s\n", service.ServiceType)
				fmt.Printf("Healthy: %v\n", service.IsHealthy)
				if service.LastHeartbeat != nil {
					fmt.Printf("Last Heartbeat: %s\n", service.LastHeartbeat.AsTime().Format(time.RFC3339))
				}
				fmt.Println("Metrics:")
				for k, v := range service.Metrics {
					fmt.Printf("  %s: %v\n", k, v)
				}
				fmt.Println("------------------")
			}

			return nil
		},
	}
)

func init() {
	listCmd.Flags().StringVarP(&endpoint, "endpoint", "e", "localhost:8080", "Control plane endpoint")
	rootCmd.AddCommand(listCmd)
}

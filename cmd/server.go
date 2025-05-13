package cmd

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/withobsrvr/flowctl/internal/api"
	pb "github.com/withobsrvr/flowctl/proto"
)

var (
	port      int
	serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Start the control plane server",
		Long:  `Start the control plane server that manages service registration and discovery.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create listener
			lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return fmt.Errorf("failed to listen: %w", err)
			}

			// Create gRPC server
			s := grpc.NewServer()

			// Register control plane service
			cp := api.NewControlPlaneServer()
			pb.RegisterControlPlaneServer(s, cp)
			reflection.Register(s)

			// Handle signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Println("\nShutting down server...")
				s.GracefulStop()
			}()

			// Start server
			fmt.Printf("Starting control plane server on port %d...\n", port)
			if err := s.Serve(lis); err != nil {
				return fmt.Errorf("failed to serve: %w", err)
			}

			return nil
		},
	}
)

func init() {
	serverCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on")
	rootCmd.AddCommand(serverCmd)
}

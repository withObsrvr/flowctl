package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/api"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	pb "github.com/withobsrvr/flowctl/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	// Options for the server command
	opts struct {
		Port            int
		Address         string
		TLSCert         string
		TLSKey          string
		HeartbeatTTL    time.Duration
		JanitorInterval time.Duration
	}
)

// NewCommand creates the server command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the control plane server",
		Long: `Run the Flow control plane server that manages pipeline deployments, 
monitoring, and scaling. This server exposes a gRPC API for pipeline components.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Starting server",
				zap.String("address", opts.Address),
				zap.Int("port", opts.Port),
				zap.Duration("heartbeat_ttl", opts.HeartbeatTTL),
				zap.Duration("janitor_interval", opts.JanitorInterval))

			// Set up the listener
			address := fmt.Sprintf("%s:%d", opts.Address, opts.Port)
			listener, err := net.Listen("tcp", address)
			if err != nil {
				return fmt.Errorf("failed to listen: %w", err)
			}

			// Create gRPC server options
			var serverOptions []grpc.ServerOption
			
			// Configure TLS if enabled
			useTLS := opts.TLSCert != "" && opts.TLSKey != ""
			if useTLS {
				logger.Info("TLS enabled",
					zap.String("cert", opts.TLSCert),
					zap.String("key", opts.TLSKey))
				
				creds, err := credentials.NewServerTLSFromFile(opts.TLSCert, opts.TLSKey)
				if err != nil {
					return fmt.Errorf("failed to load TLS credentials: %w", err)
				}
				serverOptions = append(serverOptions, grpc.Creds(creds))
			}

			// Create gRPC server
			grpcServer := grpc.NewServer(serverOptions...)
			
			// Create control plane server
			controlPlane := api.NewControlPlaneServer()
			
			// Configure heartbeat TTL and janitor interval
			if opts.HeartbeatTTL > 0 {
				controlPlane.SetHeartbeatTTL(opts.HeartbeatTTL)
			}
			if opts.JanitorInterval > 0 {
				controlPlane.SetJanitorInterval(opts.JanitorInterval)
			}
			
			// Start the health check janitor
			controlPlane.Start()
			
			// Register the control plane service
			pb.RegisterControlPlaneServer(grpcServer, controlPlane)

			// Set up signal handling for graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			// Handle signals for graceful shutdown
			shutdown := make(chan os.Signal, 1)
			signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

			// Start the server in a goroutine
			go func() {
				logger.Info("Server listening", zap.String("address", address))
				if err := grpcServer.Serve(listener); err != nil {
					logger.Error("Server error", zap.Error(err))
					cancel()
				}
			}()

			// Wait for shutdown signal
			select {
			case <-ctx.Done():
				logger.Info("Context cancelled")
			case sig := <-shutdown:
				logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
			}

			// Graceful shutdown
			logger.Info("Shutting down server...")
			grpcServer.GracefulStop()
			controlPlane.Close()
			logger.Info("Server shutdown complete")

			return nil
		},
	}

	// Add flags
	cmd.Flags().IntVar(&opts.Port, "port", 8080, "port to listen on")
	cmd.Flags().StringVar(&opts.Address, "address", "0.0.0.0", "address to listen on")
	cmd.Flags().StringVar(&opts.TLSCert, "tls-cert", "", "TLS certificate file")
	cmd.Flags().StringVar(&opts.TLSKey, "tls-key", "", "TLS key file")
	cmd.Flags().DurationVar(&opts.HeartbeatTTL, "heartbeat-ttl", 30*time.Second, "heartbeat time-to-live duration")
	cmd.Flags().DurationVar(&opts.JanitorInterval, "janitor-interval", 10*time.Second, "interval for checking service health")

	return cmd
}
package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/api"
	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/storage"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	// Options for the server command
	opts struct {
		Port            int
		Address         string
		TLSCert         string
		TLSKey          string
		TLSCACert       string
		HeartbeatTTL    time.Duration
		JanitorInterval time.Duration
		DBPath          string
		NoPersistence   bool
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
			
			// Configure TLS based on CLI flags
			tlsConfig := &config.TLSConfig{
				Mode:     config.TLSModeDisabled,
				CertFile: opts.TLSCert,
				KeyFile:  opts.TLSKey,
				CAFile:   opts.TLSCACert,
			}
			
			// Enable TLS if cert and key are provided
			if opts.TLSCert != "" && opts.TLSKey != "" {
				tlsConfig.Mode = config.TLSModeEnabled
				
				// Enable mutual TLS if CA certificate is provided
				if opts.TLSCACert != "" {
					tlsConfig.Mode = config.TLSModeMutual
				}
				
				logger.Info("TLS configuration",
					zap.String("mode", string(tlsConfig.Mode)),
					zap.String("cert", tlsConfig.CertFile),
					zap.String("key", tlsConfig.KeyFile),
					zap.String("ca_cert", tlsConfig.CAFile))
				
				// Load server credentials from TLS config
				serverCreds, err := tlsConfig.LoadServerCredentials()
				if err != nil {
					return fmt.Errorf("failed to load TLS credentials: %w", err)
				}
				
				// Add TLS credentials to server options if enabled
				if serverCreds != nil {
					serverOptions = append(serverOptions, serverCreds)
				}
			}

			// Create gRPC server
			grpcServer := grpc.NewServer(serverOptions...)
			
			// Create and configure storage if persistence is enabled
			var serviceStorage storage.ServiceStorage
			if !opts.NoPersistence {
				dbPath := opts.DBPath
				if dbPath == "" {
					// Use default path in user's home directory if not specified
					homeDir, err := os.UserHomeDir()
					if err != nil {
						return fmt.Errorf("failed to get user home directory: %w", err)
					}
					dbPath = filepath.Join(homeDir, ".flowctl", storage.DefaultBoltFilePath)
				}
				
				// Ensure directory exists
				dbDir := filepath.Dir(dbPath)
				if err := os.MkdirAll(dbDir, 0755); err != nil {
					return fmt.Errorf("failed to create directory for database: %w", err)
				}
				
				logger.Info("Using persistent storage for service registry", zap.String("path", dbPath))
				serviceStorage = storage.NewBoltDBStorage(&storage.BoltOptions{
					Path: dbPath,
				})
			} else {
				logger.Info("Persistence disabled, using in-memory service registry")
			}
			
			// Create control plane server
			controlPlane := api.NewControlPlaneServer(serviceStorage)
			
			// Configure heartbeat TTL and janitor interval
			if opts.HeartbeatTTL > 0 {
				controlPlane.SetHeartbeatTTL(opts.HeartbeatTTL)
			}
			if opts.JanitorInterval > 0 {
				controlPlane.SetJanitorInterval(opts.JanitorInterval)
			}
			
			// Start the health check janitor and initialize storage
			if err := controlPlane.Start(); err != nil {
				return fmt.Errorf("failed to start control plane server: %w", err)
			}
			
			// Register the control plane service
			flowctlv1.RegisterControlPlaneServiceServer(grpcServer, controlPlane)

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
			if err := controlPlane.Close(); err != nil {
				logger.Error("Error closing control plane", zap.Error(err))
			}
			logger.Info("Server shutdown complete")

			return nil
		},
	}

	// Add flags
	cmd.Flags().IntVar(&opts.Port, "port", 8080, "port to listen on")
	cmd.Flags().StringVar(&opts.Address, "address", "0.0.0.0", "address to listen on")
	cmd.Flags().StringVar(&opts.TLSCert, "tls-cert", "", "TLS certificate file")
	cmd.Flags().StringVar(&opts.TLSKey, "tls-key", "", "TLS key file")
	cmd.Flags().StringVar(&opts.TLSCACert, "tls-ca-cert", "", "TLS CA certificate file for mutual TLS")
	cmd.Flags().DurationVar(&opts.HeartbeatTTL, "heartbeat-ttl", 30*time.Second, "heartbeat time-to-live duration")
	cmd.Flags().DurationVar(&opts.JanitorInterval, "janitor-interval", 10*time.Second, "interval for checking service health")
	cmd.Flags().StringVar(&opts.DBPath, "db-path", "", "path to the BoltDB file for service registry persistence")
	cmd.Flags().BoolVar(&opts.NoPersistence, "no-persistence", false, "disable persistence for service registry")

	return cmd
}
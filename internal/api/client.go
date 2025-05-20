package api

import (
	"fmt"

	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	pb "github.com/withobsrvr/flowctl/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientOptions contains options for creating a gRPC client connection.
type ClientOptions struct {
	// ServerAddress is the address of the control plane server.
	ServerAddress string
	
	// TLSConfig is the TLS configuration to use for the connection.
	// If nil, an insecure connection will be used.
	TLSConfig *config.TLSConfig
	
	// Additional gRPC dial options to use.
	DialOptions []grpc.DialOption
}

// CreateControlPlaneClient creates a new control plane client with the given options.
func CreateControlPlaneClient(opts *ClientOptions) (pb.ControlPlaneClient, *grpc.ClientConn, error) {
	if opts == nil {
		return nil, nil, fmt.Errorf("client options cannot be nil")
	}
	
	if opts.ServerAddress == "" {
		return nil, nil, fmt.Errorf("server address is required")
	}
	
	// Prepare dial options
	dialOpts := make([]grpc.DialOption, 0, len(opts.DialOptions)+1)
	
	// Add TLS options if configured
	if opts.TLSConfig != nil && opts.TLSConfig.Mode != config.TLSModeDisabled {
		logger.Info("Configuring TLS for client connection",
			zap.String("server", opts.ServerAddress),
			zap.String("mode", string(opts.TLSConfig.Mode)))
			
		tlsOpt, err := opts.TLSConfig.LoadClientCredentials()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		
		dialOpts = append(dialOpts, tlsOpt)
	} else {
		// Use insecure credentials if TLS is disabled
		logger.Info("Using insecure connection (TLS disabled)",
			zap.String("server", opts.ServerAddress))
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	
	// Add user-provided dial options
	dialOpts = append(dialOpts, opts.DialOptions...)
	
	// Connect to the server
	conn, err := grpc.Dial(opts.ServerAddress, dialOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to control plane: %w", err)
	}
	
	// Create the client
	client := pb.NewControlPlaneClient(conn)
	
	return client, conn, nil
}

// MustCreateControlPlaneClient creates a new control plane client and panics if an error occurs.
// This is useful for applications that can't continue without a valid connection.
func MustCreateControlPlaneClient(opts *ClientOptions) (pb.ControlPlaneClient, *grpc.ClientConn) {
	client, conn, err := CreateControlPlaneClient(opts)
	if err != nil {
		logger.Fatal("Failed to create control plane client", zap.Error(err))
		// The above call will exit the application, but we need to return something
		// to satisfy the compiler
		panic(err)
	}
	
	return client, conn
}
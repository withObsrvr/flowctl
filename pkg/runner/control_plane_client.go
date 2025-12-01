package runner

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/withobsrvr/flowctl/proto"
)

// ControlPlaneClientImpl implements ControlPlaneClient using gRPC
type ControlPlaneClientImpl struct {
	conn   *grpc.ClientConn
	client pb.ControlPlaneClient
	logger *zap.Logger
}

// NewControlPlaneClient creates a new control plane client
func NewControlPlaneClient(endpoint string, logger *zap.Logger) (*ControlPlaneClientImpl, error) {
	if endpoint == "" {
		endpoint = "localhost:8080"
	}
	
	// Connect to control plane
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to control plane: %w", err)
	}
	
	return &ControlPlaneClientImpl{
		conn:   conn,
		client: pb.NewControlPlaneClient(conn),
		logger: logger,
	}, nil
}

// Close closes the connection to the control plane
func (c *ControlPlaneClientImpl) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RegisterPipeline registers a pipeline with the control plane
func (c *ControlPlaneClientImpl) RegisterPipeline(ctx context.Context, pipelineID string, metadata map[string]string) error {
	serviceInfo := &pb.ServiceInfo{
		ServiceId:      pipelineID,
		ServiceType:    pb.ServiceType_SERVICE_TYPE_PIPELINE,
		HealthEndpoint: fmt.Sprintf("docker://container/%s/health", pipelineID),
		MaxInflight:    100, // Default value
		Metadata:       metadata,
	}
	
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	ack, err := c.client.Register(ctx, serviceInfo)
	if err != nil {
		return fmt.Errorf("failed to register pipeline: %w", err)
	}
	
	c.logger.Info("Pipeline registered with control plane",
		zap.String("service_id", ack.ServiceId),
		zap.Strings("topics", ack.TopicNames))
	
	return nil
}

// IsServiceRegistered checks if a service is registered
func (c *ControlPlaneClientImpl) IsServiceRegistered(ctx context.Context, serviceID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	// List all services
	serviceList, err := c.client.ListServices(ctx, &emptypb.Empty{})
	if err != nil {
		return false, fmt.Errorf("failed to list services: %w", err)
	}
	
	// Check if our service is in the list
	// Match by component_id (preferred) or service_id (legacy)
	for _, svc := range serviceList.Services {
		if svc.ComponentId == serviceID || svc.ServiceId == serviceID {
			return true, nil
		}
	}

	return false, nil
}

// SendHeartbeat sends a heartbeat for a service
func (c *ControlPlaneClientImpl) SendHeartbeat(ctx context.Context, serviceID string, metrics map[string]float64) error {
	heartbeat := &pb.ServiceHeartbeat{
		ServiceId: serviceID,
		Metrics:   metrics,
		// Timestamp is set by the server
	}
	
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	_, err := c.client.Heartbeat(ctx, heartbeat)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	
	return nil
}
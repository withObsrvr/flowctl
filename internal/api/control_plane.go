package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/withobsrvr/flowctl/proto"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Service represents a registered service
type Service struct {
	Info     *pb.ServiceInfo
	Status   *pb.ServiceStatus
	LastSeen time.Time
	Conn     *grpc.ClientConn
}

// ControlPlaneServer implements the control plane gRPC service
type ControlPlaneServer struct {
	pb.UnimplementedControlPlaneServer
	mu       sync.RWMutex
	services map[string]*Service
}

// NewControlPlaneServer creates a new control plane server
func NewControlPlaneServer() *ControlPlaneServer {
	return &ControlPlaneServer{
		services: make(map[string]*Service),
	}
}

// Register implements the Register RPC
func (s *ControlPlaneServer) Register(ctx context.Context, info *pb.ServiceInfo) (*pb.RegistrationAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a unique service ID if not provided
	if info.ServiceId == "" {
		info.ServiceId = fmt.Sprintf("%s-%d", info.ServiceType, time.Now().UnixNano())
	}

	// Create service record
	service := &Service{
		Info:     info,
		Status:   &pb.ServiceStatus{ServiceId: info.ServiceId, ServiceType: info.ServiceType},
		LastSeen: time.Now(),
	}

	// Store service
	s.services[info.ServiceId] = service

	// Log service registration
	logger.Info("Service registered",
		zap.String("service_id", info.ServiceId),
		zap.String("service_type", info.ServiceType.String()),
		zap.String("health_endpoint", info.HealthEndpoint),
		zap.Strings("input_event_types", info.InputEventTypes),
		zap.Strings("output_event_types", info.OutputEventTypes),
		zap.Int32("max_inflight", info.MaxInflight),
	)

	// Generate topic names based on service type and event types
	var topicNames []string
	switch info.ServiceType {
	case pb.ServiceType_SERVICE_TYPE_SOURCE:
		for _, eventType := range info.OutputEventTypes {
			topicNames = append(topicNames, fmt.Sprintf("%s.v1", eventType))
		}
	case pb.ServiceType_SERVICE_TYPE_PROCESSOR:
		for _, eventType := range info.OutputEventTypes {
			topicNames = append(topicNames, fmt.Sprintf("%s.v1", eventType))
		}
	}

	// Return registration acknowledgment
	return &pb.RegistrationAck{
		ServiceId:  info.ServiceId,
		TopicNames: topicNames,
		ConnectionInfo: map[string]string{
			"control_plane_endpoint": "localhost:8080",
		},
	}, nil
}

// Heartbeat implements the Heartbeat RPC
func (s *ControlPlaneServer) Heartbeat(ctx context.Context, hb *pb.ServiceHeartbeat) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[hb.ServiceId]
	if !ok {
		return nil, fmt.Errorf("service %s not found", hb.ServiceId)
	}

	service.LastSeen = time.Now()
	service.Status.Metrics = hb.Metrics
	service.Status.IsHealthy = true

	// Log heartbeat receipt
	logger.Info("Received heartbeat",
		zap.String("service_id", hb.ServiceId),
		zap.String("service_type", service.Info.ServiceType.String()),
		zap.Any("metrics", hb.Metrics),
	)

	return &emptypb.Empty{}, nil
}

// GetServiceStatus implements the GetServiceStatus RPC
func (s *ControlPlaneServer) GetServiceStatus(ctx context.Context, info *pb.ServiceInfo) (*pb.ServiceStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.services[info.ServiceId]
	if !ok {
		return nil, fmt.Errorf("service %s not found", info.ServiceId)
	}

	return service.Status, nil
}

// ListServices implements the ListServices RPC
func (s *ControlPlaneServer) ListServices(ctx context.Context, _ *emptypb.Empty) (*pb.ServiceList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]*pb.ServiceStatus, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service.Status)
	}

	return &pb.ServiceList{Services: services}, nil
}

// A standalone process-driver fixture for delivery failure and completion tests.
package main

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type sourceService struct {
	flowctlv1.UnimplementedSourceServiceServer
}
type processorService struct {
	flowctlv1.UnimplementedProcessorServiceServer
}
type sinkService struct {
	flowctlv1.UnimplementedConsumerServiceServer
}

func (sourceService) StreamEvents(_ *flowctlv1.StreamRequest, stream grpc.ServerStreamingServer[flowctlv1.Event]) error {
	if os.Getenv("DELIVERY_MODE") != "fail-before" {
		if err := stream.Send(&flowctlv1.Event{Id: os.Getenv("FLOWCTL_COMPONENT_ID"), Type: "test.event"}); err != nil {
			return err
		}
	}
	switch os.Getenv("DELIVERY_MODE") {
	case "fail-before", "fail-after":
		return status.Error(codes.Internal, "synthetic source failure")
	case "unbounded-eof", "bounded":
		return nil
	default:
		<-stream.Context().Done()
		return stream.Context().Err()
	}
}
func (processorService) Process(stream grpc.BidiStreamingServer[flowctlv1.Event, flowctlv1.Event]) error {
	if os.Getenv("DELIVERY_MODE") == "processor-fail" {
		return status.Error(codes.Internal, "synthetic processor failure")
	}
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(event); err != nil {
			return err
		}
	}
}
func (sinkService) Consume(stream grpc.ClientStreamingServer[flowctlv1.Event, flowctlv1.ConsumeResponse]) error {
	if os.Getenv("DELIVERY_MODE") == "sink-fail" {
		return status.Error(codes.Internal, "synthetic sink failure")
	}
	for {
		event, err := stream.Recv()
		if err == nil {
			log.Printf("DELIVERY_RECEIVED id=%s", event.Id)
		}
		if err == io.EOF {
			return stream.SendAndClose(&flowctlv1.ConsumeResponse{})
		}
		if err != nil {
			return err
		}
	}
}
func main() {
	id, role, endpoint := os.Getenv("FLOWCTL_COMPONENT_ID"), os.Getenv("DELIVERY_ROLE"), os.Getenv("FLOWCTL_ENDPOINT")
	if id == "" || role == "" || endpoint == "" {
		log.Fatal("missing fixture environment")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	kind := flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE
	switch role {
	case "source":
		flowctlv1.RegisterSourceServiceServer(server, sourceService{})
	case "processor":
		kind = flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR
		flowctlv1.RegisterProcessorServiceServer(server, processorService{})
	case "sink":
		kind = flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER
		flowctlv1.RegisterConsumerServiceServer(server, sinkService{})
	default:
		log.Fatal("unknown fixture role")
	}
	go func() {
		if err := server.Serve(listener); err != nil {
			log.Print(err)
		}
	}()
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := flowctlv1.NewControlPlaneServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.RegisterComponent(ctx, &flowctlv1.RegisterRequest{ComponentId: id, Component: &flowctlv1.ComponentInfo{Id: id, Name: id, Type: kind, Endpoint: listener.Addr().String(), InputEventTypes: []string{"test.event"}, OutputEventTypes: []string{"test.event"}}})
	if err != nil {
		log.Fatal(err)
	}
	heartbeat := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = client.Heartbeat(ctx, &flowctlv1.HeartbeatRequest{ServiceId: resp.ServiceId, Status: flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY})
	}
	heartbeat()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(sig)
	for {
		select {
		case <-ticker.C:
			heartbeat()
		case <-sig:
			return
		}
	}
}

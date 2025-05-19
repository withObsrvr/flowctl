package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/withobsrvr/flowctl/proto"
)

func main() {
	log.Println("Starting test client...")
	// Connect to flowctl control plane
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to control plane: %v", err)
	}
	defer conn.Close()

	// Get connection info
	log.Println("Connection info:", map[string]string{
		"control_plane_endpoint": "localhost:8080",
	})

	// Create client
	client := pb.NewControlPlaneClient(conn)

	// Create service info
	serviceInfo := &pb.ServiceInfo{
		ServiceType:      pb.ServiceType_SERVICE_TYPE_SOURCE,
		OutputEventTypes: []string{"raw_ledger_service.RawLedgerChunk"},
		HealthEndpoint:   "localhost:9090/metrics",
		MaxInflight:      100,
		Metadata: map[string]string{
			"network":     "testnet",
			"ledger_type": "stellar",
		},
	}

	// Register with control plane
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ack, err := client.Register(ctx, serviceInfo)
	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}

	log.Printf("Registration successful - Service ID: %s", ack.ServiceId)
	
	// Store the service ID for future heartbeats
	serviceID := ack.ServiceId

	// Start heartbeat loop in background
	heartbeatTicker := time.NewTicker(5 * time.Second)
	defer heartbeatTicker.Stop()
	
	// Create a signal channel for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	log.Printf("Starting heartbeat loop (every 5s) - press Ctrl+C to exit")
	
	// Periodically check service status too
	statusTicker := time.NewTicker(15 * time.Second)
	defer statusTicker.Stop()
	
	for {
		select {
		case <-heartbeatTicker.C:
			// Send heartbeat
			heartbeat := &pb.ServiceHeartbeat{
				ServiceId:  serviceID,
				Timestamp:  timestamppb.Now(),
				Metrics: map[string]float64{
					"chunks_per_second": 12.5,
					"last_ledger":       45678923,
				},
			}
			
			hbCtx, hbCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, hbErr := client.Heartbeat(hbCtx, heartbeat)
			if hbErr != nil {
				log.Printf("Failed to send heartbeat: %v", hbErr)
			} else {
				log.Printf("Sent heartbeat for service %s", serviceID)
			}
			hbCancel()
			
		case <-statusTicker.C:
			// Check our status to verify health state
			stCtx, stCancel := context.WithTimeout(context.Background(), 2*time.Second)
			status, stErr := client.GetServiceStatus(stCtx, &pb.ServiceInfo{ServiceId: serviceID})
			if stErr != nil {
				log.Printf("Failed to get service status: %v", stErr)
			} else {
				log.Printf("Service status: ID=%s, Healthy=%v, Last Heartbeat=%v", 
					status.ServiceId, 
					status.IsHealthy,
					status.LastHeartbeat.AsTime())
			}
			stCancel()
			
		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down", sig)
			return
		}
	}
}
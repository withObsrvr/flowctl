//go:build ignore

package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/withobsrvr/flowctl/proto"
)

func registerProcessorWithControlPlane() {
	// Connect to flowctl control plane
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to control plane: %v", err)
	}
	defer conn.Close()

	// Create client
	client := pb.NewControlPlaneClient(conn)

	// Create service info
	serviceInfo := &pb.ServiceInfo{
		ServiceType:      pb.ServiceType_SERVICE_TYPE_PROCESSOR,
		InputEventTypes:  []string{"raw_ledger_service.RawLedgerChunk"},
		OutputEventTypes: []string{"token_transfer.TokenTransferEvent"},
		HealthEndpoint:   "localhost:9091/metrics",
		MaxInflight:      50,
		Metadata: map[string]string{
			"processor_type": "token_transfer_processor",
			"version":        "0.2.0",
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

	// Start heartbeat loop
	go func() {
		for {
			time.Sleep(10 * time.Second)
			
			// Send heartbeat
			heartbeat := &pb.ServiceHeartbeat{
				ServiceId:  serviceID,
				Timestamp:  timestamppb.Now(),
				Metrics: map[string]float64{
					"events_processed": 157.3,
					"processing_lag_ms": 234.5,
					"error_rate": 0.002,
				},
			}
			
			hbCtx, hbCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, hbErr := client.Heartbeat(hbCtx, heartbeat)
			if hbErr != nil {
				log.Printf("Failed to send heartbeat: %v", hbErr)
			}
			hbCancel()
		}
	}()
}
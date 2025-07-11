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

func registerSourceWithControlPlane() {
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

	// Start heartbeat loop
	go func() {
		for {
			time.Sleep(10 * time.Second)
			
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
			}
			hbCancel()
		}
	}()
}
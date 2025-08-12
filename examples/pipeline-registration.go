package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/withobsrvr/flowctl/proto"
)

func main() {
	// Connect to control plane
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewControlPlaneClient(conn)

	// Register a CDP pipeline service
	info := &pb.ServiceInfo{
		ServiceId:      "cdp-pipeline-001",
		ServiceType:    pb.ServiceType(4), // SERVICE_TYPE_PIPELINE
		HealthEndpoint: "http://localhost:9090/metrics",
		MaxInflight:    100,
		Metadata: map[string]string{
			"pipeline_name": "stellar_ledger_processor",
			"version":       "1.0.0",
			"source":        "captive-core",
			"processors":    "filter-payments,transform-app-payment",
			"consumer":      "save-to-postgresql",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ack, err := client.Register(ctx, info)
	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}

	fmt.Printf("Pipeline registered successfully!\n")
	fmt.Printf("Service ID: %s\n", ack.ServiceId)
	fmt.Printf("Topics: %v\n", ack.TopicNames)

	// Send periodic heartbeats
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		heartbeat := &pb.ServiceHeartbeat{
			ServiceId: ack.ServiceId,
			Timestamp: nil, // Will be set by the server
			Metrics: map[string]float64{
				"processed_ledgers": 1000,
				"processing_rate":   50.5,
				"error_count":       0,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := client.Heartbeat(ctx, heartbeat)
		cancel()

		if err != nil {
			log.Printf("Failed to send heartbeat: %v", err)
		} else {
			fmt.Printf("Heartbeat sent for pipeline %s\n", ack.ServiceId)
		}
	}
}
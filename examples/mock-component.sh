#!/bin/bash

# Mock component that simulates flowctl SDK integration
# This script demonstrates how a real component would integrate with flowctl

COMPONENT_ID=${FLOWCTL_SERVICE_ID:-"mock-component"}
COMPONENT_TYPE=${FLOWCTL_SERVICE_TYPE:-"source"}
FLOWCTL_ENDPOINT=${FLOWCTL_ENDPOINT:-"http://localhost:8080"}
HEARTBEAT_INTERVAL=${FLOWCTL_HEARTBEAT_INTERVAL:-"10s"}
ENABLE_FLOWCTL=${ENABLE_FLOWCTL:-"false"}

echo "Mock Component Starting..."
echo "Component ID: $COMPONENT_ID"
echo "Component Type: $COMPONENT_TYPE"
echo "Flowctl Endpoint: $FLOWCTL_ENDPOINT"
echo "Flowctl Enabled: $ENABLE_FLOWCTL"

# Simulate component registration if flowctl is enabled
if [ "$ENABLE_FLOWCTL" = "true" ]; then
    echo "Registering with flowctl control plane..."
    
    # In a real component, this would be done via gRPC
    # For now, we just simulate the registration
    sleep 2
    echo "Registered with control plane"
    
    # Start heartbeat loop in background
    (
        while true; do
            echo "Sending heartbeat..."
            sleep 10
        done
    ) &
    HEARTBEAT_PID=$!
    
    # Cleanup on exit
    trap "kill $HEARTBEAT_PID 2>/dev/null || true" EXIT
fi

# Simulate component work based on type
case $COMPONENT_TYPE in
    "source")
        echo "Starting source component..."
        for i in {1..10}; do
            echo "Source event $i: $(date)"
            sleep 2
        done
        ;;
    "processor")
        echo "Starting processor component..."
        while read line; do
            echo "Processed: $line"
            sleep 1
        done
        ;;
    "sink")
        echo "Starting sink component..."
        while read line; do
            echo "Sink received: $line"
            sleep 1
        done
        ;;
    *)
        echo "Unknown component type: $COMPONENT_TYPE"
        exit 1
        ;;
esac

echo "Mock component finished"
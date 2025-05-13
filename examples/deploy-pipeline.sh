#!/bin/bash
set -e

# Start the flowctl control plane server
echo "Starting flowctl control plane server..."
../bin/flowctl server --port 8080 &
SERVER_PID=$!

# Give the server time to start
sleep 2

# Apply the pipeline configuration
echo "Applying pipeline configuration..."
../bin/flowctl apply -f stellar-pipeline.yaml

# Optional: List the registered services
echo "Listing registered services..."
../bin/flowctl list

echo "Pipeline deployed successfully!"
echo "Control plane server running with PID: $SERVER_PID"
echo "To stop the server: kill $SERVER_PID"
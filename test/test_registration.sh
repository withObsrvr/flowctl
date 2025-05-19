#!/bin/bash

# Start the flowctl server in the background
echo "Starting flowctl server..."
./flowctl server &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Run the test client
echo "Running test client..."
go run test_client.go

# Cleanup
kill $SERVER_PID
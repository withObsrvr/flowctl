#!/usr/bin/env bash
# Demo script for Shapes 09-11: Pipeline Orchestration & Observability

set -e

echo "=== Flowctl Orchestration Demo ==="
echo ""
echo "This demo shows the new pipeline orchestration features:"
echo "  - Shape 09: Pipeline Run Tracking API"
echo "  - Shape 10: Pipeline Management CLI"
echo "  - Shape 11: Interactive TUI Dashboard"
echo ""

# Start control plane with persistent storage
echo "Step 1: Starting control plane with persistent storage..."
./bin/flowctl server --db-path ./flowctl-demo.db &
SERVER_PID=$!
sleep 2
echo "✓ Control plane started (PID: $SERVER_PID)"
echo ""

# Wait for control plane to be ready
echo "Step 2: Waiting for control plane to be ready..."
timeout 10 bash -c 'until nc -z 127.0.0.1 8080; do sleep 0.5; done' || {
    echo "✗ Control plane failed to start"
    kill $SERVER_PID 2>/dev/null || true
    exit 1
}
echo "✓ Control plane is ready"
echo ""

# List initial state (should be empty)
echo "Step 3: Checking initial state..."
echo ""
echo "$ flowctl pipelines list"
./bin/flowctl pipelines list || echo "(No pipelines yet - this is expected)"
echo ""

echo "$ flowctl pipelines active"
./bin/flowctl pipelines active
echo ""

# Note about running a pipeline
echo "Step 4: To test with a real pipeline..."
echo ""
echo "You can now run:"
echo "  ./bin/flowctl run --use-external-control-plane /path/to/pipeline.yaml"
echo ""
echo "This will:"
echo "  1. Create a pipeline run record in the control plane"
echo "  2. Track the run status (STARTING → RUNNING → COMPLETED)"
echo "  3. Store metrics (events processed, throughput)"
echo ""

# Show CLI commands
echo "Step 5: Available CLI commands..."
echo ""
echo "View all pipelines:"
echo "  $ flowctl pipelines list"
echo ""
echo "View run history for a pipeline:"
echo "  $ flowctl pipelines runs my-pipeline"
echo ""
echo "Get detailed run information:"
echo "  $ flowctl pipelines run-info <run-id>"
echo ""
echo "Stop a running pipeline:"
echo "  $ flowctl pipelines stop <run-id>"
echo ""
echo "List currently active runs:"
echo "  $ flowctl pipelines active"
echo ""

# Show TUI command
echo "Step 6: Interactive Dashboard..."
echo ""
echo "Launch the live TUI dashboard:"
echo "  $ flowctl dashboard"
echo ""
echo "This shows:"
echo "  - Active pipeline runs (with live metrics)"
echo "  - Component registry (with health status)"
echo "  - Auto-refreshes every 1 second"
echo "  - Press 'q' to quit, 'r' to refresh"
echo ""

# Cleanup prompt
echo "=== Demo Complete ==="
echo ""
echo "Control plane is still running (PID: $SERVER_PID)"
echo ""
read -p "Press Enter to stop the control plane and cleanup..."

kill $SERVER_PID 2>/dev/null || true
rm -f ./flowctl-demo.db
echo "✓ Cleanup complete"

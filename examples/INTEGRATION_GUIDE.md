# Integrating with flowctl Control Plane

This guide explains how to integrate stellar-live-source-datalake, ttp-processor, and consumer_app with the flowctl control plane.

## Prerequisites

- Make sure flowctl is built and available in the `bin` directory
- All components (stellar-live-source-datalake, ttp-processor, consumer_app) must be compiled and ready to run
- Kafka or Redpanda running (for message bus between components)

## Step 1: Start the Control Plane

Start the flowctl control plane server:

```bash
cd flowctl
./bin/flowctl server --port 8080
```

The server will listen on port 8080 for registration requests from services.

## Step 2: Register Schemas

Register the required schemas with the schema registry:

```bash
cd flowctl/examples
chmod +x register-schemas.sh
./register-schemas.sh
```

This will register:
- `RawLedgerChunk` schema from stellar-live-source-datalake
- `TokenTransferEvent` schema from ttp-processor

## Step 3: Update Component Code

Each component (source, processor, sink) needs to register with the control plane on startup and send periodic heartbeats.

Implement the registration logic as shown in the example files:
- `source-registration.go` for stellar-live-source-datalake
- `processor-registration.go` for ttp-processor
- `consumer-registration.go` for consumer_app

The registration process involves:
1. Connecting to the control plane server
2. Sending service information (input/output event types, health endpoint, etc.)
3. Starting a heartbeat loop to send periodic health updates

## Step 4: Deploy the Pipeline

Apply the pipeline configuration:

```bash
cd flowctl
./bin/flowctl apply -f examples/stellar-pipeline.yaml
```

This will:
1. Validate the pipeline configuration
2. Assign a unique pipeline ID
3. Deploy the components as defined in the YAML file

## Step 5: Monitor the Pipeline

Monitor the running pipeline:

```bash
# List registered services
./bin/flowctl list

# View logs for a specific component
./bin/flowctl logs ttp-processor --follow

# Check pipeline status
./bin/flowctl get pipelines
```

## Step 6: Handling Errors with DLQ

If messages fail processing, they will be sent to a Dead Letter Queue (DLQ). You can:

```bash
# List failed messages
./bin/flowctl dlq ls --pipeline stellar-token-pipeline

# Replay specific messages
./bin/flowctl dlq replay --id <message-id>
```

## Step 7: Visualize the Pipeline

Generate a visual representation of the pipeline:

```bash
./bin/flowctl graph -f examples/stellar-pipeline.yaml --format svg
```

## Developer Tips

1. **Local Development**: Use `flowctl dev up` to spin up a local development environment with mock source and sink.

2. **Schema Changes**: When modifying a schema, register the updated version:
   ```bash
   ./bin/flowctl schemas push path/to/proto --package package_name
   ```

3. **Component Updates**: When updating a component, update the image reference in the pipeline YAML and re-apply.

4. **Health Checks**: Ensure each component exposes a health endpoint compatible with Prometheus metrics format.

5. **Pipeline Replay**: To replay data for a specific ledger range:
   ```bash
   ./bin/flowctl replay --pipeline stellar-token-pipeline --from-ledger 123456 --to-ledger 123500
   ```
# Clean Implementation Prompt: flowctl run (No Proxies)

## Mission

Implement `flowctl run` as a **pure orchestrator** that starts processes or containers and lets components communicate directly. No proxy layer, no architectural debt - just clean orchestration.

## Current Situation

- **Components already work independently** (Go binaries, npm start)
- **Components have flowctl SDK integration** (can register with control plane)
- **Components communicate via gRPC** (direct communication works)
- **Current flowctl run only does mocks** (needs real orchestration)

## Your Task

Transform `flowctl run` into a process/container orchestrator that:

1. **Starts real processes/containers** instead of mocks
2. **Lets components communicate directly** via gRPC
3. **Handles component lifecycle** (start, monitor, stop)
4. **Integrates with flowctl SDK** (components register with control plane)
5. **Supports both process and container deployment**

## Target Architecture

```
flowctl run ttp-pipeline.yaml --orchestrator process
    ↓
[flowctl orchestrates]
├── Process: ./stellar-live-source-datalake/bin/stellar-live-source-datalake
├── Process: ./ttp-processor/bin/ttp-processor
├── Process: npm start -- 1 0 (in consumer_app/node/)
└── Components communicate directly via gRPC
```

**OR**

```
flowctl run ttp-pipeline.yaml --orchestrator container
    ↓
[flowctl orchestrates]
├── Container: withobsrvr/stellar-live-source-datalake:latest
├── Container: withobsrvr/ttp-processor:latest
├── Container: withobsrvr/consumer-app:latest
└── Components communicate directly via gRPC
```

## Key Implementation Requirements

### 1. Process Orchestrator
Create `internal/orchestrator/process_orchestrator.go`:
- Start Go binaries with `exec.Command()`
- Start npm processes with `exec.Command("npm", "start", "--", "1", "0")`
- Set up environment variables for flowctl SDK integration
- Monitor process health via HTTP endpoints
- Handle graceful shutdown with SIGTERM

### 2. Container Orchestrator
Create `internal/orchestrator/container_orchestrator.go`:
- Start Docker containers with `docker run`
- Set up bridge networking for inter-container communication
- Handle port mappings and environment variables
- Monitor container health via HTTP endpoints
- Clean up containers on shutdown

### 3. Pipeline Runner
Create `internal/runner/pipeline_runner.go`:
- Load pipeline configuration from YAML
- Start components in dependency order (sources → processors → sinks)
- Monitor component health
- Handle shutdown signals gracefully

### 4. Updated Run Command
Update `cmd/run.go`:
- Remove DAG pipeline complexity
- Add orchestrator type flag (`--orchestrator process|container`)
- Use simple pipeline runner instead of complex DAG

## Component Communication

### Direct gRPC Communication
Components should connect to each other directly:

```yaml
# stellar-live-source-datalake exposes gRPC on port 50051
env:
  GRPC_PORT: "50051"

# ttp-processor connects directly to stellar-live-source-datalake
env:
  LIVE_SOURCE_ENDPOINT: "localhost:50051"  # Direct connection
  GRPC_PORT: "50052"

# consumer-app connects directly to ttp-processor
env:
  TTP_SERVICE_ADDRESS: "localhost:50052"   # Direct connection
```

### flowctl SDK Integration
All components get these environment variables:
```bash
ENABLE_FLOWCTL=true
FLOWCTL_ENDPOINT=http://localhost:8080
FLOWCTL_HEARTBEAT_INTERVAL=10s
COMPONENT_ID=<component-id>
```

## Component Startup Commands

### stellar-live-source-datalake (Go)
```bash
# Process mode
./stellar-live-source-datalake/bin/stellar-live-source-datalake

# Container mode
docker run -d --name stellar-live-source-datalake \
  -p 50051:50051 -p 8088:8088 \
  -e STORAGE_TYPE=S3 \
  -e BUCKET_NAME=stellar-ledger-data \
  -e GRPC_PORT=50051 \
  -e HEALTH_PORT=8088 \
  withobsrvr/stellar-live-source-datalake:latest
```

### ttp-processor (Go)
```bash
# Process mode
./ttp-processor/bin/ttp-processor

# Container mode
docker run -d --name ttp-processor \
  -p 50052:50052 -p 8089:8089 \
  -e LIVE_SOURCE_ENDPOINT=localhost:50051 \
  -e GRPC_PORT=50052 \
  -e HEALTH_PORT=8089 \
  withobsrvr/ttp-processor:latest
```

### consumer-app (Node.js)
```bash
# Process mode
cd consumer_app/node
npm start -- 1 0

# Container mode
docker run -d --name consumer-app \
  -p 3000:3000 \
  -e TTP_SERVICE_ADDRESS=localhost:50052 \
  -e PORT=3000 \
  withobsrvr/consumer-app:latest
```

## Health Checks

All components expose HTTP health endpoints:
- `stellar-live-source-datalake`: `http://localhost:8088/health`
- `ttp-processor`: `http://localhost:8089/health`
- `consumer-app`: `http://localhost:3000/health`

Your orchestrator should:
1. Wait for each component's health check to pass
2. Only start dependent components after dependencies are healthy
3. Monitor health periodically

## Error Handling

### Startup Failures
- If a component fails to start, stop all other components
- Provide clear error messages with component context
- Clean up any partially started processes/containers

### Runtime Failures
- Monitor component health periodically
- Log when components become unhealthy
- Optionally restart failed components (stretch goal)

### Shutdown
- Send SIGTERM to processes for graceful shutdown
- Wait up to 10 seconds, then force kill
- Remove Docker containers completely
- Close log files

## File Structure

### New Files to Create
```
internal/orchestrator/
├── process_orchestrator.go     # Process management
├── container_orchestrator.go   # Docker container management
└── orchestrator.go            # Common interfaces

internal/runner/
└── pipeline_runner.go         # Simple pipeline orchestration

examples/
└── ttp-pipeline.yaml         # Clean pipeline config
```

### Files to Modify
```
cmd/run.go                     # Simplified run command
internal/model/               # May need pipeline model updates
```

### Files to Remove/Ignore
```
internal/core/pipeline_dag.go  # Complex DAG not needed
internal/core/pipeline.go      # Simple pipeline not needed
internal/testfixtures/        # No more mocks
```

## Testing Approach

### Phase 1: Process Orchestration
```bash
# Build components
cd stellar-live-source-datalake && make build
cd ttp-processor && make build
cd consumer_app/node && npm install

# Test orchestration
./bin/flowctl run examples/ttp-pipeline.yaml --orchestrator process
```

### Phase 2: Container Orchestration
```bash
# Build images
docker build -t withobsrvr/stellar-live-source-datalake:latest stellar-live-source-datalake/
docker build -t withobsrvr/ttp-processor:latest ttp-processor/
docker build -t withobsrvr/consumer-app:latest consumer_app/

# Test orchestration
./bin/flowctl run examples/ttp-pipeline.yaml --orchestrator container
```

### Phase 3: Integration Testing
```bash
# Test health endpoints
curl http://localhost:8088/health
curl http://localhost:8089/health
curl http://localhost:3000/health

# Test shutdown
Ctrl+C

# Verify cleanup
ps aux | grep stellar  # Should be empty
docker ps               # Should be empty
```

## Success Criteria

1. **Process Orchestration**: Can start Go binaries and npm processes
2. **Container Orchestration**: Can start Docker containers with networking
3. **Health Monitoring**: Waits for health checks before proceeding
4. **Direct Communication**: Components communicate via gRPC without flowctl involvement
5. **SDK Integration**: Components register with flowctl control plane
6. **Graceful Shutdown**: All processes/containers stop cleanly
7. **Error Handling**: Clear error messages and proper cleanup
8. **Logging**: Component logs captured to separate files

## Key Benefits

### Clean Architecture
- flowctl is pure orchestrator
- No proxy layer complexity
- Components unchanged from standalone operation

### Flexible Deployment
- Same pipeline config works for processes and containers
- Easy to switch between development and production modes

### Performance
- Direct gRPC communication
- No proxy overhead
- Native component performance

### Debugging
- Standard process/container debugging
- Direct log files
- Standard gRPC tools work

## Common Pitfalls to Avoid

1. **Don't route data through flowctl** - Components should communicate directly
2. **Don't create proxy objects** - Use orchestration, not proxies
3. **Handle startup order** - Start dependencies before dependents
4. **Clean up on failure** - No orphaned processes/containers
5. **Wait for health checks** - Don't assume components are ready immediately

## Expected User Experience

```bash
# Start pipeline
./bin/flowctl run examples/ttp-pipeline.yaml

# Output:
# Starting pipeline: ttp-processing-pipeline
# Starting component: stellar-live-source-datalake
# Component stellar-live-source-datalake health check passed
# Starting component: ttp-processor
# Component ttp-processor health check passed
# Starting component: consumer-app
# Component consumer-app health check passed
# Pipeline started successfully
# ^C
# Received shutdown signal
# Stopping component: consumer-app
# Stopping component: ttp-processor
# Stopping component: stellar-live-source-datalake
# Pipeline stopped successfully
```

This approach creates a clean, maintainable foundation that matches how real microservices work in production.
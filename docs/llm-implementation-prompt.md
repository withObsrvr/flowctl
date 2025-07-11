# LLM Implementation Prompt: flowctl run

## Task Overview

You are tasked with implementing a production-ready `flowctl run` command that orchestrates real Docker containers for data pipeline execution. This transforms the current mock-based system into a real container orchestration platform.

## Current State

The existing `flowctl run` command only runs mock components in memory. Your goal is to make it orchestrate real Docker containers running actual microservices from the ttp-processor-demo repository.

## Your Mission

Transform `flowctl run examples/ttp-pipeline.yaml` from running fake components to orchestrating these real containers:
- **stellar-live-source-datalake**: Streams Stellar blockchain data from data lake
- **ttp-processor**: Processes raw ledger data and extracts token transfer events  
- **consumer-app**: Consumes and processes the events

## Implementation Requirements

### Core Functionality
1. **Container Orchestration**: Start Docker containers with proper networking
2. **Health Checking**: Wait for components to be ready before proceeding
3. **gRPC Communication**: Enable real gRPC communication between components
4. **Lifecycle Management**: Clean up containers on shutdown
5. **Error Handling**: Proper error messages and cleanup on failure

### Success Criteria
- `flowctl run examples/ttp-pipeline.yaml` starts all three containers
- Components pass health checks and register with control plane
- Data flows from stellar-live-source-datalake → ttp-processor → consumer-app
- Pipeline stops and cleans up properly on Ctrl+C
- All containers are removed after pipeline stops

## Key Architecture Changes

### Before (Current)
```
flowctl run pipeline.yaml
├── Mock Source (goroutine)
├── Mock Processor (goroutine)
└── Mock Sink (goroutine)
```

### After (Your Implementation)
```
flowctl run pipeline.yaml
├── Docker: stellar-live-source-datalake
├── Docker: ttp-processor
├── Docker: consumer-app
└── flowctl orchestrates containers
```

## Implementation Strategy

Follow this phase-by-phase approach:

### Phase 1: Container Executor
Create `internal/executor/production_docker.go` with:
- Docker container management
- Bridge network creation
- Health check implementation
- gRPC readiness verification

### Phase 2: Real Component Proxies
Create gRPC-based component implementations:
- `internal/core/stellar_live_source_datalake.go`
- `internal/core/ttp_processor.go`
- `internal/core/consumer_app_sink.go`

### Phase 3: Factory Function Updates
Replace mock factory functions in `internal/core/pipeline_dag.go` with real container-starting implementations.

### Phase 4: Pipeline Configuration
Create `examples/ttp-pipeline.yaml` with proper component definitions, ports, and environment variables.

### Phase 5: Command Enhancement
Update `cmd/run.go` to use the new DAG pipeline with container orchestration.

## Important Technical Details

### Docker Networking
- Use bridge network named "flowctl-bridge"
- Map container ports to host ports
- Enable DNS resolution between containers

### Health Checks
- HTTP health endpoints (e.g., `/health` on port 8088)
- gRPC connection verification
- 30-second timeout with 1-second intervals

### gRPC Communication
- Generate Go clients from proto files
- Use `google.golang.org/grpc` for connections
- Handle streaming RPCs for data flow

### Environment Variables
Each component needs flowctl integration:
```bash
ENABLE_FLOWCTL=true
FLOWCTL_ENDPOINT=localhost:8080
FLOWCTL_HEARTBEAT_INTERVAL=10s
```

### Error Handling
- Clean up containers on startup failure
- Provide clear error messages with component context
- Ensure no orphaned containers remain

## File Structure

You'll be working with these key files:
- `cmd/run.go` - Command implementation
- `internal/core/pipeline_dag.go` - Factory functions
- `internal/executor/production_docker.go` - Container orchestration
- `internal/core/*_source_datalake.go` - Component implementations
- `examples/ttp-pipeline.yaml` - Pipeline configuration

## Proto Files

Copy these proto files from ttp-processor-demo:
- `raw_ledger_service.proto` - For stellar-live-source-datalake
- `event_service.proto` - For ttp-processor
- `token_transfer_event.proto` - For event definitions

## Testing Approach

### Component Testing
1. Verify Docker images can be built
2. Test each component starts individually
3. Verify health endpoints respond

### Integration Testing
1. Test `flowctl run` starts all containers
2. Verify network connectivity between components
3. Test gRPC communication works

### End-to-End Testing
1. Run full pipeline with real data
2. Verify data flows through all components
3. Test cleanup on shutdown

## Dependencies

Ensure these are available:
- Docker daemon running
- Docker images built for all components
- Proto generation tools installed
- Go modules updated

## Common Pitfalls to Avoid

1. **Port Conflicts**: Ensure ports aren't already in use
2. **Network Isolation**: Don't use host networking, use bridge
3. **Race Conditions**: Wait for health checks before proceeding
4. **Resource Leaks**: Always clean up containers on failure
5. **Error Messages**: Provide component context in errors

## Expected Outcome

After implementation, developers should be able to:
```bash
# Start real pipeline
./bin/flowctl run examples/ttp-pipeline.yaml

# See containers running
docker ps

# View real data flow
docker logs stellar-live-source-datalake
docker logs ttp-processor
docker logs consumer-app

# Stop pipeline
Ctrl+C

# Verify cleanup
docker ps  # No flowctl containers remain
```

## Code Quality Standards

- Follow existing Go patterns in the codebase
- Add proper error handling with context
- Include logging for debugging
- Use typed gRPC clients, not raw connections
- Implement proper resource cleanup

## Final Notes

This implementation bridges the gap between the current mock system and production-ready container orchestration. Focus on making it robust, well-tested, and production-ready rather than quick-and-dirty.

The detailed implementation guide contains all the code snippets and technical details you need. Your job is to integrate these pieces into a cohesive, working system.

**Time Estimate**: 2-3 days for thorough implementation and testing.
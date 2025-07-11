# LLM Implementation Prompt: Embedded Control Plane for flowctl

## Task Overview

You are tasked with implementing an embedded control plane architecture for the flowctl CLI tool. This will integrate the `flowctl server` functionality directly into `flowctl run`, creating a seamless developer experience where the control plane starts automatically with pipeline execution.

## Current State Analysis

### Existing Architecture
- **flowctl server**: Standalone gRPC server providing service registry, health monitoring, and metrics collection
- **flowctl run**: Currently runs mock components, doesn't orchestrate real processes
- **Control Plane API**: Well-defined gRPC API in `internal/api/control_plane.go`
- **Component Integration**: Components have flowctl SDK integration (ENABLE_FLOWCTL=true)

### Key Files to Understand
1. `cmd/server/server.go` - Current server command implementation
2. `internal/api/control_plane.go` - Control plane gRPC service
3. `cmd/run.go` - Current run command (uses mocks)
4. `internal/core/pipeline.go` - Pipeline execution logic
5. `proto/control_plane.proto` - gRPC API definitions

## Implementation Requirements

### Primary Goal
Transform `flowctl run` from a mock runner into a full orchestrator that:
1. Starts an embedded control plane gRPC server
2. Orchestrates real component processes/containers
3. Provides integrated monitoring and logging
4. Handles graceful shutdown of all components

### Architecture Changes Required

#### 1. Embedded Control Plane Manager
Create `internal/controlplane/embedded_server.go`:
- Wraps the existing `api.ControlPlaneServer`
- Manages gRPC server lifecycle
- Provides component waiting functionality
- Handles graceful shutdown

#### 2. Pipeline Runner Enhancement
Update `internal/runner/pipeline_runner.go`:
- Integrates embedded control plane
- Orchestrates component startup in dependency order
- Waits for component registration
- Provides unified monitoring

#### 3. Orchestrator Interface
Create `internal/orchestrator/` package:
- `Orchestrator` interface for different execution environments
- `ProcessOrchestrator` for local process execution
- `ContainerOrchestrator` for Docker execution (future)

#### 4. Command Updates
- Update `cmd/run.go` to use new pipeline runner
- Hide `cmd/server/server.go` (backward compatibility)
- Update `cmd/list/list.go` to detect embedded control plane

## Technical Specifications

### Component Communication Flow
```
flowctl run → EmbeddedControlPlane → gRPC Server (port 8080)
    ↓
ProcessOrchestrator → Component Processes
    ↓
Component Processes → Register() → Control Plane
Component Processes → Heartbeat() → Control Plane (every 10s)
```

### Environment Variables for Components
```bash
ENABLE_FLOWCTL=true
FLOWCTL_ENDPOINT=http://localhost:8080
FLOWCTL_HEARTBEAT_INTERVAL=10s
FLOWCTL_SERVICE_ID=<component-id>
```

### Component Types and Commands
- **stellar-live-source-datalake**: `./bin/stellar-live-source-datalake`
- **ttp-processor**: `npm start -- 1234 2345`
- **consumer-app**: `./bin/consumer-app`

## Implementation Steps

1. **Phase 1: Core Infrastructure**
   - Create embedded control plane manager
   - Create orchestrator interface and process implementation
   - Update pipeline runner to integrate both

2. **Phase 2: Command Integration**
   - Update run command to use new pipeline runner
   - Hide server command while maintaining backward compatibility
   - Update list command to work with embedded control plane

3. **Phase 3: Enhanced Features**
   - Add real-time status display
   - Implement integrated log aggregation
   - Add component health monitoring

## Code Quality Requirements

### Error Handling
- Graceful degradation when components fail to start
- Proper cleanup on shutdown
- Clear error messages for users

### Logging
- Structured logging with zap
- Component-specific log prefixes
- Integrated status updates

### Testing
- Unit tests for embedded control plane lifecycle
- Integration tests for full pipeline execution
- Backward compatibility tests

## Success Criteria

1. **Single Command Experience**
   - `flowctl run pipeline.yaml` starts everything
   - No manual server management required
   - Integrated monitoring in same terminal

2. **Component Integration**
   - All components register automatically
   - Heartbeat flow works properly
   - Health monitoring functions correctly

3. **Backward Compatibility**
   - Hidden server command still works
   - List command works with embedded control plane
   - Existing component integration preserved

4. **Clean Lifecycle**
   - Graceful shutdown on Ctrl+C
   - All processes stopped properly
   - No orphaned processes

## Important Implementation Notes

### Existing Code Preservation
- **DO NOT** modify the existing control plane gRPC API
- **DO NOT** change the component SDK integration
- **DO NOT** break existing storage interface

### Dependencies to Leverage
- Reuse `internal/api/control_plane.go` as-is
- Leverage existing storage interfaces
- Use existing logging infrastructure

### Error Scenarios to Handle
- Port conflicts (control plane port in use)
- Component startup failures
- Component registration timeouts
- Graceful shutdown interruptions

## Testing Strategy

### Unit Tests
```bash
# Test embedded control plane
go test -v ./internal/controlplane -run TestEmbeddedControlPlane

# Test pipeline runner
go test -v ./internal/runner -run TestPipelineRunner

# Test orchestrator
go test -v ./internal/orchestrator -run TestProcessOrchestrator
```

### Integration Tests
```bash
# Test full pipeline
go test -v ./test -run TestEmbeddedPipeline

# Test backward compatibility  
go test -v ./test -run TestBackwardCompatibility
```

### Manual Testing
```bash
# Test single command startup
./bin/flowctl run examples/ttp-pipeline.yaml

# Test status monitoring
./bin/flowctl list

# Test graceful shutdown
# Start pipeline, then Ctrl+C
```

## Deliverables

1. **Core Implementation**
   - `internal/controlplane/embedded_server.go`
   - `internal/runner/pipeline_runner.go`
   - `internal/orchestrator/interface.go`
   - `internal/orchestrator/process.go`

2. **Command Updates**
   - Updated `cmd/run.go`
   - Updated `cmd/root.go` (hidden server)
   - Updated `cmd/list/list.go`

3. **Tests**
   - Unit tests for all new components
   - Integration tests for full pipeline
   - Backward compatibility tests

4. **Documentation**
   - Code comments explaining architecture
   - Error handling documentation
   - Testing documentation

## Implementation Priority

1. **High Priority**: Embedded control plane and basic orchestration
2. **Medium Priority**: Enhanced monitoring and logging
3. **Low Priority**: Container orchestrator and advanced features

## Context Files

You have access to the complete flowctl codebase. Key files to examine:
- `internal/api/control_plane.go` - Existing control plane implementation
- `cmd/server/server.go` - Current server command
- `internal/storage/interface.go` - Storage interface
- `proto/control_plane.proto` - gRPC API definitions

Analyze these files to understand the current architecture before implementing the embedded control plane.

## Final Notes

This implementation should create a seamless developer experience while preserving all existing functionality. The embedded control plane should be invisible to users - they should just see `flowctl run` start their pipeline and provide monitoring, without needing to manage a separate server process.

Focus on code quality, error handling, and maintaining backward compatibility throughout the implementation.
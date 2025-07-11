# Embedded Control Plane Architecture: Integrating flowctl server into flowctl run

## Executive Summary

This document outlines the architectural changes needed to embed the `flowctl server` functionality directly into `flowctl run`, creating a seamless developer experience where the control plane starts automatically with pipeline execution. This eliminates the need for users to manually manage server lifecycle while preserving all existing functionality.

## Current Architecture Analysis

### What `flowctl server` Currently Does

The `flowctl server` command runs a **control plane gRPC server** that provides:

#### 1. Service Registry & Discovery
- **Component Registration**: Components register themselves on startup via `Register(ServiceInfo)` RPC
- **Service Metadata**: Stores service type, event types, health endpoints, and custom metadata
- **Unique Service IDs**: Generates unique IDs for each component instance

#### 2. Health Monitoring & Lifecycle Management
- **Heartbeat System**: Components send periodic heartbeats via `Heartbeat(ServiceHeartbeat)` RPC
- **Health Tracking**: Monitors component health based on heartbeat freshness
- **Janitor Process**: Background process checks for stale services every 10 seconds
- **TTL Management**: Marks services unhealthy after 30 seconds without heartbeat

#### 3. Metrics Collection & Observability
- **Metrics Aggregation**: Collects custom metrics from component heartbeats
- **Service Status API**: Provides `GetServiceStatus()` and `ListServices()` for monitoring
- **Historical Data**: Tracks last seen times and health state changes

#### 4. Persistent Storage
- **BoltDB Backend**: Optional persistent storage for service registry
- **In-Memory Mode**: Non-persistent mode for development
- **Transaction Support**: ACID transactions for registry operations

#### 5. Security & Networking
- **TLS Support**: Configurable TLS and mutual TLS for secure communication
- **gRPC Server**: Production-ready gRPC server with configurable options
- **Port Configuration**: Configurable listen address and port (default: 0.0.0.0:8080)

### Current Component Integration

The ttp-processor-demo components already have **flowctl SDK integration**:

```bash
# Components automatically integrate when these env vars are set:
ENABLE_FLOWCTL=true
FLOWCTL_ENDPOINT=http://localhost:8080
FLOWCTL_HEARTBEAT_INTERVAL=10s
```

When enabled, components:
1. **Auto-register** on startup with service type and capabilities
2. **Send heartbeats** every 10 seconds with metrics
3. **Report health status** via HTTP endpoints
4. **Handle graceful shutdown** by deregistering

### Current User Experience Problems

#### Manual Server Management
```bash
# Current workflow requires 2 terminals:

# Terminal 1: Start control plane
./bin/flowctl server

# Terminal 2: Run pipeline
./bin/flowctl run ttp-pipeline.yaml
```

#### Lifecycle Complexity
- Users must remember to start server first
- Server continues running after pipeline stops
- No automatic cleanup or lifecycle coordination
- Manual port management and conflict resolution

#### Discovery & Debugging Issues
- Components may fail to start if server isn't running
- No integrated logging between server and pipeline
- Separate monitoring required (`flowctl list` in another terminal)

## Proposed Architecture: Embedded Control Plane

### Design Principles

1. **Transparent Operation**: Control plane starts/stops automatically with pipeline
2. **Zero Configuration**: No manual server management required
3. **Preserved Functionality**: All existing control plane features maintained
4. **Integrated Monitoring**: Pipeline and control plane logs unified
5. **Clean Lifecycle**: Server lifecycle tied to pipeline lifecycle

### New Architecture Overview

```
flowctl run ttp-pipeline.yaml
    ↓
[flowctl process]
├── Embedded Control Plane (gRPC server on :8080)
│   ├── Service Registry
│   ├── Health Monitoring  
│   ├── Metrics Collection
│   └── Storage Backend
│
├── Process/Container Orchestrator
│   ├── stellar-live-source-datalake
│   ├── ttp-processor
│   └── consumer-app
│
└── Unified Monitoring & Logging
    ├── Component Logs
    ├── Control Plane Events
    └── Health Status
```

### Component Communication Flow

```
Component Startup:
stellar-live-source-datalake → Register() → Embedded Control Plane
                            ← RegistrationAck ←

Ongoing Operations:
stellar-live-source-datalake → Heartbeat() → Embedded Control Plane (every 10s)
ttp-processor               → Heartbeat() → Embedded Control Plane (every 10s)  
consumer-app                → Heartbeat() → Embedded Control Plane (every 10s)

Direct Data Flow:
stellar-live-source-datalake ─(gRPC)→ ttp-processor ─(gRPC)→ consumer-app

Monitoring:
flowctl run → GetServiceStatus() → Embedded Control Plane
```

## Implementation Plan

### Phase 1: Control Plane Embedding

#### 1.1 Create Embedded Control Plane Manager

**File:** `internal/controlplane/embedded_server.go`

```go
package controlplane

import (
    "context"
    "fmt"
    "net"
    "sync"
    "time"
    
    "google.golang.org/grpc"
    "github.com/withobsrvr/flowctl/internal/api"
    "github.com/withobsrvr/flowctl/internal/storage"
    "github.com/withobsrvr/flowctl/internal/utils/logger"
    "go.uber.org/zap"
)

type EmbeddedControlPlane struct {
    server          *grpc.Server
    controlPlane    *api.ControlPlaneServer
    listener        net.Listener
    address         string
    port            int
    started         bool
    stopped         bool
    mu              sync.RWMutex
}

type Config struct {
    Address         string
    Port            int
    HeartbeatTTL    time.Duration
    JanitorInterval time.Duration
    Storage         storage.ServiceStorage
    TLSConfig       *TLSConfig
}

func NewEmbeddedControlPlane(config Config) *EmbeddedControlPlane {
    return &EmbeddedControlPlane{
        address: config.Address,
        port:    config.Port,
    }
}

func (e *EmbeddedControlPlane) Start(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    if e.started {
        return fmt.Errorf("control plane already started")
    }
    
    // Create listener
    address := fmt.Sprintf("%s:%d", e.address, e.port)
    listener, err := net.Listen("tcp", address)
    if err != nil {
        return fmt.Errorf("failed to create listener: %w", err)
    }
    e.listener = listener
    
    // Create gRPC server
    e.server = grpc.NewServer()
    
    // Create control plane
    e.controlPlane = api.NewControlPlaneServer(nil) // In-memory for now
    if err := e.controlPlane.Start(); err != nil {
        return fmt.Errorf("failed to start control plane: %w", err)
    }
    
    // Register service
    pb.RegisterControlPlaneServer(e.server, e.controlPlane)
    
    // Start server in background
    go func() {
        logger.Info("Embedded control plane listening", zap.String("address", address))
        if err := e.server.Serve(listener); err != nil {
            if !e.stopped {
                logger.Error("Control plane server error", zap.Error(err))
            }
        }
    }()
    
    e.started = true
    logger.Info("Embedded control plane started", zap.String("address", address))
    
    return nil
}

func (e *EmbeddedControlPlane) Stop() error {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    if !e.started || e.stopped {
        return nil
    }
    
    logger.Info("Stopping embedded control plane")
    
    e.stopped = true
    
    // Graceful shutdown
    if e.server != nil {
        e.server.GracefulStop()
    }
    
    // Close control plane
    if e.controlPlane != nil {
        if err := e.controlPlane.Close(); err != nil {
            logger.Error("Error closing control plane", zap.Error(err))
        }
    }
    
    logger.Info("Embedded control plane stopped")
    return nil
}

func (e *EmbeddedControlPlane) GetEndpoint() string {
    return fmt.Sprintf("http://%s:%d", e.address, e.port)
}

func (e *EmbeddedControlPlane) GetServiceList() ([]*pb.ServiceStatus, error) {
    if !e.started || e.stopped {
        return nil, fmt.Errorf("control plane not running")
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    resp, err := e.controlPlane.ListServices(ctx, &emptypb.Empty{})
    if err != nil {
        return nil, err
    }
    
    return resp.Services, nil
}

func (e *EmbeddedControlPlane) WaitForComponent(componentID string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    
    for time.Now().Before(deadline) {
        services, err := e.GetServiceList()
        if err != nil {
            time.Sleep(1 * time.Second)
            continue
        }
        
        for _, service := range services {
            if service.ServiceId == componentID && service.IsHealthy {
                return nil
            }
        }
        
        time.Sleep(1 * time.Second)
    }
    
    return fmt.Errorf("timeout waiting for component %s to register", componentID)
}
```

#### 1.2 Integrate into Pipeline Runner

**File:** `internal/runner/pipeline_runner.go` (updated)

```go
package runner

import (
    "context"
    "fmt"
    "time"
    
    "github.com/withobsrvr/flowctl/internal/model"
    "github.com/withobsrvr/flowctl/internal/orchestrator"
    "github.com/withobsrvr/flowctl/internal/controlplane"
    "github.com/withobsrvr/flowctl/internal/utils/logger"
    "go.uber.org/zap"
)

type PipelineRunner struct {
    pipeline      *model.Pipeline
    orchestrator  Orchestrator
    controlPlane  *controlplane.EmbeddedControlPlane
    ctx           context.Context
    cancel        context.CancelFunc
}

func NewPipelineRunner(pipeline *model.Pipeline, orchestratorType string) (*PipelineRunner, error) {
    // Create embedded control plane
    controlPlaneConfig := controlplane.Config{
        Address:         "127.0.0.1",
        Port:            8080,
        HeartbeatTTL:    30 * time.Second,
        JanitorInterval: 10 * time.Second,
        Storage:         nil, // In-memory for now
    }
    
    embeddedCP := controlplane.NewEmbeddedControlPlane(controlPlaneConfig)
    
    // Create orchestrator with control plane endpoint
    var orch Orchestrator
    controlPlaneAddr := embeddedCP.GetEndpoint()
    
    switch orchestratorType {
    case "process":
        orch = orchestrator.NewProcessOrchestrator(controlPlaneAddr)
    case "container", "docker":
        orch = orchestrator.NewContainerOrchestrator(controlPlaneAddr)
    default:
        return nil, fmt.Errorf("unknown orchestrator type: %s", orchestratorType)
    }
    
    ctx, cancel := context.WithCancel(context.Background())
    
    return &PipelineRunner{
        pipeline:     pipeline,
        orchestrator: orch,
        controlPlane: embeddedCP,
        ctx:          ctx,
        cancel:       cancel,
    }, nil
}

func (r *PipelineRunner) Run() error {
    logger.Info("Starting pipeline with embedded control plane", 
        zap.String("pipeline", r.pipeline.Metadata.Name))
    
    // Start embedded control plane first
    if err := r.controlPlane.Start(r.ctx); err != nil {
        return fmt.Errorf("failed to start control plane: %w", err)
    }
    
    // Wait a moment for control plane to be ready
    time.Sleep(1 * time.Second)
    
    // Start components in dependency order
    if err := r.startComponents(); err != nil {
        r.controlPlane.Stop()
        return err
    }
    
    // Wait for all components to register
    if err := r.waitForComponentRegistration(); err != nil {
        r.controlPlane.Stop()
        return err
    }
    
    // Start monitoring
    go r.monitorPipeline()
    
    logger.Info("Pipeline started successfully with control plane integration")
    
    // Set up signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    // Wait for shutdown
    select {
    case <-sigChan:
        logger.Info("Received shutdown signal")
    case <-r.ctx.Done():
        logger.Info("Context cancelled")
    }
    
    return r.shutdown()
}

func (r *PipelineRunner) waitForComponentRegistration() error {
    logger.Info("Waiting for components to register with control plane")
    
    // Wait for each component to register
    allComponents := append(r.pipeline.Spec.Sources, r.pipeline.Spec.Processors...)
    allComponents = append(allComponents, r.pipeline.Spec.Sinks...)
    
    for _, component := range allComponents {
        logger.Info("Waiting for component registration", zap.String("component", component.ID))
        if err := r.controlPlane.WaitForComponent(component.ID, 30*time.Second); err != nil {
            return fmt.Errorf("component %s failed to register: %w", component.ID, err)
        }
        logger.Info("Component registered successfully", zap.String("component", component.ID))
    }
    
    return nil
}

func (r *PipelineRunner) monitorPipeline() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-r.ctx.Done():
            return
        case <-ticker.C:
            r.logPipelineStatus()
        }
    }
}

func (r *PipelineRunner) logPipelineStatus() {
    services, err := r.controlPlane.GetServiceList()
    if err != nil {
        logger.Error("Failed to get service list", zap.Error(err))
        return
    }
    
    healthyCount := 0
    for _, service := range services {
        if service.IsHealthy {
            healthyCount++
        }
    }
    
    logger.Info("Pipeline status", 
        zap.Int("total_components", len(services)),
        zap.Int("healthy_components", healthyCount))
}

func (r *PipelineRunner) shutdown() error {
    logger.Info("Shutting down pipeline and control plane")
    
    // Cancel context
    r.cancel()
    
    // Stop orchestrator
    if err := r.orchestrator.StopAll(); err != nil {
        logger.Error("Error stopping orchestrator", zap.Error(err))
    }
    
    // Stop control plane
    if err := r.controlPlane.Stop(); err != nil {
        logger.Error("Error stopping control plane", zap.Error(err))
    }
    
    logger.Info("Pipeline and control plane stopped successfully")
    return nil
}
```

### Phase 2: Command Integration

#### 2.1 Update Run Command

**File:** `cmd/run.go` (updated)

```go
package cmd

import (
    "fmt"
    "os"
    
    "github.com/spf13/cobra"
    "github.com/withobsrvr/flowctl/internal/model"
    "github.com/withobsrvr/flowctl/internal/runner"
    "github.com/withobsrvr/flowctl/internal/utils/logger"
    "go.uber.org/zap"
)

var (
    orchestratorType     string
    controlPlanePort     int
    controlPlaneAddress  string
    showStatus          bool
)

var runCmd = &cobra.Command{
    Use:   "run [pipeline-file]",
    Short: "Run a data pipeline with integrated control plane",
    Long: `Run a data pipeline by starting components and an embedded control plane.
The control plane automatically starts and components register themselves for monitoring.`,
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Default pipeline file
        pipelineFile := "flow.yml"
        if len(args) > 0 {
            pipelineFile = args[0]
        }
        
        // Check if file exists
        if _, err := os.Stat(pipelineFile); err != nil {
            return fmt.Errorf("pipeline file not found: %s", pipelineFile)
        }
        
        // Load pipeline configuration
        pipeline, err := model.LoadPipelineFromFile(pipelineFile)
        if err != nil {
            return fmt.Errorf("failed to load pipeline: %w", err)
        }
        
        logger.Info("Starting pipeline with embedded control plane", 
            zap.String("name", pipeline.Metadata.Name),
            zap.String("file", pipelineFile),
            zap.String("orchestrator", orchestratorType),
            zap.Int("control_plane_port", controlPlanePort))
        
        // Create logs directory
        if err := os.MkdirAll("logs", 0755); err != nil {
            return fmt.Errorf("failed to create logs directory: %w", err)
        }
        
        // Create pipeline runner with embedded control plane
        runner, err := runner.NewPipelineRunner(pipeline, orchestratorType)
        if err != nil {
            return fmt.Errorf("failed to create pipeline runner: %w", err)
        }
        
        // Run pipeline with integrated control plane
        return runner.Run()
    },
}

func init() {
    rootCmd.AddCommand(runCmd)
    runCmd.Flags().StringVar(&orchestratorType, "orchestrator", "process", "orchestrator type (process|container)")
    runCmd.Flags().IntVar(&controlPlanePort, "control-plane-port", 8080, "embedded control plane port")
    runCmd.Flags().StringVar(&controlPlaneAddress, "control-plane-address", "127.0.0.1", "embedded control plane address")
    runCmd.Flags().BoolVar(&showStatus, "show-status", true, "show periodic status updates")
}
```

#### 2.2 Hide Server Command

**File:** `cmd/root.go` (updated)

```go
func init() {
    cobra.OnInitialize(initConfig)
    
    // ... existing flags ...
    
    // Add visible commands
    rootCmd.AddCommand(apply.NewCommand())
    rootCmd.AddCommand(list.NewCommand())
    rootCmd.AddCommand(sandbox.NewCommand())
    rootCmd.AddCommand(translate.NewCommand())
    rootCmd.AddCommand(version.NewCommand())
    
    // Add hidden server command (for backward compatibility)
    serverCmd := server.NewCommand()
    serverCmd.Hidden = true
    rootCmd.AddCommand(serverCmd)
}
```

#### 2.3 Update List Command

**File:** `cmd/list/list.go` (updated)

```go
var (
    endpoint     string
    waitForServer bool
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List registered services",
    Long: `List all services registered with the control plane.
If a pipeline is running, it will connect to the embedded control plane automatically.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // If endpoint is default, check if embedded control plane is running
        if endpoint == "localhost:8080" && waitForServer {
            // Try to wait a bit for embedded control plane
            logger.Info("Checking for embedded control plane...")
            time.Sleep(2 * time.Second)
        }
        
        // Rest of implementation remains the same...
    },
}

func init() {
    listCmd.Flags().StringVarP(&endpoint, "endpoint", "e", "localhost:8080", "Control plane endpoint")
    listCmd.Flags().BoolVar(&waitForServer, "wait", true, "Wait briefly for embedded control plane")
}
```

### Phase 3: Enhanced Integration Features

#### 3.1 Real-time Status Display

**File:** `internal/runner/status_display.go`

```go
package runner

import (
    "fmt"
    "time"
    "os"
    
    "github.com/fatih/color"
    pb "github.com/withobsrvr/flowctl/proto"
)

type StatusDisplay struct {
    runner *PipelineRunner
    quit   chan struct{}
}

func NewStatusDisplay(runner *PipelineRunner) *StatusDisplay {
    return &StatusDisplay{
        runner: runner,
        quit:   make(chan struct{}),
    }
}

func (s *StatusDisplay) Start() {
    go s.displayLoop()
}

func (s *StatusDisplay) Stop() {
    close(s.quit)
}

func (s *StatusDisplay) displayLoop() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-s.quit:
            return
        case <-ticker.C:
            s.displayStatus()
        }
    }
}

func (s *StatusDisplay) displayStatus() {
    services, err := s.runner.controlPlane.GetServiceList()
    if err != nil {
        return
    }
    
    // Clear screen
    fmt.Print("\033[2J\033[H")
    
    // Header
    fmt.Printf("🚀 Pipeline: %s\n", s.runner.pipeline.Metadata.Name)
    fmt.Printf("⏰ Status as of: %s\n\n", time.Now().Format("15:04:05"))
    
    // Component status
    for _, service := range services {
        status := "🔴 UNHEALTHY"
        if service.IsHealthy {
            status = "🟢 HEALTHY"
        }
        
        fmt.Printf("%-30s %s\n", service.ServiceId, status)
        
        if len(service.Metrics) > 0 {
            for k, v := range service.Metrics {
                fmt.Printf("  %s: %.2f\n", k, v)
            }
        }
    }
    
    fmt.Printf("\nPress Ctrl+C to stop pipeline\n")
}
```

#### 3.2 Component Logs Integration

**File:** `internal/runner/log_aggregator.go`

```go
package runner

import (
    "bufio"
    "fmt"
    "io"
    "sync"
    "time"
    
    "github.com/fatih/color"
)

type LogAggregator struct {
    streams map[string]io.ReadCloser
    colors  map[string]*color.Color
    mu      sync.RWMutex
    quit    chan struct{}
}

func NewLogAggregator() *LogAggregator {
    return &LogAggregator{
        streams: make(map[string]io.ReadCloser),
        colors:  make(map[string]*color.Color),
        quit:    make(chan struct{}),
    }
}

func (l *LogAggregator) AddStream(componentID string, stream io.ReadCloser) {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    l.streams[componentID] = stream
    l.assignColor(componentID)
    
    go l.streamLogs(componentID, stream)
}

func (l *LogAggregator) assignColor(componentID string) {
    colors := []*color.Color{
        color.New(color.FgCyan),
        color.New(color.FgGreen),
        color.New(color.FgYellow),
        color.New(color.FgMagenta),
        color.New(color.FgBlue),
    }
    
    l.colors[componentID] = colors[len(l.colors)%len(colors)]
}

func (l *LogAggregator) streamLogs(componentID string, stream io.ReadCloser) {
    scanner := bufio.NewScanner(stream)
    for scanner.Scan() {
        select {
        case <-l.quit:
            return
        default:
            l.printLog(componentID, scanner.Text())
        }
    }
}

func (l *LogAggregator) printLog(componentID, message string) {
    l.mu.RLock()
    color := l.colors[componentID]
    l.mu.RUnlock()
    
    timestamp := time.Now().Format("15:04:05")
    prefix := fmt.Sprintf("[%s] %s:", timestamp, componentID)
    
    if color != nil {
        color.Printf("%-30s %s\n", prefix, message)
    } else {
        fmt.Printf("%-30s %s\n", prefix, message)
    }
}

func (l *LogAggregator) Stop() {
    close(l.quit)
    
    l.mu.Lock()
    defer l.mu.Unlock()
    
    for _, stream := range l.streams {
        stream.Close()
    }
}
```

## Benefits of Embedded Architecture

### Developer Experience
1. **Single Command**: `flowctl run pipeline.yaml` starts everything
2. **Automatic Setup**: No manual server management required
3. **Integrated Monitoring**: Real-time status display in same terminal
4. **Unified Logging**: All component logs in one place with timestamps
5. **Clean Lifecycle**: Everything starts and stops together

### Production Benefits
1. **Simplified Deployment**: One process to manage instead of two
2. **Resource Efficiency**: No separate server process overhead
3. **Better Monitoring**: Tight integration between orchestrator and control plane
4. **Consistent Configuration**: Single source of truth for networking/ports

### Backward Compatibility
1. **Hidden Server Command**: `flowctl server` still works but is hidden
2. **Same gRPC API**: Components don't need changes
3. **Same Environment Variables**: Existing component integration preserved
4. **List Command Works**: `flowctl list` works with embedded control plane

## Migration Plan

### Phase 1: Implementation (Week 1)
- Create embedded control plane manager
- Integrate into pipeline runner
- Update run command
- Hide server command

### Phase 2: Enhanced Features (Week 2)
- Real-time status display
- Integrated log aggregation
- Component registration waiting
- Health monitoring integration

### Phase 3: Production Features (Week 3)
- Persistent storage options
- TLS configuration passthrough
- Performance optimization
- Monitoring enhancements

## Testing Strategy

### Unit Tests
- Embedded control plane lifecycle
- Component registration flow
- Health monitoring integration
- Error handling scenarios

### Integration Tests
- Full pipeline with real components
- Component registration and heartbeats
- Graceful shutdown scenarios
- Port conflict handling

### User Experience Tests
- Single command pipeline startup
- Status monitoring during execution
- Error message clarity
- Cleanup verification

## Risk Mitigation

| Risk | Mitigation | Status |
|------|------------|--------|
| Port conflicts | Configurable control plane port | ✅ Addressed |
| Component startup race | Wait for control plane ready before components | ✅ Addressed |
| Shutdown ordering | Orchestrator stops before control plane | ✅ Addressed |
| Backward compatibility | Hidden server command, same API | ✅ Addressed |
| Resource usage | Embedded vs separate process overhead | ⚠️ Monitor |

## Success Criteria

1. **Single Command**: `flowctl run pipeline.yaml` starts pipeline with control plane
2. **Component Registration**: All components register automatically
3. **Health Monitoring**: Real-time health status available
4. **Integrated Logging**: Unified log display with component identification
5. **Clean Shutdown**: All processes stop cleanly on Ctrl+C
6. **Backward Compatibility**: Existing workflows still work
7. **Error Handling**: Clear error messages for common issues

This architecture creates a seamless developer experience while preserving all the powerful monitoring and management capabilities of the existing control plane system.
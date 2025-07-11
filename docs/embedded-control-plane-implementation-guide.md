# Embedded Control Plane Implementation Guide

## Overview

This guide provides step-by-step instructions for implementing the embedded control plane architecture that integrates `flowctl server` functionality directly into `flowctl run`. This creates a seamless developer experience where the control plane starts automatically with pipeline execution.

## Prerequisites

Before implementing, ensure you understand:
- The existing `flowctl server` command and its gRPC API
- The current `flowctl run` implementation with mock components
- The control plane's service registry and heartbeat system
- The storage interface and BoltDB backend

## Implementation Steps

### Step 1: Create Embedded Control Plane Manager

Create a new file `internal/controlplane/embedded_server.go`:

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
    pb "github.com/withobsrvr/flowctl/proto"
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

### Step 2: Create Pipeline Runner with Embedded Control Plane

Create or update `internal/runner/pipeline_runner.go`:

```go
package runner

import (
    "context"
    "fmt"
    "time"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/withobsrvr/flowctl/internal/model"
    "github.com/withobsrvr/flowctl/internal/orchestrator"
    "github.com/withobsrvr/flowctl/internal/controlplane"
    "github.com/withobsrvr/flowctl/internal/utils/logger"
    "go.uber.org/zap"
)

type PipelineRunner struct {
    pipeline      *model.Pipeline
    orchestrator  orchestrator.Orchestrator
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
    var orch orchestrator.Orchestrator
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

func (r *PipelineRunner) startComponents() error {
    // Start sources first
    for _, source := range r.pipeline.Spec.Sources {
        if err := r.orchestrator.StartComponent(source); err != nil {
            return fmt.Errorf("failed to start source %s: %w", source.ID, err)
        }
        logger.Info("Started source component", zap.String("id", source.ID))
    }
    
    // Start processors
    for _, processor := range r.pipeline.Spec.Processors {
        if err := r.orchestrator.StartComponent(processor); err != nil {
            return fmt.Errorf("failed to start processor %s: %w", processor.ID, err)
        }
        logger.Info("Started processor component", zap.String("id", processor.ID))
    }
    
    // Start sinks
    for _, sink := range r.pipeline.Spec.Sinks {
        if err := r.orchestrator.StartComponent(sink); err != nil {
            return fmt.Errorf("failed to start sink %s: %w", sink.ID, err)
        }
        logger.Info("Started sink component", zap.String("id", sink.ID))
    }
    
    return nil
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

### Step 3: Update Run Command

Update `cmd/run.go`:

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

### Step 4: Hide Server Command

Update `cmd/root.go` to hide the server command:

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

### Step 5: Create Orchestrator Interface

Create `internal/orchestrator/interface.go`:

```go
package orchestrator

import (
    "github.com/withobsrvr/flowctl/internal/model"
)

type Orchestrator interface {
    StartComponent(component model.Component) error
    StopComponent(componentID string) error
    StopAll() error
    GetStatus() (map[string]ComponentStatus, error)
}

type ComponentStatus struct {
    ID       string
    Status   string
    PID      int
    Endpoint string
    Error    error
}
```

### Step 6: Implement Process Orchestrator

Create `internal/orchestrator/process.go`:

```go
package orchestrator

import (
    "fmt"
    "os"
    "os/exec"
    "sync"
    "syscall"
    
    "github.com/withobsrvr/flowctl/internal/model"
    "github.com/withobsrvr/flowctl/internal/utils/logger"
    "go.uber.org/zap"
)

type ProcessOrchestrator struct {
    controlPlaneEndpoint string
    processes            map[string]*os.Process
    mu                   sync.RWMutex
}

func NewProcessOrchestrator(controlPlaneEndpoint string) *ProcessOrchestrator {
    return &ProcessOrchestrator{
        controlPlaneEndpoint: controlPlaneEndpoint,
        processes:            make(map[string]*os.Process),
    }
}

func (p *ProcessOrchestrator) StartComponent(component model.Component) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    cmd := p.buildCommand(component)
    
    // Set environment variables
    cmd.Env = append(os.Environ(),
        "ENABLE_FLOWCTL=true",
        fmt.Sprintf("FLOWCTL_ENDPOINT=%s", p.controlPlaneEndpoint),
        "FLOWCTL_HEARTBEAT_INTERVAL=10s",
        fmt.Sprintf("FLOWCTL_SERVICE_ID=%s", component.ID),
    )
    
    // Start process
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start component %s: %w", component.ID, err)
    }
    
    p.processes[component.ID] = cmd.Process
    
    logger.Info("Started component process", 
        zap.String("id", component.ID),
        zap.Int("pid", cmd.Process.Pid))
    
    return nil
}

func (p *ProcessOrchestrator) buildCommand(component model.Component) *exec.Cmd {
    switch component.Type {
    case "stellar-live-source-datalake":
        return exec.Command("./bin/stellar-live-source-datalake")
    case "ttp-processor":
        return exec.Command("npm", "start", "--", "1234", "2345")
    case "consumer-app":
        return exec.Command("./bin/consumer-app")
    default:
        return exec.Command("echo", "Unknown component type: "+component.Type)
    }
}

func (p *ProcessOrchestrator) StopComponent(componentID string) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    process, exists := p.processes[componentID]
    if !exists {
        return fmt.Errorf("component %s not found", componentID)
    }
    
    // Send SIGTERM for graceful shutdown
    if err := process.Signal(syscall.SIGTERM); err != nil {
        logger.Error("Failed to send SIGTERM", zap.String("id", componentID), zap.Error(err))
        // Force kill if graceful shutdown fails
        process.Kill()
    }
    
    delete(p.processes, componentID)
    
    logger.Info("Stopped component process", zap.String("id", componentID))
    return nil
}

func (p *ProcessOrchestrator) StopAll() error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    for id, process := range p.processes {
        if err := process.Signal(syscall.SIGTERM); err != nil {
            logger.Error("Failed to stop process", zap.String("id", id), zap.Error(err))
            process.Kill()
        }
    }
    
    p.processes = make(map[string]*os.Process)
    return nil
}

func (p *ProcessOrchestrator) GetStatus() (map[string]ComponentStatus, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    status := make(map[string]ComponentStatus)
    for id, process := range p.processes {
        status[id] = ComponentStatus{
            ID:     id,
            Status: "running",
            PID:    process.Pid,
        }
    }
    
    return status, nil
}
```

## Testing Strategy

### Unit Tests

1. **Embedded Control Plane Tests**
   - Test Start/Stop lifecycle
   - Test service registration forwarding
   - Test component waiting functionality

2. **Pipeline Runner Tests**
   - Test component startup order
   - Test graceful shutdown
   - Test error handling

3. **Orchestrator Tests**
   - Test process spawning
   - Test environment variable setting
   - Test cleanup on shutdown

### Integration Tests

1. **Full Pipeline Test**
   - Start pipeline with embedded control plane
   - Verify component registration
   - Test heartbeat flow
   - Verify graceful shutdown

2. **Backward Compatibility Test**
   - Test hidden server command still works
   - Test list command works with embedded control plane

### Manual Testing

1. **Basic Functionality**
   ```bash
   # Test single command startup
   ./bin/flowctl run ttp-pipeline.yaml
   
   # Verify in another terminal
   ./bin/flowctl list
   ```

2. **Error Scenarios**
   - Test port conflicts
   - Test missing components
   - Test component startup failures

## Common Issues and Solutions

### Port Conflicts
- **Issue**: Control plane port 8080 already in use
- **Solution**: Add `--control-plane-port` flag to run command

### Component Registration Timeout
- **Issue**: Components fail to register within timeout
- **Solution**: Increase timeout or fix component startup issues

### Graceful Shutdown
- **Issue**: Components don't stop cleanly
- **Solution**: Implement proper signal handling in orchestrator

## Verification Steps

1. **Control Plane Integration**
   - [ ] Embedded control plane starts with pipeline
   - [ ] Components register automatically
   - [ ] Heartbeats flow properly
   - [ ] Service list shows all components

2. **User Experience**
   - [ ] Single command starts everything
   - [ ] No manual server management needed
   - [ ] Integrated logging works
   - [ ] Clean shutdown on Ctrl+C

3. **Backward Compatibility**
   - [ ] Hidden server command still works
   - [ ] List command works with embedded control plane
   - [ ] Existing component integration preserved

## Success Criteria

- ✅ `flowctl run pipeline.yaml` starts pipeline with embedded control plane
- ✅ Components register automatically without manual server
- ✅ Real-time monitoring integrated into run command
- ✅ Clean shutdown stops all components and control plane
- ✅ Backward compatibility maintained
- ✅ All tests pass

This implementation creates a seamless developer experience while preserving all existing functionality and maintaining backward compatibility.
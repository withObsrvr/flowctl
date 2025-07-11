# Clean Implementation Guide: flowctl run (No Proxies)

## Overview

This guide implements `flowctl run` as a **pure orchestrator** that starts processes/containers and lets components communicate directly via gRPC. No proxy layer, no architectural debt - just clean container/process orchestration.

## Architecture

```
flowctl run ttp-pipeline.yaml
    ↓
[flowctl orchestrates]
├── Process: stellar-live-source-datalake (Go binary)
├── Process: ttp-processor (Go binary)  
├── Process: consumer-app (npm start)
└── Components talk directly via gRPC
```

**Key Principles:**
1. **flowctl = pure orchestrator** (starts/stops/monitors processes)
2. **Components = independent microservices** (communicate directly)
3. **No proxy layer** (no data flows through flowctl)
4. **Process or container execution** (flexible deployment)

## Implementation Plan

### Phase 1: Process Orchestrator Foundation

#### 1.1 Create Process Orchestrator

**File:** `internal/orchestrator/process_orchestrator.go`

```go
package orchestrator

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "time"
    "net/http"
    "sync"
    "syscall"
    
    "github.com/withobsrvr/flowctl/internal/model"
    "github.com/withobsrvr/flowctl/internal/utils/logger"
    "go.uber.org/zap"
)

type ProcessOrchestrator struct {
    controlPlaneAddr string
    processes        map[string]*ProcessHandle
    mu               sync.RWMutex
}

type ProcessHandle struct {
    ID          string
    Component   model.Component
    Cmd         *exec.Cmd
    Process     *os.Process
    StartTime   time.Time
    Status      ProcessStatus
    HealthURL   string
    LogFile     *os.File
}

type ProcessStatus string

const (
    StatusStarting ProcessStatus = "starting"
    StatusRunning  ProcessStatus = "running"
    StatusStopped  ProcessStatus = "stopped"
    StatusFailed   ProcessStatus = "failed"
)

func NewProcessOrchestrator(controlPlaneAddr string) *ProcessOrchestrator {
    return &ProcessOrchestrator{
        controlPlaneAddr: controlPlaneAddr,
        processes:        make(map[string]*ProcessHandle),
    }
}

func (o *ProcessOrchestrator) StartComponent(ctx context.Context, component model.Component) error {
    o.mu.Lock()
    defer o.mu.Unlock()
    
    logger.Info("Starting component", zap.String("component", component.ID))
    
    // 1. Build command based on component type
    cmd, err := o.buildCommand(component)
    if err != nil {
        return fmt.Errorf("failed to build command for %s: %w", component.ID, err)
    }
    
    // 2. Set up environment variables
    cmd.Env = o.buildEnvironment(component)
    
    // 3. Set up logging
    logFile, err := os.Create(fmt.Sprintf("logs/%s.log", component.ID))
    if err != nil {
        return fmt.Errorf("failed to create log file: %w", err)
    }
    
    cmd.Stdout = logFile
    cmd.Stderr = logFile
    
    // 4. Start the process
    if err := cmd.Start(); err != nil {
        logFile.Close()
        return fmt.Errorf("failed to start %s: %w", component.ID, err)
    }
    
    // 5. Create process handle
    handle := &ProcessHandle{
        ID:        component.ID,
        Component: component,
        Cmd:       cmd,
        Process:   cmd.Process,
        StartTime: time.Now(),
        Status:    StatusStarting,
        HealthURL: fmt.Sprintf("http://localhost:%d/health", component.HealthPort),
        LogFile:   logFile,
    }
    
    o.processes[component.ID] = handle
    
    // 6. Wait for process to be ready
    if err := o.waitForReady(ctx, handle); err != nil {
        o.stopProcess(handle)
        return fmt.Errorf("component %s failed to become ready: %w", component.ID, err)
    }
    
    handle.Status = StatusRunning
    logger.Info("Component started successfully", zap.String("component", component.ID))
    
    return nil
}

func (o *ProcessOrchestrator) buildCommand(component model.Component) (*exec.Cmd, error) {
    switch component.Type {
    case "stellar-live-source-datalake":
        // Go binary: ./stellar-live-source-datalake/bin/stellar-live-source-datalake
        return exec.Command("./stellar-live-source-datalake/bin/stellar-live-source-datalake"), nil
        
    case "ttp-processor":
        // Go binary: ./ttp-processor/bin/ttp-processor
        return exec.Command("./ttp-processor/bin/ttp-processor"), nil
        
    case "consumer-app":
        // Node.js: npm start -- <start_ledger> <end_ledger>
        args := []string{"start", "--"}
        if startLedger := component.Env["START_LEDGER"]; startLedger != "" {
            args = append(args, startLedger)
        }
        if endLedger := component.Env["END_LEDGER"]; endLedger != "" {
            args = append(args, endLedger)
        }
        
        cmd := exec.Command("npm", args...)
        cmd.Dir = "./consumer_app/node"
        return cmd, nil
        
    default:
        return nil, fmt.Errorf("unknown component type: %s", component.Type)
    }
}

func (o *ProcessOrchestrator) buildEnvironment(component model.Component) []string {
    env := os.Environ()
    
    // Add component-specific environment variables
    for k, v := range component.Env {
        env = append(env, fmt.Sprintf("%s=%s", k, v))
    }
    
    // Add flowctl SDK integration
    env = append(env,
        fmt.Sprintf("ENABLE_FLOWCTL=%s", "true"),
        fmt.Sprintf("FLOWCTL_ENDPOINT=%s", o.controlPlaneAddr),
        fmt.Sprintf("FLOWCTL_HEARTBEAT_INTERVAL=%s", "10s"),
        fmt.Sprintf("COMPONENT_ID=%s", component.ID),
    )
    
    return env
}

func (o *ProcessOrchestrator) waitForReady(ctx context.Context, handle *ProcessHandle) error {
    if handle.Component.HealthPort == 0 {
        // No health check, just wait a bit
        time.Sleep(3 * time.Second)
        return nil
    }
    
    client := &http.Client{Timeout: 2 * time.Second}
    
    for i := 0; i < 30; i++ { // 30 seconds timeout
        // Check if process is still running
        if handle.Process.Signal(syscall.Signal(0)) != nil {
            return fmt.Errorf("process died during startup")
        }
        
        // Check health endpoint
        resp, err := client.Get(handle.HealthURL)
        if err == nil && resp.StatusCode == 200 {
            resp.Body.Close()
            logger.Info("Component health check passed", zap.String("component", handle.ID))
            return nil
        }
        if resp != nil {
            resp.Body.Close()
        }
        
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(1 * time.Second):
            continue
        }
    }
    
    return fmt.Errorf("health check timeout for component %s", handle.ID)
}

func (o *ProcessOrchestrator) StopComponent(componentID string) error {
    o.mu.Lock()
    defer o.mu.Unlock()
    
    handle, exists := o.processes[componentID]
    if !exists {
        return fmt.Errorf("component %s not found", componentID)
    }
    
    return o.stopProcess(handle)
}

func (o *ProcessOrchestrator) stopProcess(handle *ProcessHandle) error {
    logger.Info("Stopping component", zap.String("component", handle.ID))
    
    if handle.Process != nil {
        // Send SIGTERM first
        if err := handle.Process.Signal(syscall.SIGTERM); err != nil {
            logger.Warn("Failed to send SIGTERM", zap.String("component", handle.ID), zap.Error(err))
        }
        
        // Wait for graceful shutdown
        done := make(chan error, 1)
        go func() {
            _, err := handle.Process.Wait()
            done <- err
        }()
        
        select {
        case <-done:
            // Process stopped gracefully
        case <-time.After(10 * time.Second):
            // Force kill
            logger.Warn("Force killing component", zap.String("component", handle.ID))
            handle.Process.Kill()
        }
    }
    
    if handle.LogFile != nil {
        handle.LogFile.Close()
    }
    
    handle.Status = StatusStopped
    delete(o.processes, handle.ID)
    
    logger.Info("Component stopped", zap.String("component", handle.ID))
    return nil
}

func (o *ProcessOrchestrator) StopAll() error {
    o.mu.Lock()
    defer o.mu.Unlock()
    
    var errors []error
    for _, handle := range o.processes {
        if err := o.stopProcess(handle); err != nil {
            errors = append(errors, err)
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("failed to stop some components: %v", errors)
    }
    
    return nil
}

func (o *ProcessOrchestrator) GetStatus() map[string]ProcessStatus {
    o.mu.RLock()
    defer o.mu.RUnlock()
    
    status := make(map[string]ProcessStatus)
    for id, handle := range o.processes {
        status[id] = handle.Status
    }
    
    return status
}

func (o *ProcessOrchestrator) MonitorComponents(ctx context.Context) error {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            o.checkComponentHealth()
        }
    }
}

func (o *ProcessOrchestrator) checkComponentHealth() {
    o.mu.RLock()
    defer o.mu.RUnlock()
    
    for _, handle := range o.processes {
        if handle.Status != StatusRunning {
            continue
        }
        
        // Check if process is still alive
        if handle.Process.Signal(syscall.Signal(0)) != nil {
            logger.Warn("Component process died", zap.String("component", handle.ID))
            handle.Status = StatusFailed
        }
    }
}
```

#### 1.2 Create Container Orchestrator (Alternative)

**File:** `internal/orchestrator/container_orchestrator.go`

```go
package orchestrator

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
    "time"
    "net/http"
    "sync"
    
    "github.com/withobsrvr/flowctl/internal/model"
    "github.com/withobsrvr/flowctl/internal/utils/logger"
    "go.uber.org/zap"
)

type ContainerOrchestrator struct {
    network          string
    controlPlaneAddr string
    containers       map[string]*ContainerHandle
    mu               sync.RWMutex
}

type ContainerHandle struct {
    ID          string
    Component   model.Component
    ContainerID string
    Status      ContainerStatus
    StartTime   time.Time
    HealthURL   string
}

type ContainerStatus string

const (
    ContainerStatusStarting ContainerStatus = "starting"
    ContainerStatusRunning  ContainerStatus = "running"
    ContainerStatusStopped  ContainerStatus = "stopped"
    ContainerStatusFailed   ContainerStatus = "failed"
)

func NewContainerOrchestrator(controlPlaneAddr string) *ContainerOrchestrator {
    return &ContainerOrchestrator{
        network:          "flowctl-bridge",
        controlPlaneAddr: controlPlaneAddr,
        containers:       make(map[string]*ContainerHandle),
    }
}

func (o *ContainerOrchestrator) StartComponent(ctx context.Context, component model.Component) error {
    o.mu.Lock()
    defer o.mu.Unlock()
    
    logger.Info("Starting container", zap.String("component", component.ID))
    
    // 1. Ensure bridge network exists
    if err := o.ensureNetwork(); err != nil {
        return fmt.Errorf("failed to ensure network: %w", err)
    }
    
    // 2. Build docker run command
    args := []string{
        "run", "-d",
        "--name", component.ID,
        "--network", o.network,
        "--restart", "unless-stopped",
    }
    
    // 3. Add environment variables
    for k, v := range component.Env {
        args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
    }
    
    // Add flowctl SDK integration
    args = append(args,
        "-e", fmt.Sprintf("ENABLE_FLOWCTL=%s", "true"),
        "-e", fmt.Sprintf("FLOWCTL_ENDPOINT=%s", o.controlPlaneAddr),
        "-e", fmt.Sprintf("FLOWCTL_HEARTBEAT_INTERVAL=%s", "10s"),
        "-e", fmt.Sprintf("COMPONENT_ID=%s", component.ID),
    )
    
    // 4. Add port bindings
    for _, port := range component.Ports {
        args = append(args, "-p", fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort))
    }
    
    if component.HealthPort > 0 {
        args = append(args, "-p", fmt.Sprintf("%d:%d", component.HealthPort, component.HealthPort))
    }
    
    // 5. Add image and command
    args = append(args, component.Image)
    if len(component.Command) > 0 {
        args = append(args, component.Command...)
    }
    
    // 6. Start container
    cmd := exec.CommandContext(ctx, "docker", args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to start container %s: %w, output: %s", component.ID, err, output)
    }
    
    containerID := strings.TrimSpace(string(output))
    
    // 7. Create container handle
    handle := &ContainerHandle{
        ID:          component.ID,
        Component:   component,
        ContainerID: containerID,
        Status:      ContainerStatusStarting,
        StartTime:   time.Now(),
        HealthURL:   fmt.Sprintf("http://localhost:%d/health", component.HealthPort),
    }
    
    o.containers[component.ID] = handle
    
    // 8. Wait for container to be ready
    if err := o.waitForReady(ctx, handle); err != nil {
        o.stopContainer(handle)
        return fmt.Errorf("container %s failed to become ready: %w", component.ID, err)
    }
    
    handle.Status = ContainerStatusRunning
    logger.Info("Container started successfully", zap.String("component", component.ID))
    
    return nil
}

func (o *ContainerOrchestrator) ensureNetwork() error {
    cmd := exec.Command("docker", "network", "inspect", o.network)
    if err := cmd.Run(); err == nil {
        return nil
    }
    
    cmd = exec.Command("docker", "network", "create", "--driver", "bridge", o.network)
    return cmd.Run()
}

func (o *ContainerOrchestrator) waitForReady(ctx context.Context, handle *ContainerHandle) error {
    if handle.Component.HealthPort == 0 {
        time.Sleep(3 * time.Second)
        return nil
    }
    
    client := &http.Client{Timeout: 2 * time.Second}
    
    for i := 0; i < 30; i++ {
        resp, err := client.Get(handle.HealthURL)
        if err == nil && resp.StatusCode == 200 {
            resp.Body.Close()
            return nil
        }
        if resp != nil {
            resp.Body.Close()
        }
        
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(1 * time.Second):
            continue
        }
    }
    
    return fmt.Errorf("health check timeout for container %s", handle.ID)
}

func (o *ContainerOrchestrator) stopContainer(handle *ContainerHandle) error {
    cmd := exec.Command("docker", "rm", "-f", handle.ContainerID)
    return cmd.Run()
}

func (o *ContainerOrchestrator) StopAll() error {
    o.mu.Lock()
    defer o.mu.Unlock()
    
    var errors []error
    for _, handle := range o.containers {
        if err := o.stopContainer(handle); err != nil {
            errors = append(errors, err)
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("failed to stop some containers: %v", errors)
    }
    
    return nil
}
```

### Phase 2: Simplified Pipeline Runner

#### 2.1 Create Pipeline Runner

**File:** `internal/runner/pipeline_runner.go`

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
    "github.com/withobsrvr/flowctl/internal/utils/logger"
    "go.uber.org/zap"
)

type PipelineRunner struct {
    pipeline     *model.Pipeline
    orchestrator Orchestrator
    ctx          context.Context
    cancel       context.CancelFunc
}

type Orchestrator interface {
    StartComponent(ctx context.Context, component model.Component) error
    StopAll() error
    GetStatus() map[string]interface{}
    MonitorComponents(ctx context.Context) error
}

func NewPipelineRunner(pipeline *model.Pipeline, orchestratorType string, controlPlaneAddr string) (*PipelineRunner, error) {
    var orch Orchestrator
    
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
        ctx:          ctx,
        cancel:       cancel,
    }, nil
}

func (r *PipelineRunner) Run() error {
    logger.Info("Starting pipeline", zap.String("pipeline", r.pipeline.Metadata.Name))
    
    // Set up signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    // Start components in dependency order
    if err := r.startComponents(); err != nil {
        return err
    }
    
    // Start monitoring
    go func() {
        if err := r.orchestrator.MonitorComponents(r.ctx); err != nil {
            logger.Error("Monitor error", zap.Error(err))
        }
    }()
    
    logger.Info("Pipeline started successfully")
    
    // Wait for shutdown signal
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
        if err := r.orchestrator.StartComponent(r.ctx, source); err != nil {
            return fmt.Errorf("failed to start source %s: %w", source.ID, err)
        }
        logger.Info("Started source", zap.String("source", source.ID))
    }
    
    // Wait a bit for sources to be ready
    time.Sleep(2 * time.Second)
    
    // Start processors
    for _, processor := range r.pipeline.Spec.Processors {
        if err := r.orchestrator.StartComponent(r.ctx, processor); err != nil {
            return fmt.Errorf("failed to start processor %s: %w", processor.ID, err)
        }
        logger.Info("Started processor", zap.String("processor", processor.ID))
    }
    
    // Wait a bit for processors to be ready
    time.Sleep(2 * time.Second)
    
    // Start sinks
    for _, sink := range r.pipeline.Spec.Sinks {
        if err := r.orchestrator.StartComponent(r.ctx, sink); err != nil {
            return fmt.Errorf("failed to start sink %s: %w", sink.ID, err)
        }
        logger.Info("Started sink", zap.String("sink", sink.ID))
    }
    
    return nil
}

func (r *PipelineRunner) shutdown() error {
    logger.Info("Shutting down pipeline")
    
    // Cancel context to stop monitoring
    r.cancel()
    
    // Stop all components
    if err := r.orchestrator.StopAll(); err != nil {
        return fmt.Errorf("failed to stop components: %w", err)
    }
    
    logger.Info("Pipeline stopped successfully")
    return nil
}

func (r *PipelineRunner) GetStatus() map[string]interface{} {
    return r.orchestrator.GetStatus()
}
```

### Phase 3: Update Run Command

#### 3.1 Simplify Run Command

**File:** `cmd/run.go`

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
    orchestratorType string
    controlPlaneAddr string
)

var runCmd = &cobra.Command{
    Use:   "run [pipeline-file]",
    Short: "Run a data pipeline",
    Long:  `Run a data pipeline by starting and orchestrating its components.`,
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
        
        logger.Info("Loaded pipeline", 
            zap.String("name", pipeline.Metadata.Name),
            zap.String("file", pipelineFile))
        
        // Create logs directory
        if err := os.MkdirAll("logs", 0755); err != nil {
            return fmt.Errorf("failed to create logs directory: %w", err)
        }
        
        // Create pipeline runner
        runner, err := runner.NewPipelineRunner(pipeline, orchestratorType, controlPlaneAddr)
        if err != nil {
            return fmt.Errorf("failed to create pipeline runner: %w", err)
        }
        
        // Run pipeline
        return runner.Run()
    },
}

func init() {
    rootCmd.AddCommand(runCmd)
    runCmd.Flags().StringVar(&orchestratorType, "orchestrator", "process", "orchestrator type (process|container)")
    runCmd.Flags().StringVar(&controlPlaneAddr, "control-plane", "http://localhost:8080", "control plane address")
}
```

### Phase 4: Pipeline Configuration

#### 4.1 Create Clean Pipeline Config

**File:** `examples/ttp-pipeline.yaml`

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: ttp-processing-pipeline
spec:
  sources:
    - id: stellar-live-source-datalake
      type: stellar-live-source-datalake
      # For process orchestrator
      binary: "./stellar-live-source-datalake/bin/stellar-live-source-datalake"
      # For container orchestrator  
      image: "withobsrvr/stellar-live-source-datalake:latest"
      health_port: 8088
      env:
        STORAGE_TYPE: "S3"
        BUCKET_NAME: "stellar-ledger-data"
        AWS_REGION: "us-east-1"
        LEDGERS_PER_FILE: "64"
        FILES_PER_PARTITION: "10"
        STELLAR_NETWORK: "testnet"
        GRPC_PORT: "50051"
        HEALTH_PORT: "8088"
        LOG_LEVEL: "info"
      ports:
        - containerPort: 50051
          hostPort: 50051
        
  processors:
    - id: ttp-processor
      type: ttp-processor
      # For process orchestrator
      binary: "./ttp-processor/bin/ttp-processor"
      # For container orchestrator
      image: "withobsrvr/ttp-processor:latest"
      # Components connect to each other directly
      depends_on: ["stellar-live-source-datalake"]
      health_port: 8089
      env:
        LIVE_SOURCE_ENDPOINT: "localhost:50051"  # Direct connection
        GRPC_PORT: "50052"
        HEALTH_PORT: "8089"
        LOG_LEVEL: "info"
        START_LEDGER: "1"
        END_LEDGER: "0"
      ports:
        - containerPort: 50052
          hostPort: 50052
        
  sinks:
    - id: consumer-app
      type: consumer-app
      # For process orchestrator
      binary: "npm"
      args: ["start", "--", "1", "0"]  # start_ledger, end_ledger
      working_dir: "./consumer_app/node"
      # For container orchestrator
      image: "withobsrvr/consumer-app:latest"
      depends_on: ["ttp-processor"]
      health_port: 3000
      env:
        TTP_SERVICE_ADDRESS: "localhost:50052"  # Direct connection
        PORT: "3000"
        START_LEDGER: "1"
        END_LEDGER: "0"
        LOG_LEVEL: "info"
      ports:
        - containerPort: 3000
          hostPort: 3000
```

### Phase 5: Build Scripts

#### 5.1 Component Build Scripts

**File:** `scripts/build-components.sh`

```bash
#!/bin/bash

set -e

echo "Building ttp-processor-demo components..."

# Build stellar-live-source-datalake
echo "Building stellar-live-source-datalake..."
cd stellar-live-source-datalake
make build
mkdir -p bin
mv stellar-live-source-datalake bin/
cd ..

# Build ttp-processor
echo "Building ttp-processor..."
cd ttp-processor
make build
mkdir -p bin
mv ttp-processor bin/
cd ..

# Build consumer-app
echo "Building consumer-app..."
cd consumer_app/node
npm install
cd ../..

echo "All components built successfully!"
```

#### 5.2 Docker Build Scripts

**File:** `scripts/build-docker-images.sh`

```bash
#!/bin/bash

set -e

echo "Building Docker images..."

# Build stellar-live-source-datalake
docker build -t withobsrvr/stellar-live-source-datalake:latest \
  -f stellar-live-source-datalake/Dockerfile \
  stellar-live-source-datalake/

# Build ttp-processor
docker build -t withobsrvr/ttp-processor:latest \
  -f ttp-processor/Dockerfile \
  ttp-processor/

# Build consumer-app
docker build -t withobsrvr/consumer-app:latest \
  -f consumer_app/Dockerfile \
  consumer_app/

echo "All Docker images built successfully!"
```

## Usage Examples

### Process Orchestration (Default)
```bash
# Build components first
./scripts/build-components.sh

# Run pipeline with process orchestrator
./bin/flowctl run examples/ttp-pipeline.yaml

# Or explicitly specify
./bin/flowctl run examples/ttp-pipeline.yaml --orchestrator process
```

### Container Orchestration
```bash
# Build Docker images first
./scripts/build-docker-images.sh

# Run pipeline with container orchestrator
./bin/flowctl run examples/ttp-pipeline.yaml --orchestrator container
```

### Check Status
```bash
# View logs
tail -f logs/stellar-live-source-datalake.log
tail -f logs/ttp-processor.log
tail -f logs/consumer-app.log

# Check health
curl http://localhost:8088/health  # stellar-live-source-datalake
curl http://localhost:8089/health  # ttp-processor  
curl http://localhost:3000/health  # consumer-app
```

## Benefits of This Approach

### 1. **Clean Architecture**
- flowctl is pure orchestrator
- No proxy layer
- Components communicate directly
- Matches real-world microservices

### 2. **Flexible Deployment**
- Process orchestration for development
- Container orchestration for production
- Same pipeline config works for both

### 3. **Language Independence**
- Go binaries work
- Node.js apps work
- Any language with HTTP health checks

### 4. **Performance**
- Direct gRPC communication
- No proxy overhead
- Native component performance

### 5. **Debugging**
- Standard process/container debugging
- Direct log files
- Standard gRPC debugging tools

## Testing Plan

### Phase 1: Process Orchestration
```bash
# Build components
./scripts/build-components.sh

# Test individual components
./stellar-live-source-datalake/bin/stellar-live-source-datalake &
./ttp-processor/bin/ttp-processor &
cd consumer_app/node && npm start -- 1 0 &

# Test orchestrated pipeline
./bin/flowctl run examples/ttp-pipeline.yaml --orchestrator process
```

### Phase 2: Container Orchestration
```bash
# Build images
./scripts/build-docker-images.sh

# Test orchestrated pipeline
./bin/flowctl run examples/ttp-pipeline.yaml --orchestrator container
```

### Phase 3: Integration Testing
```bash
# Test data flow
curl http://localhost:8088/health
curl http://localhost:8089/health
curl http://localhost:3000/health

# Test shutdown
Ctrl+C

# Verify cleanup
ps aux | grep "stellar-live-source-datalake\|ttp-processor\|consumer-app"
# or
docker ps
```

## Success Criteria

1. **Process Orchestration**: `flowctl run` starts all Go binaries and npm process
2. **Container Orchestration**: `flowctl run` starts all Docker containers
3. **Health Checks**: All components pass health checks
4. **Direct Communication**: Components communicate via gRPC without proxies
5. **flowctl SDK Integration**: Components register with flowctl control plane
6. **Logging**: Component logs written to separate files
7. **Cleanup**: All processes/containers stop cleanly on shutdown

This approach eliminates architectural debt and creates a clean, maintainable foundation for flowctl orchestration.
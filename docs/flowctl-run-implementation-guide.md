# Complete Implementation Guide: flowctl run

## Overview

This guide provides step-by-step instructions to transform `flowctl run` from its current mock implementation to a production-ready container orchestration system that can run real components from the ttp-processor-demo repository.

## Current State Analysis

### What Works
- Basic CLI command structure exists (`cmd/run.go`)
- Pipeline configuration loading (`internal/config/`)
- DAG pipeline architecture (`internal/core/pipeline_dag.go`)
- Basic YAML configuration parsing

### What's Broken
- Factory functions return mock components (`testfixtures.CreateMockSource()`)
- No container orchestration (everything runs in-memory)
- No real gRPC communication between components
- No component lifecycle management

## Target Architecture

```
flowctl run ttp-pipeline.yaml
    ↓
[flowctl process orchestrates]
├── Docker Container: stellar-live-source-datalake
├── Docker Container: ttp-processor  
├── Docker Container: consumer-app
└── gRPC communication between containers
```

## Implementation Plan

### Phase 1: Container Orchestration Foundation

#### 1.1 Create Production Docker Executor

**File:** `internal/executor/production_docker.go`

```go
package executor

import (
    "context"
    "fmt"
    "os/exec"
    "time"
    "net/http"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/health/grpc_health_v1"
    
    "github.com/withobsrvr/flowctl/internal/model"
)

type ProductionDockerExecutor struct {
    network          string
    controlPlaneAddr string
    containers       map[string]*ContainerHandle
}

type ContainerHandle struct {
    ID            string
    ContainerID   string
    Image         string
    HealthPort    int
    HealthCheck   string
    GRPCPort      int
}

func NewProductionDockerExecutor(controlPlaneAddr string) *ProductionDockerExecutor {
    return &ProductionDockerExecutor{
        network:          "flowctl-bridge",
        controlPlaneAddr: controlPlaneAddr,
        containers:       make(map[string]*ContainerHandle),
    }
}

func (e *ProductionDockerExecutor) StartComponent(ctx context.Context, component model.Component) (*ContainerHandle, error) {
    // 1. Ensure bridge network exists
    if err := e.ensureNetwork(); err != nil {
        return nil, fmt.Errorf("failed to ensure network: %w", err)
    }
    
    // 2. Build docker run command
    args := []string{
        "run", "-d",
        "--name", component.ID,
        "--network", e.network,
        "--restart", "unless-stopped",
    }
    
    // 3. Add flowctl integration environment variables
    flowctlEnv := map[string]string{
        "ENABLE_FLOWCTL":         "true",
        "FLOWCTL_ENDPOINT":       e.controlPlaneAddr,
        "FLOWCTL_HEARTBEAT_INTERVAL": "10s",
        "COMPONENT_ID":           component.ID,
    }
    
    // 4. Add environment variables
    for k, v := range component.Env {
        args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
    }
    for k, v := range flowctlEnv {
        args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
    }
    
    // 5. Add port bindings
    var healthPort, grpcPort int
    for _, port := range component.Ports {
        args = append(args, "-p", fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort))
        if port.ContainerPort == 50051 || port.ContainerPort == 50052 {
            grpcPort = port.HostPort
        }
    }
    
    if component.HealthPort > 0 {
        healthPort = component.HealthPort
        args = append(args, "-p", fmt.Sprintf("%d:%d", healthPort, healthPort))
    }
    
    // 6. Add image and command
    args = append(args, component.Image)
    if len(component.Command) > 0 {
        args = append(args, component.Command...)
    }
    
    // 7. Start container
    cmd := exec.CommandContext(ctx, "docker", args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("failed to start container %s: %w, output: %s", component.ID, err, output)
    }
    
    containerID := strings.TrimSpace(string(output))
    
    // 8. Create handle
    handle := &ContainerHandle{
        ID:          component.ID,
        ContainerID: containerID,
        Image:       component.Image,
        HealthPort:  healthPort,
        HealthCheck: component.HealthCheck,
        GRPCPort:    grpcPort,
    }
    
    e.containers[component.ID] = handle
    
    // 9. Wait for readiness
    if err := e.waitForReady(ctx, handle); err != nil {
        // Cleanup on failure
        exec.Command("docker", "rm", "-f", component.ID).Run()
        return nil, fmt.Errorf("component %s failed to become ready: %w", component.ID, err)
    }
    
    return handle, nil
}

func (e *ProductionDockerExecutor) waitForReady(ctx context.Context, handle *ContainerHandle) error {
    // Wait for HTTP health check
    if handle.HealthPort > 0 {
        client := &http.Client{Timeout: 2 * time.Second}
        url := fmt.Sprintf("http://localhost:%d/health", handle.HealthPort)
        
        for i := 0; i < 30; i++ {
            resp, err := client.Get(url)
            if err == nil && resp.StatusCode == 200 {
                resp.Body.Close()
                break
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
    }
    
    // Wait for gRPC readiness
    if handle.GRPCPort > 0 {
        grpcAddr := fmt.Sprintf("localhost:%d", handle.GRPCPort)
        
        for i := 0; i < 30; i++ {
            conn, err := grpc.DialContext(ctx, grpcAddr, 
                grpc.WithTransportCredentials(insecure.NewCredentials()),
                grpc.WithBlock(),
                grpc.WithTimeout(2*time.Second))
            
            if err == nil {
                conn.Close()
                return nil
            }
            
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(1 * time.Second):
                continue
            }
        }
        
        return fmt.Errorf("gRPC readiness check timeout for component %s", handle.ID)
    }
    
    return nil
}

func (e *ProductionDockerExecutor) ensureNetwork() error {
    // Check if network exists
    cmd := exec.Command("docker", "network", "inspect", e.network)
    if err := cmd.Run(); err == nil {
        return nil
    }
    
    // Create bridge network
    cmd = exec.Command("docker", "network", "create", 
        "--driver", "bridge",
        "--attachable",
        e.network)
    return cmd.Run()
}

func (e *ProductionDockerExecutor) StopComponent(componentID string) error {
    if handle, exists := e.containers[componentID]; exists {
        cmd := exec.Command("docker", "rm", "-f", handle.ContainerID)
        delete(e.containers, componentID)
        return cmd.Run()
    }
    return nil
}

func (e *ProductionDockerExecutor) StopAll() error {
    for componentID := range e.containers {
        if err := e.StopComponent(componentID); err != nil {
            return err
        }
    }
    return nil
}
```

#### 1.2 Generate gRPC Proto Files

**Create directory structure:**
```bash
mkdir -p internal/grpc/gen/raw_ledger_service
mkdir -p internal/grpc/gen/event_service
mkdir -p internal/grpc/gen/token_transfer
```

**Add to Makefile:**
```makefile
generate-grpc:
	protoc --go_out=./internal/grpc/gen --go_opt=paths=source_relative \
	       --go-grpc_out=./internal/grpc/gen --go-grpc_opt=paths=source_relative \
	       --proto_path=./proto \
	       raw_ledger_service/raw_ledger_service.proto \
	       event_service/event_service.proto \
	       token_transfer/token_transfer_event.proto
```

#### 1.3 Copy Proto Files

**Copy from ttp-processor-demo:**
```bash
# Copy proto files
cp ../ttp-processor-demo/stellar-live-source-datalake/protos/raw_ledger_service/raw_ledger_service.proto \
   proto/raw_ledger_service/

cp ../ttp-processor-demo/ttp-processor/protos/event_service/event_service.proto \
   proto/event_service/

cp ../ttp-processor-demo/ttp-processor/protos/ingest/processors/token_transfer/token_transfer_event.proto \
   proto/token_transfer/
```

### Phase 2: Real Component Implementations

#### 2.1 Stellar Live Source DataLake

**File:** `internal/core/stellar_live_source_datalake.go`

```go
package core

import (
    "context"
    "fmt"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    
    "github.com/withobsrvr/flowctl/internal/source"
    "github.com/withobsrvr/flowctl/internal/executor"
    pb "github.com/withobsrvr/flowctl/internal/grpc/gen/raw_ledger_service"
)

type StellarLiveSourceDataLake struct {
    componentID string
    endpoint    string
    executor    *executor.ProductionDockerExecutor
    conn        *grpc.ClientConn
    client      pb.RawLedgerServiceClient
    startLedger uint32
}

func NewStellarLiveSourceDataLake(componentID, endpoint string, executor *executor.ProductionDockerExecutor, startLedger uint32) *StellarLiveSourceDataLake {
    return &StellarLiveSourceDataLake{
        componentID: componentID,
        endpoint:    endpoint,
        executor:    executor,
        startLedger: startLedger,
    }
}

func (s *StellarLiveSourceDataLake) Open(ctx context.Context) error {
    conn, err := grpc.DialContext(ctx, s.endpoint, 
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return fmt.Errorf("failed to connect to stellar-live-source-datalake at %s: %w", s.endpoint, err)
    }
    
    s.conn = conn
    s.client = pb.NewRawLedgerServiceClient(conn)
    return nil
}

func (s *StellarLiveSourceDataLake) Events(ctx context.Context, out chan<- source.EventEnvelope) error {
    req := &pb.StreamLedgersRequest{
        StartLedger: s.startLedger,
    }
    
    stream, err := s.client.StreamRawLedgers(ctx, req)
    if err != nil {
        return fmt.Errorf("failed to start streaming from data lake: %w", err)
    }
    
    for {
        ledger, err := stream.Recv()
        if err != nil {
            return fmt.Errorf("stream error: %w", err)
        }
        
        event := source.EventEnvelope{
            LedgerSeq: ledger.Sequence,
            Payload:   ledger.LedgerCloseMetaXdr,
            Cursor:    fmt.Sprintf("datalake-ledger-%d", ledger.Sequence),
        }
        
        select {
        case out <- event:
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (s *StellarLiveSourceDataLake) Close() error {
    if s.conn != nil {
        s.conn.Close()
    }
    return nil
}

func (s *StellarLiveSourceDataLake) Healthy() error {
    if s.conn == nil {
        return fmt.Errorf("no connection to %s", s.componentID)
    }
    return nil
}
```

#### 2.2 TTP Processor

**File:** `internal/core/ttp_processor.go`

```go
package core

import (
    "context"
    "fmt"
    "time"
    "encoding/json"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    
    "github.com/withobsrvr/flowctl/internal/processor"
    "github.com/withobsrvr/flowctl/internal/source"
    "github.com/withobsrvr/flowctl/internal/executor"
    pb "github.com/withobsrvr/flowctl/internal/grpc/gen/event_service"
)

type TTPProcessor struct {
    componentID string
    endpoint    string
    executor    *executor.ProductionDockerExecutor
    conn        *grpc.ClientConn
    client      pb.EventServiceClient
    startLedger uint32
    endLedger   uint32
}

func NewTTPProcessor(componentID, endpoint string, executor *executor.ProductionDockerExecutor, startLedger, endLedger uint32) *TTPProcessor {
    return &TTPProcessor{
        componentID: componentID,
        endpoint:    endpoint,
        executor:    executor,
        startLedger: startLedger,
        endLedger:   endLedger,
    }
}

func (t *TTPProcessor) Init(cfg map[string]any) error {
    conn, err := grpc.Dial(t.endpoint, 
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return fmt.Errorf("failed to connect to ttp-processor at %s: %w", t.endpoint, err)
    }
    
    t.conn = conn
    t.client = pb.NewEventServiceClient(conn)
    return nil
}

func (t *TTPProcessor) Process(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error) {
    req := &pb.GetEventsRequest{
        StartLedger: t.startLedger,
        EndLedger:   t.endLedger,
    }
    
    stream, err := t.client.GetTTPEvents(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to get TTP events: %w", err)
    }
    
    var messages []*processor.Message
    
    eventCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    for {
        select {
        case <-eventCtx.Done():
            return messages, nil
        default:
            event, err := stream.Recv()
            if err != nil {
                return messages, nil
            }
            
            // Convert TokenTransferEvent to JSON
            eventData, err := json.Marshal(map[string]interface{}{
                "sequence":  event.Meta.LedgerSequence,
                "timestamp": event.Meta.Timestamp,
                "tx_hash":   event.Meta.TxHash,
                "component": t.componentID,
                "event":     event,
            })
            if err != nil {
                continue
            }
            
            msg := &processor.Message{
                Type:    "token.transfer",
                Payload: eventData,
            }
            
            messages = append(messages, msg)
        }
    }
}

func (t *TTPProcessor) Flush(ctx context.Context) error {
    return nil
}

func (t *TTPProcessor) Name() string {
    return t.componentID
}

func (t *TTPProcessor) Close() error {
    if t.conn != nil {
        t.conn.Close()
    }
    return nil
}
```

#### 2.3 Consumer App Sink

**File:** `internal/core/consumer_app_sink.go`

```go
package core

import (
    "context"
    "fmt"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    
    "github.com/withobsrvr/flowctl/internal/sink"
    "github.com/withobsrvr/flowctl/internal/processor"
    "github.com/withobsrvr/flowctl/internal/executor"
)

type ConsumerAppSink struct {
    componentID string
    endpoint    string
    executor    *executor.ProductionDockerExecutor
    conn        *grpc.ClientConn
}

func NewConsumerAppSink(componentID, endpoint string, executor *executor.ProductionDockerExecutor) *ConsumerAppSink {
    return &ConsumerAppSink{
        componentID: componentID,
        endpoint:    endpoint,
        executor:    executor,
    }
}

func (c *ConsumerAppSink) Connect(cfg map[string]any) error {
    conn, err := grpc.Dial(c.endpoint, 
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return fmt.Errorf("failed to connect to consumer-app at %s: %w", c.endpoint, err)
    }
    
    c.conn = conn
    return nil
}

func (c *ConsumerAppSink) Write(ctx context.Context, messages []*processor.Message) error {
    for _, msg := range messages {
        fmt.Printf("[%s] %s\n", c.componentID, string(msg.Payload))
    }
    return nil
}

func (c *ConsumerAppSink) Close() error {
    if c.conn != nil {
        c.conn.Close()
    }
    return nil
}
```

### Phase 3: Factory Function Updates

#### 3.1 Update Factory Functions

**File:** `internal/core/pipeline_dag.go`

**Replace existing factory functions:**

```go
// Add global executor
var globalExecutor *executor.ProductionDockerExecutor

func init() {
    globalExecutor = executor.NewProductionDockerExecutor("http://localhost:8080")
}

func createSource(component model.Component) (source.Source, error) {
    // Start the component container
    handle, err := globalExecutor.StartComponent(context.Background(), component)
    if err != nil {
        return nil, fmt.Errorf("failed to start source container: %w", err)
    }
    
    // Create component-specific source
    switch component.ID {
    case "stellar-live-source-datalake":
        endpoint := fmt.Sprintf("localhost:%d", handle.GRPCPort)
        return NewStellarLiveSourceDataLake(component.ID, endpoint, globalExecutor, 1), nil
    default:
        return nil, fmt.Errorf("unknown source component: %s", component.ID)
    }
}

func createProcessor(component model.Component) (processor.Processor, error) {
    handle, err := globalExecutor.StartComponent(context.Background(), component)
    if err != nil {
        return nil, fmt.Errorf("failed to start processor container: %w", err)
    }
    
    switch component.ID {
    case "ttp-processor":
        endpoint := fmt.Sprintf("localhost:%d", handle.GRPCPort)
        return NewTTPProcessor(component.ID, endpoint, globalExecutor, 1, 0), nil
    default:
        return nil, fmt.Errorf("unknown processor component: %s", component.ID)
    }
}

func createSink(component model.Component) (sink.Sink, error) {
    handle, err := globalExecutor.StartComponent(context.Background(), component)
    if err != nil {
        return nil, fmt.Errorf("failed to start sink container: %w", err)
    }
    
    switch component.ID {
    case "consumer-app":
        endpoint := fmt.Sprintf("localhost:%d", handle.GRPCPort)
        return NewConsumerAppSink(component.ID, endpoint, globalExecutor), nil
    default:
        return nil, fmt.Errorf("unknown sink component: %s", component.ID)
    }
}
```

#### 3.2 Add Cleanup to Pipeline

**In `pipeline_dag.go`, add cleanup method:**

```go
func (p *DAGPipeline) Stop() error {
    // Stop DAG
    if err := p.dag.Stop(); err != nil {
        return fmt.Errorf("failed to stop DAG: %w", err)
    }
    
    // Stop all containers
    if err := globalExecutor.StopAll(); err != nil {
        return fmt.Errorf("failed to stop containers: %w", err)
    }
    
    return nil
}
```

### Phase 4: Pipeline Configuration

#### 4.1 Create Production Pipeline Config

**File:** `examples/ttp-pipeline.yaml`

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: ttp-processing-pipeline
spec:
  driver: dag
  
  sources:
    - id: stellar-live-source-datalake
      type: source
      image: withobsrvr/stellar-live-source-datalake:latest
      command: ["./stellar-live-source-datalake"]
      health_port: 8088
      health_check: "/health"
      ports:
        - containerPort: 50051
          hostPort: 50051
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
        
  processors:
    - id: ttp-processor
      type: processor
      image: withobsrvr/ttp-processor:latest
      command: ["./ttp-processor"]
      inputs: ["stellar-live-source-datalake"]
      health_port: 8089
      health_check: "/health"
      ports:
        - containerPort: 50052
          hostPort: 50052
      env:
        LIVE_SOURCE_ENDPOINT: "stellar-live-source-datalake:50051"
        GRPC_PORT: "50052"
        HEALTH_PORT: "8089"
        LOG_LEVEL: "info"
        START_LEDGER: "1"
        END_LEDGER: "0"
        
  sinks:
    - id: consumer-app
      type: sink
      image: withobsrvr/consumer-app:latest
      command: ["npm", "start"]
      inputs: ["ttp-processor"]
      health_port: 3000
      ports:
        - containerPort: 3000
          hostPort: 3000
      env:
        TTP_SERVICE_ADDRESS: "ttp-processor:50052"
        PORT: "3000"
        START_LEDGER: "1"
        END_LEDGER: "0"
        LOG_LEVEL: "info"
```

### Phase 5: Update Run Command

#### 5.1 Enhance Run Command

**File:** `cmd/run.go`

**Update to support new DAG pipeline:**

```go
func runCmd = &cobra.Command{
    Use:   "run [config-file]",
    Short: "Run a data pipeline",
    Long:  `Run a data pipeline using the specified configuration file.`,
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Default config file
        configFile := "flow.yml"
        if len(args) > 0 {
            configFile = args[0]
        }
        
        // Load pipeline configuration
        pipelineModel, err := model.LoadPipelineFromFile(configFile)
        if err != nil {
            return fmt.Errorf("failed to load pipeline config: %w", err)
        }
        
        // Create DAG pipeline
        cfg := &config.Config{} // Basic config
        pipeline, err := core.NewDAGPipeline(cfg, pipelineModel)
        if err != nil {
            return fmt.Errorf("failed to create pipeline: %w", err)
        }
        
        // Set up context with cancellation
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        
        // Handle signals
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        go func() {
            <-sigChan
            fmt.Println("\nShutting down...")
            cancel()
        }()
        
        // Start pipeline
        fmt.Printf("Starting pipeline: %s\n", pipelineModel.Metadata.Name)
        if err := pipeline.Start(ctx); err != nil {
            return fmt.Errorf("failed to start pipeline: %w", err)
        }
        
        // Wait for context cancellation
        <-ctx.Done()
        
        // Stop pipeline and cleanup containers
        fmt.Println("Stopping pipeline and cleaning up...")
        if err := pipeline.Stop(); err != nil {
            return fmt.Errorf("failed to stop pipeline: %w", err)
        }
        
        fmt.Println("Pipeline stopped successfully")
        return nil
    },
}
```

### Phase 6: Docker Images

#### 6.1 Build Docker Images

**Create Dockerfiles for each component:**

```dockerfile
# stellar-live-source-datalake/Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o stellar-live-source-datalake go/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/stellar-live-source-datalake .
EXPOSE 50051 8088
CMD ["./stellar-live-source-datalake"]
```

```dockerfile
# ttp-processor/Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o ttp-processor go/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/ttp-processor .
EXPOSE 50052 8089
CMD ["./ttp-processor"]
```

```dockerfile
# consumer_app/Dockerfile
FROM node:22-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["npm", "start"]
```

**Build commands:**
```bash
# Build images
docker build -t withobsrvr/stellar-live-source-datalake:latest -f stellar-live-source-datalake/Dockerfile .
docker build -t withobsrvr/ttp-processor:latest -f ttp-processor/Dockerfile .
docker build -t withobsrvr/consumer-app:latest -f consumer_app/Dockerfile .
```

## Testing Plan

### Phase 1: Component Testing
```bash
# Test each component individually
docker run --rm withobsrvr/stellar-live-source-datalake:latest
docker run --rm withobsrvr/ttp-processor:latest
docker run --rm withobsrvr/consumer-app:latest
```

### Phase 2: Integration Testing
```bash
# Build flowctl
make build

# Test pipeline
./bin/flowctl run examples/ttp-pipeline.yaml

# Check running containers
docker ps

# View logs
docker logs stellar-live-source-datalake
docker logs ttp-processor
docker logs consumer-app
```

### Phase 3: End-to-End Testing
```bash
# Run full pipeline
./bin/flowctl run examples/ttp-pipeline.yaml

# Verify data flow
curl http://localhost:3000/health
curl http://localhost:8088/health
curl http://localhost:8089/health

# Stop pipeline
Ctrl+C

# Verify cleanup
docker ps  # Should show no flowctl containers
```

## Dependencies

### Required Packages
```bash
go mod tidy
```

### Proto Generation
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Docker Images
All three component images must be built and available locally.

## Success Criteria

1. **Container Startup**: `flowctl run` starts all three Docker containers
2. **Health Checks**: All components pass health checks
3. **gRPC Communication**: Components communicate via gRPC
4. **Data Flow**: Data flows from source → processor → sink
5. **Cleanup**: All containers stop when pipeline stops
6. **Error Handling**: Proper error messages and cleanup on failure

## Common Issues and Solutions

### Issue: Container startup fails
**Solution**: Check Docker images exist and ports are available

### Issue: Health checks timeout
**Solution**: Increase timeout values or check component health endpoints

### Issue: gRPC connection fails
**Solution**: Verify network connectivity and port mappings

### Issue: Components don't register
**Solution**: Check flowctl integration environment variables

## File Summary

**New Files to Create:**
- `internal/executor/production_docker.go`
- `internal/core/stellar_live_source_datalake.go`
- `internal/core/ttp_processor.go`
- `internal/core/consumer_app_sink.go`
- `examples/ttp-pipeline.yaml`
- `proto/` directory with copied proto files

**Files to Modify:**
- `internal/core/pipeline_dag.go` (factory functions)
- `cmd/run.go` (command implementation)
- `Makefile` (add gRPC generation)

**Total Implementation Time:** 2-3 days for experienced developer
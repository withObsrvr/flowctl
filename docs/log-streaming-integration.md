# Log Streaming Integration for Flowctl

## Overview

### Current State

Currently, flowctl and ttp-processor-demo components handle logging independently:

- **Flowctl**: Captures component stdout/stderr to individual log files in the `logs/` directory
- **TTP-Processor-Demo**: Components write logs to stdout/stderr using structured logging (zap)
- **Integration**: Limited to control plane communication and health checks

### Vision

Enable real-time, aggregated log streaming from all pipeline components through flowctl, similar to Docker Compose or Dagger.io, providing:

- Unified log view across all components
- Real-time streaming with color-coded output
- Filtering by component, level, and time
- Historical log retrieval
- Integration with external logging systems

### Benefits

1. **Developer Experience**: See all component logs in one place during development
2. **Debugging**: Correlate events across components with synchronized timestamps
3. **Monitoring**: Real-time visibility into pipeline execution
4. **Operations**: Easy log collection for troubleshooting production issues

## Current Architecture Analysis

### Flowctl Logging System

#### Key Components

1. **Logger Utility** (`internal/utils/logger/logger.go`)
   - Uses zap for structured logging
   - Configurable log levels
   - JSON or console output formats

2. **Log Aggregator** (`internal/runner/log_aggregator.go`)
   - Aggregates logs from multiple components
   - Color-coded output by component
   - Implements `io.Reader` interface

3. **Process Orchestrator** (`internal/orchestrator/process.go`)
   - Captures stdout/stderr to log files
   - Creates `logs/{componentID}.log` files
   - Provides `ProcessLogStream` for reading logs

4. **Logs Command** (`cmd/logs.go`)
   - Currently a placeholder with TODO
   - Intended for fetching logs from runtime

#### Current Flow

```
Component Process → stdout/stderr → Process Orchestrator → Log File
                                                         ↓
                                                    Log Aggregator → Console Output
```

### TTP-Processor-Demo Logging

#### Components

1. **ttp-processor** (Go)
   - Uses zap logger for structured logging
   - Outputs to stdout/stderr
   - Supports JSON and console formats

2. **stellar-live-source** (Go)
   - Similar zap-based logging
   - Configurable log levels

3. **consumer-app** (Node.js)
   - Uses console.log for output
   - Less structured than Go components

#### Integration Points

- `ENABLE_FLOWCTL` environment variable enables flowctl integration
- Components register with control plane via gRPC
- Regular heartbeats with metrics (but not logs)
- Health endpoints for status checking

### Existing Infrastructure

1. **Control Plane Protocol** (`proto/control_plane.proto`)
   - Service registration and heartbeat
   - No log streaming RPCs currently

2. **Orchestrator Interface**
   - `GetLogs()` method returns LogStream
   - Only works for local process execution

3. **Log Storage**
   - File-based only
   - No persistent storage backend
   - No remote log access

## Implementation Plan

### Phase 1: Enhance Log Streaming Infrastructure

#### 1.1 Extend Control Plane Protocol

```protobuf
// Add to control_plane.proto

message LogEntry {
    string component_id = 1;
    string component_type = 2;
    google.protobuf.Timestamp timestamp = 3;
    string level = 4;
    string message = 5;
    map<string, string> fields = 6;
}

message StreamLogsRequest {
    repeated string component_ids = 1;
    repeated string levels = 2;
    google.protobuf.Timestamp since = 3;
    bool follow = 4;
}

service ControlPlane {
    // Existing methods...
    
    // Bidirectional streaming for logs
    rpc StreamLogs(stream LogEntry) returns (stream LogEntry);
    
    // Query historical logs
    rpc GetLogs(StreamLogsRequest) returns (stream LogEntry);
}
```

#### 1.2 Implement Log Buffer in Control Plane

```go
type LogBuffer struct {
    mu       sync.RWMutex
    entries  []LogEntry
    maxSize  int
    subscribers map[string]chan<- LogEntry
}

func (lb *LogBuffer) Add(entry LogEntry) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    lb.entries = append(lb.entries, entry)
    if len(lb.entries) > lb.maxSize {
        lb.entries = lb.entries[1:]
    }
    
    // Notify subscribers
    for _, ch := range lb.subscribers {
        select {
        case ch <- entry:
        default:
            // Handle slow consumers
        }
    }
}
```

#### 1.3 Create Log Streaming Service

```go
type LogStreamingService struct {
    buffer *LogBuffer
    components map[string]LogStream
}

func (s *LogStreamingService) StreamLogs(stream pb.ControlPlane_StreamLogsServer) error {
    // Handle bidirectional streaming
    // Receive logs from components
    // Send logs to clients
}
```

### Phase 2: Component Integration

#### 2.1 Update TTP-Processor Components

Add log interceptor to capture zap output:

```go
// log_interceptor.go
type FlowctlLogWriter struct {
    client pb.ControlPlaneClient
    stream pb.ControlPlane_StreamLogsClient
    componentID string
}

func (w *FlowctlLogWriter) Write(p []byte) (n int, err error) {
    // Parse log entry
    // Send to control plane
    // Also write to original destination
}

// Integration in main.go
if os.Getenv("ENABLE_FLOWCTL") == "true" {
    // Create interceptor
    interceptor := &FlowctlLogWriter{
        client: flowctlClient,
        componentID: componentID,
    }
    
    // Configure zap to use interceptor
    config := zap.NewProductionConfig()
    config.OutputPaths = []string{"stdout", "flowctl://"}
}
```

#### 2.2 Update Consumer App (Node.js)

Intercept console.log calls:

```javascript
// log_interceptor.js
const originalLog = console.log;
const grpc = require('@grpc/grpc-js');

class FlowctlLogger {
    constructor(client, componentId) {
        this.client = client;
        this.componentId = componentId;
        this.stream = client.streamLogs();
    }
    
    intercept() {
        console.log = (...args) => {
            // Send to flowctl
            this.stream.write({
                componentId: this.componentId,
                timestamp: new Date(),
                level: 'info',
                message: args.join(' ')
            });
            
            // Call original
            originalLog.apply(console, args);
        };
    }
}
```

#### 2.3 Enhance FlowctlController

```go
func (fc *FlowctlController) StartLogStreaming() error {
    stream, err := fc.client.StreamLogs(context.Background())
    if err != nil {
        return err
    }
    
    fc.logStream = stream
    
    // Start goroutine to send logs
    go fc.streamLogs()
    
    return nil
}
```

### Phase 3: Flowctl CLI Implementation

#### 3.1 Implement `flowctl logs` Command

```go
// cmd/logs.go
func logsCmd() *cobra.Command {
    var (
        follow bool
        since string
        components []string
        levels []string
    )
    
    cmd := &cobra.Command{
        Use:   "logs [pipeline]",
        Short: "Fetch logs from pipeline components",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Connect to control plane
            // Stream logs
            // Display with color coding
        },
    }
    
    cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
    cmd.Flags().StringVarP(&since, "since", "s", "", "Show logs since timestamp")
    cmd.Flags().StringSliceVarP(&components, "components", "c", nil, "Filter by components")
    cmd.Flags().StringSliceVarP(&levels, "levels", "l", nil, "Filter by log levels")
    
    return cmd
}
```

#### 3.2 Add Log Display

```go
type LogDisplay struct {
    colors map[string]string
    mu     sync.Mutex
}

func (ld *LogDisplay) DisplayEntry(entry *pb.LogEntry) {
    ld.mu.Lock()
    defer ld.mu.Unlock()
    
    color := ld.getColor(entry.ComponentId)
    timestamp := entry.Timestamp.AsTime().Format("15:04:05")
    
    fmt.Printf("%s[%s] %s[%s]%s %s\n",
        color,
        entry.ComponentId,
        timestamp,
        entry.Level,
        resetColor,
        entry.Message,
    )
}
```

#### 3.3 Create Log Persistence (Optional)

```go
type LogStore interface {
    Store(entry *pb.LogEntry) error
    Query(filter LogFilter) ([]*pb.LogEntry, error)
}

type SQLiteLogStore struct {
    db *sql.DB
}

func (s *SQLiteLogStore) Store(entry *pb.LogEntry) error {
    // Store in SQLite for historical queries
}
```

### Phase 4: Advanced Features

#### 4.1 Log Search and Filtering

```go
type LogFilter struct {
    ComponentIDs []string
    Levels      []string
    Since       time.Time
    Until       time.Time
    Pattern     *regexp.Regexp
}

func (lf *LogFilter) Matches(entry *pb.LogEntry) bool {
    // Implement filtering logic
}
```

#### 4.2 Log Export

```go
type LogExporter interface {
    Export(entries []*pb.LogEntry, writer io.Writer) error
}

type JSONExporter struct{}
type CSVExporter struct{}

func exportLogs(cmd *cobra.Command, args []string) error {
    // Export logs in various formats
}
```

#### 4.3 Performance Optimization

- Implement log batching to reduce network overhead
- Add compression for log transmission
- Implement rate limiting to prevent log flooding
- Use circular buffers with configurable sizes

## Technical Details

### Protocol Extensions

The gRPC protocol needs to be extended with:

1. **LogEntry Message**: Structured log representation
2. **StreamLogs RPC**: Bidirectional streaming for real-time logs
3. **GetLogs RPC**: Query historical logs

### Component Modifications

1. **Go Components**:
   - Add log interceptor for zap
   - Implement log streaming client
   - Handle connection failures gracefully

2. **Node.js Components**:
   - Intercept console methods
   - Convert to structured format
   - Implement gRPC client

3. **Control Plane**:
   - Add log buffer and storage
   - Implement streaming service
   - Handle multiple subscribers

### CLI Command Implementation

The `flowctl logs` command will:

1. Connect to control plane
2. Subscribe to log stream
3. Apply filters
4. Display formatted output
5. Handle interrupts gracefully

### Performance Considerations

1. **Buffering**: Use circular buffers to limit memory usage
2. **Batching**: Send logs in batches to reduce overhead
3. **Compression**: Optional compression for large log volumes
4. **Rate Limiting**: Prevent log flooding from affecting performance

## Example Usage

### Basic Log Streaming

```bash
# Stream all logs from a running pipeline
$ flowctl logs my-pipeline

# Follow logs in real-time (like tail -f)
$ flowctl logs -f my-pipeline

# Filter by component
$ flowctl logs -c ttp-processor,consumer-app my-pipeline

# Filter by log level
$ flowctl logs -l error,warn my-pipeline
```

### Advanced Filtering

```bash
# Show logs since a specific time
$ flowctl logs --since "2024-01-20 10:00:00" my-pipeline

# Show logs matching a pattern
$ flowctl logs --grep "error processing" my-pipeline

# Export logs to a file
$ flowctl logs --export json > logs.json
```

### Configuration Example

```yaml
# pipeline.yaml
pipeline:
  name: my-pipeline
  logging:
    enabled: true
    buffer_size: 10000
    retention: 24h
    export:
      format: json
      destination: s3://my-bucket/logs/
  
  components:
    - name: ttp-processor
      type: processor
      logging:
        level: debug
        format: json
```

### Programmatic Access

```go
// Using the SDK
client := flowctl.NewClient()
pipeline := client.GetPipeline("my-pipeline")

// Stream logs
stream, err := pipeline.StreamLogs(flowctl.LogOptions{
    Follow: true,
    Components: []string{"ttp-processor"},
})

for entry := range stream {
    fmt.Printf("[%s] %s\n", entry.Component, entry.Message)
}
```

## Troubleshooting

### Common Issues

1. **No logs appearing**:
   - Check `ENABLE_FLOWCTL` is set
   - Verify component registration
   - Check network connectivity

2. **Missing logs**:
   - Increase buffer size
   - Check rate limiting settings
   - Verify log levels

3. **Performance issues**:
   - Enable batching
   - Use compression
   - Filter unnecessary logs

### Debug Mode

```bash
# Enable debug logging for flowctl itself
$ FLOWCTL_LOG_LEVEL=debug flowctl logs my-pipeline

# Check component registration
$ flowctl status my-pipeline
```

## Future Enhancements

1. **Integration with External Systems**:
   - Elasticsearch/OpenSearch
   - Splunk
   - Datadog
   - CloudWatch

2. **Advanced Analytics**:
   - Log pattern detection
   - Anomaly detection
   - Performance metrics from logs

3. **Multi-cluster Support**:
   - Aggregate logs from multiple deployments
   - Cross-region log streaming

4. **Security**:
   - Log encryption in transit
   - Access control for sensitive logs
   - Audit logging

This implementation plan provides a comprehensive approach to integrating log streaming between flowctl and ttp-processor-demo components, enabling a developer experience similar to Docker Compose while building on the existing architecture.
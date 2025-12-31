# Building Components for flowctl

This guide covers building custom sources, processors, and sinks using the flowctl-sdk.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Component Types](#component-types)
- [Building a Source](#building-a-source)
- [Building a Processor](#building-a-processor)
- [Building a Sink](#building-a-sink)
- [Testing Components](#testing-components)
- [Deployment](#deployment)

## Overview

Components are **separate Go programs** that implement the flowctl component interface. The **flowctl-sdk** provides packages that handle:

✅ gRPC server setup
✅ Control plane registration
✅ Health checks and heartbeats
✅ Configuration management
✅ Graceful shutdown
✅ Error handling
✅ Metrics collection

**You focus on:** The business logic of processing data.

## Prerequisites

### Required Tools

- **Go 1.21+**: Install from https://go.dev/dl/
- **Git**: For cloning repositories
- **flowctl**: Built and available at `./bin/flowctl`

### Required Knowledge

- Basic Go programming
- Understanding of gRPC concepts (not required to implement, just understand)
- Familiarity with Protobuf (SDK handles most complexity)

### Get the SDK

```bash
# Clone the SDK
git clone https://github.com/withObsrvr/flowctl-sdk.git
cd flowctl-sdk

# Explore examples
ls -la examples/
```

## Component Types

### Source (Data Producer)

**Purpose:** Generate or fetch data from external systems

**Examples:**
- Stellar ledger streamer
- Kafka consumer
- REST API poller
- Database change streamer
- File watcher

**SDK Package:** `github.com/withObsrvr/flowctl-sdk/pkg/source`

**Output:** Protobuf events to downstream processors or sinks

### Processor (Data Transformer)

**Purpose:** Transform data as it flows through the pipeline

**Examples:**
- Event extractor (e.g., contract events from ledgers)
- Data filter
- Data enricher
- Aggregator
- Format converter

**SDK Package:** `github.com/withObsrvr/flowctl-sdk/pkg/processor`

**Input:** Protobuf events from sources or other processors
**Output:** Transformed Protobuf events to downstream components

### Sink (Data Consumer)

**Purpose:** Write data to storage or external systems

**Examples:**
- PostgreSQL writer
- Webhook caller
- File writer
- Kafka producer
- S3 uploader

**SDK Package:**
- `github.com/withObsrvr/flowctl-sdk/pkg/consumer` (legacy)
- `github.com/withObsrvr/flowctl-sdk/pkg/sink` (new)

**Input:** Protobuf events from sources or processors
**Output:** None (terminal node)

## Building a Source

Sources produce data for the pipeline.

### Basic Source Structure

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/withObsrvr/flowctl-sdk/pkg/source"
    flowpb "github.com/withObsrvr/flow-proto/gen/go/flow/v1"
)

func main() {
    // Create source with configuration
    src := source.New(source.Config{
        Name:        "my-data-source",
        Description: "Produces data from external API",
        Version:     "1.0.0",
        OutputType:  "myorg.data.v1", // Event type this source produces
    })

    // Set the data production function
    src.SetProduceFunc(produceData)

    // Run the source (blocks until shutdown)
    if err := src.Run(); err != nil {
        log.Fatalf("Source failed: %v", err)
    }
}

// produceData is called to generate events
func produceData(ctx context.Context) ([]*flowpb.Event, error) {
    // TODO: Fetch data from your source
    // This could be: API call, database query, message queue, etc.

    events := make([]*flowpb.Event, 0)

    // Example: Create an event
    event := &flowpb.Event{
        Id:        "event-" + time.Now().Format("20060102150405"),
        Type:      "myorg.data.v1",
        Timestamp: time.Now().Unix(),
        Data:      []byte(`{"key": "value"}`), // Your data as JSON or Protobuf
    }

    events = append(events, event)

    return events, nil
}
```

### Real Example: Polling Source

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/withObsrvr/flowctl-sdk/pkg/source"
    flowpb "github.com/withObsrvr/flow-proto/gen/go/flow/v1"
)

type APIResponse struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Data      string    `json:"data"`
}

func main() {
    apiEndpoint := os.Getenv("API_ENDPOINT") // e.g., https://api.example.com/events
    pollInterval := getEnvDuration("POLL_INTERVAL", 5*time.Second)

    src := source.New(source.Config{
        Name:        "api-poller-source",
        Description: "Polls external API for events",
        Version:     "1.0.0",
        OutputType:  "myorg.api.event.v1",
    })

    // Polling state
    lastPoll := time.Now()

    src.SetProduceFunc(func(ctx context.Context) ([]*flowpb.Event, error) {
        // Wait for poll interval
        time.Sleep(pollInterval)

        // Call API
        resp, err := http.Get(apiEndpoint)
        if err != nil {
            return nil, fmt.Errorf("API call failed: %w", err)
        }
        defer resp.Body.Close()

        body, err := io.ReadAll(resp.Body)
        if err != nil {
            return nil, fmt.Errorf("reading response failed: %w", err)
        }

        var apiResp APIResponse
        if err := json.Unmarshal(body, &apiResp); err != nil {
            return nil, fmt.Errorf("parsing response failed: %w", err)
        }

        // Convert to flowctl event
        event := &flowpb.Event{
            Id:        apiResp.ID,
            Type:      "myorg.api.event.v1",
            Timestamp: apiResp.Timestamp.Unix(),
            Data:      body, // Store original JSON
        }

        lastPoll = time.Now()
        log.Printf("Produced event: %s at %v", event.Id, lastPoll)

        return []*flowpb.Event{event}, nil
    })

    if err := src.Run(); err != nil {
        log.Fatalf("Source failed: %v", err)
    }
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
    val := os.Getenv(key)
    if val == "" {
        return defaultVal
    }
    duration, err := time.ParseDuration(val)
    if err != nil {
        log.Printf("Invalid duration for %s: %v, using default", key, err)
        return defaultVal
    }
    return duration
}
```

### Configuration

Sources are configured via environment variables:

```yaml
sources:
  - id: my-source
    command: ["/path/to/bin/my-source"]
    env:
      # flowctl integration
      ENABLE_FLOWCTL: "true"
      FLOWCTL_ENDPOINT: "127.0.0.1:8080"

      # Component ports
      PORT: ":50051"
      HEALTH_PORT: "8088"

      # Source-specific config
      API_ENDPOINT: "https://api.example.com/events"
      POLL_INTERVAL: "5s"
```

## Building a Processor

Processors transform data flowing through the pipeline.

### Basic Processor Structure

```go
package main

import (
    "context"
    "log"

    "github.com/withObsrvr/flowctl-sdk/pkg/processor"
    flowpb "github.com/withObsrvr/flow-proto/gen/go/flow/v1"
)

func main() {
    proc := processor.New(processor.Config{
        Name:        "my-processor",
        Description: "Transforms events",
        Version:     "1.0.0",
        InputType:   "myorg.data.v1",      // Type this processor accepts
        OutputType:  "myorg.processed.v1", // Type this processor outputs
    })

    // Set the processing function
    proc.SetProcessFunc(processEvent)

    // Run the processor (blocks until shutdown)
    if err := proc.Run(); err != nil {
        log.Fatalf("Processor failed: %v", err)
    }
}

// processEvent transforms a single event
func processEvent(ctx context.Context, event *flowpb.Event) ([]*flowpb.Event, error) {
    // TODO: Transform the event
    // Input: event.Data contains input data
    // Output: Return transformed events

    // Example: Add a field to JSON data
    // (In production, use proper JSON or Protobuf parsing)

    transformedData := append(event.Data, []byte(`,"processed":true`)...)

    transformedEvent := &flowpb.Event{
        Id:        event.Id + "-processed",
        Type:      "myorg.processed.v1",
        Timestamp: event.Timestamp,
        Data:      transformedData,
    }

    return []*flowpb.Event{transformedEvent}, nil
}
```

### Real Example: Filter Processor

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "strings"

    "github.com/withObsrvr/flowctl-sdk/pkg/processor"
    flowpb "github.com/withObsrvr/flow-proto/gen/go/flow/v1"
)

type EventData struct {
    Type   string `json:"type"`
    Value  string `json:"value"`
    Status string `json:"status"`
}

func main() {
    // Configuration from environment
    filterType := os.Getenv("FILTER_TYPE")   // e.g., "transfer"
    filterStatus := os.Getenv("FILTER_STATUS") // e.g., "success"

    proc := processor.New(processor.Config{
        Name:        "event-filter",
        Description: "Filters events by type and status",
        Version:     "1.0.0",
        InputType:   "myorg.event.v1",
        OutputType:  "myorg.event.v1", // Same type, just filtered
    })

    proc.SetProcessFunc(func(ctx context.Context, event *flowpb.Event) ([]*flowpb.Event, error) {
        // Parse event data
        var data EventData
        if err := json.Unmarshal(event.Data, &data); err != nil {
            log.Printf("Failed to parse event %s: %v", event.Id, err)
            return nil, nil // Skip invalid events
        }

        // Apply filters
        if filterType != "" && !strings.EqualFold(data.Type, filterType) {
            log.Printf("Event %s filtered out: type mismatch", event.Id)
            return nil, nil // Filter out
        }

        if filterStatus != "" && !strings.EqualFold(data.Status, filterStatus) {
            log.Printf("Event %s filtered out: status mismatch", event.Id)
            return nil, nil // Filter out
        }

        log.Printf("Event %s passed filter", event.Id)
        return []*flowpb.Event{event}, nil // Pass through
    })

    if err := proc.Run(); err != nil {
        log.Fatalf("Processor failed: %v", err)
    }
}
```

### Configuration

```yaml
processors:
  - id: event-filter
    command: ["/path/to/bin/event-filter"]
    inputs: ["upstream-source-id"]
    env:
      ENABLE_FLOWCTL: "true"
      FLOWCTL_ENDPOINT: "127.0.0.1:8080"
      PORT: ":50052"
      HEALTH_PORT: "8089"

      # Processor-specific config
      FILTER_TYPE: "transfer"
      FILTER_STATUS: "success"
```

## Building a Sink

Sinks consume data and write to storage or external systems.

### Basic Sink Structure

```go
package main

import (
    "context"
    "log"

    "github.com/withObsrvr/flowctl-sdk/pkg/consumer"
    flowpb "github.com/withObsrvr/flow-proto/gen/go/flow/v1"
)

func main() {
    sink := consumer.New(consumer.Config{
        Name:        "my-sink",
        Description: "Writes events to storage",
        Version:     "1.0.0",
        InputType:   "myorg.processed.v1", // Type this sink accepts
    })

    // Set the consumption function
    sink.SetConsumeFunc(consumeEvent)

    // Run the sink (blocks until shutdown)
    if err := sink.Run(); err != nil {
        log.Fatalf("Sink failed: %v", err)
    }
}

// consumeEvent writes a single event
func consumeEvent(ctx context.Context, event *flowpb.Event) error {
    // TODO: Write event to storage
    // Examples: Database insert, file write, HTTP POST, etc.

    log.Printf("Consumed event: %s", event.Id)

    // Example: Log to stdout
    log.Printf("Event data: %s", string(event.Data))

    return nil
}
```

### Real Example: PostgreSQL Sink

```go
package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "os"

    _ "github.com/lib/pq"
    "github.com/withObsrvr/flowctl-sdk/pkg/consumer"
    flowpb "github.com/withObsrvr/flow-proto/gen/go/flow/v1"
)

type EventData struct {
    Type      string `json:"type"`
    Value     string `json:"value"`
    Status    string `json:"status"`
    Timestamp int64  `json:"timestamp"`
}

func main() {
    // Database configuration from environment
    dbHost := os.Getenv("POSTGRES_HOST")
    dbPort := os.Getenv("POSTGRES_PORT")
    dbName := os.Getenv("POSTGRES_DB")
    dbUser := os.Getenv("POSTGRES_USER")
    dbPass := os.Getenv("POSTGRES_PASSWORD")

    // Connect to PostgreSQL
    connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
        dbHost, dbPort, dbName, dbUser, dbPass)

    db, err := sql.Open("postgres", connStr)
    if err != nil {
        log.Fatalf("Database connection failed: %v", err)
    }
    defer db.Close()

    // Verify connection
    if err := db.Ping(); err != nil {
        log.Fatalf("Database ping failed: %v", err)
    }

    // Create table if not exists
    createTable := `
        CREATE TABLE IF NOT EXISTS events (
            id TEXT PRIMARY KEY,
            type TEXT NOT NULL,
            value TEXT,
            status TEXT,
            timestamp BIGINT,
            raw_data JSONB,
            created_at TIMESTAMPTZ DEFAULT NOW()
        )
    `
    if _, err := db.Exec(createTable); err != nil {
        log.Fatalf("Table creation failed: %v", err)
    }

    // Create sink
    sink := consumer.New(consumer.Config{
        Name:        "postgresql-sink",
        Description: "Writes events to PostgreSQL",
        Version:     "1.0.0",
        InputType:   "myorg.event.v1",
    })

    sink.SetConsumeFunc(func(ctx context.Context, event *flowpb.Event) error {
        // Parse event data
        var data EventData
        if err := json.Unmarshal(event.Data, &data); err != nil {
            return fmt.Errorf("failed to parse event: %w", err)
        }

        // Insert into database
        query := `
            INSERT INTO events (id, type, value, status, timestamp, raw_data)
            VALUES ($1, $2, $3, $4, $5, $6)
            ON CONFLICT (id) DO UPDATE SET
                type = EXCLUDED.type,
                value = EXCLUDED.value,
                status = EXCLUDED.status,
                timestamp = EXCLUDED.timestamp,
                raw_data = EXCLUDED.raw_data
        `

        _, err := db.ExecContext(ctx, query,
            event.Id,
            data.Type,
            data.Value,
            data.Status,
            data.Timestamp,
            event.Data,
        )

        if err != nil {
            return fmt.Errorf("database insert failed: %w", err)
        }

        log.Printf("Inserted event: %s", event.Id)
        return nil
    })

    if err := sink.Run(); err != nil {
        log.Fatalf("Sink failed: %v", err)
    }
}
```

### Configuration

```yaml
sinks:
  - id: postgresql-sink
    command: ["/path/to/bin/postgresql-sink"]
    inputs: ["upstream-processor-id"]
    env:
      ENABLE_FLOWCTL: "true"
      FLOWCTL_ENDPOINT: "127.0.0.1:8080"
      PORT: ":50053"
      HEALTH_PORT: "8090"

      # Database configuration
      POSTGRES_HOST: "localhost"
      POSTGRES_PORT: "5432"
      POSTGRES_DB: "events_db"
      POSTGRES_USER: "postgres"
      POSTGRES_PASSWORD: "password"
```

## Testing Components

### Unit Testing

Test your processing logic separately from the SDK:

```go
package main

import (
    "context"
    "testing"

    flowpb "github.com/withObsrvr/flow-proto/gen/go/flow/v1"
)

func TestProcessEvent(t *testing.T) {
    ctx := context.Background()

    input := &flowpb.Event{
        Id:        "test-1",
        Type:      "myorg.data.v1",
        Timestamp: 1234567890,
        Data:      []byte(`{"key":"value"}`),
    }

    output, err := processEvent(ctx, input)
    if err != nil {
        t.Fatalf("processEvent failed: %v", err)
    }

    if len(output) != 1 {
        t.Fatalf("expected 1 output event, got %d", len(output))
    }

    if output[0].Type != "myorg.processed.v1" {
        t.Errorf("expected type myorg.processed.v1, got %s", output[0].Type)
    }
}
```

### Integration Testing

Test component with flowctl:

```bash
# 1. Build your component
go build -o bin/my-component main.go

# 2. Create test pipeline
cat > test-pipeline.yaml <<EOF
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: test-pipeline

spec:
  driver: process

  sources:
    - id: mock-source
      command: ["sh", "-c", "echo '{\"test\":\"data\"}'"]

  processors:
    - id: my-component
      command: ["$(pwd)/bin/my-component"]
      inputs: ["mock-source"]

  sinks:
    - id: stdout-sink
      command: ["sh", "-c", "cat"]
      inputs: ["my-component"]
EOF

# 3. Run test
./bin/flowctl run test-pipeline.yaml
```

### Manual Testing

Test component standalone:

```bash
# 1. Build component
go build -o bin/my-component main.go

# 2. Run with environment variables
ENABLE_FLOWCTL=false \
PORT=:50051 \
HEALTH_PORT=8088 \
./bin/my-component

# 3. In another terminal, test health endpoint
curl http://localhost:8088/health
```

## Deployment

### Building Binaries

```bash
# Build for current platform
go build -o bin/my-component main.go

# Build for Linux (common deployment target)
GOOS=linux GOARCH=amd64 go build -o bin/my-component-linux-amd64 main.go

# Build for multiple platforms
make build-all
```

### Using in Pipelines

#### Local Execution (Process Driver)

```yaml
spec:
  driver: process

  sources:
    - id: my-source
      command: ["/absolute/path/to/bin/my-source"]
      env:
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
```

**Requirements:**
- Binary must exist at specified path
- Binary must be executable (`chmod +x`)
- Use absolute paths

#### Docker Execution

```yaml
spec:
  driver: docker

  sources:
    - id: my-source
      image: "ghcr.io/myorg/my-source:v1.0.0"
      command: ["/app/my-source"]
      env:
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "control-plane:8080"
```

**Requirements:**
- Container image must be built and pushed
- Binary must exist in container at specified path

**Build container:**

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o bin/my-source main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/my-source .
ENTRYPOINT ["/app/my-source"]
```

```bash
docker build -t ghcr.io/myorg/my-source:v1.0.0 .
docker push ghcr.io/myorg/my-source:v1.0.0
```

### Best Practices

#### 1. Configuration via Environment Variables

```go
// ✅ Good: Use environment variables
apiEndpoint := os.Getenv("API_ENDPOINT")

// ❌ Bad: Hardcode configuration
apiEndpoint := "https://api.example.com"
```

#### 2. Graceful Shutdown

The SDK handles this automatically, but respect context cancellation:

```go
func produceData(ctx context.Context) ([]*flowpb.Event, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
        // Continue processing
    }
}
```

#### 3. Error Handling

```go
// ✅ Good: Return errors
if err != nil {
    return nil, fmt.Errorf("processing failed: %w", err)
}

// ❌ Bad: Log and continue
if err != nil {
    log.Printf("error: %v", err)
    // Continue anyway
}
```

#### 4. Logging

```go
// ✅ Good: Use structured logging
log.Printf("Processed event: id=%s type=%s", event.Id, event.Type)

// ❌ Bad: Unstructured logs
log.Printf("Processed: %v", event)
```

#### 5. Health Checks

The SDK provides automatic health checks. For custom health logic:

```go
// SDK automatically provides /health endpoint
// Returns healthy when component is running and registered
```

## Complete Example Projects

### Study These Examples

1. **Stellar Live Source**
   - Location: `flowctl-sdk/examples/stellar-live-source/`
   - Shows: Complex source with RPC backend, configuration, health checks

2. **Contract Events Processor**
   - Location: `flowctl-sdk/examples/contract-events-processor/`
   - Shows: Event extraction, data transformation, error handling

3. **PostgreSQL Consumer**
   - Location: `flowctl-sdk/examples/postgresql-consumer/`
   - Shows: Database connection, batch inserts, transaction management

4. **Complete Pipeline**
   - Location: `flowctl-sdk/examples/contract-events-pipeline/`
   - Shows: All components working together in production configuration

## Troubleshooting

### Component Won't Start

```bash
# Check binary exists and is executable
ls -la /path/to/component
chmod +x /path/to/component

# Check for port conflicts
lsof -i :50051

# Run with debug logging
LOG_LEVEL=debug ./bin/my-component
```

### Component Not Registering

```bash
# Verify ENABLE_FLOWCTL is set
echo $ENABLE_FLOWCTL  # Should be "true"

# Check control plane is running
# Look for: "Starting control plane on 127.0.0.1:8080"

# Verify FLOWCTL_ENDPOINT is correct
echo $FLOWCTL_ENDPOINT  # Should be "127.0.0.1:8080"
```

### Data Not Flowing

```bash
# Check inputs are correct in YAML
inputs: ["correct-upstream-id"]

# Verify event types match
# Source OutputType must match Processor InputType

# Check logs for processing errors
./bin/flowctl run pipeline.yaml 2>&1 | grep ERROR
```

## Next Steps

1. **Clone the SDK**: `git clone https://github.com/withObsrvr/flowctl-sdk`
2. **Study examples**: `cd flowctl-sdk/examples/`
3. **Run the demo**: `cd contract-events-pipeline && ./demo.sh`
4. **Build your component**: Start with a simple source or sink
5. **Test locally**: Use `flowctl run` with your pipeline config
6. **Deploy**: Use Docker or Kubernetes for production

## Additional Resources

- **flowctl-sdk Repository**: https://github.com/withObsrvr/flowctl-sdk
- **Configuration Guide**: `configuration.md`
- **Getting Started**: `../examples/getting-started/README.md`
- **Real-world Demo**: https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline

## Questions?

- **GitHub Issues**: https://github.com/withobsrvr/flowctl/issues
- **SDK Issues**: https://github.com/withObsrvr/flowctl-sdk/issues

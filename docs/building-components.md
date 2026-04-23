# Building Components for flowctl

This guide covers building custom sources, processors, and sinks using the flowctl-sdk.

If you are specifically building **processors**, read this first:

- [`docs/BUILDING_PROCESSORS.md`](./BUILDING_PROCESSORS.md) — focused guide for processor authors
- [`docs/REFERENCE_PROCESSORS.md`](./REFERENCE_PROCESSORS.md) — the initial reference processor set for the flagship cycle

Those documents make the intended path explicit:

```text
Prototype in nebu
  ↓
stabilize schema and behavior
  ↓
promote to a production-ready flowctl processor
```

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

## Getting Started: Choose Your Path

### New to Stellar Processing?

Start with **nebu** for rapid prototyping, then graduate to flowctl for production:

1. **Learn proto-first development**: [BUILDING_PROTO_PROCESSORS.md](https://github.com/withObsrvr/nebu-processor-registry/blob/main/BUILDING_PROTO_PROCESSORS.md)
2. **Build and test** your processor with nebu's Unix pipe model
3. **Graduate to flowctl**: [GRADUATING_TO_FLOWCTL.md](https://github.com/withObsrvr/nebu-processor-registry/blob/main/docs/GRADUATING_TO_FLOWCTL.md)

### Ready for Production?

Jump straight to the **flowctl-sdk quickstart**:

- **5-minute quickstart**: [flowctl-sdk/docs/QUICKSTART.md](https://github.com/withObsrvr/flowctl-sdk/blob/main/docs/QUICKSTART.md)
- **Complete examples**: [flowctl-sdk/examples/](https://github.com/withObsrvr/flowctl-sdk/tree/main/examples)

### Terminology

| Role | Description | SDK Package |
|------|-------------|-------------|
| **Source** | Produces events (data producer) | `flowctl-sdk/pkg/source` |
| **Processor** | Transforms events | `flowctl-sdk/pkg/processor` |
| **Sink** | Consumes events (data consumer) | `flowctl-sdk/pkg/consumer` |

The core API for Stellar processors is `EventsFromLedger()`, matching Stellar's official processor pattern.

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

## Proto-First Development

**IMPORTANT:** Always use the **proto-first** approach when building flowctl components. This ensures:

- **Type safety** via protobuf schemas
- **Consistency** across all components in the ecosystem
- **Automatic serialization** with efficient binary encoding
- **Language interoperability** if needed in the future

### The flow-proto Repository

All shared protobuf definitions live in the [flow-proto](https://github.com/withObsrvr/flow-proto) repository:

```bash
# Clone flow-proto for reference
git clone https://github.com/withObsrvr/flow-proto.git
cd flow-proto

# Key proto files:
# proto/flowctl/v1/event.proto     - Core event types
# proto/stellar/v1/ledger.proto    - Stellar ledger data
# proto/stellar/v1/contract_events.proto - Contract events
```

### When to Use Existing vs Custom Protos

| Scenario | Recommendation |
|----------|----------------|
| Processing Stellar data | Use `flow-proto/stellar/v1/*` types |
| Generic event handling | Use `flow-proto/flowctl/v1/Event` |
| Custom domain events | Define new proto, submit PR to flow-proto |
| Internal transformations | Can use JSON, but proto preferred |

## Working with Protobuf Messages

### Importing flow-proto Types

Your `go.mod` should include flow-proto:

```go
require (
    github.com/withObsrvr/flow-proto v0.0.0
    github.com/withObsrvr/flowctl-sdk v0.0.0
    google.golang.org/protobuf v1.36.0
)

// For local development, use replace directives:
replace github.com/withObsrvr/flow-proto => /path/to/flow-proto
replace github.com/withObsrvr/flowctl-sdk => /path/to/flowctl-sdk
```

### Parsing Event Payloads

Events arrive as `*flowctlv1.Event` with a `Payload` field containing serialized protobuf:

```go
import (
    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
    stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
    "google.golang.org/protobuf/proto"
)

func handleEvent(ctx context.Context, event *flowctlv1.Event) error {
    switch event.Type {
    case "stellar.contract.events.v1":
        var batch stellarv1.ContractEventBatch
        if err := proto.Unmarshal(event.Payload, &batch); err != nil {
            return fmt.Errorf("failed to parse ContractEventBatch: %w", err)
        }

        for _, ce := range batch.Events {
            // Process each contract event
            log.Printf("Contract: %s, Type: %s", ce.ContractId, ce.EventType)
        }

    case "stellar.raw.ledger.v1":
        var ledger stellarv1.RawLedger
        if err := proto.Unmarshal(event.Payload, &ledger); err != nil {
            return fmt.Errorf("failed to parse RawLedger: %w", err)
        }

        log.Printf("Ledger: %d, Network: %s", ledger.Sequence, ledger.Network)
    }

    return nil
}
```

### Available Stellar Event Types

| Event Type | Proto Message | Description |
|------------|---------------|-------------|
| `stellar.raw.ledger.v1` | `stellarv1.RawLedger` | Raw ledger with XDR data |
| `stellar.ledger.v1` | `stellarv1.RawLedger` | Alias for raw ledger |
| `stellar.contract.events.v1` | `stellarv1.ContractEventBatch` | Batch of contract events |

### Thread-Safe Metrics with Atomic Counters

For components that need to track metrics across concurrent goroutines:

```go
import "sync/atomic"

var eventsProcessed atomic.Int64
var contractEventsProcessed atomic.Int64

func handleEvent(ctx context.Context, event *flowctlv1.Event) error {
    eventsProcessed.Add(1)

    // ... process event ...

    // Log progress periodically
    count := eventsProcessed.Load()
    if count%1000 == 0 {
        log.Printf("Processed %d events", count)
    }

    return nil
}
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

### Basic Sink Structure (Using consumer.Run())

The flowctl-sdk provides a zero-config `consumer.Run()` function that handles all the gRPC setup, control plane registration, health checks, and event loop:

```go
package main

import (
    "context"
    "log"
    "os"
    "sync/atomic"

    "github.com/withObsrvr/flowctl-sdk/pkg/consumer"
    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

// Thread-safe counters for metrics
var eventsProcessed atomic.Int64

func main() {
    componentID := os.Getenv("FLOWCTL_COMPONENT_ID")
    if componentID == "" {
        componentID = "my-sink"
    }

    // Set default port (avoid conflicts with other components)
    if os.Getenv("PORT") == "" {
        os.Setenv("PORT", ":50053")
    }

    log.Printf("Starting My Sink (componentID: %s)", componentID)

    // consumer.Run() handles everything:
    // - gRPC server setup
    // - Control plane registration
    // - Health checks
    // - Graceful shutdown
    consumer.Run(consumer.ConsumerConfig{
        ConsumerName: "My Sink",
        InputType:    "myorg.event.v1",  // Event type to subscribe to
        OnEvent:      handleEvent,        // Your event handler
        ComponentID:  componentID,
    })

    // Only reached on shutdown
    log.Printf("Total events processed: %d", eventsProcessed.Load())
}

// handleEvent processes each incoming event
func handleEvent(ctx context.Context, event *flowctlv1.Event) error {
    eventsProcessed.Add(1)

    // Your business logic here
    log.Printf("Received event: %s (type: %s)", event.Id, event.Type)

    return nil
}
```

### Real Example: PostgreSQL Sink with Protobuf

This example shows a production-ready PostgreSQL sink that:
- Uses `consumer.Run()` for zero-config setup
- Parses protobuf events using `proto.Unmarshal()`
- Uses JSONB for flexible querying
- Implements batch inserts with transactions
- Tracks metrics with atomic counters

```go
package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "sync/atomic"

    _ "github.com/lib/pq"
    "github.com/withObsrvr/flowctl-sdk/pkg/consumer"
    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
    stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
    "google.golang.org/protobuf/proto"
)

// Global sink and metrics
var sink *PostgreSQLSink
var eventsProcessed atomic.Int64
var contractEventsProcessed atomic.Int64

type PostgreSQLSink struct {
    db *sql.DB
}

func main() {
    // Configuration from environment
    pgHost := getEnv("POSTGRES_HOST", "localhost")
    pgPort := getEnv("POSTGRES_PORT", "5432")
    pgDB := getEnv("POSTGRES_DB", "stellar_events")
    pgUser := getEnv("POSTGRES_USER", "postgres")
    pgPassword := getEnv("POSTGRES_PASSWORD", "")
    componentID := getEnv("FLOWCTL_COMPONENT_ID", "postgres-consumer")

    // Build connection string
    connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
        pgHost, pgPort, pgDB, pgUser, pgPassword)

    // Set default port
    if os.Getenv("PORT") == "" {
        os.Setenv("PORT", ":50053")
    }

    // Initialize PostgreSQL
    var err error
    sink, err = NewPostgreSQLSink(connStr)
    if err != nil {
        log.Fatalf("Failed to initialize PostgreSQL: %v", err)
    }
    defer sink.Close()

    // Run consumer (blocks until shutdown)
    consumer.Run(consumer.ConsumerConfig{
        ConsumerName: "PostgreSQL Consumer",
        InputType:    "stellar.contract.events.v1",
        OnEvent:      handleEvent,
        ComponentID:  componentID,
    })

    log.Printf("Contract events stored: %d", contractEventsProcessed.Load())
}

func handleEvent(ctx context.Context, event *flowctlv1.Event) error {
    eventsProcessed.Add(1)

    switch event.Type {
    case "stellar.contract.events.v1":
        // Parse protobuf payload
        var batch stellarv1.ContractEventBatch
        if err := proto.Unmarshal(event.Payload, &batch); err != nil {
            return fmt.Errorf("failed to parse ContractEventBatch: %w", err)
        }

        // Batch insert with transaction
        tx, err := sink.db.Begin()
        if err != nil {
            return err
        }

        for _, ce := range batch.Events {
            if err := insertContractEvent(tx, ce); err != nil {
                tx.Rollback()
                return err
            }
        }

        if err := tx.Commit(); err != nil {
            return err
        }

        contractEventsProcessed.Add(int64(len(batch.Events)))
    }

    return nil
}

func insertContractEvent(tx *sql.Tx, ce *stellarv1.ContractEvent) error {
    // Extract topics as JSONB
    topicsJSON, _ := json.Marshal(ce.Topics)

    _, err := tx.Exec(`
        INSERT INTO contract_events (id, ledger_sequence, contract_id, event_type, topics, data)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (id) DO UPDATE SET topics = EXCLUDED.topics, data = EXCLUDED.data
    `,
        fmt.Sprintf("ce-%d-%s-%d", ce.Meta.LedgerSequence, ce.ContractId, ce.EventIndex),
        ce.Meta.LedgerSequence,
        ce.ContractId,
        ce.EventType,
        string(topicsJSON),
        ce.Data.Json,
    )
    return err
}

// NewPostgreSQLSink, Close, getEnv implementations...
// See full example: https://github.com/withObsrvr/ttp-processor-demo/postgres-consumer
```

**Reference Implementation:** See [postgres-consumer](https://github.com/withObsrvr/ttp-processor-demo/tree/main/postgres-consumer) for the complete source code.

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

3. **PostgreSQL Consumer** (Recommended Reference)
   - Location: `ttp-processor-demo/postgres-consumer/`
   - Shows: Proto-first development, `consumer.Run()` API, JSONB storage, atomic counters
   - Key patterns:
     - Using `proto.Unmarshal()` to parse event payloads
     - Transaction-based batch inserts
     - GIN indexes for JSONB queries
     - Thread-safe metrics with `sync/atomic`

4. **DuckDB Consumer**
   - Location: `ttp-processor-demo/duckdb-consumer/`
   - Shows: Embedded database, similar patterns to PostgreSQL consumer

5. **Complete Pipeline**
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

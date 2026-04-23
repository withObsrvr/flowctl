# Getting Started with flowctl

Welcome! This guide will help you understand flowctl and build your first pipeline.

## Table of Contents

- [What is flowctl?](#what-is-flowctl)
- [Installation](#installation)
- [Core Concepts](#core-concepts)
- [Your First Pipeline](#your-first-pipeline)
- [Next Steps](#next-steps)

## What is flowctl?

**flowctl is a pipeline orchestrator** - think of it as a conductor coordinating an orchestra of data processing components.

### What flowctl Does

✅ **Starts and manages** your components (sources, processors, sinks)
✅ **Routes data** between components via gRPC streams
✅ **Monitors health** of all components with heartbeats
✅ **Provides observability** with metrics and structured logging
✅ **Handles failures** with automatic restarts and error handling

### What flowctl Doesn't Do

❌ **Process data itself** - Components do the actual work
❌ **Replace your data tools** - It coordinates them
❌ **Store data** - Components write to your chosen storage

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    flowctl (Orchestrator)                │
│  • Control Plane (gRPC API)                             │
│  • Component Registry                                    │
│  • Health Monitoring                                     │
│  • Stream Management                                     │
└─────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Source     │───▶│  Processor   │───▶│    Sink      │
│              │    │              │    │              │
│ (Produces    │    │ (Transforms  │    │ (Consumes    │
│  data)       │    │  data)       │    │  data)       │
└──────────────┘    └──────────────┘    └──────────────┘
```

## Installation

### Prerequisites

- **Go 1.21+** (for building flowctl and components)
- **Git** (for cloning repositories)
- Optional: **Docker** (for containerized deployments)

### Install flowctl

```bash
# Clone the repository
git clone https://github.com/withobsrvr/flowctl.git
cd flowctl

# Build flowctl
make build

# Verify installation
./bin/flowctl version
```

### Install flowctl-sdk (for building components)

```bash
# Clone the SDK repository
git clone https://github.com/withObsrvr/flowctl-sdk.git
cd flowctl-sdk

# The SDK is a Go module - you'll use it as a dependency
# See "Building Components" section below
```

## Core Concepts

### Components

Components are **separate programs** that do the actual data processing:

#### Sources (Data Producers)
- Produce data for the pipeline
- Examples: Stellar ledger streams, Kafka consumers, API pollers
- No inputs, only outputs
- Built using `flowctl-sdk/pkg/source`

#### Processors (Data Transformers)
- Transform data as it flows through
- Examples: Event extractors, filters, aggregators
- Receive input from sources or other processors
- Built using `flowctl-sdk/pkg/processor`

#### Sinks (Data Consumers)
- Consume data (terminal nodes)
- Examples: PostgreSQL, webhooks, file writers
- Receive input from sources or processors
- Built using `flowctl-sdk/pkg/consumer` (legacy) or `flowctl-sdk/pkg/sink`

### Pipeline Configuration

Pipelines are defined in **YAML files** using Kubernetes-style syntax:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: my-pipeline

spec:
  driver: process  # Where to run: process, docker, kubernetes, nomad

  sources:
    - id: my-source
      command: ["/path/to/source-binary"]

  processors:
    - id: my-processor
      command: ["/path/to/processor-binary"]
      inputs: ["my-source"]

  sinks:
    - id: my-sink
      command: ["/path/to/sink-binary"]
      inputs: ["my-processor"]
```

### Control Plane

When you run `flowctl run pipeline.yaml`, flowctl starts an **embedded control plane**:

- **Component Registry**: Components register themselves on startup
- **Health Monitoring**: Regular heartbeat checks
- **Stream Orchestration**: Routes data between components
- **Metrics Collection**: Exposes Prometheus metrics

Components connect to the control plane automatically when `ENABLE_FLOWCTL=true`.

## Your First Pipeline

Let's walk through understanding a simple pipeline. We'll use the generated starter pipeline shape shown below.

### Step 1: Examine the Configuration

```bash
flowctl init --preset testnet-duckdb -o /tmp/stellar-pipeline.yaml
cat /tmp/stellar-pipeline.yaml
```

You'll see:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: stellar-pipeline
  description: Process stellar contract events on testnet

spec:
  driver: process

  sources:
    - id: stellar-source
      type: stellar-live-source@v1.0.0
      config:
        network_passphrase: "Test SDF Network ; September 2015"
        backend_type: RPC
        rpc_endpoint: https://soroban-testnet.stellar.org

  processors:
    - id: contract-events
      type: contract-events-processor@v1.0.0
      config:
        network_passphrase: "Test SDF Network ; September 2015"
      inputs: ["stellar-source"]

  sinks:
    - id: duckdb-sink
      type: duckdb-consumer@v1.0.0
      config:
        database_path: ./stellar-pipeline.duckdb
      inputs: ["contract-events"]
```

**Understanding the flow:**
1. `stellar-source` streams Stellar ledgers from RPC
2. `contract-events` extracts Soroban contract events
3. `duckdb-sink` stores the extracted events in DuckDB

### Step 2: Run the Pipeline

```bash
./bin/flowctl run /tmp/stellar-pipeline.yaml
```

**What happens:**
1. flowctl starts the embedded control plane
2. Starts each component as a separate process
3. Routes data: source → processor → sink
4. Monitors component health
5. Logs all activity

You'll see output like component startup logs followed by contract event extraction and sink writes.

### Step 3: Stop the Pipeline

Press `Ctrl+C` to stop all components gracefully.

### Step 4: Understand the Components

This starter pipeline uses real downloadable components, so it exercises the normal operator path:
- components register with the control plane
- health and status commands work
- data is written to a real DuckDB file

## Building Real Components

To build production-ready components, you'll use the **flowctl-sdk**.

### Quick Start: Clone the Demo

The fastest way to learn is to examine a working example:

```bash
# Clone the SDK and run the demo
git clone https://github.com/withObsrvr/flowctl-sdk.git
cd flowctl-sdk/examples/contract-events-pipeline
./demo.sh
```

This demonstrates:
- **Stellar Live Source**: Streams Stellar ledger data
- **Contract Events Processor**: Extracts contract events
- **PostgreSQL Consumer**: Stores events in PostgreSQL

### Component Structure

All components follow this pattern:

```go
package main

import (
    "github.com/withObsrvr/flowctl-sdk/pkg/processor"
)

func main() {
    // Create processor with configuration
    proc := processor.New(processor.Config{
        Name:        "my-processor",
        Description: "Transforms data",
        Version:     "1.0.0",
        InputType:   "stellar.ledger.v1",
        OutputType:  "stellar.events.v1",
    })

    // Set processing function
    proc.SetProcessFunc(func(ctx context.Context, event *flowpb.Event) ([]*flowpb.Event, error) {
        // Transform event data here
        return transformedEvents, nil
    })

    // Start the processor
    if err := proc.Run(); err != nil {
        log.Fatal(err)
    }
}
```

**Key features provided by SDK:**
- ✅ Automatic control plane registration
- ✅ Health check endpoint
- ✅ Heartbeat management
- ✅ gRPC server setup
- ✅ Graceful shutdown
- ✅ Structured logging
- ✅ Metrics collection

### Building Your Component

```bash
# 1. Create your component
mkdir my-component
cd my-component
go mod init github.com/yourorg/my-component

# 2. Add flowctl-sdk dependency
go get github.com/withObsrvr/flowctl-sdk

# 3. Write your component (see examples/)

# 4. Build the binary
go build -o bin/my-component main.go

# 5. Use in pipeline
# Update pipeline YAML with: command: ["/path/to/bin/my-component"]
```

See [Building Components Guide](../../docs/building-components.md) for detailed instructions.

## Debugging Your Pipeline

### Common Issues

#### 1. Component Not Starting

```bash
# Check if binary exists and is executable
ls -la /path/to/component
chmod +x /path/to/component

# Check component logs
./bin/flowctl run pipeline.yaml 2>&1 | grep "component-id"
```

#### 2. Components Not Connecting

```bash
# Verify control plane is running
# Look for: "Starting control plane on 127.0.0.1:8080"

# Check component registration
# Look for: "Component registered: component-id"
```

#### 3. No Data Flowing

```bash
# Check inputs are correct
# Verify: inputs: ["upstream-component-id"]

# Check component is producing output
# Add debug logging to your component
```

### Debug Mode

Run flowctl with verbose logging:

```bash
./bin/flowctl run pipeline.yaml --log-level=debug
```

### Health Checks

Check component health while pipeline is running:

```bash
# If component has HEALTH_PORT=8088
curl http://localhost:8088/health
```

## Pipeline Patterns

### Simple Linear Pipeline

```
Source → Processor → Sink
```

**Use case:** Basic ETL (Extract, Transform, Load)

```yaml
spec:
  sources:
    - id: source
      command: ["./bin/source"]

  processors:
    - id: processor
      command: ["./bin/processor"]
      inputs: ["source"]

  sinks:
    - id: sink
      command: ["./bin/sink"]
      inputs: ["processor"]
```

### Multiple Sinks (Fan-Out)

```
Source → Processor → ┬→ PostgreSQL
                     ├→ Webhook
                     └→ File Storage
```

**Use case:** Send data to multiple destinations

```yaml
spec:
  sources:
    - id: source
      command: ["./bin/source"]

  processors:
    - id: processor
      command: ["./bin/processor"]
      inputs: ["source"]

  sinks:
    - id: postgres-sink
      command: ["./bin/postgres-sink"]
      inputs: ["processor"]

    - id: webhook-sink
      command: ["./bin/webhook-sink"]
      inputs: ["processor"]

    - id: file-sink
      command: ["./bin/file-sink"]
      inputs: ["processor"]
```

### Multiple Sources (Fan-In)

```
Source A ┬
Source B ┴→ Processor → Sink
```

**Use case:** Combine data from multiple sources

```yaml
spec:
  sources:
    - id: source-a
      command: ["./bin/source-a"]

    - id: source-b
      command: ["./bin/source-b"]

  processors:
    - id: merger
      command: ["./bin/merger"]
      inputs: ["source-a", "source-b"]

  sinks:
    - id: sink
      command: ["./bin/sink"]
      inputs: ["merger"]
```

### Processing Chain

```
Source → Filter → Transform → Aggregate → Sink
```

**Use case:** Multi-stage data transformation

```yaml
spec:
  sources:
    - id: source
      command: ["./bin/source"]

  processors:
    - id: filter
      command: ["./bin/filter"]
      inputs: ["source"]

    - id: transform
      command: ["./bin/transform"]
      inputs: ["filter"]

    - id: aggregate
      command: ["./bin/aggregate"]
      inputs: ["transform"]

  sinks:
    - id: sink
      command: ["./bin/sink"]
      inputs: ["aggregate"]
```

## Configuration Tips

### Environment Variables

Components are configured via environment variables:

```yaml
sources:
  - id: my-source
    command: ["./bin/my-source"]
    env:
      # flowctl integration
      ENABLE_FLOWCTL: "true"
      FLOWCTL_ENDPOINT: "127.0.0.1:8080"

      # Component ports
      PORT: ":50051"
      HEALTH_PORT: "8088"

      # Component-specific config
      POLL_INTERVAL: "5s"
      API_ENDPOINT: "https://api.example.com"
```

### Absolute Paths

When using `driver: process`, always use **absolute paths**:

```yaml
# ✅ Good
command: ["/home/user/flowctl-sdk/examples/stellar-live-source/bin/stellar-live-source"]

# ❌ Bad - Relative paths may fail
command: ["./bin/stellar-live-source"]
```

### Port Configuration

Each component needs unique ports:

```yaml
sources:
  - id: source-1
    env:
      PORT: ":50051"
      HEALTH_PORT: "8088"

processors:
  - id: processor-1
    env:
      PORT: ":50052"
      HEALTH_PORT: "8089"

sinks:
  - id: sink-1
    env:
      PORT: ":50053"
      HEALTH_PORT: "8090"
```

## Next Steps

### Learn More

1. **Real-World Example**: Study the complete contract events pipeline
   ```bash
   git clone https://github.com/withObsrvr/flowctl-sdk.git
   cd flowctl-sdk/examples/contract-events-pipeline
   cat README.md
   ./demo.sh
   ```

2. **Build Your First Component**: Follow the [Building Components Guide](../../docs/building-components.md)

3. **Explore Configuration**: Read the [Configuration Guide](../../docs/configuration.md)

4. **Advanced Patterns**: Check out `examples/dag-pipeline.yaml` for complex topologies

### Example Repositories

- **flowctl-sdk**: https://github.com/withObsrvr/flowctl-sdk
  - Source: `pkg/source/`
  - Processor: `pkg/processor/`
  - Sink: `pkg/consumer/` and `pkg/sink/`
  - Complete examples: `examples/`

- **Real-world demo**: https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline

### Community

- **GitHub Issues**: https://github.com/withobsrvr/flowctl/issues
- **Documentation**: https://github.com/withobsrvr/flowctl

## Quick Reference

### flowctl Commands

```bash
# Run a pipeline
./bin/flowctl run pipeline.yaml

# Validate without running
./bin/flowctl validate pipeline.yaml

# Translate image-based pipelines to deployment format
# (the default `flowctl init` starter pipelines are meant to run directly with `flowctl run`)
./bin/flowctl translate -f image-based-pipeline.yaml -o docker-compose

# Check version
./bin/flowctl version
```

### Component SDK Packages

| Package | Purpose | Use For |
|---------|---------|---------|
| `pkg/source` | Data producers | Reading from APIs, databases, streams |
| `pkg/processor` | Data transformers | Filtering, enriching, aggregating |
| `pkg/consumer` | Data consumers (legacy) | Writing to databases, webhooks |
| `pkg/sink` | Data consumers (new) | Modern sink implementation |

### Configuration Keys

| Key | Required | Values | Description |
|-----|----------|--------|-------------|
| `apiVersion` | Yes | `flowctl/v1` | Config schema version |
| `kind` | Yes | `Pipeline` | Resource type |
| `metadata.name` | Yes | string | Pipeline identifier |
| `spec.driver` | Yes | `process`, `docker`, `kubernetes`, `nomad` | Where components run |
| `sources` | Yes | array | Data producers |
| `processors` | No | array | Data transformers |
| `sinks` | Yes | array | Data consumers |

### Environment Variables (Components)

| Variable | Required | Example | Description |
|----------|----------|---------|-------------|
| `ENABLE_FLOWCTL` | Recommended | `"true"` | Enable control plane integration |
| `FLOWCTL_ENDPOINT` | If ENABLE_FLOWCTL=true | `"127.0.0.1:8080"` | Control plane address |
| `PORT` | Yes | `":50051"` | gRPC server port |
| `HEALTH_PORT` | Recommended | `"8088"` | HTTP health check port |

## Troubleshooting

### "Component not found"

```bash
# Check binary path
ls -la /path/to/component

# Use absolute path in YAML
command: ["/absolute/path/to/component"]
```

### "Address already in use"

```bash
# Check for port conflicts
lsof -i :50051

# Use unique ports for each component
# Change PORT and HEALTH_PORT in env
```

### "No data received"

```bash
# Verify inputs are correct
inputs: ["correct-component-id"]

# Check component logs
./bin/flowctl run pipeline.yaml 2>&1 | grep ERROR

# Enable debug logging
./bin/flowctl run pipeline.yaml --log-level=debug
```

### "Control plane connection failed"

```bash
# Check FLOWCTL_ENDPOINT is correct
# Default: 127.0.0.1:8080

# Verify ENABLE_FLOWCTL is set
env:
  ENABLE_FLOWCTL: "true"
  FLOWCTL_ENDPOINT: "127.0.0.1:8080"
```

For more troubleshooting help, see the [Troubleshooting Guide](../../README.md#troubleshooting).

---

**Ready to build?** Start with the [Building Components Guide](../../docs/building-components.md)!

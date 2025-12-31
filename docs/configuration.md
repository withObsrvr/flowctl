# flowctl Configuration Guide

This guide covers the complete configuration schema for flowctl pipelines.

## Table of Contents

- [Configuration Formats](#configuration-formats)
- [Standard Pipeline Format](#standard-pipeline-format)
- [Quickstart Pipeline Format](#quickstart-pipeline-format)
- [Component Configuration](#component-configuration)
- [Driver Types](#driver-types)
- [Complete Examples](#complete-examples)

## Configuration Formats

flowctl supports two distinct configuration formats:

### 1. Standard Pipeline Format

**Use for:** Custom pipelines with full control over components

```yaml
apiVersion: flowctl/v1
kind: Pipeline
```

- Full flexibility to define sources, processors, and sinks
- Requires building component binaries (use flowctl-sdk)
- Run with: `flowctl run pipeline.yaml`

### 2. Quickstart Pipeline Format

**Use for:** Asset Balance Indexer (opinionated, pre-configured)

```yaml
apiVersion: flowctl.io/v1alpha1
kind: QuickstartPipeline
```

- Specialized for Stellar asset balance indexing
- Built-in components (no binary building required)
- Run with: `flowctl quickstart run pipeline.yaml`
- See: `examples/quickstart/` for examples

## Standard Pipeline Format

### Basic Structure

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: my-pipeline
  description: Optional description of what this pipeline does

spec:
  driver: process  # Where to run components: process, docker, kubernetes, nomad

  sources:
    - id: source-1
      command: ["/path/to/source-binary"]
      env:
        KEY: value

  processors:
    - id: processor-1
      command: ["/path/to/processor-binary"]
      inputs: ["source-1"]
      env:
        KEY: value

  sinks:
    - id: sink-1
      command: ["/path/to/sink-binary"]
      inputs: ["processor-1"]
      env:
        KEY: value
```

### Metadata Section

```yaml
metadata:
  name: pipeline-name              # Required: Unique pipeline identifier
  description: Human description   # Optional: What this pipeline does
  namespace: default               # Optional: Namespace for organization
  annotations:                     # Optional: Custom key-value metadata
    team: data-engineering
    environment: production
```

### Spec Section

#### Required Fields

```yaml
spec:
  driver: process  # Required: Execution environment
```

#### Driver Types

| Driver | Description | Use Case |
|--------|-------------|----------|
| `process` | Local process execution | Development, single-machine deployments |
| `docker` | Docker containers | Isolated environments, easy distribution |
| `kubernetes` | Kubernetes pods | Production clusters, auto-scaling |
| `nomad` | HashiCorp Nomad | Multi-region deployments, edge computing |

### Component Configuration

All components (sources, processors, sinks) share a common structure:

```yaml
id: component-identifier        # Required: Unique component ID
command: ["/path/to/binary"]    # Required: Path to executable
inputs: ["upstream-id"]         # Optional: Connect to upstream components
env:                            # Optional: Environment variables
  KEY: "value"
  ANOTHER_KEY: "another_value"
```

#### Sources

**Purpose:** Produce data for the pipeline

```yaml
sources:
  - id: stellar-source
    command: ["/home/user/bin/stellar-live-source"]
    env:
      BACKEND_TYPE: "RPC"
      RPC_ENDPOINT: "https://soroban-testnet.stellar.org:443"
      NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"
      START_LEDGER: "1000000"
      END_LEDGER: "1001000"
      PORT: "localhost:50052"
      HEALTH_PORT: "8090"
      ENABLE_FLOWCTL: "true"
      FLOWCTL_ENDPOINT: "127.0.0.1:8080"
```

**Key Points:**
- No `inputs` field (sources are data producers)
- Must implement source gRPC interface
- Should register with flowctl control plane

#### Processors

**Purpose:** Transform data flowing through the pipeline

```yaml
processors:
  - id: contract-events-processor
    command: ["/home/user/bin/contract-events-processor"]
    inputs: ["stellar-source"]  # Connect to source output
    env:
      NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"
      PORT: ":50051"
      HEALTH_PORT: "8089"
      ENABLE_FLOWCTL: "true"
      FLOWCTL_ENDPOINT: "127.0.0.1:8080"
```

**Key Points:**
- Must specify `inputs` to connect to upstream components
- Can have multiple inputs (fan-in pattern)
- Multiple processors can read from same source (fan-out pattern)

#### Sinks

**Purpose:** Consume data (terminal nodes)

```yaml
sinks:
  - id: postgresql-consumer
    command: ["/home/user/bin/postgresql-consumer"]
    inputs: ["contract-events-processor"]  # Connect to processor output
    env:
      POSTGRES_HOST: "localhost"
      POSTGRES_PORT: "5432"
      POSTGRES_DB: "stellar_events"
      POSTGRES_USER: "postgres"
      POSTGRES_PASSWORD: "password"
      POSTGRES_SSLMODE: "disable"
      PORT: ":9090"
      HEALTH_PORT: "9088"
      ENABLE_FLOWCTL: "true"
      FLOWCTL_ENDPOINT: "127.0.0.1:8080"
```

**Key Points:**
- Must specify `inputs` to connect to upstream components
- Terminal nodes (no other components read from sinks)
- Can have multiple sinks reading from same processor

### Pipeline Topologies

#### Linear Pipeline (Simple)

```
Source → Processor → Sink
```

```yaml
spec:
  sources:
    - id: source-1
      command: ["..."]

  processors:
    - id: processor-1
      inputs: ["source-1"]
      command: ["..."]

  sinks:
    - id: sink-1
      inputs: ["processor-1"]
      command: ["..."]
```

#### Fan-Out Pattern

```
Source → Processor ─┬→ Sink 1
                    └→ Sink 2
```

```yaml
spec:
  sources:
    - id: source-1
      command: ["..."]

  processors:
    - id: processor-1
      inputs: ["source-1"]
      command: ["..."]

  sinks:
    - id: sink-1
      inputs: ["processor-1"]
      command: ["..."]

    - id: sink-2
      inputs: ["processor-1"]  # Same input as sink-1
      command: ["..."]
```

#### Fan-In Pattern

```
Source 1 ─┬
Source 2 ─┴→ Processor → Sink
```

```yaml
spec:
  sources:
    - id: source-1
      command: ["..."]

    - id: source-2
      command: ["..."]

  processors:
    - id: processor-1
      inputs: ["source-1", "source-2"]  # Multiple inputs
      command: ["..."]

  sinks:
    - id: sink-1
      inputs: ["processor-1"]
      command: ["..."]
```

#### Complex DAG (Diamond Pattern)

```
       Source
      ↙      ↘
Proc-1      Proc-2
      ↘      ↙
        Sink
```

```yaml
spec:
  sources:
    - id: source-1
      command: ["..."]

  processors:
    - id: processor-1
      inputs: ["source-1"]
      command: ["..."]

    - id: processor-2
      inputs: ["source-1"]
      command: ["..."]

  sinks:
    - id: sink-1
      inputs: ["processor-1", "processor-2"]  # Fan-in from multiple processors
      command: ["..."]
```

## Quickstart Pipeline Format

For the Asset Balance Indexer quickstart format, see: `examples/quickstart/README.md`

**Quick reference:**

```yaml
apiVersion: flowctl.io/v1alpha1
kind: QuickstartPipeline
metadata:
  name: asset-balance-indexer

spec:
  source:
    type: stellar-live-source-datalake
    network: testnet
    start_ledger: 1000000
    end_ledger: 1001000

  assets:
    - code: USDC
      issuer: GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5

  sink:
    type: duckdb
    database: balances.duckdb
    table: account_balances
```

## Component Configuration

### Environment Variables

Components are configured via environment variables. Common patterns:

#### flowctl Integration

```yaml
env:
  ENABLE_FLOWCTL: "true"              # Enable control plane registration
  FLOWCTL_ENDPOINT: "127.0.0.1:8080" # Control plane address
  COMPONENT_ID: "my-component"        # Optional: Override component ID
```

#### Network Configuration

```yaml
env:
  PORT: ":50051"        # gRPC server port (format: :port or host:port)
  HEALTH_PORT: "8088"   # Health check HTTP port
```

#### Database Configuration

```yaml
env:
  POSTGRES_HOST: "localhost"
  POSTGRES_PORT: "5432"
  POSTGRES_DB: "database_name"
  POSTGRES_USER: "username"
  POSTGRES_PASSWORD: "password"
  POSTGRES_SSLMODE: "disable"  # or require, verify-ca, verify-full
```

#### Stellar Configuration

```yaml
env:
  BACKEND_TYPE: "RPC"  # or CAPTIVECORE
  RPC_ENDPOINT: "https://soroban-testnet.stellar.org:443"
  NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"
  START_LEDGER: "1000000"
  END_LEDGER: "1001000"  # Optional: omit for continuous streaming
```

## Driver Types

### Process Driver

**Best for:** Development, single-machine deployments

```yaml
spec:
  driver: process

  sources:
    - id: my-source
      command: ["/absolute/path/to/binary"]  # Must be absolute path
      env:
        PORT: "localhost:50052"  # Use localhost or 127.0.0.1
```

**Characteristics:**
- Components run as local processes
- Direct process management by flowctl
- Fastest startup time
- Use absolute paths for binaries
- Use localhost for networking

### Docker Driver

**Best for:** Containerized environments, easy distribution

```yaml
spec:
  driver: docker

  sources:
    - id: my-source
      command: ["/path/to/binary"]  # Path inside container
      image: "ghcr.io/myorg/my-source:latest"  # Container image
      env:
        PORT: ":50052"  # Container-internal port
```

**Characteristics:**
- Components run in Docker containers
- Requires Docker or nerdctl installed
- Isolated environments
- Network managed by Docker

### Kubernetes Driver

**Best for:** Production clusters, auto-scaling

```yaml
spec:
  driver: kubernetes

  sources:
    - id: my-source
      image: "ghcr.io/myorg/my-source:latest"
      env:
        PORT: ":50052"
```

**Characteristics:**
- Components run as Kubernetes pods
- Leverages Kubernetes features (scaling, health checks, etc.)
- Requires active Kubernetes cluster
- Use `flowctl translate` to generate K8s manifests

### Nomad Driver

**Best for:** Multi-region deployments, edge computing

```yaml
spec:
  driver: nomad

  sources:
    - id: my-source
      image: "ghcr.io/myorg/my-source:latest"
      env:
        PORT: ":50052"
```

**Characteristics:**
- Components run as Nomad tasks
- Multi-region support
- Flexible deployment targets
- Use `flowctl translate` to generate Nomad job specs

## Complete Examples

### Minimal Working Example

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: minimal-pipeline

spec:
  driver: process

  sources:
    - id: data-source
      command: ["/home/user/bin/data-source"]

  sinks:
    - id: console-sink
      command: ["/home/user/bin/console-sink"]
      inputs: ["data-source"]
```

### Full-Featured Example

See the complete working example at:
- **Repository:** https://github.com/withObsrvr/flowctl-sdk
- **Path:** `examples/contract-events-pipeline/`
- **Pipeline Config:** `contract-events-pipeline.yaml`

This demonstrates:
- Stellar RPC source
- Contract events processor
- PostgreSQL sink
- Full environment variable configuration
- Health checks and monitoring

### More Examples

- **Minimal:** `examples/minimal.yaml`
- **Docker:** `examples/docker-pipeline.yaml`
- **DAG:** `examples/dag-pipeline.yaml`
- **Quickstart:** `examples/quickstart/*.yaml`

## Validation

flowctl validates configuration files using CUE schema:

```bash
# Validate a pipeline configuration
flowctl run --dry-run pipeline.yaml
```

Schema definition: `schemas/cue/schema.cue`

## Building Components

To build custom components, use the [flowctl-sdk](https://github.com/withObsrvr/flowctl-sdk):

```go
package main

import (
    "github.com/withObsrvr/flowctl-sdk/pkg/processor"
)

func main() {
    proc := processor.New(processor.Config{
        Name:        "my-processor",
        Description: "Custom data processor",
        Version:     "1.0.0",
    })

    proc.Run()
}
```

## Additional Resources

- **Main README:** `../README.md`
- **Quickstart Guide:** `../examples/quickstart/README.md`
- **flowctl-sdk:** https://github.com/withObsrvr/flowctl-sdk
- **Real-world demo:** https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline

## Questions?

- GitHub Issues: https://github.com/withObsrvr/flowctl/issues
- Documentation: https://github.com/withObsrvr/flowctl

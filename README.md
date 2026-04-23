# flowctl

flowctl is a **production pipeline orchestrator** for Stellar data components. It manages sources, processors, and sinks through an embedded control plane so you can define pipelines declaratively, run them as supervised processes, and observe their health and status.

## What flowctl is for

flowctl is the **orchestration/production layer** in the ecosystem:
- define pipelines in `flowctl/v1` YAML
- resolve and launch components
- run with an embedded or external control plane
- track registration, heartbeats, and run state
- inspect status and active runs

If you are rapidly prototyping processors or doing ad-hoc local analysis, use **nebu** first. When a processor is ready to be operated, monitored, and reused in production, it should live behind **flowctl**.

## Current focus

The primary supported runtime path today is:
- **Process orchestration** (`spec.driver: process` / `--orchestrator process`)
- **Embedded control plane** (default)
- **`flowctl/v1` pipelines**

Container/deployment backends exist in the repo but should be treated as secondary until they are hardened and fully tested.

## Installation

Prerequisites for the fastest local path:
- **Go 1.21+**
- **Git**
- Internet access to download components on first run

```bash
# Clone the repository
git clone https://github.com/withobsrvr/flowctl.git
cd flowctl

# Build the binary
make build

# Verify installation
./bin/flowctl version
```

`make deps` is only needed if you want to refresh Go module dependencies while developing.

## Quick Start

If you are new to flowctl, start here.

### Fastest path to a working repo checkout

```bash
# 1. Build flowctl
git clone https://github.com/withobsrvr/flowctl.git && cd flowctl && make build

# 2. Optional: use the helper script to generate and validate a starter pipeline
./scripts/quickstart.sh
```

That script builds `flowctl` if needed, generates `stellar-pipeline.yaml`, and validates it.

### 5-minute real pipeline path

```bash
# 1. Build flowctl
git clone https://github.com/withobsrvr/flowctl.git && cd flowctl && make build

# 2. Generate a starter pipeline
./bin/flowctl init --non-interactive --network testnet --destination duckdb

# 3. Validate it
./bin/flowctl validate stellar-pipeline.yaml

# 4. Run it
./bin/flowctl run stellar-pipeline.yaml
```

In another terminal:

```bash
# 5. Check component health
./bin/flowctl status

# 6. Inspect active runs
./bin/flowctl pipelines active
```

Notes:
- The DuckDB path is the simplest way to get started.
- Components are downloaded automatically on first run.
- **A Docker daemon is not required** for the default DuckDB quickstart.

### Core operator workflow

```bash
./bin/flowctl init
./bin/flowctl validate stellar-pipeline.yaml
./bin/flowctl run stellar-pipeline.yaml
./bin/flowctl status
./bin/flowctl pipelines active
./bin/flowctl pipelines run-info <run-id>
./bin/flowctl pipelines stop <run-id>
```

### Recommended operator flags

```bash
# Use a non-default control plane port
./bin/flowctl run --control-plane-port 9090 stellar-pipeline.yaml
./bin/flowctl status --control-plane-address 127.0.0.1 --control-plane-port 9090
./bin/flowctl pipelines active --control-plane-address 127.0.0.1 --control-plane-port 9090

# Let flowctl auto-select a free control plane port
./bin/flowctl run --control-plane-port 0 stellar-pipeline.yaml

# Persist embedded run history explicitly
./bin/flowctl run --db-path ~/.flowctl/flowctl-service-registry.db stellar-pipeline.yaml

# Or disable persistence for a one-off local run
./bin/flowctl run --no-persistence stellar-pipeline.yaml
```

### DuckDB (Simplest - No Setup Required)

This is the recommended first run for anyone cloning the repo.

```bash
./bin/flowctl init --non-interactive --network testnet --destination duckdb
./bin/flowctl run stellar-pipeline.yaml

# Stop the pipeline first, then query your data
duckdb stellar-pipeline.duckdb "SELECT event_type, COUNT(*) FROM contract_events GROUP BY event_type"
```

### PostgreSQL (Production-Ready)

```bash
# Start PostgreSQL (if not already running)
docker run --name flowctl-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:16
docker exec flowctl-postgres createdb -U postgres stellar_events

# Create and run pipeline
./bin/flowctl init --non-interactive --network testnet --destination postgres
./bin/flowctl run stellar-pipeline.yaml

# If you use a non-default PostgreSQL password, edit stellar-pipeline.yaml
# and update spec.sinks[0].config.postgres_password before running.

# Query your data
docker exec flowctl-postgres psql -U postgres -d stellar_events \
  -c "SELECT event_type, COUNT(*) FROM contract_events GROUP BY event_type"
```

**See also:** [Quickstart Examples](examples/quickstart/) | [flowctl init Reference](docs/init-command.md)

---

## Starter Pipelines

The `flowctl init` command creates starter pipelines through an interactive wizard or via flags.

Common examples:

```bash
./bin/flowctl init
./bin/flowctl init --non-interactive --network testnet --destination duckdb
./bin/flowctl init --non-interactive --network mainnet --destination postgres -o prod-pipeline.yaml
```

Destination prerequisites:

| Destination | Prerequisite |
|-------------|--------------|
| `duckdb` | None - embedded database, just works |
| `postgres` | PostgreSQL running on `localhost:5432` with database `stellar_events` |

Components are downloaded automatically on first run and cached locally under `~/.flowctl/`.

Full reference:
- [docs/init-command.md](docs/init-command.md)
- [examples/quickstart/](examples/quickstart/)

## Understanding Components

**flowctl is a pipeline orchestrator**, not a data processor itself. It coordinates separate component binaries and manages data flow and component lifecycle via a control plane.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    flowctl (Orchestrator)                    │
│  • Control Plane (gRPC API)                                 │
│  • Component Registry & Health Monitoring                   │
│  • Stream Management                                        │
└─────────────────────────────┬───────────────────────────────┘
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

**Components are separate programs** that you build or install independently:
- **Sources**: Produce data (e.g., Stellar ledger streams, Kafka consumers)
- **Processors**: Transform data (e.g., extract contract events, filter transactions)
- **Sinks**: Consume data (e.g., PostgreSQL, webhooks, file storage)

To build your own components, use the [flowctl-sdk](https://github.com/withObsrvr/flowctl-sdk) which provides:
- Component registration and health checks
- Automatic heartbeat management
- gRPC streaming infrastructure
- Standardized configuration patterns

**Learn more:** [Getting Started Guide](examples/getting-started/README.md) | [Architecture Details](docs/architecture.md)

## Configuration

Pipelines are configured using Kubernetes-style YAML files. Here's a minimal example:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: hello-world-pipeline
  description: A simple pipeline demonstrating flowctl basics

spec:
  driver: process  # Run components as local processes

  sources:
    - id: data-generator
      command: ["./bin/data-generator"]
      env:
        INTERVAL_MS: "1000"
        OUTPUT_FORMAT: "json"

  processors:
    - id: data-transformer
      command: ["./bin/data-transformer"]
      env:
        TRANSFORM_TYPE: "uppercase"

  sinks:
    - id: console-logger
      command: ["./bin/console-logger"]
      env:
        LOG_FORMAT: "pretty"
```

**Key configuration concepts**:
- `apiVersion: flowctl/v1` - Standard pipeline format
- `spec.driver` - Execution environment (currently focus on `process`)
- Component `command` - Full path to the component binary
- Component `env` - Environment variables for configuration

For more examples, see:
- `examples/quickstart/testnet-duckdb-pipeline.yaml` - Quickstart DuckDB pipeline
- `examples/quickstart/testnet-postgres-pipeline.yaml` - Quickstart PostgreSQL pipeline
- `examples/config/example.yaml` - Configuration example
- **[Real-world demo](https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline)** - Complete Stellar contract events pipeline with PostgreSQL

### Logging

flowctl uses structured logging powered by Uber's Zap library. You can control the log level using:

- Configuration file: Set the `log_level` field in your YAML configuration
- Command line: Use the `--log-level` flag (e.g., `--log-level=debug`)

Available log levels:
- `debug`: Detailed information for debugging
- `info`: General operational information (default)
- `warn`: Warning conditions
- `error`: Error conditions

Example:
```bash
./bin/flowctl run examples/quickstart/testnet-duckdb-pipeline.yaml --log-level=debug
```

## Translation and Docker-based Workflows

flowctl still includes translation and Docker/container-oriented workflows, but they are **not the primary supported runtime path** today.

If you are getting started with flowctl, skip these for now and use:
- `flowctl init`
- `flowctl validate`
- `flowctl run`
- `flowctl status`
- `flowctl pipelines`

If you specifically need Docker Compose translation, local execution generation, or container troubleshooting, see:
- [docs/docker-and-translation.md](docs/docker-and-translation.md)

## Advanced Features

### DAG-Based Processing

flowctl supports a Directed Acyclic Graph (DAG) based processing pipeline, which allows complex topologies beyond simple linear chains. This enables:

- Parallel processing of data streams
- Fan-out/fan-in patterns
- Buffered channels for flow control
- Strongly typed event connections

For more information, see [DAG-Based Processing](docs/dag-processing.md).

### Secure Communication with TLS

flowctl supports secure communication between components using Transport Layer Security (TLS):

- Server-side TLS for encrypted communication
- Mutual TLS (mTLS) where both client and server authenticate each other
- Certificate validation options, including CA certificate support
- TLS skip verification for development environments (not recommended for production)

For server configuration:
```bash
# Start a server with TLS
flowctl server --port 8080 --tls-cert server.crt --tls-key server.key

# Start a server with mutual TLS
flowctl server --port 8080 --tls-cert server.crt --tls-key server.key --tls-ca-cert ca.crt
```

For client configuration, add a `tls` section to your YAML configuration:
```yaml
tls:
  mode: "enabled"  # Options: disabled, enabled, mutual
  cert_file: "client.crt"
  key_file: "client.key"
  ca_file: "ca.crt"  # Required for mutual TLS
  skip_verify: false
  server_name: "flowctl-server.example.com"  # For SNI verification
```

For more information, see [TLS Configuration](docs/tls-configuration.md).

## Processor Discovery

Discover and inspect processors registered with the control plane at runtime.

**Note:** These commands require a running pipeline with an active control plane.

### List Processors

```bash
# List all healthy processors
./bin/flowctl processors list

# Include unhealthy processors
./bin/flowctl processors list --include-unhealthy

# Output as JSON
./bin/flowctl processors list -o json
```

### Find Processors by Type

```bash
# Find processors that accept stellar.ledger.v1 events
./bin/flowctl processors find --input stellar.ledger.v1

# Find processors that produce token.transfer events
./bin/flowctl processors find --output stellar.token.transfer.v1

# Find processors for a specific network
./bin/flowctl processors find --metadata network=testnet
```

### Show Processor Details

```bash
# Show full details for a processor
./bin/flowctl processors show ttp-processor-v1
```

**Full reference:** [docs/processor-discovery.md](docs/processor-discovery.md)

---

## Troubleshooting

### Common Issues

#### Component Not Starting

**Symptoms:** Pipeline starts but component fails immediately

**Solutions:**

```bash
# 1. Check binary exists and is executable
ls -la /path/to/component
chmod +x /path/to/component

# 2. Verify absolute path is used (required for driver: process)
# ✅ Good: /home/user/bin/my-component
# ❌ Bad: ./bin/my-component

# 3. Test component standalone
/path/to/component

# 4. Check component logs in flowctl output
./bin/flowctl run pipeline.yaml 2>&1 | grep "component-id"
```

#### Component Not Registering with Control Plane

**Symptoms:** Component starts but doesn't appear in registry, no health checks

**Solutions:**

```bash
# 1. Verify control plane endpoint matches the running pipeline
./bin/flowctl status --control-plane-address 127.0.0.1 --control-plane-port 8080

# 2. If 8080 is busy, run on a different port or auto-select one
./bin/flowctl run --control-plane-port 9090 pipeline.yaml
./bin/flowctl run --control-plane-port 0 pipeline.yaml

# 3. Verify ENABLE_FLOWCTL is set
# In pipeline YAML:
env:
  ENABLE_FLOWCTL: "true"
  FLOWCTL_ENDPOINT: "127.0.0.1:8080"

# 4. Check component built with flowctl-sdk
# Components must use SDK for registration:
# github.com/withObsrvr/flowctl-sdk/pkg/{source,processor,consumer}
```

#### No Data Flowing Between Components

**Symptoms:** Components start but no data appears in sink

**Solutions:**

```bash
# 1. Verify inputs are correctly specified
processors:
  - id: my-processor
    inputs: ["correct-source-id"]  # Must match source id

# 2. Check event types match
# Source OutputType must match Processor InputType
# Check component code or logs

# 3. Enable debug logging
./bin/flowctl run pipeline.yaml --log-level=debug

# 4. Check for processing errors
./bin/flowctl run pipeline.yaml 2>&1 | grep -i error
```

#### Port Already in Use

**Symptoms:** "bind: address already in use"

**Solutions:**

```bash
# 1. Check what's using the port
lsof -i :50051
netstat -tulpn | grep 50051

# 2. For the embedded control plane, use another port
./bin/flowctl run --control-plane-port 9090 pipeline.yaml

# 3. Or let flowctl choose a free control plane port automatically
./bin/flowctl run --control-plane-port 0 pipeline.yaml

# 4. Use unique ports for each component
sources:
  - id: source-1
    env:
      PORT: ":50051"
      HEALTH_PORT: "8088"

processors:
  - id: processor-1
    env:
      PORT: ":50052"  # Different port
      HEALTH_PORT: "8089"

# 5. Kill conflicting process if needed
kill <PID>
```

**Tip:** In process mode, flowctl now injects a default `HEALTH_PORT` for components that do not define one, which reduces accidental health-port collisions. Explicitly setting `HEALTH_PORT` is still recommended for production pipelines.

#### Pipeline Configuration Invalid

**Symptoms:** "validation failed" or schema errors

**Solutions:**

```bash
# 1. Validate configuration before running
./bin/flowctl validate pipeline.yaml

# 2. Check required fields
# - apiVersion: flowctl/v1
# - kind: Pipeline
# - metadata.name
# - spec.driver
# - At least one source and one sink

# 3. Verify YAML syntax
# Use yamllint or online YAML validator

# 4. Check for common mistakes:
# - Singular "source" instead of "sources" (array)
# - Wrong apiVersion (should be flowctl/v1)
# - Missing inputs on processors/sinks
```

#### Component Crashes or Exits

**Symptoms:** Component starts then immediately exits

**Solutions:**

```bash
# 1. Check environment variables are set
# Components may require config via env vars

# 2. Test component standalone with env vars
ENABLE_FLOWCTL=false \
PORT=:50051 \
HEALTH_PORT=8088 \
CONFIG_KEY=value \
/path/to/component

# 3. Check component logs for errors
# Look for initialization failures, missing dependencies

# 4. Verify dependencies are available
# Database connections, API endpoints, file paths, etc.
```

#### Inspecting Active and Historical Runs

**Examples:**

```bash
# Active runs on a specific control plane
./bin/flowctl pipelines active --control-plane-address 127.0.0.1 --control-plane-port 9090

# Inspect one run in detail (full id or unique prefix)
./bin/flowctl pipelines run-info abc12345 --control-plane-address 127.0.0.1 --control-plane-port 9090

# Stop a running pipeline
./bin/flowctl pipelines stop abc12345 --control-plane-address 127.0.0.1 --control-plane-port 9090
```

**Persistence note:** embedded control plane mode now uses BoltDB-backed storage by default, so run history can survive process restarts. Use `--db-path` to choose the storage location or `--no-persistence` for ephemeral local runs.

### Docker and Container Issues

Docker/container troubleshooting has moved to:
- [docs/docker-and-translation.md](docs/docker-and-translation.md)

### Getting Help

#### Enable Debug Logging

```bash
# Run with verbose output
./bin/flowctl run pipeline.yaml --log-level=debug

# Save logs to file
./bin/flowctl run pipeline.yaml --log-level=debug 2>&1 | tee pipeline.log
```

#### Check Component Health

```bash
# While pipeline is running, check health endpoints
# (if component has HEALTH_PORT configured)

curl http://localhost:8088/health  # Source
curl http://localhost:8089/health  # Processor
curl http://localhost:8090/health  # Sink
```

#### Examine Control Plane State

```bash
# Control plane runs on 127.0.0.1:8080 by default
# Check registered components (requires gRPC client like grpcurl)

# Or check logs for registration messages
./bin/flowctl run pipeline.yaml 2>&1 | grep "Component registered"
```

#### Review Documentation

- **Getting Started**: [examples/getting-started/README.md](examples/getting-started/README.md)
- **Configuration Guide**: [docs/configuration.md](docs/configuration.md)
- **Building Components**: [docs/building-components.md](docs/building-components.md)

#### Report Issues

If you're still having trouble:

1. Check existing issues: https://github.com/withobsrvr/flowctl/issues
2. Create a new issue with:
   - flowctl version (`./bin/flowctl version`)
   - Pipeline YAML (redact sensitive info)
   - Full error output
   - Steps to reproduce

### Sandbox Issues

For sandbox-specific issues, refer to [docs/sandbox.md](docs/sandbox.md) for detailed troubleshooting steps.

## Development

### Building

```bash
# Build for current platform
make build

# Build for multiple platforms
make build-all
```

### Testing

```bash
make test
```

### Running the Example

```bash
make run-example
```

## Documentation

### Getting Started
- **[Getting Started Guide](examples/getting-started/README.md)** - Complete beginner's guide to flowctl
- **[Examples Overview](examples/README.md)** - Navigate all examples by use case

### Configuration & Development
- **[Configuration Guide](docs/configuration.md)** - Complete schema reference and examples
- **[Building Components](docs/building-components.md)** - How to build sources, processors, and sinks
- **[Migration Guide](docs/migration-guide.md)** - Migrate from old config format to flowctl/v1

### Advanced Topics
- **[Architecture](docs/architecture.md)** - System design, data flow, and component lifecycle
- **[Performance Tuning](docs/performance-tuning.md)** - Optimization strategies and scaling

### Real-World Example
- **[Contract Events Pipeline](https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline)** - Complete Stellar → PostgreSQL pipeline (< 5 min to run)

## License

MPL-2.0

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request
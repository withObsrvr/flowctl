# flowctl

flowctl orchestrates Stellar data pipelines with a control plane for component registration, run history, and health. In process mode, it launches components and supervises gRPC delivery between sources, optional processors, and sinks.

## What flowctl is for

flowctl is the **orchestration/production layer** in the ecosystem:
- define pipelines in `flowctl/v1` YAML
- resolve and launch components
- run with an embedded or external control plane
- track registration, heartbeats, and run state
- fail a run on delivery errors and cancel its other stream edges
- inspect status, active runs, and recorded failure context

If you are rapidly prototyping processors or doing ad-hoc local analysis, use **nebu** first. When a processor is ready to be operated, monitored, and reused in production, it should live behind **flowctl**.

## Current focus

The primary supported runtime path today is:
- **Process orchestration** (`spec.driver: process` / `--orchestrator process`)
- **Embedded control plane** (default)
- **`flowctl/v1` pipelines**

Container/deployment backends exist in the repo but should be treated as secondary until they are hardened and fully tested. Process-mode forwarding currently supports a sequential processor chain (or direct source-to-sink delivery) and fan-out to sinks; it is not an arbitrary DAG runtime.

## Installation

The fastest user path does **not** require cloning this repository.

### Option 1: `go install`

```bash
go install github.com/withobsrvr/flowctl@latest
export PATH="$HOME/go/bin:$PATH"
flowctl version
```

### Option 2: curl installer

```bash
curl -sSL https://flowctl.withobsrvr.com/install.sh | sh
flowctl version
```

### Option 3: build from source

```bash
git clone https://github.com/withobsrvr/flowctl.git
cd flowctl
make build
./bin/flowctl version
```

Notes:
- Internet access is required on first run so flowctl can download pipeline components.
- The default DuckDB starter path does **not** require a local Docker daemon.
- `make deps` is only needed if you are developing in this repository.

## Quick Start

If you are new to flowctl, use this path first.

### 2-minute first pipeline

```bash
# 1. Install flowctl
go install github.com/withobsrvr/flowctl@latest
export PATH="$HOME/go/bin:$PATH"

# 2. Generate a starter pipeline
flowctl init --preset testnet-duckdb

# 3. Validate it
flowctl validate stellar-pipeline.yaml

# 4. Run it
flowctl run stellar-pipeline.yaml
```

In another terminal:

```bash
# 5. Check component health (while the control plane is running)
flowctl status

# 6. Inspect active runs
flowctl pipelines active
```

Optional, after stopping the pipeline:

```bash
duckdb stellar-pipeline.duckdb "SELECT event_type, COUNT(*) FROM contract_events GROUP BY event_type"
```

### Core operator workflow

```bash
flowctl init
flowctl validate stellar-pipeline.yaml
flowctl run stellar-pipeline.yaml
flowctl status
flowctl pipelines active
flowctl pipelines run-info <run-id>
flowctl pipelines stop <run-id>
```

`flowctl run` waits for delivery to complete or for an operator to stop it. A
bounded run completes successfully once every edge closes and sinks acknowledge;
a source, processor, or sink delivery failure marks the run failed and exits
nonzero. An unexpected EOF from an unbounded source is a failure. Currently
bounded completion is inferred from the source's `END_LEDGER` environment
variable; a source with a different finite stopping condition will be treated
as unbounded. On interruption or `pipelines stop`, the run is stopped rather
than failed. Restart and checkpoints remain the operator's responsibility.

### Recommended operator flags

```bash
# Use a non-default control plane port
flowctl run --control-plane-port 9090 stellar-pipeline.yaml
flowctl status --control-plane-address 127.0.0.1 --control-plane-port 9090
flowctl pipelines active --control-plane-address 127.0.0.1 --control-plane-port 9090

# Let flowctl auto-select a free control plane port
flowctl run --control-plane-port 0 stellar-pipeline.yaml

# Persist embedded run history explicitly
flowctl run --db-path ~/.flowctl/flowctl-service-registry.db stellar-pipeline.yaml

# Or disable persistence for a one-off local run
flowctl run --no-persistence stellar-pipeline.yaml
```

### DuckDB (Recommended first run)

```bash
flowctl init --preset testnet-duckdb
flowctl run stellar-pipeline.yaml
```

This path is the recommended first success story because:
- components are downloaded automatically on first run
- data is written locally to `stellar-pipeline.duckdb`
- **a Docker daemon is not required**

### PostgreSQL (when you want an external database)

```bash
# Start PostgreSQL (if not already running)
docker run --name flowctl-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:16
docker exec flowctl-postgres createdb -U postgres stellar_events

# Create and run pipeline
flowctl init --non-interactive --network testnet --destination postgres
flowctl run stellar-pipeline.yaml

# If you use a non-default PostgreSQL password, edit stellar-pipeline.yaml
# and update spec.sinks[0].config.postgres_password before running.

# Query your data
docker exec flowctl-postgres psql -U postgres -d stellar_events \
  -c "SELECT event_type, COUNT(*) FROM contract_events GROUP BY event_type"
```

If the `postgres-consumer@v1.0.0` component image is not published in your registry yet, use the DuckDB path instead.

**See also:** [Starter pipeline examples](examples/quickstart/) | [HTML quickstart](docs/quickstart.html) | [flowctl init reference](docs/init-command.md) | [Onboarding plan](docs/ONBOARDING_PLAN.md)

---

## Starter Pipelines

The `flowctl init` command creates starter pipelines through an interactive wizard or via flags.

Common examples:

```bash
flowctl init
flowctl init --preset testnet-duckdb
flowctl init --non-interactive --network mainnet --destination postgres -o prod-pipeline.yaml
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
- [docs/quickstart.html](docs/quickstart.html)

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

**Components are separate programs** that you build or install independently. A
component must register a reachable gRPC endpoint and implement its source,
processor, or consumer stream service to participate in supervised delivery;
registration and heartbeats alone do not make a data path healthy.

- **Sources**: Produce data (e.g., Stellar ledger streams, Kafka consumers)
- **Processors**: Transform data (e.g., extract contract events, filter transactions)
- **Sinks**: Consume data (e.g., PostgreSQL, webhooks, file storage)

For auto-resolved component refs in `type:`:
- `stellar-live-source@v1.0.0` resolves to `docker.io/withobsrvr/stellar-live-source:v1.0.0`
- `myorg/custom-source@v1.2.3` resolves to `docker.io/myorg/custom-source:v1.2.3`

If you do not want auto-download/resolution, use an explicit local binary path with `command:`.

To build your own components, you now have two SDK options:

- **Go**: [flowctl-sdk](https://github.com/withObsrvr/flowctl-sdk)
- **JavaScript / TypeScript**: [js-flowctl-sdk](https://github.com/withObsrvr/js-flowctl-sdk) (`npm install @withobsrvr/js-flowctl-sdk`)

Both SDKs provide:
- Component registration and health checks
- Automatic heartbeat management
- gRPC streaming infrastructure
- Standardized configuration patterns

The JavaScript SDK also includes a working `flowctl` process-mode end-to-end example for non-Go teams.

**Learn more:** [Getting Started Guide](examples/getting-started/README.md) | [Architecture Details](docs/architecture.md)

## Configuration

Pipelines are configured using Kubernetes-style YAML files. This example shows the shape of a process pipeline; the three `command` paths must point to binaries that implement the corresponding gRPC stream services:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: hello-world-pipeline

spec:
  description: A simple pipeline demonstrating flowctl basics
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
      inputs: [data-generator]
      env:
        TRANSFORM_TYPE: "uppercase"

  sinks:
    - id: console-logger
      command: ["./bin/console-logger"]
      inputs: [data-transformer]
      env:
        LOG_FORMAT: "pretty"
```

**Key configuration concepts**:
- `apiVersion: flowctl/v1` - Standard pipeline format
- `spec.driver` - Execution environment (currently focus on `process`)
- Component `command` - Executable path (relative paths resolve from the working directory where `flowctl run` is invoked)
- Component `env` - Environment variables for configuration
- Component `inputs` - Upstream component IDs; processors are wired in list order in process mode

Processors are optional: when none are defined, flowctl wires each source
directly to the sinks. Multiple sinks receive matching event types; a slow
sink applies backpressure rather than causing dropped events. Use
`spec.registrationOnly: true` **only** for intentional monitor-only pipelines:
it disables stream wiring and delivery supervision, so `RUNNING` then means
components registered, not that events are flowing. For example:

```yaml
spec:
  registrationOnly: true # intentional control-plane registration/health only
  sources:
    - id: my-source
      command: ["/path/to/source"]
  sinks:
    - id: my-sink
      inputs: [my-source]
      command: ["/path/to/sink"]
```

A real source-to-sink pipeline should omit `registrationOnly` and supply
stream-capable components. For now, bounded sources are recognized by
`END_LEDGER` in the source's `env` (including when a component reference
translates `config.end_ledger` into that environment variable).

For more examples, see:
- `examples/quickstart/testnet-duckdb-pipeline.yaml` - Starter DuckDB pipeline
- `examples/quickstart/testnet-postgres-pipeline.yaml` - Starter PostgreSQL pipeline
- `examples/config/example.yaml` - Configuration example
- **[Real-world demo](https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline)** - Complete Stellar contract events pipeline with PostgreSQL

### Logging

flowctl uses structured logging powered by Uber's Zap library. Use the
`--log-level` flag (e.g., `--log-level=debug`) to control **flowctl's** logs;
component binaries may have separate logging settings.

Available log levels:
- `debug`: Detailed information for debugging
- `info`: General operational information (default)
- `warn`: Warning conditions
- `error`: Error conditions

Example:
```bash
flowctl run examples/quickstart/testnet-duckdb-pipeline.yaml --log-level=debug
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

The repository also contains DAG execution code, but the primary `flowctl run`
process path currently wires processors in list order and fans out to sinks.
It does not implement arbitrary DAG fan-in/fan-out or buffered sink queues.
See [Architecture: DAG event flow](docs/architecture.md#event-flow-dag---diamond-pattern) for the separate DAG model.

### Secure Communication with TLS

The repository has TLS configuration for server/client integrations. **The primary
process-mode stream orchestrator currently connects to component gRPC endpoints
using insecure transport.** Do not assume the TLS settings below protect the
`flowctl run` source-to-sink data path. The available TLS options include:

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

These server flags do not enable TLS on the process-mode component streams.
For background, see [Architecture: Communication Security](docs/architecture.md#communication-security).

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

# 2. Confirm the command path is valid from flowctl's working directory
# Absolute paths are less sensitive to where you start flowctl.
# Relative paths such as ./bin/my-component also work from that directory.

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

# 3. Check the component's registration, heartbeat, and service logs.
# SDK packages: github.com/withObsrvr/flowctl-sdk/pkg/{source,processor,consumer}
```

A component must register a reachable gRPC stream endpoint. Some SDK-based
components require these pipeline environment settings (check that component's
configuration and use the actual control-plane address):

```yaml
env:
  ENABLE_FLOWCTL: "true"  # Component-specific, not universal
  FLOWCTL_ENDPOINT: "127.0.0.1:8080"
```

#### No Data Flowing Between Components

**Symptoms:** Components start but no data appears in sink

**Solutions:**

```bash
1. Verify that `inputs` names the upstream component ID. Without processors,
   a sink can name a source directly. Check registered event types too: sinks
   with a nonempty input type list receive only matching events.
2. Check `flowctl run`'s exit code and the failing edge in its output. A failed
   edge cancels the other streams rather than dropping events:

```bash
./bin/flowctl run pipeline.yaml --log-level=debug
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
# - At least one source and one sink (also required in registration-only mode)

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

# Inspect one run in detail (full id or unique prefix, including delivery error)
./bin/flowctl pipelines run-info abc12345 --control-plane-address 127.0.0.1 --control-plane-port 9090

# Stop a running pipeline
./bin/flowctl pipelines stop abc12345 --control-plane-address 127.0.0.1 --control-plane-port 9090
```

**Persistence note:** embedded mode uses BoltDB at `~/.flowctl/flowctl-service-registry.db` by default. Use `--db-path` to choose a file or `--no-persistence` for an ephemeral run. A failed run stores the first delivery error as a structured, credential-free record in the run's `Error` field; `run-info` displays it. `pipelines run-info` requires a *running* control plane: after an embedded run exits, start a standalone `flowctl server --db-path <same-file>` (with no other process using the DB) to inspect persisted history.

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

For sandbox and container-related issues, see [Docker and translation workflows](docs/docker-and-translation.md).

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
- **[Delivery modernization](docs/modernization-plan.md)** - Supervised edges, error reporting, and current limitations
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
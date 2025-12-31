# flowctl

flowctl is a **pipeline orchestrator** that coordinates data flow between components. It manages sources (data producers), processors (transformers), and sinks (consumers) through an embedded control plane, enabling you to build robust data pipelines with minimal boilerplate.

## Features

- Unix-style CLI interface
- Pluggable processor architecture
- Support for multiple sink types
- YAML-based configuration
- Structured logging with Uber's Zap
- Built-in health monitoring
- Prometheus metrics
- Docker Compose deployment
- DAG-based processor chaining with buffered channels
- Flexible pipeline topologies with fan-out/fan-in support
- Secure communication with TLS and mutual TLS support

## Installation

```bash
# Clone the repository
git clone https://github.com/withobsrvr/flowctl.git
cd flowctl

# Build the binary
make build

# Install dependencies
make deps
```

## Quick Start

1. Create a pipeline configuration file (see `examples/minimal.yaml` for a template)
2. Run the pipeline:

```bash
./bin/flowctl run examples/minimal.yaml
```

## Understanding Components

**flowctl is a pipeline orchestrator**, not a data processor itself. It coordinates separate component binaries (sources, processors, sinks) and manages the data flow between them via an embedded control plane.

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
- `spec.driver` - Execution environment (`process`, `docker`, `kubernetes`, `nomad`)
- Component `command` - Full path to the component binary
- Component `env` - Environment variables for configuration

For more complex examples, see:
- `examples/minimal.yaml` - Minimal working pipeline
- `examples/docker-pipeline.yaml` - Pipeline with Docker deployment
- `examples/dag-pipeline.yaml` - DAG-based pipeline with complex topology
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
./bin/flowctl apply -f examples/minimal.yaml --log-level=debug
```

## Pipeline Translation

flowctl supports translating pipeline configurations to different deployment formats:

```bash
# Translate a pipeline to Docker Compose
./bin/flowctl translate -f examples/docker-pipeline.yaml -o docker-compose

# Save the output to a file
./bin/flowctl translate -f examples/docker-pipeline.yaml -o docker-compose --to-file docker-compose.yml

# Add a resource prefix for naming consistency
./bin/flowctl translate -f examples/docker-pipeline.yaml -o docker-compose --prefix myapp

# Specify a container registry
./bin/flowctl translate -f examples/docker-pipeline.yaml -o docker-compose --registry ghcr.io/myorg

# Generate local execution script
./bin/flowctl translate -f examples/local-pipeline.yaml -o local --to-file run_pipeline.sh
```

Supported output formats:
- `docker-compose`: Docker Compose YAML
- `local`: Local execution configuration (Docker Compose or bash script)

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

### Local Execution

When using the `local` output format, flowctl generates a configuration for running your pipeline locally. By default, it creates a Docker Compose configuration with profiles, but you can also use the legacy bash script generator if needed.

#### Docker Compose-based Local Execution (Default)

The Docker Compose-based local generator creates:
1. A Docker Compose configuration file with profile support
2. An environment file with all required variables
3. Proper dependency ordering between components
4. Health check monitoring
5. Volume management for persistent data and logs

```bash
# Generate Docker Compose configuration for local execution
./bin/flowctl translate -f examples/local-pipeline.yaml -o local --to-file docker-compose.yaml

# Start the pipeline
docker compose --profile local up -d

# View logs
docker compose logs -f

# Stop the pipeline
docker compose down
```

See [Local Execution with Docker Compose](docs/local-execution.md) for more details.

#### Legacy Bash Script Generator

For compatibility with existing workflows, you can still use the bash script generator:

```bash
# Set environment variable to use bash script generator
export FLOWCTL_LOCAL_GENERATOR_TYPE=bash

# Generate local execution script
./bin/flowctl translate -f examples/local-pipeline.yaml -o local --to-file run_pipeline.sh

# Make the script executable
chmod +x run_pipeline.sh

# Run the pipeline
./run_pipeline.sh
```

The bash script generator creates:
1. A bash script to start and manage pipeline components
2. An environment file with all required variables
3. Proper dependency ordering between components
4. Health check monitoring
5. Process supervision with automatic restart

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
# 1. Verify ENABLE_FLOWCTL is set
# In pipeline YAML:
env:
  ENABLE_FLOWCTL: "true"
  FLOWCTL_ENDPOINT: "127.0.0.1:8080"

# 2. Check control plane started
# Look for log: "Starting control plane on 127.0.0.1:8080"

# 3. Check component built with flowctl-sdk
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

# 2. Use unique ports for each component
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

# 3. Kill conflicting process if needed
kill <PID>
```

#### Pipeline Configuration Invalid

**Symptoms:** "validation failed" or schema errors

**Solutions:**

```bash
# 1. Validate configuration
./bin/flowctl run --dry-run pipeline.yaml

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

### Docker and Container Issues

#### Docker Permission Errors

If you encounter permission errors when running flowctl sandbox commands, the tool will provide platform-specific guidance. Common solutions include:

**Linux:**
- Run with sudo: `sudo flowctl sandbox start ...`
- Add your user to the docker group: `sudo usermod -aG docker $USER` (then log out and back in)
- Check if Docker service is running: `sudo systemctl status docker`

**macOS:**
- Ensure Docker Desktop is running
- Check Docker Desktop permissions in System Preferences
- Try restarting Docker Desktop

**NixOS:**
- See detailed setup guide: [docs/nixos-docker-setup.md](docs/nixos-docker-setup.md)
- Quick fix: Run with sudo
- Permanent fix: Add user to docker group in configuration.nix

#### Container Runtime Not Found

If flowctl cannot find Docker or nerdctl:

1. Install Docker: https://docs.docker.com/get-docker/
2. Or install nerdctl: https://github.com/containerd/nerdctl
3. flowctl uses your system-installed container runtime by default

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
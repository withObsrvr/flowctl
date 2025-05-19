# flowctl

flowctl is a CLI-driven event-stream engine that ingests Stellar ledger data, transforms it through pluggable processors, and delivers typed Protobuf messages to user-defined sinks.

## Features

- Unix-style CLI interface
- Pluggable processor architecture
- Support for multiple sink types
- YAML-based configuration
- Structured logging with Uber's Zap
- Built-in health monitoring
- Prometheus metrics
- Multi-platform deployment (Docker, Kubernetes, Nomad)

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

## Configuration

The pipeline is configured using YAML files. Here's a minimal example:

```yaml
version: 0.1
log_level: info

source:
  type: mock
  params:
    interval_ms: 1000

processors:
  - name: mock_processor
    plugin: mock
    params:
      pass_through: true

sink:
  type: stdout
  params:
    pretty: true
```

For a more complex example, see `examples/stellar-pipeline.yaml`.

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
./bin/flowctl translate -f examples/docker-test.yaml -o docker-compose

# Save the output to a file
./bin/flowctl translate -f examples/docker-test.yaml -o docker-compose --to-file docker-compose.yml

# Add a resource prefix for naming consistency
./bin/flowctl translate -f examples/docker-test.yaml -o docker-compose --prefix myapp

# Specify a container registry
./bin/flowctl translate -f examples/docker-test.yaml -o docker-compose --registry ghcr.io/myorg

# Generate local execution script
./bin/flowctl translate -f examples/local-test.yaml -o local --to-file run_pipeline.sh
```

Supported output formats:
- `docker-compose`: Docker Compose YAML
- `kubernetes`: Kubernetes manifests (coming soon)
- `nomad`: Nomad job specifications (coming soon)
- `local`: Local execution configuration (Docker Compose or bash script)

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
./bin/flowctl translate -f examples/local-test.yaml -o local --to-file docker-compose.yaml

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
./bin/flowctl translate -f examples/local-test.yaml -o local --to-file run_pipeline.sh

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

## License

MPL-2.0

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request
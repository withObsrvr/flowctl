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

For more complex examples, see:
- `examples/stellar-pipeline.yaml` - Traditional pipeline
- `examples/dag-pipeline.yaml` - DAG-based pipeline with complex topology

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

## Troubleshooting

### Docker Permission Errors

If you encounter permission errors when running flowctl sandbox commands, the tool will provide platform-specific guidance. Common solutions include:

#### Linux
- Run with sudo: `sudo flowctl sandbox start --use-system-runtime ...`
- Add your user to the docker group: `sudo usermod -aG docker $USER` (then log out and back in)
- Check if Docker service is running: `sudo systemctl status docker`

#### macOS
- Ensure Docker Desktop is running
- Check Docker Desktop permissions in System Preferences
- Try restarting Docker Desktop

#### NixOS
- See detailed setup guide: [docs/nixos-docker-setup.md](docs/nixos-docker-setup.md)
- Quick fix: Run with sudo
- Permanent fix: Add user to docker group in configuration.nix

### Container Runtime Not Found

If flowctl cannot find Docker or nerdctl:

1. Install Docker: https://docs.docker.com/get-docker/
2. Or install nerdctl: https://github.com/containerd/nerdctl
3. Always use the `--use-system-runtime` flag (bundled runtime is not yet available)

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

## License

MPL-2.0

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request
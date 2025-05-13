# flowctl

flowctl is a CLI-driven event-stream engine that ingests Stellar ledger data, transforms it through pluggable processors, and delivers typed Protobuf messages to user-defined sinks.

## Features

- Unix-style CLI interface
- Pluggable processor architecture
- Support for multiple sink types
- YAML-based configuration
- Built-in health monitoring
- Prometheus metrics

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
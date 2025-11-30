# Component Name

> **Brief description of what your component does**

[![Release](https://img.shields.io/github/v/release/YOUR_ORG/YOUR_REPO)](https://github.com/YOUR_ORG/YOUR_REPO/releases)
[![Container](https://img.shields.io/badge/container-ghcr.io-blue)](https://ghcr.io/YOUR_ORG/YOUR_REPO)
[![License](https://img.shields.io/github/license/YOUR_ORG/YOUR_REPO)](LICENSE)

## Overview

[Provide a longer description of your component, what problem it solves, and how it fits into the flowctl ecosystem]

## Features

- ✅ Feature 1
- ✅ Feature 2
- ✅ Feature 3
- ✅ Multi-platform support (amd64, arm64)
- ✅ Health check endpoint

## Quick Start

### Using in a Flowctl Pipeline

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: example-pipeline

spec:
  sources:
    - id: my-source
      type: your-component-type
      image: ghcr.io/YOUR_ORG/YOUR_REPO:v1.0.0
      env:
        # Add your configuration here
        LOG_LEVEL: "info"
```

### Running Standalone

```bash
docker run ghcr.io/YOUR_ORG/YOUR_REPO:v1.0.0 --help
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LOG_LEVEL` | No | `info` | Logging level (debug, info, warn, error) |
| `YOUR_VAR` | Yes | - | Description of your variable |

### Command-Line Flags

```
--flag1 value     Description of flag1
--flag2 value     Description of flag2
--help            Show help message
--version         Show version information
```

## Example Usage

### Example 1: Basic Configuration

```yaml
sources:
  - id: basic-source
    type: your-component
    image: ghcr.io/YOUR_ORG/YOUR_REPO:v1.0.0
    env:
      LOG_LEVEL: "debug"
```

### Example 2: Advanced Configuration

```yaml
sources:
  - id: advanced-source
    type: your-component
    image: ghcr.io/YOUR_ORG/YOUR_REPO:v1.0.0
    env:
      LOG_LEVEL: "info"
      YOUR_VAR: "custom-value"
    volumes:
      - hostPath: /data
        containerPath: /app/data
        readonly: false
```

## Development

### Prerequisites

- Go 1.25 or later
- Docker (for building images)
- Make (optional, for convenience commands)

### Building Locally

```bash
# Clone the repository
git clone https://github.com/YOUR_ORG/YOUR_REPO
cd YOUR_REPO

# Install dependencies
go mod download

# Build binary
go build -o bin/your-component ./cmd/your-component

# Run locally
./bin/your-component --help
```

### Building Docker Image

```bash
# Build for local platform
docker build -t your-component:dev \
  --build-arg COMPONENT_NAME=your-component \
  --build-arg VERSION=dev \
  .

# Test the image
docker run --rm your-component:dev --version
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -v ./internal/processor -run TestSpecificFunction
```

## Architecture

[Describe your component's architecture, data flow, and key design decisions]

```
Input → [Processing Step 1] → [Processing Step 2] → Output
```

## Performance

[Document performance characteristics, limitations, and benchmarks]

- **Throughput:** X records/second
- **Latency:** Y milliseconds (p99)
- **Memory:** Z MB typical

## Health Checks

This component exposes a health check endpoint at `:8081/health`.

```bash
curl http://localhost:8081/health
# Response: OK (200)
```

In Kubernetes:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8081
  initialDelaySeconds: 10
  periodSeconds: 30
```

## Monitoring

[Describe available metrics, logging, and observability features]

### Metrics

- `component_records_processed_total` - Total records processed
- `component_errors_total` - Total errors encountered
- `component_processing_duration_seconds` - Processing time histogram

### Logs

Logs are written to stdout in JSON format:

```json
{
  "level": "info",
  "timestamp": "2025-11-21T10:00:00Z",
  "message": "Processing record",
  "record_id": "12345"
}
```

## Troubleshooting

### Common Issues

#### Issue: Component fails to start

**Symptoms:**
```
Error: connection refused
```

**Solution:**
Check that required environment variables are set and the upstream service is accessible.

#### Issue: High memory usage

**Symptoms:**
Memory usage grows over time.

**Solution:**
Adjust batch size or enable streaming mode.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Releases

Releases are automated through GitHub Actions. To create a new release:

```bash
# Create and push a version tag
git tag v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

The CI pipeline will automatically:
- Build multi-platform images (amd64, arm64)
- Push to GitHub Container Registry
- Create a GitHub release with notes

See [Component Publishing Guide](https://github.com/withobsrvr/flowctl/blob/main/docs/component-publishing-guide.md) for more details.

## License

[Your chosen license - e.g., Apache 2.0, MIT]

## Support

- **Documentation:** [Link to docs]
- **Issues:** [GitHub Issues](https://github.com/YOUR_ORG/YOUR_REPO/issues)
- **Discussions:** [GitHub Discussions](https://github.com/YOUR_ORG/YOUR_REPO/discussions)

## Acknowledgments

- Built with [Flowctl](https://github.com/withobsrvr/flowctl)
- [Any other acknowledgments]

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build Commands
- `make build` - Build the binary with CGO disabled and install schema files
- `make build-all` - Build for multiple platforms (linux-amd64, linux-arm64, darwin-amd64)
- `make clean` - Clean build artifacts

### Test Commands
- `make test` - Run all tests with verbose output
- `go test -v ./...` - Run all tests directly
- `./test/test_registration.sh` - Test registration functionality

### Development Commands
- `make deps` - Update Go module dependencies
- `make run-example` - Build and run the example pipeline
- `./scripts/generate.sh` - Generate Go code from proto files

### Sandbox Commands
The sandbox provides a local containerized environment for development:

**Prerequisites**: Docker or nerdctl must be installed on your system.

```bash
# Start sandbox
./bin/flowctl sandbox start --pipeline examples/sandbox-pipeline.yaml --services examples/sandbox.yaml

# Check status
./bin/flowctl sandbox status

# View logs
./bin/flowctl sandbox logs

# Stop sandbox
./bin/flowctl sandbox stop
```

**Note**: The `--use-system-runtime` flag is still supported but no longer required (it's now the default behavior).

### Running a Single Test
```bash
go test -v ./internal/core -run TestPipelineDAG
```

## Architecture Overview

### Command Structure
The application is a CLI tool built with Cobra framework:
- **Root**: `flowctl` - Base command with global flags
- **Subcommands**: `apply`, `list`, `translate`, `server`, `version`

### Core Components

**Pipeline Execution** (`internal/core/`)
- Two execution models: Simple (linear) and DAG (complex topologies)
- Components (sources, processors, sinks) run as separate containers/processes
- Communication via gRPC with typed events

**Configuration** (`internal/config/`)
- Kubernetes-style YAML (apiVersion, kind, metadata, spec)
- CUE schema validation in `schemas/cue/schema.cue`
- TLS/mTLS configuration support

**Translation System** (`internal/translator/`)
- Converts pipeline YAML to deployment formats
- Supports Docker Compose, Kubernetes (planned), Nomad (planned), Local execution
- Uses CUE for validation during translation

**Control Plane** (`internal/api/`)
- gRPC-based service for component registration
- Health monitoring and service discovery
- Storage backends: BoltDB (embedded) or in-memory

### Key Design Patterns

1. **Plugin Architecture**: Sources, processors, and sinks are pluggable components implementing standard interfaces
2. **Event-Driven**: Components communicate via typed events with buffered channels for backpressure
3. **Interface-Based**: Heavy use of Go interfaces for extensibility
4. **Cloud-Native**: Designed for containerized deployment with multiple orchestrator support

### Component Interaction Flow
```
Sources → Processors → Sinks
    ↓          ↓         ↓
    └──────────┴─────────┘
               ↓
         Control Plane
         (gRPC API)
```

Components register with the control plane on startup and send periodic heartbeats for health monitoring.

### Important Files
- Pipeline examples: `examples/minimal.yaml`, `examples/dag-pipeline.yaml`
- Schema definition: `schemas/cue/schema.cue`
- Proto definitions: `proto/` directory
- Storage interface: `internal/storage/storage.go`
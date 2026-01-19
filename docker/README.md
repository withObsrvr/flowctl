# Component Docker Images

This directory contains Dockerfiles for building flowctl component images.

## Quick Start

Build and push all components to Docker Hub:

```bash
# Login to Docker Hub
docker login

# Set required environment variable pointing to component source code
export COMPONENTS_BASE=/path/to/ttp-processor-demo

# Build and push all components
./docker/build-and-push.sh
```

## Available Components

| Component | Image | Description |
|-----------|-------|-------------|
| `stellar-live-source` | `docker.io/withobsrvr/stellar-live-source:v1.0.0` | Stellar blockchain live data source |
| `duckdb-consumer` | `docker.io/withobsrvr/duckdb-consumer:v1.0.0` | DuckDB sink for writing events |

## Build Options

```bash
# Build all components (no push)
./docker/build-and-push.sh --build-only

# Build specific component
./docker/build-and-push.sh stellar-live-source

# Build with custom version
VERSION=v1.1.0 ./docker/build-and-push.sh

# List available components
./docker/build-and-push.sh --list
```

## Prerequisites

1. **Docker** installed and running
2. **Component source code** at expected paths (see Configuration below)
3. **Docker Hub access** for pushing images

## Configuration

### Required Environment Variables

The `COMPONENTS_BASE` environment variable **must be set** before running build scripts. This points to the directory containing component source code.

```bash
export COMPONENTS_BASE=/path/to/ttp-processor-demo
```

The build script expects the following directory structure under `COMPONENTS_BASE`:

| Component | Expected Path |
|-----------|---------------|
| `stellar-live-source` | `$COMPONENTS_BASE/stellar-live-source-sdk/` |
| `duckdb-consumer` | `$COMPONENTS_BASE/duckdb-consumer/` |

### Optional Environment Variables

```bash
# Use different registry (default: docker.io/withobsrvr)
export REGISTRY=myregistry.io/myorg

# Use different version tag (default: v1.0.0)
export VERSION=v1.1.0
```

### Individual Build Scripts

Each component has a `build-in-docker.sh` script for building the Go binary inside Docker (useful for glibc compatibility). These scripts require additional environment variables:

```bash
# For stellar-live-source
export STELLAR_SOURCE_PATH=/path/to/stellar-live-source-sdk
export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk
export FLOW_PROTO_PATH=/path/to/flow-proto
./docker/stellar-live-source/build-in-docker.sh

# For duckdb-consumer
export DUCKDB_CONSUMER_PATH=/path/to/duckdb-consumer
export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk
export FLOW_PROTO_PATH=/path/to/flow-proto
./docker/duckdb-consumer/build-in-docker.sh

# For contract-events-processor
export CONTRACT_EVENTS_PATH=/path/to/contract-events-processor-sdk
export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk
export FLOW_PROTO_PATH=/path/to/flow-proto
./docker/contract-events-processor/build-in-docker.sh
```

## Image Structure

Images are built to work with the flowctl component resolver:

```
/component       # Executable binary
/metadata.json   # Component metadata (name, version, types)
```

The resolver downloads these images and extracts the binary to `~/.flowctl/components/`.

## Building Manually

If you prefer to build images manually:

```bash
# stellar-live-source
docker build \
    -t docker.io/withobsrvr/stellar-live-source:v1.0.0 \
    -f docker/stellar-live-source/Dockerfile \
    /path/to/stellar-live-source-sdk

# duckdb-consumer
docker build \
    -t docker.io/withobsrvr/duckdb-consumer:v1.0.0 \
    -f docker/duckdb-consumer/Dockerfile \
    /path/to/duckdb-consumer
```

## Adding New Components

1. Create a new directory: `docker/<component-name>/`
2. Add a `Dockerfile` following the pattern:
   - Multi-stage build (builder + runtime)
   - Binary at `/component`
   - Metadata at `/metadata.json`
3. Update `COMPONENT_PATHS` in `build-and-push.sh`
4. Ensure component has a `metadata.json` file

## Troubleshooting

### Build fails with "go.mod not found"

Ensure the source directory structure matches expectations:
- `go/go.mod` and `go/go.sum` for Go modules
- `metadata.json` at the component root

### Push fails with "access denied"

1. Verify Docker Hub login: `docker login`
2. Check you have push access to the `withobsrvr` organization
3. Ensure the image tag is correct

### Binary not found at /component

The Dockerfile expects to build from `main.go` in the source directory. Verify:
- Go source files exist
- Build command produces output

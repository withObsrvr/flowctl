#!/usr/bin/env bash
# Build duckdb-consumer binary inside Docker for glibc compatibility
# This is needed because DuckDB requires CGO and the binary must be linked
# against the same glibc as the runtime container.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FLOWCTL_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Component paths - must be set via environment variables
# Example:
#   export DUCKDB_CONSUMER_PATH=/path/to/duckdb-consumer
#   export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk
#   export FLOW_PROTO_PATH=/path/to/flow-proto

if [ -z "${DUCKDB_CONSUMER_PATH}" ] || [ -z "${FLOWCTL_SDK_PATH}" ] || [ -z "${FLOW_PROTO_PATH}" ]; then
    echo "ERROR: Required environment variables not set"
    echo ""
    echo "Please set the following environment variables:"
    echo "  DUCKDB_CONSUMER_PATH - Path to duckdb-consumer source"
    echo "  FLOWCTL_SDK_PATH     - Path to flowctl-sdk source"
    echo "  FLOW_PROTO_PATH      - Path to flow-proto source"
    echo ""
    echo "Example:"
    echo "  export DUCKDB_CONSUMER_PATH=/path/to/ttp-processor-demo/duckdb-consumer"
    echo "  export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk"
    echo "  export FLOW_PROTO_PATH=/path/to/flow-proto"
    echo "  $0"
    exit 1
fi

echo "Building duckdb-consumer in Docker..."
echo "  Source: $DUCKDB_CONSUMER_PATH"
echo "  SDK: $FLOWCTL_SDK_PATH"
echo "  Proto: $FLOW_PROTO_PATH"

# Create a temporary go.mod without local replace directives
# We'll use bind mounts instead

# Build in Docker with bind mounts
docker run --rm \
    -v "$DUCKDB_CONSUMER_PATH/go:/src/duckdb-consumer" \
    -v "$FLOWCTL_SDK_PATH:/src/flowctl-sdk" \
    -v "$FLOW_PROTO_PATH:/src/flow-proto" \
    -v "$DUCKDB_CONSUMER_PATH/bin:/output" \
    -w /src/duckdb-consumer \
    golang:1.24-bookworm \
    bash -c '
        # Update replace directives to use container paths
        cp go.mod go.mod.bak
        # Replace any local paths with container paths
        sed -i "s|replace github.com/withObsrvr/flowctl-sdk =>.*|replace github.com/withObsrvr/flowctl-sdk => /src/flowctl-sdk|g" go.mod
        sed -i "s|replace github.com/withObsrvr/flow-proto =>.*|replace github.com/withObsrvr/flow-proto => /src/flow-proto|g" go.mod

        # Build
        echo "Running go mod tidy..."
        go mod tidy

        echo "Building binary..."
        CGO_ENABLED=1 go build -ldflags="-w -s" -o /output/duckdb-consumer .

        # Restore original go.mod
        mv go.mod.bak go.mod

        echo "Build complete!"
        ls -la /output/duckdb-consumer
    '

echo ""
echo "Binary built at: $DUCKDB_CONSUMER_PATH/bin/duckdb-consumer"

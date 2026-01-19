#!/usr/bin/env bash
# Build contract-events-processor binary inside Docker for glibc compatibility
# This ensures the binary is linked against the same glibc as the runtime container.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FLOWCTL_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Component paths - must be set via environment variables
# Example:
#   export CONTRACT_EVENTS_PATH=/path/to/contract-events-processor-sdk
#   export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk
#   export FLOW_PROTO_PATH=/path/to/flow-proto

if [ -z "${CONTRACT_EVENTS_PATH}" ] || [ -z "${FLOWCTL_SDK_PATH}" ] || [ -z "${FLOW_PROTO_PATH}" ]; then
    echo "ERROR: Required environment variables not set"
    echo ""
    echo "Please set the following environment variables:"
    echo "  CONTRACT_EVENTS_PATH - Path to contract-events-processor-sdk source"
    echo "  FLOWCTL_SDK_PATH     - Path to flowctl-sdk source"
    echo "  FLOW_PROTO_PATH      - Path to flow-proto source"
    echo ""
    echo "Example:"
    echo "  export CONTRACT_EVENTS_PATH=/path/to/ttp-processor-demo/contract-events-processor-sdk"
    echo "  export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk"
    echo "  export FLOW_PROTO_PATH=/path/to/flow-proto"
    echo "  $0"
    exit 1
fi

echo "Building contract-events-processor in Docker..."
echo "  Source: $CONTRACT_EVENTS_PATH"
echo "  SDK: $FLOWCTL_SDK_PATH"
echo "  Proto: $FLOW_PROTO_PATH"

# Build in Docker with bind mounts
docker run --rm \
    -v "$CONTRACT_EVENTS_PATH/go:/src/contract-events-processor" \
    -v "$FLOWCTL_SDK_PATH:/src/flowctl-sdk" \
    -v "$FLOW_PROTO_PATH:/src/flow-proto" \
    -v "$SCRIPT_DIR/bin:/output" \
    -w /src/contract-events-processor \
    golang:1.24-bookworm \
    bash -c '
        # Backup original go.mod
        cp go.mod go.mod.bak

        # Add replace directives for local builds
        echo "" >> go.mod
        echo "replace github.com/withObsrvr/flowctl-sdk => /src/flowctl-sdk" >> go.mod
        echo "replace github.com/withObsrvr/flow-proto => /src/flow-proto" >> go.mod

        # Build
        echo "Running go mod tidy..."
        go mod tidy

        echo "Building binary..."
        CGO_ENABLED=0 go build -ldflags="-w -s" -o /output/contract-events-processor .

        # Restore original go.mod
        mv go.mod.bak go.mod

        echo "Build complete!"
        ls -la /output/contract-events-processor
    '

echo ""
echo "Binary built at: $SCRIPT_DIR/bin/contract-events-processor"

#!/bin/bash
# Build stellar-live-source binary inside Docker for glibc compatibility
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Component paths - must be set via environment variables
# Example:
#   export STELLAR_SOURCE_PATH=/path/to/stellar-live-source-sdk
#   export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk
#   export FLOW_PROTO_PATH=/path/to/flow-proto

if [ -z "${STELLAR_SOURCE_PATH}" ] || [ -z "${FLOWCTL_SDK_PATH}" ] || [ -z "${FLOW_PROTO_PATH}" ]; then
    echo "ERROR: Required environment variables not set"
    echo ""
    echo "Please set the following environment variables:"
    echo "  STELLAR_SOURCE_PATH - Path to stellar-live-source-sdk source"
    echo "  FLOWCTL_SDK_PATH    - Path to flowctl-sdk source"
    echo "  FLOW_PROTO_PATH     - Path to flow-proto source"
    echo ""
    echo "Example:"
    echo "  export STELLAR_SOURCE_PATH=/path/to/ttp-processor-demo/stellar-live-source-sdk"
    echo "  export FLOWCTL_SDK_PATH=/path/to/flowctl-sdk"
    echo "  export FLOW_PROTO_PATH=/path/to/flow-proto"
    echo "  $0"
    exit 1
fi

# Create output directory
mkdir -p "${STELLAR_SOURCE_PATH}/go/bin"

echo "Building stellar-live-source in Docker..."
echo "  Source: ${STELLAR_SOURCE_PATH}"
echo "  SDK: ${FLOWCTL_SDK_PATH}"
echo "  Proto: ${FLOW_PROTO_PATH}"

docker run --rm \
    -v "${STELLAR_SOURCE_PATH}/go:/src/stellar-live-source" \
    -v "${FLOWCTL_SDK_PATH}:/src/flowctl-sdk" \
    -v "${FLOW_PROTO_PATH}:/src/flow-proto" \
    -w /src/stellar-live-source \
    golang:1.25-bookworm \
    bash -c '
        # Fix replace directive for container paths
        sed -i "s|../../../flowctl-sdk|/src/flowctl-sdk|g" go.mod

        echo "Running go mod tidy..."
        go mod tidy 2>&1 || true

        echo "Building binary..."
        CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/stellar-live-source .

        echo "Build complete!"
        ls -la bin/
    '

echo ""
echo "Binary built at: ${STELLAR_SOURCE_PATH}/go/bin/stellar-live-source"

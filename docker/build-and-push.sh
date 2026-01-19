#!/usr/bin/env bash
# build-and-push.sh - Build and push flowctl component images to Docker Hub
#
# Usage:
#   ./docker/build-and-push.sh                    # Build and push all components
#   ./docker/build-and-push.sh stellar-live-source  # Build specific component
#   ./docker/build-and-push.sh --build-only       # Build without pushing
#   ./docker/build-and-push.sh --skip-go-build    # Skip Go build, use existing binaries
#
# Prerequisites:
#   - Docker installed and running
#   - Logged into Docker Hub: docker login
#   - Component source code available at expected paths
#   - Go 1.25+ (for building binaries)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FLOWCTL_DIR="$(dirname "$SCRIPT_DIR")"

# Configuration
REGISTRY="${REGISTRY:-docker.io/withobsrvr}"
VERSION="${VERSION:-v1.0.0}"

# Component source paths - MUST be set via environment variable
# Example: export COMPONENTS_BASE=/path/to/ttp-processor-demo
if [ -z "${COMPONENTS_BASE}" ]; then
    echo "ERROR: COMPONENTS_BASE environment variable is required"
    echo "Set it to the directory containing component source code (e.g., ttp-processor-demo)"
    echo ""
    echo "Example:"
    echo "  export COMPONENTS_BASE=/path/to/ttp-processor-demo"
    echo "  $0"
    exit 1
fi

declare -A COMPONENT_PATHS=(
    ["stellar-live-source"]="stellar-live-source-sdk"
    ["duckdb-consumer"]="duckdb-consumer"
)

declare -A COMPONENT_BUILD_DIRS=(
    ["stellar-live-source"]="go"
    ["duckdb-consumer"]="go"
)

declare -A COMPONENT_BINARY_PATHS=(
    ["stellar-live-source"]="go/bin/stellar-live-source"
    ["duckdb-consumer"]="bin/duckdb-consumer"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

build_go_binary() {
    local component=$1
    local source_dir="${COMPONENTS_BASE}/${COMPONENT_PATHS[$component]}"
    local build_dir="${source_dir}/${COMPONENT_BUILD_DIRS[$component]}"
    local binary_path="${source_dir}/${COMPONENT_BINARY_PATHS[$component]}"

    log_info "Building Go binary for ${component}..."

    if [ ! -d "$build_dir" ]; then
        log_error "Build directory not found: $build_dir"
        return 1
    fi

    # Create output directory
    mkdir -p "$(dirname "$binary_path")"

    # Build with CGO disabled for static binary
    (
        cd "$build_dir"
        log_info "  Building in: $build_dir"

        # Use GOWORK=off to avoid workspace issues
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOWORK=off go build \
            -ldflags="-w -s" \
            -o "$binary_path" \
            .
    )

    if [ -f "$binary_path" ]; then
        log_info "  Binary created: $binary_path"
        ls -lh "$binary_path"
    else
        log_error "  Failed to create binary"
        return 1
    fi
}

build_docker_image() {
    local component=$1
    local source_dir="${COMPONENTS_BASE}/${COMPONENT_PATHS[$component]}"
    local dockerfile="${SCRIPT_DIR}/${component}/Dockerfile"
    local image="${REGISTRY}/${component}:${VERSION}"
    local binary_path="${source_dir}/${COMPONENT_BINARY_PATHS[$component]}"

    log_info "Building Docker image for ${component}..."

    if [ ! -d "$source_dir" ]; then
        log_error "Source directory not found: $source_dir"
        return 1
    fi

    if [ ! -f "$dockerfile" ]; then
        log_error "Dockerfile not found: $dockerfile"
        return 1
    fi

    if [ ! -f "$binary_path" ]; then
        log_error "Binary not found: $binary_path"
        log_error "Run without --skip-go-build to build the binary first"
        return 1
    fi

    log_info "  Source: $source_dir"
    log_info "  Binary: $binary_path"
    log_info "  Dockerfile: $dockerfile"
    log_info "  Image: $image"

    docker build \
        --platform linux/amd64 \
        -t "$image" \
        -f "$dockerfile" \
        "$source_dir"

    log_info "Successfully built $image"
}

push_component() {
    local component=$1
    local image="${REGISTRY}/${component}:${VERSION}"

    log_info "Pushing ${image}..."
    docker push "$image"
    log_info "Successfully pushed $image"
}

build_all() {
    for component in "${!COMPONENT_PATHS[@]}"; do
        if [ "$SKIP_GO_BUILD" != true ]; then
            build_go_binary "$component"
        fi
        build_docker_image "$component"
    done
}

push_all() {
    for component in "${!COMPONENT_PATHS[@]}"; do
        push_component "$component"
    done
}

show_usage() {
    cat <<EOF
Usage: $0 [OPTIONS] [COMPONENT]

Build and push flowctl component images to Docker Hub.

Options:
    --build-only      Build images without pushing
    --push-only       Push pre-built images (assumes they exist)
    --skip-go-build   Skip Go build, use existing binaries
    --list            List available components
    -h, --help        Show this help message

Components:
    stellar-live-source    Stellar blockchain live data source
    duckdb-consumer        DuckDB sink for writing events

Environment Variables:
    REGISTRY        Docker registry (default: docker.io/withobsrvr)
    VERSION         Image version tag (default: v1.0.0)
    COMPONENTS_BASE Base directory for component source code

Examples:
    $0                              # Build Go + Docker, then push all
    $0 stellar-live-source          # Build and push specific component
    $0 --build-only                 # Build all without pushing
    $0 --skip-go-build              # Use existing binaries
    VERSION=v1.1.0 $0               # Build with custom version
EOF
}

list_components() {
    echo "Available components:"
    for component in "${!COMPONENT_PATHS[@]}"; do
        echo "  - $component"
        echo "      Source: ${COMPONENTS_BASE}/${COMPONENT_PATHS[$component]}"
        echo "      Binary: ${COMPONENT_BINARY_PATHS[$component]}"
    done
}

# Parse arguments
BUILD_ONLY=false
PUSH_ONLY=false
SKIP_GO_BUILD=false
COMPONENT=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --build-only)
            BUILD_ONLY=true
            shift
            ;;
        --push-only)
            PUSH_ONLY=true
            shift
            ;;
        --skip-go-build)
            SKIP_GO_BUILD=true
            shift
            ;;
        --list)
            list_components
            exit 0
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        -*)
            log_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
        *)
            COMPONENT=$1
            shift
            ;;
    esac
done

# Main logic
log_info "Registry: $REGISTRY"
log_info "Version: $VERSION"
log_info "Components base: $COMPONENTS_BASE"
log_info "Skip Go build: $SKIP_GO_BUILD"
echo

if [ -n "$COMPONENT" ]; then
    # Build/push specific component
    if [ -z "${COMPONENT_PATHS[$COMPONENT]}" ]; then
        log_error "Unknown component: $COMPONENT"
        list_components
        exit 1
    fi

    if [ "$PUSH_ONLY" = true ]; then
        push_component "$COMPONENT"
    elif [ "$BUILD_ONLY" = true ]; then
        if [ "$SKIP_GO_BUILD" != true ]; then
            build_go_binary "$COMPONENT"
        fi
        build_docker_image "$COMPONENT"
    else
        if [ "$SKIP_GO_BUILD" != true ]; then
            build_go_binary "$COMPONENT"
        fi
        build_docker_image "$COMPONENT"
        push_component "$COMPONENT"
    fi
else
    # Build/push all components
    if [ "$PUSH_ONLY" = true ]; then
        push_all
    elif [ "$BUILD_ONLY" = true ]; then
        build_all
    else
        build_all
        echo
        log_info "All images built successfully. Pushing to registry..."
        echo
        push_all
    fi
fi

echo
log_info "Done!"

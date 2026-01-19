# Dockerfile for duckdb-consumer component
# Multi-stage build: builds the Go binary with CGO and packages it
#
# Build context should be the flowctl directory:
#   docker build -t docker.io/withobsrvr/duckdb-consumer:v1.0.0 \
#     -f docker/duckdb-consumer/Dockerfile \
#     --build-arg COMPONENTS_BASE=/path/to/components .
#
# Or with pre-built binary:
#   docker build -t docker.io/withobsrvr/duckdb-consumer:v1.0.0 \
#     -f docker/duckdb-consumer/Dockerfile /path/to/duckdb-consumer

# Build stage
FROM golang:1.24-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set up workspace
WORKDIR /build

# Copy all required source directories
# Note: Adjust paths based on your setup
COPY . /build/

# Note: This Dockerfile expects the binary to be pre-built using build-in-docker.sh
# The build context should be the duckdb-consumer directory with:
#   - bin/duckdb-consumer (the compiled binary)
#   - metadata.json (component metadata)

# Runtime stage - use debian slim for glibc compatibility
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create app directory
WORKDIR /app

# Copy the pre-built binary
# Note: Build the binary with: CGO_ENABLED=1 go build -o bin/duckdb-consumer .
# on a glibc-based system (not NixOS) to ensure compatibility
COPY bin/duckdb-consumer /component

# Copy metadata
COPY metadata.json /metadata.json

# Set environment defaults
ENV DUCKDB_PATH=/data/stellar.duckdb
ENV PORT=:50053
ENV HEALTH_PORT=8090

# Create data directory
RUN mkdir -p /data

# The binary is the entrypoint
ENTRYPOINT ["/component"]

# Multi-stage Dockerfile for Flowctl Components
#
# This template builds a component following the Flowctl Component Image Specification.
# See: docs/component-image-spec.md
#
# Usage:
# 1. Replace COMPONENT_NAME with your component's binary name
# 2. Update the OCI labels with your component information
# 3. Adjust the build path if your main.go is not in cmd/COMPONENT_NAME
#
# Build command:
#   docker build -t ghcr.io/YOUR_ORG/YOUR_COMPONENT:VERSION .
#
# Multi-platform build:
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -t ghcr.io/YOUR_ORG/YOUR_COMPONENT:VERSION --push .

###############################################################################
# Stage 1: Build the component binary
###############################################################################
FROM golang:1.25-alpine AS builder

# Install build dependencies (if needed)
# RUN apk add --no-cache git

WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# Replace 'COMPONENT_NAME' with your actual component binary name
# Example: stellar-source, ttp-processor, duckdb-consumer
ARG COMPONENT_NAME=your-component
ARG VERSION=dev
ARG COMMIT_SHA=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.CommitSHA=${COMMIT_SHA}" \
    -o ${COMPONENT_NAME} \
    ./cmd/${COMPONENT_NAME}

# Make binary executable
RUN chmod +x ${COMPONENT_NAME}

###############################################################################
# Stage 2: Create the runtime image
###############################################################################
FROM gcr.io/distroless/base-debian12

# Copy binary from builder stage
# The binary will be placed in /app/ following the spec
ARG COMPONENT_NAME=your-component
COPY --from=builder /build/${COMPONENT_NAME} /app/${COMPONENT_NAME}

###############################################################################
# OCI Labels (REQUIRED - Update these for your component)
###############################################################################
LABEL org.opencontainers.image.title="Your Component Title"
LABEL org.opencontainers.image.description="Description of what your component does"
LABEL org.opencontainers.image.version="1.0.0"
LABEL org.opencontainers.image.source="https://github.com/YOUR_ORG/YOUR_REPO"
LABEL org.opencontainers.image.authors="Your Name <you@example.com>"
LABEL org.opencontainers.image.documentation="https://github.com/YOUR_ORG/YOUR_REPO/blob/main/README.md"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Flowctl-specific labels (REQUIRED)
LABEL io.flowctl.component.type="source"
LABEL io.flowctl.component.api-version="v1"

# Optional flowctl labels
LABEL io.flowctl.component.capabilities="streaming,batch"

###############################################################################
# Port Configuration (if your component exposes ports)
###############################################################################
# Health check endpoint (common pattern)
# EXPOSE 8081

# Main service port (if applicable)
# EXPOSE 8080

###############################################################################
# Runtime Configuration
###############################################################################
# Run as non-root user (security best practice)
USER nobody:nobody

# Set working directory
WORKDIR /app

# Entrypoint - dynamically set based on build arg
ARG COMPONENT_NAME=your-component
ENTRYPOINT /app/${COMPONENT_NAME}

# Default command arguments (can be overridden)
CMD []

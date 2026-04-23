.PHONY: all generate proto build build-all install test clean run server deps fmt lint help

SHELL := /usr/bin/env bash

# Version information for reproducible release builds.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/withobsrvr/flowctl/cmd/version.Version=$(VERSION) \
	-X github.com/withobsrvr/flowctl/cmd/version.Commit=$(COMMIT) \
	-X github.com/withobsrvr/flowctl/cmd/version.BuildDate=$(BUILD_DATE)

# Default target
all: generate build

# Generate protobuf files
generate: proto

proto:
	@echo "Generating protobuf files..."
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto
	@echo "Protobuf generation complete!"

# Build the binary
build:
	@echo "Building flowctl $(VERSION)..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/flowctl .
	@echo "Binary built at bin/flowctl"

# Build release binaries for common platforms
build-all:
	@echo "Building release binaries for linux/darwin amd64/arm64..."
	@mkdir -p dist
	@for goos in linux darwin; do \
		for goarch in amd64 arm64; do \
			out="dist/flowctl_$(VERSION)_$${goos}_$${goarch}"; \
			echo "  -> $$out"; \
			CGO_ENABLED=0 GOOS=$${goos} GOARCH=$${goarch} go build -ldflags "$(LDFLAGS)" -o "$$out" .; \
		done; \
	done

install:
	@echo "Installing flowctl $(VERSION)..."
	@CGO_ENABLED=0 go install -ldflags "$(LDFLAGS)" .
	@echo "Installed flowctl to $$GOPATH/bin (or ~/go/bin)"

# Run tests
# Use a filesystem-based package list so local runtime directories (for example
# unreadable data mounts under ./data) do not break package discovery.
test:
	@echo "Running tests..."
	@GOFILES=$$(find . \
		\( -path './.git' -o -path './data' -o -path './bin' -o -path './internal/testfixtures/source' -o -path './internal/testfixtures/processor' -o -path './internal/testfixtures/sink' \) -prune \
		-o -name '*.go' -print); \
		if [ -n "$$GOFILES" ]; then \
			PKGS=$$(printf '%s\n' "$$GOFILES" | xargs -n1 dirname | sort -u | sed 's#^#./#' | sed 's#\.//#./#'); \
			go test -v $$PKGS; \
		else \
			echo "No Go packages found."; \
		fi

# Run the application
run:
	@go run ./cmd/flowctl $(ARGS)

# Run the server
server:
	@go run ./cmd/flowctl server

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/ dist/
	@go clean
	@echo "Clean complete!"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Install protoc plugins
install-proto-tools:
	@echo "Installing protobuf Go plugins..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Protobuf tools installed!"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Formatting complete!"

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run
	@echo "Linting complete!"

# Help target
help:
	@echo "Available targets:"
	@echo "  make generate    - Generate protobuf files"
	@echo "  make build       - Build the flowctl binary"
	@echo "  make build-all   - Build release binaries for common platforms"
	@echo "  make install     - Install flowctl to GOPATH/bin"
	@echo "  make test        - Run tests"
	@echo "  make run         - Run flowctl (use ARGS='server' for server mode)"
	@echo "  make server      - Run flowctl server"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make deps        - Install Go dependencies"
	@echo "  make fmt         - Format Go code"
	@echo "  make lint        - Lint Go code"
	@echo "  make help        - Show this help message"
.PHONY: generate build test clean run proto

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
	@echo "Building flowctl..."
	@go build -o bin/flowctl ./cmd/flowctl
	@echo "Binary built at bin/flowctl"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run the application
run:
	@go run ./cmd/flowctl $(ARGS)

# Run the server
server:
	@go run ./cmd/flowctl server

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
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
	@echo "  make test        - Run tests"
	@echo "  make run         - Run flowctl (use ARGS='server' for server mode)"
	@echo "  make server      - Run flowctl server"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make deps        - Install Go dependencies"
	@echo "  make fmt         - Format Go code"
	@echo "  make lint        - Lint Go code"
	@echo "  make help        - Show this help message"
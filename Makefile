.PHONY: build test clean

# Build the binary
build:
	CGO_ENABLED=0 go build -o bin/flowctl

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod tidy

# Run the example pipeline
run-example: build
	./bin/flowctl run examples/minimal.yaml

# Build for multiple platforms
build-all: clean
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/flowctl-linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/flowctl-linux-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/flowctl-darwin-amd64
.PHONY: all generate proto proto-lint proto-breaking build build-all install test test-race clean run server deps fmt lint vulncheck verify-repro help

SHELL := /usr/bin/env bash

# Version information for reproducible release builds.
#
# BUILD_DATE is the *commit* date, not the wall clock: BUILD-3 requires release
# builds to embed no build time other than the commit's, so that building the
# same tag twice produces the same bytes. SOURCE_DATE_EPOCH overrides it, per the
# reproducible-builds.org convention.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
# Derived from the commit, never the wall clock. Note this deliberately does not
# read an ambient SOURCE_DATE_EPOCH: the Nix dev shell exports 315532800 (1980),
# which would stamp every local build with a date that tells you nothing. The
# commit date is both reproducible and meaningful.
COMMIT_EPOCH := $(shell git log -1 --pretty=%ct 2>/dev/null || echo 0)
BUILD_DATE ?= $(shell date -u -d @$(COMMIT_EPOCH) +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -r $(COMMIT_EPOCH) +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)
export SOURCE_DATE_EPOCH = $(COMMIT_EPOCH)
# -trimpath keeps absolute paths from the build machine out of the binary.
GOFLAGS_REPRO := -trimpath
LDFLAGS := -s -w \
	-X github.com/withobsrvr/flowctl/cmd/version.Version=$(VERSION) \
	-X github.com/withobsrvr/flowctl/cmd/version.Commit=$(COMMIT) \
	-X github.com/withobsrvr/flowctl/cmd/version.BuildDate=$(BUILD_DATE)

# Default target
all: generate build

# Generate protobuf files
generate: proto

# buf is the single generator (see buf.gen.yaml). Plugin versions are pinned
# there, so this produces the same bytes on every machine and in CI — which is
# what lets CI check that the committed output is current (DATA-5).
BUF ?= go run github.com/bufbuild/buf/cmd/buf@v1.47.2

proto:
	@echo "Generating protobuf files..."
	@$(BUF) generate
	@echo "Protobuf generation complete!"

# Lint and breaking-change checks for the protobuf contracts (VER-5).
proto-lint:
	@$(BUF) lint

proto-breaking:
	@latest=$$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null); \
	if [ -z "$$latest" ]; then echo "No release tag; skipping."; exit 0; fi; \
	echo "Comparing protobuf against $$latest"; \
	$(BUF) breaking --against ".git#tag=$$latest"

# Build the binary
build:
	@echo "Building flowctl $(VERSION)..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build $(GOFLAGS_REPRO) -ldflags "$(LDFLAGS)" -o bin/flowctl .
	@echo "Binary built at bin/flowctl"

# Build release binaries for common platforms
build-all:
	@echo "Building release binaries for linux/darwin amd64/arm64..."
	@mkdir -p dist
	@for goos in linux darwin; do \
		for goarch in amd64 arm64; do \
			out="dist/flowctl_$(VERSION)_$${goos}_$${goarch}"; \
			echo "  -> $$out"; \
			CGO_ENABLED=0 GOOS=$${goos} GOARCH=$${goarch} go build $(GOFLAGS_REPRO) -ldflags "$(LDFLAGS)" -o "$$out" .; \
		done; \
	done

install:
	@echo "Installing flowctl $(VERSION)..."
	@CGO_ENABLED=0 go install $(GOFLAGS_REPRO) -ldflags "$(LDFLAGS)" .
	@echo "Installed flowctl to $$GOPATH/bin (or ~/go/bin)"

# Package list for the test targets.
#
# Built from the filesystem rather than ./... so that a local runtime directory
# — an unreadable ./data mount, say — cannot break package discovery. The
# testfixtures subpackages carry a `test` build tag and are excluded here for
# the same reason `go list ./...` omits them.
GOFILES = $(shell find . \
	\( -path './.git' -o -path './data' -o -path './bin' -o -path './dist' \
	   -o -path './internal/testfixtures/source' -o -path './internal/testfixtures/processor' \
	   -o -path './internal/testfixtures/sink' \) -prune \
	-o -name '*.go' -print)
PKGS = $(shell printf '%s\n' $(GOFILES) | xargs -r -n1 dirname | sort -u | sed 's#^#./#' | sed 's#\.//#./#')

# Run tests
test:
	@echo "Running tests..."
	@go test -v $(PKGS)

# Run tests with the race detector (TEST-4). The runner and control plane are
# heavily concurrent, so this is a merge gate, not an optional extra.
test-race:
	@echo "Running tests with -race..."
	@go test -race $(PKGS)

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

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Formatting complete!"

# Lint code. These are the §9 merge-gate checks; CI runs the same set.
lint:
	@echo "Checking formatting..."
	@test -z "$$(gofmt -l . | grep -v '^data/')" || { echo "gofmt needed:"; gofmt -l . | grep -v '^data/'; exit 1; }
	@echo "Running go vet..."
	@go vet ./...
	@echo "Running staticcheck..."
	@staticcheck ./...
	@echo "Linting complete!"

# Scan dependencies for known vulnerabilities (SEC-9). Findings that genuinely
# do not affect flowctl are exempted one at a time, with a justification, in
# .vex/flowctl.openvex.json (SEC-12) — never by ignoring the exit code.
vulncheck:
	@./scripts/govulncheck.sh

# Verify a release build is reproducible: build twice, compare hashes (BUILD-3).
verify-repro:
	@echo "Building twice and comparing..."
	@$(MAKE) --no-print-directory build && sha256sum bin/flowctl > /tmp/flowctl.sha1
	@rm -f bin/flowctl
	@$(MAKE) --no-print-directory build && sha256sum bin/flowctl | sed 's#bin/flowctl#bin/flowctl#' > /tmp/flowctl.sha2
	@diff <(cut -d" " -f1 /tmp/flowctl.sha1) <(cut -d" " -f1 /tmp/flowctl.sha2) \
		&& echo "Reproducible: hashes match" \
		|| { echo "NOT reproducible: hashes differ"; exit 1; }

# Help target
help:
	@echo "Available targets:"
	@echo "  make generate    - Generate protobuf files (via buf)"
	@echo "  make proto-lint  - Lint the protobuf contracts"
	@echo "  make build       - Build the flowctl binary"
	@echo "  make build-all   - Build release binaries for common platforms"
	@echo "  make install     - Install flowctl to GOPATH/bin"
	@echo "  make test        - Run tests"
	@echo "  make test-race   - Run tests with the race detector"
	@echo "  make lint        - gofmt, go vet and staticcheck (the CI merge gate)"
	@echo "  make vulncheck   - Scan dependencies with govulncheck"
	@echo "  make verify-repro- Build twice and compare hashes"
	@echo "  make run         - Run flowctl (use ARGS='server' for server mode)"
	@echo "  make server      - Run flowctl server"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make deps        - Install Go dependencies"
	@echo "  make fmt         - Format Go code"
	@echo "  make help        - Show this help message"
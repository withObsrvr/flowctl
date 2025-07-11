# Build System Architecture: Local vs Remote

## Overview

The intelligent build system operates in a **hybrid architecture** with both local and remote capabilities:

- **Remote Build Service**: Handles component registry builds (publishing)
- **Local Build Capability**: Handles development and local testing
- **flowctl CLI**: Orchestrates both local and remote builds

## Architecture Components

```
┌─────────────────────────────────────────────────────────────────┐
│                        Developer Machine                         │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────┐    ┌──────────────┐  │
│  │   flowctl CLI   │    │ Local Builder   │    │   Docker     │  │
│  │                 │    │                 │    │   Runtime    │  │
│  │ - component     │◄──►│ - Language      │◄──►│              │  │
│  │   build         │    │   detection     │    │ - Build      │  │
│  │ - bundle build  │    │ - Native builds │    │   containers │  │
│  │ - dev mode      │    │ - Local caching │    │ - Run tests  │  │
│  └─────────────────┘    └─────────────────┘    └──────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                    │
                              HTTPS │ API Calls
                                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Component Registry                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────┐    ┌──────────────┐  │
│  │  Registry API   │    │  Build Service  │    │   Artifact   │  │
│  │                 │    │                 │    │   Storage    │  │
│  │ - Component     │◄──►│ - Multi-lang    │◄──►│              │  │
│  │   metadata      │    │   builders      │    │ - Binaries   │  │
│  │ - Version mgmt  │    │ - Multi-platform│    │ - Containers │  │
│  │ - Downloads     │    │ - Build queue   │    │ - Signatures │  │
│  └─────────────────┘    └─────────────────┘    └──────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                    │
                              GitHub│ Webhooks
                                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                       GitHub Repository                          │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  Source Code + component.yaml/bundle.yaml                  │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

### flowctl CLI (Local)

**Primary Role**: Development workflow orchestration

```bash
# Local development builds
flowctl component build              # Build locally for testing
flowctl component test               # Run tests locally
flowctl dev pipeline.yaml           # Local development mode

# Publishing (triggers remote builds)
flowctl component publish           # Publish to registry (remote build)
flowctl bundle publish              # Publish bundle to registry (remote build)

# Component management
flowctl component install           # Download pre-built artifacts
flowctl bundle install              # Download bundle artifacts
```

**Local Capabilities:**
- Language detection and basic builds
- Development mode with hot reload
- Local testing and validation
- Artifact installation and caching

**What flowctl CLI Contains:**
```go
// flowctl includes basic build capabilities
package build

type LocalBuilder struct {
    detectors map[string]LanguageDetector
    builders  map[string]SimpleBuilder
    cache     LocalCache
}

// Simple build implementations for development
type SimpleBuilder interface {
    Detect(path string) bool
    Build(ctx context.Context, path string) error
    Test(ctx context.Context, path string) error
}
```

### Remote Build Service (Registry)

**Primary Role**: Production-grade multi-platform builds for the registry

**Capabilities:**
- Multi-language, multi-platform builds
- Advanced caching and optimization
- Container image generation
- Artifact signing and distribution
- Build queuing and scaling

**Triggered By:**
- Component/bundle publishing
- GitHub webhooks (tags, releases)
- Manual build requests

## Build Workflows

### 1. Local Development Workflow

```bash
# Developer working on a component
cd my-stellar-source/

# Local build for testing (flowctl CLI handles this)
flowctl component build
# ✓ Detects Go project
# ✓ Runs: go mod download && go build
# ✓ Creates local binary for testing

# Test locally
flowctl component test
# ✓ Runs component tests
# ✓ Validates component interface

# Local development mode
flowctl dev my-pipeline.yaml --mode native
# ✓ Uses locally built component
# ✓ Hot reload on changes
# ✓ No registry interaction needed
```

**Local Builder Implementation:**
```go
// In flowctl CLI codebase
func (cli *CLI) buildComponent(path string) error {
    // Simple language detection
    detector := &build.LanguageDetector{}
    lang := detector.Detect(path)
    
    // Use simple local builder
    builder := cli.localBuilders[lang]
    return builder.Build(context.Background(), path)
}

// Simple Go builder for local development
type LocalGoBuilder struct{}

func (b *LocalGoBuilder) Build(ctx context.Context, path string) error {
    cmd := exec.CommandContext(ctx, "go", "build", "./...")
    cmd.Dir = path
    return cmd.Run()
}
```

### 2. Publishing Workflow (Remote Builds)

```bash
# Developer ready to publish
flowctl component publish --namespace myorg

# flowctl CLI:
# 1. Validates component locally
# 2. Pushes to GitHub (if not already there)
# 3. Calls registry API to register component
# 4. Registry API triggers remote build service

# Remote Build Service:
# 1. Clones repository
# 2. Advanced language detection
# 3. Multi-platform builds (linux/amd64, linux/arm64, darwin/amd64)
# 4. Container image generation
# 5. Artifact signing and storage
# 6. Registry metadata update
```

**Publishing Flow:**
```bash
flowctl component publish --namespace stellar
```

Internally:
```go
func (cli *CLI) publishComponent(namespace string) error {
    // 1. Local validation
    if err := cli.validateComponent("."); err != nil {
        return err
    }
    
    // 2. Call registry API
    registryClient := registry.NewClient(cli.config.RegistryURL)
    publishReq := &registry.PublishRequest{
        Namespace:  namespace,
        Repository: detectGitRepo("."),
        Version:    detectVersion("."),
    }
    
    // 3. Registry handles the rest
    return registryClient.PublishComponent(publishReq)
}
```

### 3. Installation Workflow

```bash
# User wants to use a component
flowctl component install stellar/stellar-source@v1.2.3

# flowctl CLI:
# 1. Calls registry API for component metadata
# 2. Downloads pre-built artifacts (native + container)
# 3. Caches locally for immediate use
# 4. Component ready for use in pipelines
```

## Local Builder vs Remote Builder

### Local Builder (in flowctl CLI)

**Purpose**: Fast development iteration

```go
// Simple, fast builders for development
type LocalBuilder interface {
    Build(path string) error        // Basic build
    Test(path string) error         // Run tests
    Clean(path string) error        // Clean artifacts
}

// Example: Local Go builder
type LocalGoBuilder struct{}

func (b *LocalGoBuilder) Build(path string) error {
    // Simple go build for development
    cmd := exec.Command("go", "build", "./...")
    cmd.Dir = path
    return cmd.Run()
}
```

**Characteristics:**
- Fast and simple
- Single platform (developer's machine)
- Basic error handling
- No advanced optimization
- Good enough for development

### Remote Builder (Registry Service)

**Purpose**: Production-grade, multi-platform builds

```go
// Advanced builder in registry service
type RemoteBuilder interface {
    BuildMultiPlatform(spec BuildSpec) (*MultiPlatformArtifact, error)
    BuildContainer(spec BuildSpec) (*ContainerImage, error)
    SignArtifact(artifact *Artifact) error
}

// Example: Advanced Go builder
type RemoteGoBuilder struct {
    cache      BuildCache
    platforms  []Platform
    signingKey *ecdsa.PrivateKey
}

func (b *RemoteGoBuilder) BuildMultiPlatform(spec BuildSpec) (*MultiPlatformArtifact, error) {
    // Build for multiple platforms
    // Advanced caching
    // Optimization
    // Cross-compilation
    // Security scanning
}
```

**Characteristics:**
- Comprehensive and robust
- Multi-platform builds
- Advanced caching and optimization
- Security scanning and signing
- Production-ready artifacts

## Development vs Production Artifacts

### Development (Local Builds)

```bash
# Local build output
my-component/
├── my-component               # Simple binary for local testing
├── go.mod
├── main.go
└── component.yaml
```

### Production (Registry Builds)

```bash
# Registry build output
stellar/stellar-source@v1.2.3:
├── binaries/
│   ├── stellar-source-linux-amd64      # Native binary
│   ├── stellar-source-linux-arm64      # ARM64 binary
│   ├── stellar-source-darwin-amd64     # macOS binary
│   └── stellar-source.sig              # Signature
├── containers/
│   ├── stellar-source:v1.2.3-amd64     # Container image
│   ├── stellar-source:v1.2.3-arm64     # ARM64 container
│   └── manifest.json                   # Multi-arch manifest
└── metadata/
    ├── component.yaml                   # Component spec
    ├── build-info.json                 # Build metadata
    └── security-scan.json              # Security scan results
```

## Configuration

### flowctl Configuration

```yaml
# ~/.flowctl/config.yaml
registry:
  url: "https://registry.flowctl.io"
  auth:
    token: "${FLOWCTL_TOKEN}"

build:
  # Local build settings
  local:
    cache_dir: "~/.flowctl/build-cache"
    parallel_jobs: 4
    
  # Remote build preferences  
  remote:
    platforms: ["linux/amd64", "linux/arm64"]
    sign_artifacts: true
```

### Local Builder Settings

```go
// In flowctl CLI
type LocalBuildConfig struct {
    CacheDir     string
    ParallelJobs int
    Timeout      time.Duration
    Languages    map[string]LanguageConfig
}

type LanguageConfig struct {
    Builder     string   // go, npm, pip, cargo
    BuildArgs   []string // Additional build arguments
    TestCommand string   // Test command override
}
```

## Implementation Strategy

### Phase 1: Local Builder in flowctl CLI
```bash
# Basic local build capabilities
flowctl component build    # Local builds for development
flowctl component test     # Local testing
flowctl dev               # Development mode with local builds
```

### Phase 2: Remote Build Service
```bash
# Registry build service
flowctl component publish  # Triggers remote builds
flowctl bundle publish     # Bundle builds in registry
```

### Phase 3: Optimization
```bash
# Advanced features
flowctl component build --cache      # Advanced local caching
flowctl component build --platforms  # Local multi-platform builds
```

## Key Benefits of This Architecture

### 1. **Fast Local Development**
- No need for complex build infrastructure on developer machines
- Simple, fast builds for immediate testing
- Works offline for development

### 2. **Production-Grade Publishing**
- Multi-platform builds handled by registry
- Advanced optimization and security
- Scalable build infrastructure

### 3. **Separation of Concerns**
- flowctl CLI focuses on developer experience
- Registry service focuses on production builds
- Each optimized for their use case

### 4. **Flexibility**
- Developers can build locally for testing
- Registry provides optimized artifacts for production
- Both use same component specifications

## Summary

**flowctl CLI includes basic local build capabilities** for development workflow, but the **intelligent build system is primarily a separate service** in the component registry that handles production-grade, multi-platform builds.

This hybrid approach gives developers fast local iteration while ensuring production artifacts are properly built, optimized, and secured by the registry infrastructure.

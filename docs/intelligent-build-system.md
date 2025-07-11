# Intelligent Build System for Multi-Language Components

## Problem Statement

The component registry needs to automatically build components written in different languages (Go, TypeScript, Python, Rust) with different build requirements:

- **Go components**: `go build` with specific flags
- **TypeScript components**: `npm install && npm run build`
- **Python components**: `pip install` with virtual environments
- **Rust components**: `cargo build --release`
- **Multi-language bundles**: Build multiple components with different languages
- **Native vs Container builds**: Same source, different build targets

The registry must automatically detect languages, dependencies, and build requirements from repository structure without requiring complex configuration.

## Solution: Convention-Based Build Detection

### Core Principle: Convention Over Configuration

The build system follows these principles:
1. **Auto-detect language and build requirements** from repository structure
2. **Minimal configuration required** - sensible defaults for everything
3. **Override when needed** - allow custom build configurations
4. **Multi-target builds** - produce both native binaries and container images
5. **Dependency caching** - fast builds through intelligent caching

## Build Detection Algorithm

### 1. Language Detection

The build system scans repository structure to detect languages:

```
Repository Structure Analysis:
├── go.mod + *.go files          → Go project
├── package.json + *.ts files    → TypeScript project  
├── package.json + *.js files    → JavaScript project
├── requirements.txt + *.py      → Python project
├── Cargo.toml + *.rs files      → Rust project
├── Dockerfile                   → Custom container build
└── component.yaml/bundle.yaml   → Explicit configuration
```

**Detection Priority:**
1. Explicit configuration in `component.yaml`
2. Primary language based on file count and structure
3. Multi-language detection for bundles
4. Fallback to Dockerfile if present

### 2. Build Configuration Detection

For each detected language, the system looks for standard build configurations:

**Go Projects:**
```
Detection:
├── go.mod                    → Go modules project
├── main.go                   → Entry point detection
├── cmd/*/main.go             → Multi-binary project
├── Makefile                  → Custom build commands
└── .goreleaser.yml           → Release configuration

Default Build:
- go mod download
- go build -o <binary-name> ./cmd/<component> OR .
- CGO_ENABLED=0 for static binaries
```

**TypeScript/JavaScript Projects:**
```
Detection:
├── package.json              → npm/yarn project
├── tsconfig.json             → TypeScript configuration
├── webpack.config.js         → Webpack build
├── rollup.config.js          → Rollup build
├── vite.config.js            → Vite build
└── yarn.lock vs package-lock.json → Package manager

Default Build:
- npm install OR yarn install
- npm run build OR yarn build
- npm run test (if test script exists)
```

**Python Projects:**
```
Detection:
├── requirements.txt          → pip dependencies
├── pyproject.toml            → Modern Python project
├── setup.py                  → Legacy Python project
├── poetry.lock               → Poetry project
├── Pipfile                   → Pipenv project
└── conda.yml                 → Conda environment

Default Build:
- python -m venv .venv
- source .venv/bin/activate
- pip install -r requirements.txt
- python -m pytest (if tests exist)
```

**Rust Projects:**
```
Detection:
├── Cargo.toml                → Rust project
├── Cargo.lock                → Dependency lock
├── src/main.rs               → Binary project
├── src/lib.rs                → Library project
└── .cargo/config.toml        → Cargo configuration

Default Build:
- cargo build --release
- cargo test
- Target-specific builds for cross-compilation
```

## Build System Architecture

### Build Service Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Build Orchestrator                       │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────────┐  ┌───────────────┐  ┌─────────────────┐  │
│  │   Language    │  │   Artifact    │  │    Cache        │  │
│  │   Detector    │  │   Builder     │  │   Manager       │  │
│  └───────────────┘  └───────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────────┐  ┌───────────────┐  ┌─────────────────┐  │
│  │   Go Builder  │  │ TypeScript    │  │  Python Builder │  │
│  │               │  │ Builder       │  │                 │  │
│  └───────────────┘  └───────────────┘  └─────────────────┘  │
│  ┌───────────────┐  ┌───────────────┐  ┌─────────────────┐  │
│  │ Rust Builder  │  │  Container    │  │  Multi-Language │  │
│  │               │  │  Builder      │  │  Bundle Builder │  │
│  └───────────────┘  └───────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Build Process Flow

```
1. Repository Analysis
   ├── Clone repository at specific version
   ├── Scan directory structure
   ├── Detect languages and frameworks
   ├── Parse component.yaml/bundle.yaml
   └── Generate build plan

2. Dependency Resolution
   ├── Cache check for dependencies
   ├── Language-specific dependency installation
   ├── Cross-platform dependency handling
   └── Dependency caching for future builds

3. Multi-Target Build
   ├── Native binary build (Go, Rust direct; others via runtime)
   ├── Container image build (all languages)
   ├── Platform-specific builds (linux/amd64, linux/arm64, darwin/amd64)
   └── Artifact signing and verification

4. Artifact Packaging
   ├── Native binaries with metadata
   ├── Container images with tags
   ├── Multi-architecture manifests
   └── Registry upload with signatures
```

## Language-Specific Build Implementations

### Go Builder

```go
type GoBuilder struct {
    cache      BuildCache
    goVersion  string
}

func (g *GoBuilder) Detect(repo *Repository) bool {
    return repo.HasFile("go.mod") && repo.HasGoFiles()
}

func (g *GoBuilder) Build(ctx context.Context, spec BuildSpec) (*Artifact, error) {
    // 1. Detect build configuration
    config := g.detectGoConfig(spec.Repository)
    
    // 2. Install dependencies with caching
    if err := g.installDependencies(ctx, config); err != nil {
        return nil, err
    }
    
    // 3. Build native binary
    nativeBinary, err := g.buildNative(ctx, config)
    if err != nil {
        return nil, err
    }
    
    // 4. Build container image
    containerImage, err := g.buildContainer(ctx, config, nativeBinary)
    if err != nil {
        return nil, err
    }
    
    return &Artifact{
        Native:    nativeBinary,
        Container: containerImage,
        Metadata:  config.Metadata,
    }, nil
}

func (g *GoBuilder) detectGoConfig(repo *Repository) *GoConfig {
    config := &GoConfig{
        GoVersion: g.extractGoVersion(repo.File("go.mod")),
        Module:    g.extractModuleName(repo.File("go.mod")),
    }
    
    // Detect main package
    if repo.HasFile("main.go") {
        config.MainPackage = "."
    } else if mainPkgs := repo.FindMainPackages(); len(mainPkgs) > 0 {
        config.MainPackage = mainPkgs[0] // or build all
    }
    
    // Detect build flags
    if makefile := repo.File("Makefile"); makefile != nil {
        config.BuildFlags = g.extractBuildFlags(makefile)
    }
    
    return config
}

func (g *GoBuilder) buildNative(ctx context.Context, config *GoConfig) (*NativeBinary, error) {
    cmd := exec.CommandContext(ctx, "go", "build",
        "-o", config.BinaryName,
        "-ldflags", config.LDFlags,
        config.MainPackage,
    )
    cmd.Env = append(os.Environ(),
        "CGO_ENABLED=0",           // Static binary
        "GOOS="+config.TargetOS,   // Target OS
        "GOARCH="+config.TargetArch, // Target architecture
    )
    
    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("go build failed: %w", err)
    }
    
    return &NativeBinary{
        Path:     config.BinaryName,
        OS:       config.TargetOS,
        Arch:     config.TargetArch,
        Language: "go",
    }, nil
}
```

### TypeScript Builder

```go
type TypeScriptBuilder struct {
    cache       BuildCache
    nodeVersion string
}

func (t *TypeScriptBuilder) Detect(repo *Repository) bool {
    return repo.HasFile("package.json") && 
           (repo.HasFile("tsconfig.json") || repo.HasTSFiles())
}

func (t *TypeScriptBuilder) Build(ctx context.Context, spec BuildSpec) (*Artifact, error) {
    config := t.detectTSConfig(spec.Repository)
    
    // 1. Install dependencies
    if err := t.installDependencies(ctx, config); err != nil {
        return nil, err
    }
    
    // 2. Build TypeScript
    if err := t.buildTypeScript(ctx, config); err != nil {
        return nil, err
    }
    
    // 3. Create native runtime wrapper
    nativeWrapper, err := t.createNativeWrapper(ctx, config)
    if err != nil {
        return nil, err
    }
    
    // 4. Build container
    containerImage, err := t.buildContainer(ctx, config)
    if err != nil {
        return nil, err
    }
    
    return &Artifact{
        Native:    nativeWrapper,
        Container: containerImage,
        Metadata:  config.Metadata,
    }, nil
}

func (t *TypeScriptBuilder) createNativeWrapper(ctx context.Context, config *TSConfig) (*NativeBinary, error) {
    // Create a shell script wrapper for native execution
    wrapper := fmt.Sprintf(`#!/bin/bash
cd %s
exec node %s "$@"
`, config.BuildDir, config.EntryPoint)
    
    wrapperPath := filepath.Join(config.BuildDir, config.ComponentName)
    if err := os.WriteFile(wrapperPath, []byte(wrapper), 0755); err != nil {
        return nil, err
    }
    
    return &NativeBinary{
        Path:     wrapperPath,
        Language: "typescript",
        Runtime:  "node",
        WorkDir:  config.BuildDir,
    }, nil
}
```

### Python Builder

```go
type PythonBuilder struct {
    cache         BuildCache
    pythonVersion string
}

func (p *PythonBuilder) Build(ctx context.Context, spec BuildSpec) (*Artifact, error) {
    config := p.detectPythonConfig(spec.Repository)
    
    // 1. Create virtual environment
    venvPath, err := p.createVirtualEnv(ctx, config)
    if err != nil {
        return nil, err
    }
    
    // 2. Install dependencies
    if err := p.installDependencies(ctx, config, venvPath); err != nil {
        return nil, err
    }
    
    // 3. Create native wrapper
    nativeWrapper, err := p.createNativeWrapper(ctx, config, venvPath)
    if err != nil {
        return nil, err
    }
    
    // 4. Build container
    containerImage, err := p.buildContainer(ctx, config)
    if err != nil {
        return nil, err
    }
    
    return &Artifact{
        Native:    nativeWrapper,
        Container: containerImage,
        Metadata:  config.Metadata,
    }, nil
}

func (p *PythonBuilder) createNativeWrapper(ctx context.Context, config *PythonConfig, venvPath string) (*NativeBinary, error) {
    // Create activation script and Python runner
    wrapper := fmt.Sprintf(`#!/bin/bash
source %s/bin/activate
cd %s
exec python %s "$@"
`, venvPath, config.SourceDir, config.EntryPoint)
    
    wrapperPath := filepath.Join(config.BuildDir, config.ComponentName)
    if err := os.WriteFile(wrapperPath, []byte(wrapper), 0755); err != nil {
        return nil, err
    }
    
    return &NativeBinary{
        Path:     wrapperPath,
        Language: "python",
        Runtime:  "python",
        VenvPath: venvPath,
        WorkDir:  config.SourceDir,
    }, nil
}
```

## Bundle Build Orchestration

### Multi-Component Build

For bundles like ttp-processor-demo with multiple languages:

```go
type BundleBuilder struct {
    builders map[string]LanguageBuilder
}

func (b *BundleBuilder) BuildBundle(ctx context.Context, bundle *ComponentBundle) (*BundleArtifact, error) {
    artifact := &BundleArtifact{
        Bundle:     bundle,
        Components: make(map[string]*ComponentArtifact),
    }
    
    // Build each component in parallel
    var wg sync.WaitGroup
    errChan := make(chan error, len(bundle.Components))
    
    for _, comp := range bundle.Components {
        wg.Add(1)
        go func(component ComponentDef) {
            defer wg.Done()
            
            // Select appropriate builder
            builder := b.builders[component.Language]
            if builder == nil {
                errChan <- fmt.Errorf("no builder for language: %s", component.Language)
                return
            }
            
            // Build component
            compArtifact, err := builder.Build(ctx, BuildSpec{
                Repository: bundle.Repository,
                Component:  component,
                Path:       component.Path,
            })
            if err != nil {
                errChan <- fmt.Errorf("failed to build %s: %w", component.Name, err)
                return
            }
            
            artifact.Components[component.Name] = compArtifact
        }(comp)
    }
    
    wg.Wait()
    close(errChan)
    
    // Check for errors
    if err := <-errChan; err != nil {
        return nil, err
    }
    
    // Package bundle artifacts
    return b.packageBundle(ctx, artifact)
}
```

## Intelligent Caching

### Multi-Level Caching Strategy

```go
type BuildCache struct {
    dependencyCache  map[string]*DependencySet
    buildCache      map[string]*BuildArtifact
    layerCache      map[string]*ContainerLayer
}

// Language-specific dependency caching
func (c *BuildCache) GetGoDependencies(goMod []byte) (*DependencySet, bool) {
    hash := sha256.Sum256(goMod)
    key := fmt.Sprintf("go-deps-%x", hash)
    return c.dependencyCache[key], c.dependencyCache[key] != nil
}

func (c *BuildCache) GetNodeDependencies(packageJSON, packageLock []byte) (*DependencySet, bool) {
    hash := sha256.Sum256(append(packageJSON, packageLock...))
    key := fmt.Sprintf("node-deps-%x", hash)
    return c.dependencyCache[key], c.dependencyCache[key] != nil
}

// Build result caching
func (c *BuildCache) GetBuild(sourceHash, configHash string) (*BuildArtifact, bool) {
    key := fmt.Sprintf("build-%s-%s", sourceHash, configHash)
    return c.buildCache[key], c.buildCache[key] != nil
}
```

## Build Configuration Override

### Component-Level Build Customization

Components can override default build behavior:

```yaml
# component.yaml
apiVersion: component.flowctl.io/v1
kind: ComponentSpec
metadata:
  name: custom-stellar-source
spec:
  type: source
  
  # Custom build configuration
  build:
    language: go
    
    # Override default build commands
    native:
      commands:
        - go mod download
        - go generate ./...
        - go build -tags=production -o stellar-source ./cmd/source
      environment:
        CGO_ENABLED: "1"  # Enable CGO for this component
        CC: gcc
        
    # Custom container build
    container:
      dockerfile: |
        FROM golang:1.21-alpine AS builder
        WORKDIR /src
        COPY go.mod go.sum ./
        RUN go mod download
        COPY . .
        RUN go build -o stellar-source ./cmd/source
        
        FROM alpine:latest
        RUN apk --no-cache add ca-certificates
        COPY --from=builder /src/stellar-source /usr/local/bin/
        ENTRYPOINT ["stellar-source"]
        
    # Build-time dependencies
    dependencies:
      system:
        - build-essential
        - pkg-config
      build:
        - protoc
        - protoc-gen-go
```

### Bundle-Level Build Orchestration

```yaml
# bundle.yaml
spec:
  # Shared build configuration
  build:
    parallel: true
    cache: aggressive
    
    # Shared dependencies
    dependencies:
      system:
        - ca-certificates
        - curl
      build:
        - protoc
        - protoc-gen-go
        - protoc-gen-go-grpc
    
    # Build order (if not parallel)
    order:
      - stellar-live-source    # Build first (provides proto definitions)
      - ttp-processor         # Depends on proto files
      - consumer-app          # Can build in parallel with processor
      
  # Component-specific overrides
  components:
    - name: stellar-live-source
      build:
        environment:
          GOFLAGS: "-buildmode=plugin"
          
    - name: ttp-processor
      build:
        commands:
          - npm ci
          - npm run generate-proto  # Custom proto generation
          - npm run build
```

## Registry Build API

### Build Triggering

```bash
# Manual build trigger
curl -X POST https://registry.flowctl.io/v1/components/stellar/stellar-source/build \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "version": "v1.2.3",
    "platforms": ["linux/amd64", "linux/arm64", "darwin/amd64"],
    "targets": ["native", "container"]
  }'

# Webhook-triggered builds (GitHub integration)
# Automatically triggered on:
# - Git tag creation (v1.2.3)
# - Pull request (build for testing)
# - Main branch push (build latest)
```

### Build Status and Logs

```bash
# Check build status
curl https://registry.flowctl.io/v1/components/stellar/stellar-source/builds/12345

# Stream build logs
curl -N https://registry.flowctl.io/v1/components/stellar/stellar-source/builds/12345/logs

# Download artifacts
curl https://registry.flowctl.io/v1/components/stellar/stellar-source/v1.2.3/download?platform=linux/amd64&target=native
```

## Benefits

### 1. **Zero Configuration for Standard Projects**
- Automatic language detection
- Sensible build defaults
- Standard project structure support

### 2. **Multi-Language Bundle Support**
- Build Go + TypeScript + Python in single operation
- Parallel builds for performance
- Shared dependency caching

### 3. **Multi-Target Artifacts**
- Native binaries for development
- Container images for production
- Cross-platform builds

### 4. **Intelligent Caching**
- Dependency-level caching
- Build result caching
- Container layer caching

### 5. **Flexible Override System**
- Component-level customization
- Bundle-level orchestration
- Environment-specific builds

## Implementation Roadmap

### Phase 1: Core Build Detection (Month 1)
- [ ] Language detection algorithms
- [ ] Go, TypeScript, Python builders
- [ ] Basic dependency caching
- [ ] Native binary generation

### Phase 2: Advanced Build Features (Month 2)
- [ ] Rust and additional language support
- [ ] Container image building
- [ ] Multi-platform builds
- [ ] Bundle build orchestration

### Phase 3: Optimization and Caching (Month 3)
- [ ] Advanced caching strategies
- [ ] Build parallelization
- [ ] Performance optimization
- [ ] Build analytics and monitoring

This intelligent build system ensures that flowctl can automatically build and package components regardless of their language or complexity, while providing flexibility for custom build requirements.
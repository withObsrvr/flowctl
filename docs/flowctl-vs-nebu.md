# Flowctl vs Nebu: Understanding the Ecosystem

## Executive Summary

**flowctl** and **nebu** are complementary tools in the Stellar data processing ecosystem, each designed for different use cases and development stages. While they share similar goals of processing blockchain data through composable pipelines, they take fundamentally different architectural approaches.

**TL;DR:**
- **nebu**: Lightweight, Unix-philosophy tool for rapid prototyping and simple pipelines
- **flowctl**: Production-grade orchestrator for complex, monitored, multi-component systems

Think of them as **bash scripts vs systemd** - both valuable, serving different needs.

---

## Project Overviews

### Flowctl: Production Pipeline Orchestrator

**Purpose**: Full-featured data pipeline orchestrator with embedded control plane for managing complex, production-grade data processing systems.

**Key Characteristics:**
- Process/container lifecycle management
- Embedded gRPC control plane for component coordination
- Health monitoring, heartbeats, and service discovery
- Component registration and stream management
- Kubernetes-style YAML configuration
- Deployment translation (Docker Compose, local execution)
- Built-in observability (Prometheus metrics, structured logging, dashboard)

**Architecture Pattern:**
```
┌─────────────────────────────────────┐
│   flowctl CLI / Orchestrator        │
│  • Control Plane (gRPC)             │
│  • Component Registry & Discovery   │
│  • Health Monitoring                │
│  • Stream Management                │
└──────────────┬──────────────────────┘
               │
    ┌──────────┼──────────┐
    ▼          ▼          ▼
  Source → Processor → Sink
   (Data)  (Transform) (Store)
  Producer  Logic      Data
```

**Target Use Cases:**
- Production deployments requiring high availability
- Complex multi-stage pipelines with multiple processors
- Systems requiring centralized monitoring and observability
- Pipelines with stateful components
- Environments needing process lifecycle management

**Example Projects:**
- **cdp-pipeline-workflow**: 70+ processors, 40+ consumers, production Stellar data processing
- **ttp-processor-demo**: Microservices architecture for Token Transfer Protocol processing

---

### Nebu: Minimal Streaming Runtime

**Purpose**: Lightweight, IDL-first streaming runtime for rapid prototyping and composable data extraction from Stellar.

**Key Characteristics:**
- Unix philosophy: "Do one thing well"
- Standalone processor binaries
- stdin/stdout communication with newline-delimited JSON
- CLI-first developer experience
- Minimal orchestration overhead
- Shell-scriptable pipelines
- Registry-based processor discovery

**Architecture Pattern:**
```
┌───────────────────┐
│   Stellar RPC     │  (Data source)
└────────┬──────────┘
         │ LedgerCloseMeta (XDR)
         ▼
┌────────────────────┐
│      ORIGIN        │  (e.g., token-transfer)
│  (extracts events) │  Standalone binary
└────────┬───────────┘
         │ Newline-delimited JSON (stdin/stdout)
         ▼
┌────────────────────┐
│     TRANSFORM      │  (e.g., filters, dedup)
│  (filters/modify)  │  Optional processing
└────────┬───────────┘
         │ Filtered events
         ▼
┌────────────────────┐
│       SINK         │  (e.g., json-file, DuckDB)
│  (stores/outputs)  │  Final destination
└────────────────────┘
```

**Target Use Cases:**
- Rapid prototyping and experimentation
- Ad-hoc data queries and exploration
- Simple, single-purpose pipelines
- Local development and testing
- Learning Stellar data structures
- Shell-scriptable automation

**Example Usage:**
```bash
# Simple token transfer tracking
nebu fetch 100 200 | token-transfer | usdc-filter | json-file-sink

# Pipe to DuckDB for analytics
nebu fetch 100 200 | token-transfer | duckdb analytics.db
```

---

## Detailed Comparison

### Philosophy and Design

| Aspect | flowctl | nebu |
|--------|---------|------|
| **Philosophy** | Orchestrator-managed components | Unix pipes and composition |
| **Communication** | gRPC streaming | stdin/stdout (newline-delimited JSON) |
| **Process Model** | Managed by control plane | Independent binaries |
| **Configuration** | YAML pipeline definitions | Command-line flags and pipes |
| **State Management** | Centralized in control plane | Processor-local or stateless |
| **Observability** | Built-in metrics, logging, dashboard | Processor-specific, lightweight |

### Component Lifecycle

**flowctl:**
```yaml
# flowctl manages the entire lifecycle
pipeline:
  name: stellar-indexer
  sources:
    - name: stellar-rpc
      type: stellar-rpc-source
      config:
        endpoint: https://soroban-testnet.stellar.org
  processors:
    - name: token-processor
      type: token-transfer-processor
  sinks:
    - name: postgres
      type: postgres-sink
```

Components:
- Started/stopped by flowctl
- Register with control plane on startup
- Maintain heartbeat connections
- Report health status continuously

**nebu:**
```bash
# Developer composes binaries via shell
nebu fetch 100 200 | \
  token-transfer | \
  usdc-filter --min-amount 1000 | \
  json-file-sink output.jsonl
```

Components:
- Run as independent processes
- Exit when stdin closes
- No central coordination required
- Simple process exit codes for errors

### Development Workflow

**flowctl Development Cycle:**
1. Define pipeline in YAML
2. Implement component with flowctl SDK/interfaces
3. Build component (supports gRPC control plane integration)
4. Deploy via flowctl (manages process startup)
5. Monitor via control plane dashboard
6. Iterate with configuration changes

**nebu Development Cycle:**
1. Create processor binary with simple interface
2. Implement stdin/stdout processing
3. Test locally with nebu fetch
4. Compose with other processors via pipes
5. Iterate quickly with recompilation
6. Deploy to production (optional: wrap for flowctl)

### Complexity vs Simplicity

**flowctl Handles:**
- Multi-component coordination
- Service discovery
- Health checks and automatic restarts
- Distributed logging aggregation
- Metrics collection and export
- Complex dependency graphs (DAG support)
- Multiple deployment targets

**nebu Handles:**
- Simple data transformation
- Quick experimentation
- CLI-friendly interfaces
- Minimal setup and configuration
- Direct processor composition
- Fast iteration cycles

---

## How They Complement Each Other

### 1. Development Lifecycle Synergy

The ideal workflow leverages both tools:

```
Development → Testing → Production
   nebu    →   nebu   →   flowctl
```

**Phase 1: Prototype with nebu**
- Quick experimentation
- Test processor logic
- Validate data transformations
- Explore Stellar data structures

```bash
# Rapid prototyping
nebu fetch 1000 1100 | token-transfer | jq '.amount'
```

**Phase 2: Test with nebu**
- Integration testing with real data
- Performance validation
- Output verification

```bash
# Test full pipeline locally
nebu fetch 10000 11000 | \
  token-transfer | \
  usdc-filter | \
  json-file-sink test-output.jsonl
```

**Phase 3: Deploy with flowctl**
- Move to production orchestration
- Add monitoring and alerting
- Scale with multiple instances
- Enable health checks

```yaml
# Production deployment with flowctl
pipeline:
  name: usdc-tracker-prod
  sources:
    - name: stellar-mainnet
      type: stellar-rpc-source
  processors:
    - name: token-transfers
      type: token-transfer
    - name: usdc-only
      type: usdc-filter
  sinks:
    - name: postgres-prod
      type: postgres-sink
```

### 2. Processor Portability

Processors can be designed to work with both systems:

**Dual-Compatible Processor Design:**
```go
// Processor can work standalone (nebu mode)
func main() {
    if os.Getenv("FLOWCTL_CONTROL_PLANE") != "" {
        // Register with flowctl control plane
        runWithFlowctl()
    } else {
        // Run in standalone mode (nebu)
        runStandalone()
    }
}
```

**Benefits:**
- Develop with nebu's simple interface
- Deploy to flowctl without rewriting
- Test locally without orchestration
- Production-ready when needed

### 3. Use Case Matrix

| Scenario | Recommended Tool | Rationale |
|----------|-----------------|-----------|
| **Quick data exploration** | nebu | Minimal setup, pipe to jq/DuckDB |
| **Ad-hoc analytics** | nebu | Shell-scriptable, fast iteration |
| **Local development** | nebu | No control plane needed |
| **Learning Stellar data** | nebu | Lower barrier to entry |
| **Production pipeline** | flowctl | Health monitoring, orchestration |
| **Multi-stage transformation** | flowctl | Better for complex DAGs |
| **Stateful processing** | flowctl | Centralized state management |
| **High availability systems** | flowctl | Automatic restarts, health checks |
| **Microservices architecture** | flowctl | Service discovery, coordination |
| **Simple event extraction** | nebu | Less overhead, faster |
| **Complex observability needs** | flowctl | Built-in metrics, logging |
| **Prototyping new processors** | nebu | Faster development cycle |

### 4. Migration Path

**Graduating from nebu to flowctl:**

```bash
# Step 1: Develop with nebu
nebu fetch 100 200 | my-processor | json-file-sink

# Step 2: Test standalone processor
./my-processor < ledger-data.jsonl > output.jsonl

# Step 3: Add flowctl integration
# Modify my-processor to support control plane registration

# Step 4: Deploy with flowctl
flowctl run --config production-pipeline.yaml
```

**Key Adaptation Points:**
1. Add control plane registration
2. Implement health check endpoint
3. Convert stdin/stdout to gRPC streams (if needed)
4. Add structured logging
5. Expose Prometheus metrics

---

## Technical Integration Points

### Shared Technologies

Both projects leverage:
- **Go**: Primary implementation language
- **gRPC/Protocol Buffers**: Data serialization (flowctl for control plane, nebu internally)
- **Stellar Go SDK**: XDR processing, ledger structures
- **Nix**: Reproducible builds

### Communication Patterns

**flowctl:**
- gRPC bidirectional streaming between components
- Control plane uses `flow-proto` protocol definitions
- Components register streams with control plane
- Health checks via gRPC health service

**nebu:**
- Newline-delimited JSON on stdin/stdout
- Stellar RPC client for ledger fetching
- Simple process pipes for composition
- Exit codes for error signaling

### Data Formats

**flowctl:**
- Flexible: supports custom gRPC message types
- Typed events via Protocol Buffers
- XDR unmarshaling in source components

**nebu:**
- Standardized: newline-delimited JSON
- Schema versioning for compatibility
- XDR to JSON conversion in origin processors

---

## Real-World Examples

### cdp-pipeline-workflow (flowctl-based)

**Architecture:**
- 70+ processor types (payments, contracts, DEX trades, etc.)
- 40+ consumer types (PostgreSQL, DuckDB, Redis, Parquet, etc.)
- Multiple data sources (Captive Core, S3, GCS, RPC, DuckLake)
- Production-grade with monitoring and metrics

**Why flowctl?**
- Complex multi-component orchestration required
- Health monitoring for 24/7 operation
- Multiple processors need coordination
- Production observability essential

**Could nebu replace it?**
- No - too complex for simple pipes
- Requires centralized state management
- Needs process lifecycle control
- Benefits from control plane coordination

### ttp-processor-demo (flowctl-integrated)

**Architecture:**
- Microservices: stellar-live-source → ttp-processor → consumers
- gRPC streaming between services
- Multiple consumer implementations (Node.js, Go WASM, Rust WASM)
- Optional flowctl integration for monitoring

**Why flowctl?**
- Microservices need service discovery
- gRPC communication fits flowctl model
- Health monitoring across services
- Control plane provides coordination

**Could nebu work here?**
- Partially - for simple single-processor use cases
- Token transfer extraction could be a nebu origin processor
- Loses microservices benefits
- Better for ad-hoc queries than production

### nebu Use Cases

**Example 1: Quick Token Analysis**
```bash
# Find all USDC transfers over $10,000 in ledger range
nebu fetch 1000000 1001000 | \
  token-transfer | \
  jq 'select(.asset_code == "USDC" and (.amount | tonumber) > 10000)' | \
  wc -l
```

**Example 2: DuckDB Analytics**
```bash
# Load token transfers into DuckDB for analysis
nebu fetch 500000 600000 | \
  token-transfer | \
  duckdb analytics.db -c "
    CREATE TABLE transfers AS
    SELECT * FROM read_json_auto('/dev/stdin')
  "
```

**Example 3: Processor Development**
```bash
# Test new processor logic locally
nebu fetch 100 200 | \
  ./my-new-processor --debug | \
  head -n 10
```

---

## Decision Framework

### Choose nebu when:

✅ You're exploring Stellar data for the first time
✅ You need quick ad-hoc analytics
✅ You're prototyping a new processor idea
✅ Your pipeline is simple (1-3 stages)
✅ You want shell-scriptable automation
✅ You're developing locally without infrastructure
✅ You need minimal setup and configuration
✅ Your processors are stateless

### Choose flowctl when:

✅ You're deploying to production
✅ You need health monitoring and alerting
✅ Your pipeline has complex dependencies (DAG)
✅ You require high availability
✅ You have multiple coordinated components
✅ You need centralized observability
✅ Your processors maintain state
✅ You're building a microservices architecture

### Use both when:

🔄 Developing new processors (nebu) for existing production pipelines (flowctl)
🔄 Running ad-hoc queries (nebu) against production data sources managed by flowctl
🔄 Prototyping features (nebu) before production deployment (flowctl)
🔄 Testing processor logic locally (nebu) before integration (flowctl)

---

## Future Integration Possibilities

### 1. Flowctl Could Support Nebu Processors

**Concept**: Flowctl adapter for nebu-style processors

```yaml
pipeline:
  processors:
    - name: token-filter
      type: nebu-processor
      config:
        binary: ./token-filter
        args: ["--min-amount", "1000"]
      # flowctl wraps stdin/stdout to gRPC
```

**Benefits:**
- Leverage nebu's simple development model
- Deploy nebu processors in flowctl pipelines
- Best of both worlds: simple dev, robust deployment

### 2. Nebu Could Generate Flowctl Configs

**Concept**: Convert nebu pipeline to flowctl YAML

```bash
# Define pipeline with nebu syntax
nebu pipeline create \
  --source stellar-rpc \
  --processor token-transfer \
  --processor usdc-filter \
  --sink postgres \
  --export flowctl > pipeline.yaml

# Deploy with flowctl
flowctl run --config pipeline.yaml
```

**Benefits:**
- Prototype with nebu CLI
- Generate production config automatically
- Smooth transition path

### 3. Shared Processor Registry

**Concept**: Common registry for both ecosystems

```yaml
# processor-registry.yaml
processors:
  - name: token-transfer
    type: origin
    nebu: github.com/stellar/nebu-processors/token-transfer
    flowctl: github.com/stellar/flowctl-components/token-transfer
    description: Extract token transfer events
```

**Benefits:**
- Unified processor discovery
- Clear compatibility matrix
- Easier ecosystem navigation

---

## Ecosystem Positioning

```
┌─────────────────────────────────────────────────────────┐
│                  Stellar Data Processing Ecosystem       │
└─────────────────────────────────────────────────────────┘
                              │
                              │
        ┌─────────────────────┴─────────────────────┐
        │                                           │
        ▼                                           ▼
  ┌──────────┐                               ┌──────────┐
  │   nebu   │                               │ flowctl  │
  └──────────┘                               └──────────┘
  Rapid Proto                                Production
  Simple Pipes                               Orchestration
  CLI-First                                  Control Plane
        │                                           │
        │                                           │
        ▼                                           ▼
  ┌──────────┐                               ┌──────────┐
  │ Use Cases│                               │ Use Cases│
  └──────────┘                               └──────────┘
  • Dev/Test                                 • Production
  • Analytics                                • Multi-component
  • Exploration                              • High Availability
  • Learning                                 • Monitoring
  • Scripts                                  • Coordination
```

**Complementary Strengths:**
- nebu lowers the barrier to entry
- flowctl provides production robustness
- Both enable different parts of the data lifecycle
- Together, they cover development through production

---

## Recommendations

### For New Developers

1. **Start with nebu**:
   - Lower learning curve
   - Faster feedback loops
   - Explore Stellar data structures
   - Build confidence with simple pipelines

2. **Graduate to flowctl**:
   - When pipelines become complex
   - When moving to production
   - When monitoring becomes critical
   - When coordination is needed

### For Production Teams

1. **Use flowctl for production**:
   - Leverage control plane for operations
   - Benefit from built-in observability
   - Handle complex orchestration needs
   - Ensure high availability

2. **Use nebu for operations**:
   - Ad-hoc data queries
   - Debugging production issues
   - One-off data migrations
   - Exploratory analytics

### For Processor Developers

1. **Design for portability**:
   - Start with nebu's simple interface
   - Add flowctl integration when ready
   - Support both modes if possible
   - Document compatibility

2. **Publish to both ecosystems**:
   - List in nebu processor registry
   - Provide flowctl component spec
   - Maintain compatibility layer
   - Enable broader adoption

---

## Conclusion

**flowctl** and **nebu** represent two complementary approaches to Stellar data processing:

- **nebu** excels at developer experience, rapid prototyping, and simplicity
- **flowctl** excels at production orchestration, monitoring, and complex coordination

Rather than competing, they serve different stages of the development lifecycle and different operational needs. The ideal Stellar data processing workflow leverages both:

- **Develop** with nebu's simplicity
- **Test** with nebu's flexibility
- **Deploy** with flowctl's robustness
- **Operate** with flowctl's observability
- **Debug** with nebu's accessibility

Together, they provide a complete toolkit for working with Stellar blockchain data, from first exploration to production-grade systems.

---

## Additional Resources

### flowctl
- **Repository**: /flowctl
- **Documentation**: /flowctl/docs
- **Architecture**: /flowctl/docs/architecture.md
- **Building Components**: /flowctl/docs/building-components.md

### nebu
- **Repository**: /nebu
- **Documentation**: /nebu/README.md
- **Processor Guide**: /nebu/docs/BUILDING_PROCESSORS.md
- **Architecture Decisions**: /nebu/docs/ARCHITECTURE_DECISIONS.md

### Example Projects
- **cdp-pipeline-workflow**: Production flowctl deployment
- **ttp-processor-demo**: Microservices with flowctl integration
- **nebu examples**: /nebu/examples/processors

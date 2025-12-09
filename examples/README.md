# flowctl Examples

This directory contains example pipeline configurations demonstrating various flowctl features and patterns.

## 📚 Navigation

### For Beginners

**Start here if you're new to flowctl:**

1. **[Getting Started Guide](getting-started/README.md)** ⭐ **Start Here!**
   - What is flowctl and how does it work?
   - Core concepts (sources, processors, sinks)
   - Your first pipeline walkthrough
   - Debugging tips and common patterns

### Example Pipelines

#### Deployment Examples

| File | Description | Purpose |
|------|-------------|---------|
| **[sandbox.yaml](sandbox.yaml)** | Sandbox services configuration (Redis, Kafka, PostgreSQL, Prometheus, Grafana) | Service dependencies for development/testing |

### Configuration Resources

| Directory | Contents |
|-----------|----------|
| **[config/](config/)** | Shared configuration files (Prometheus, base configs) |

## 🎯 Quick Reference

### Example by Use Case

**"I want to learn flowctl"**
→ Start with [Getting Started Guide](getting-started/README.md)

**"I want to build a custom pipeline"**
→ Study the [Building Components Guide](../docs/building-components.md) and see real working example at `/home/tillman/Documents/ttp-processor-demo/ttp-pipeline.yaml`

**"I want to see a real production pipeline"**
→ Check out `/home/tillman/Documents/ttp-processor-demo/ttp-pipeline.yaml` - a complete working pipeline with Stellar source, TTP processor, and Node consumer

## 📖 Example Details

### sandbox.yaml

**What it shows:**
- `kind: SandboxServices` format for infrastructure services
- Docker Compose-style service definitions
- Service dependencies (depends_on)
- Health checks for services
- Volume management
- Network configuration

**Services included:**
- Redis (event streaming and caching)
- Kafka + Zookeeper (message queuing)
- PostgreSQL (data storage)
- Prometheus + Pushgateway (metrics)
- Grafana (visualization)

**Use with:**
```bash
./bin/flowctl sandbox start --services examples/sandbox.yaml --pipeline examples/sandbox-pipeline.yaml
```

## 🏗️ Building Custom Pipelines

### Step 1: Choose Your Components

Decide what you need:
- **Source**: Where does data come from? (API, database, stream)
- **Processor**: What transformations are needed? (filter, enrich, aggregate)
- **Sink**: Where does data go? (PostgreSQL, webhooks, files)

### Step 2: Build Components

Use the [flowctl-sdk](https://github.com/withObsrvr/flowctl-sdk) to build components:

```bash
# Clone the SDK
git clone https://github.com/withObsrvr/flowctl-sdk.git

# Study the examples
cd flowctl-sdk/examples/

# Build your component
# See: docs/building-components.md
```

### Step 3: Create Pipeline Configuration

Use this template for your pipeline:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: my-pipeline
  description: My custom pipeline

spec:
  driver: process

  sources:
    - id: my-source
      type: source
      command: ["/path/to/bin/my-source"]
      env:
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: "localhost:50051"
        HEALTH_PORT: "8088"

  processors:
    - id: my-processor
      type: processor
      command: ["/path/to/bin/my-processor"]
      env:
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50052"
        HEALTH_PORT: "8089"

  sinks:
    - id: my-sink
      type: sink
      command: ["/path/to/bin/my-sink"]
      env:
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50053"
        HEALTH_PORT: "8090"
```

**Note:** See `/home/tillman/Documents/ttp-processor-demo/ttp-pipeline.yaml` for a complete working example.

### Step 4: Run and Test

```bash
# Validate configuration
./bin/flowctl run --dry-run my-pipeline.yaml

# Run pipeline
./bin/flowctl run my-pipeline.yaml

# Debug if needed
./bin/flowctl run my-pipeline.yaml --log-level=debug
```

## 📚 Additional Resources

### Documentation

- **[Main README](../README.md)** - flowctl overview and installation
- **[Configuration Guide](../docs/configuration.md)** - Complete schema reference
- **[Building Components Guide](../docs/building-components.md)** - How to build sources, processors, sinks
- **[Getting Started](getting-started/README.md)** - Comprehensive beginner guide

### Real-World Examples

- **[Contract Events Pipeline](https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline)**
  - Complete Stellar contract events → PostgreSQL pipeline
  - Shows: Real components, proper configuration, database integration
  - **< 5 minutes to run from cold start**

### SDK Repository

- **[flowctl-sdk](https://github.com/withObsrvr/flowctl-sdk)**
  - Source package: `pkg/source/`
  - Processor package: `pkg/processor/`
  - Sink packages: `pkg/consumer/`, `pkg/sink/`
  - Complete working examples: `examples/`

## 🎓 Learning Path

**Beginner:**
1. Read [Getting Started Guide](getting-started/README.md)
2. Study the working pipeline at `/home/tillman/Documents/ttp-processor-demo/ttp-pipeline.yaml`

**Intermediate:**
1. Read [Building Components Guide](../docs/building-components.md)
2. Build a simple source or sink
3. Create your own pipeline based on the template above

**Advanced:**
1. Read [Configuration Guide](../docs/configuration.md) for all options
2. Build complete pipelines with multiple components
3. Deploy using Docker or Kubernetes

## ❓ Common Questions

**Q: Where can I find a working example pipeline?**
A: See `/home/tillman/Documents/ttp-processor-demo/ttp-pipeline.yaml` for a complete working pipeline with real components (Stellar source, TTP processor, Node consumer).

**Q: How do I deploy to production?**
A: Build component Docker images, use `driver: docker` in your pipeline spec, and deploy to your orchestration platform. Use `flowctl translate` to generate Docker Compose or Kubernetes configs.

**Q: Where can I find help?**
A: Check [Troubleshooting](../README.md#troubleshooting), open a [GitHub issue](https://github.com/withobsrvr/flowctl/issues), or see [Getting Help](getting-started/README.md#troubleshooting).

## 🔧 Tips

### Configuration Tips

```yaml
# ✅ Good: Use absolute paths for driver: process
command: ["/home/user/bin/my-component"]

# ❌ Bad: Relative paths may fail
command: ["./bin/my-component"]

# ✅ Good: Unique ports for each component
sources:
  - id: source-1
    env:
      PORT: ":50051"
      HEALTH_PORT: "8088"

processors:
  - id: processor-1
    env:
      PORT: ":50052"  # Different
      HEALTH_PORT: "8089"

# ✅ Good: Always specify inputs for processors and sinks
processors:
  - id: my-processor
    inputs: ["source-id"]  # Required!
```

### Debugging Tips

```bash
# Enable verbose logging
./bin/flowctl run pipeline.yaml --log-level=debug

# Check specific component
./bin/flowctl run pipeline.yaml 2>&1 | grep "component-id"

# Validate before running
./bin/flowctl run --dry-run pipeline.yaml

# Check component health while running
curl http://localhost:8088/health
```

### Development Tips

```bash
# Test components standalone before adding to pipeline
ENABLE_FLOWCTL=false PORT=:50051 /path/to/component

# Study the working example
cat /home/tillman/Documents/ttp-processor-demo/ttp-pipeline.yaml

# Build components with flowctl-sdk for proper integration
# github.com/withObsrvr/flowctl-sdk/pkg/{source,processor,consumer}
```

---

**Ready to start?** → [Getting Started Guide](getting-started/README.md)

**Need help?** → [Troubleshooting](../README.md#troubleshooting) | [GitHub Issues](https://github.com/withobsrvr/flowctl/issues)

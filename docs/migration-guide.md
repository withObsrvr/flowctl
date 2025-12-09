# Migration Guide: Old Config Format to flowctl/v1

This guide helps you migrate from the old configuration format to the new `apiVersion: flowctl/v1` standard format.

## Table of Contents

- [Overview](#overview)
- [Key Differences](#key-differences)
- [Migration Steps](#migration-steps)
- [Field Mappings](#field-mappings)
- [Example Migrations](#example-migrations)
- [Common Pitfalls](#common-pitfalls)
- [Testing Your Migration](#testing-your-migration)

## Overview

### Old Format (Deprecated)

```yaml
version: 0.1
log_level: info

source:
  type: mock
  params:
    interval_ms: 1000

processors:
  - name: mock_processor
    plugin: mock
    params:
      pass_through: true

sink:
  type: stdout
  params:
    pretty: true
```

### New Format (Current)

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: my-pipeline
  description: Pipeline description

spec:
  driver: process

  sources:
    - id: mock-source
      command: ["/path/to/source-binary"]
      env:
        INTERVAL_MS: "1000"

  processors:
    - id: mock-processor
      command: ["/path/to/processor-binary"]
      inputs: ["mock-source"]
      env:
        PASS_THROUGH: "true"

  sinks:
    - id: stdout-sink
      command: ["/path/to/sink-binary"]
      inputs: ["mock-processor"]
      env:
        PRETTY: "true"
```

## Key Differences

### 1. API Version and Kind

**Old:**
```yaml
version: 0.1
```

**New:**
```yaml
apiVersion: flowctl/v1
kind: Pipeline
```

**Why:** Kubernetes-style versioning allows for schema evolution and clear API contracts.

### 2. Metadata Section

**Old:** No metadata section

**New:**
```yaml
metadata:
  name: pipeline-name
  description: Optional description
```

**Why:** Proper identification and documentation of pipelines.

### 3. Singular vs. Plural

**Old:**
```yaml
source:  # Singular
  type: mock

sink:    # Singular
  type: stdout
```

**New:**
```yaml
sources:  # Plural, array
  - id: source-1

sinks:    # Plural, array
  - id: sink-1
```

**Why:** Support for multiple sources and sinks (fan-in, fan-out patterns).

### 4. Component Identification

**Old:**
```yaml
processors:
  - name: my-processor  # "name" field
    plugin: mock        # "plugin" field
```

**New:**
```yaml
processors:
  - id: my-processor    # "id" field
    command: [...]      # "command" field (path to binary)
```

**Why:**
- `id` is more consistent
- `command` makes it clear components are separate binaries

### 5. Configuration Method

**Old:**
```yaml
source:
  type: mock
  params:               # "params" object
    interval_ms: 1000
    key: value
```

**New:**
```yaml
sources:
  - id: mock-source
    command: [...]
    env:                # "env" object (environment variables)
      INTERVAL_MS: "1000"
      KEY: "value"
```

**Why:** Environment variables are standard for configuring processes/containers.

### 6. Driver Specification

**Old:** Implicit (always local process)

**New:**
```yaml
spec:
  driver: process  # Explicit: process, docker, kubernetes, nomad
```

**Why:** Support for multiple execution environments.

### 7. Component Connections

**Old:** Implicit (linear flow assumed)

**New:**
```yaml
processors:
  - id: processor-1
    inputs: ["source-id"]  # Explicit connections

sinks:
  - id: sink-1
    inputs: ["processor-id"]  # Explicit connections
```

**Why:** Support for complex topologies (DAG, fan-out, fan-in).

### 8. Log Level

**Old:**
```yaml
log_level: info  # Top-level
```

**New:**
```yaml
# Set via CLI flag or component environment variable
sources:
  - id: source-1
    env:
      LOG_LEVEL: "info"  # Per-component
```

**Why:** Fine-grained control per component.

## Migration Steps

### Step 1: Update Header

**Old:**
```yaml
version: 0.1
log_level: info
```

**New:**
```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: my-pipeline  # Choose a descriptive name
  description: Migrated from old format
```

### Step 2: Add Spec Section with Driver

```yaml
spec:
  driver: process  # For local execution (was default in old format)
```

### Step 3: Migrate Source

**Old:**
```yaml
source:
  type: stellar
  params:
    rpc_endpoint: "https://..."
    start_ledger: 1000
```

**New:**
```yaml
spec:
  sources:  # Note: plural and array
    - id: stellar-source  # Add unique ID
      command: ["/path/to/stellar-source-binary"]  # Must provide binary
      env:  # params → env, UPPERCASE keys
        RPC_ENDPOINT: "https://..."
        START_LEDGER: "1000"
```

### Step 4: Migrate Processors

**Old:**
```yaml
processors:
  - name: event-extractor
    plugin: extract_events
    params:
      event_type: transfer
```

**New:**
```yaml
  processors:
    - id: event-extractor  # name → id
      command: ["/path/to/event-extractor-binary"]  # plugin → command
      inputs: ["stellar-source"]  # ADD: explicit connection to source
      env:  # params → env
        EVENT_TYPE: "transfer"
```

### Step 5: Migrate Sink

**Old:**
```yaml
sink:
  type: postgresql
  params:
    host: localhost
    database: events_db
```

**New:**
```yaml
  sinks:  # Note: plural and array
    - id: postgresql-sink  # Add unique ID
      command: ["/path/to/postgresql-sink-binary"]  # Must provide binary
      inputs: ["event-extractor"]  # ADD: explicit connection
      env:  # params → env, UPPERCASE keys
        POSTGRES_HOST: "localhost"
        POSTGRES_DB: "events_db"
```

### Step 6: Add flowctl Integration

For components built with flowctl-sdk, enable control plane integration:

```yaml
  sources:
    - id: my-source
      command: [...]
      env:
        # Add these for flowctl integration
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50051"
        HEALTH_PORT: "8088"
        # ... other config ...
```

## Field Mappings

### Complete Field Mapping

| Old Format | New Format | Notes |
|------------|------------|-------|
| `version` | `apiVersion` | Use `flowctl/v1` |
| (none) | `kind` | Always `Pipeline` |
| (none) | `metadata.name` | Required |
| (none) | `metadata.description` | Optional |
| `log_level` | (CLI flag or component env) | `--log-level` or `LOG_LEVEL` |
| `source` | `spec.sources` | Singular → plural array |
| `source.type` | `sources[].command` | Type → binary path |
| `source.params` | `sources[].env` | Params → env vars |
| `processors[].name` | `processors[].id` | Name → id |
| `processors[].plugin` | `processors[].command` | Plugin → binary path |
| `processors[].params` | `processors[].env` | Params → env vars |
| (implicit) | `processors[].inputs` | NEW: explicit connections |
| `sink` | `spec.sinks` | Singular → plural array |
| `sink.type` | `sinks[].command` | Type → binary path |
| `sink.params` | `sinks[].env` | Params → env vars |
| (implicit) | `sinks[].inputs` | NEW: explicit connections |
| (none) | `spec.driver` | NEW: `process`, `docker`, etc. |

## Example Migrations

### Example 1: Simple Pipeline

**Before (Old Format):**
```yaml
version: 0.1
log_level: info

source:
  type: data_generator
  params:
    interval_ms: 1000
    output_type: json

processors:
  - name: uppercase_processor
    plugin: uppercase
    params:
      fields: [name, description]

sink:
  type: file_writer
  params:
    output_path: /tmp/output.jsonl
    append: true
```

**After (New Format):**
```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: data-generator-pipeline
  description: Generates data, transforms to uppercase, writes to file

spec:
  driver: process

  sources:
    - id: data-generator
      command: ["/home/user/bin/data-generator"]
      env:
        INTERVAL_MS: "1000"
        OUTPUT_TYPE: "json"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50051"
        HEALTH_PORT: "8088"

  processors:
    - id: uppercase-processor
      command: ["/home/user/bin/uppercase-processor"]
      inputs: ["data-generator"]
      env:
        FIELDS: "name,description"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50052"
        HEALTH_PORT: "8089"

  sinks:
    - id: file-writer
      command: ["/home/user/bin/file-writer"]
      inputs: ["uppercase-processor"]
      env:
        OUTPUT_PATH: "/tmp/output.jsonl"
        APPEND: "true"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50053"
        HEALTH_PORT: "8090"
```

### Example 2: Stellar Pipeline

**Before (Old Format):**
```yaml
version: 0.1
log_level: debug

source:
  type: stellar_ledger_stream
  params:
    rpc_endpoint: "https://soroban-testnet.stellar.org:443"
    network_passphrase: "Test SDF Network ; September 2015"
    start_ledger: 1000000

processors:
  - name: contract_events_extractor
    plugin: extract_contract_events
    params:
      network_passphrase: "Test SDF Network ; September 2015"

sink:
  type: postgresql
  params:
    host: localhost
    port: 5432
    database: stellar_events
    user: postgres
    password: password
```

**After (New Format):**
```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: stellar-contract-events-pipeline
  description: Extracts contract events from Stellar testnet to PostgreSQL

spec:
  driver: process

  sources:
    - id: stellar-live-source
      command: ["/home/user/bin/stellar-live-source"]
      env:
        BACKEND_TYPE: "RPC"
        RPC_ENDPOINT: "https://soroban-testnet.stellar.org:443"
        NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"
        START_LEDGER: "1000000"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: "localhost:50052"
        HEALTH_PORT: "8090"
        LOG_LEVEL: "debug"

  processors:
    - id: contract-events-processor
      command: ["/home/user/bin/contract-events-processor"]
      inputs: ["stellar-live-source"]
      env:
        NETWORK_PASSPHRASE: "Test SDF Network ; September 2015"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50051"
        HEALTH_PORT: "8089"
        LOG_LEVEL: "debug"

  sinks:
    - id: postgresql-consumer
      command: ["/home/user/bin/postgresql-consumer"]
      inputs: ["contract-events-processor"]
      env:
        POSTGRES_HOST: "localhost"
        POSTGRES_PORT: "5432"
        POSTGRES_DB: "stellar_events"
        POSTGRES_USER: "postgres"
        POSTGRES_PASSWORD: "password"
        POSTGRES_SSLMODE: "disable"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":9090"
        HEALTH_PORT: "9088"
        LOG_LEVEL: "debug"
```

### Example 3: Multiple Sinks (Fan-Out)

**Before (Old Format):**
```yaml
# Old format didn't support multiple sinks easily
version: 0.1

source:
  type: api_poller
  params:
    endpoint: "https://api.example.com/events"

processors:
  - name: event_filter
    plugin: filter
    params:
      status: success

sink:
  type: webhook
  params:
    url: "https://hooks.example.com/events"
```

**After (New Format with Fan-Out):**
```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: api-events-fanout-pipeline
  description: Poll API, filter events, send to multiple destinations

spec:
  driver: process

  sources:
    - id: api-poller
      command: ["/home/user/bin/api-poller"]
      env:
        ENDPOINT: "https://api.example.com/events"
        POLL_INTERVAL: "5s"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50051"
        HEALTH_PORT: "8088"

  processors:
    - id: event-filter
      command: ["/home/user/bin/event-filter"]
      inputs: ["api-poller"]
      env:
        STATUS_FILTER: "success"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50052"
        HEALTH_PORT: "8089"

  sinks:
    # NEW: Multiple sinks reading from same processor
    - id: webhook-sink
      command: ["/home/user/bin/webhook-sink"]
      inputs: ["event-filter"]
      env:
        WEBHOOK_URL: "https://hooks.example.com/events"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50053"
        HEALTH_PORT: "8090"

    - id: postgresql-sink
      command: ["/home/user/bin/postgresql-sink"]
      inputs: ["event-filter"]  # Same input as webhook-sink
      env:
        POSTGRES_HOST: "localhost"
        POSTGRES_DB: "events"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50054"
        HEALTH_PORT: "8091"

    - id: file-sink
      command: ["/home/user/bin/file-sink"]
      inputs: ["event-filter"]  # Same input
      env:
        OUTPUT_PATH: "/var/log/events.jsonl"
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:8080"
        PORT: ":50055"
        HEALTH_PORT: "8092"
```

## Common Pitfalls

### Pitfall 1: Forgetting to Pluralize

❌ **Wrong:**
```yaml
spec:
  source:  # Singular
    - id: my-source
```

✅ **Correct:**
```yaml
spec:
  sources:  # Plural
    - id: my-source
```

### Pitfall 2: Missing Component Binaries

The new format requires **actual component binaries**:

❌ **Wrong:**
```yaml
sources:
  - id: mock-source
    type: mock  # Old "type" approach won't work
```

✅ **Correct:**
```yaml
sources:
  - id: mock-source
    command: ["/path/to/bin/mock-source"]  # Must provide binary
```

**Solution:** Build components using [flowctl-sdk](https://github.com/withObsrvr/flowctl-sdk).

### Pitfall 3: Forgetting Explicit Connections

❌ **Wrong:**
```yaml
processors:
  - id: my-processor
    command: [...]
    # Missing inputs!

sinks:
  - id: my-sink
    command: [...]
    # Missing inputs!
```

✅ **Correct:**
```yaml
processors:
  - id: my-processor
    command: [...]
    inputs: ["source-id"]  # Explicit connection

sinks:
  - id: my-sink
    command: [...]
    inputs: ["processor-id"]  # Explicit connection
```

### Pitfall 4: Using Relative Paths

❌ **Wrong (with driver: process):**
```yaml
sources:
  - id: my-source
    command: ["./bin/my-source"]  # Relative path
```

✅ **Correct:**
```yaml
sources:
  - id: my-source
    command: ["/home/user/bin/my-source"]  # Absolute path
```

### Pitfall 5: Lowercase Environment Variables

❌ **Wrong:**
```yaml
env:
  postgres_host: "localhost"  # Lowercase
  postgres_port: "5432"
```

✅ **Correct (Conventional):**
```yaml
env:
  POSTGRES_HOST: "localhost"  # UPPERCASE
  POSTGRES_PORT: "5432"
```

**Note:** Technically lowercase works, but UPPERCASE is conventional for environment variables.

### Pitfall 6: Port Conflicts

❌ **Wrong:**
```yaml
sources:
  - id: source-1
    env:
      PORT: ":50051"

processors:
  - id: processor-1
    env:
      PORT: ":50051"  # Same port as source!
```

✅ **Correct:**
```yaml
sources:
  - id: source-1
    env:
      PORT: ":50051"

processors:
  - id: processor-1
    env:
      PORT: ":50052"  # Different port
```

### Pitfall 7: Missing metadata.name

❌ **Wrong:**
```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  # Missing name!
spec:
  ...
```

✅ **Correct:**
```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: my-pipeline  # Required
spec:
  ...
```

## Testing Your Migration

### Step 1: Validate Configuration

```bash
# Validate without running
./bin/flowctl run --dry-run migrated-pipeline.yaml
```

Look for validation errors about:
- Missing required fields
- Invalid field names
- Schema violations

### Step 2: Check Components Exist

```bash
# Verify all component binaries exist
ls -la /path/to/source-binary
ls -la /path/to/processor-binary
ls -la /path/to/sink-binary

# Verify they're executable
chmod +x /path/to/*-binary
```

### Step 3: Run with Debug Logging

```bash
./bin/flowctl run migrated-pipeline.yaml --log-level=debug
```

Watch for:
- Component registration messages
- Connection establishment
- Data flow between components

### Step 4: Verify Data Flow

```bash
# While pipeline is running, check health endpoints
curl http://localhost:8088/health  # Source
curl http://localhost:8089/health  # Processor
curl http://localhost:8090/health  # Sink

# Check logs for data processing
./bin/flowctl run migrated-pipeline.yaml 2>&1 | grep "event"
```

### Step 5: Compare Output

Run both old and new pipelines side-by-side (if possible) and verify:
- Same events produced
- Same transformations applied
- Same data in sinks

## Migration Checklist

Use this checklist when migrating:

- [ ] Update `version: 0.1` → `apiVersion: flowctl/v1`
- [ ] Add `kind: Pipeline`
- [ ] Add `metadata` section with `name`
- [ ] Add `spec.driver` (usually `process`)
- [ ] Change `source` → `sources` (plural array)
- [ ] Change `sink` → `sinks` (plural array)
- [ ] Change `name` → `id` for all components
- [ ] Change `type`/`plugin` → `command` with binary path
- [ ] Change `params` → `env` with UPPERCASE keys
- [ ] Add `inputs` array to all processors and sinks
- [ ] Add flowctl integration env vars (`ENABLE_FLOWCTL`, etc.)
- [ ] Use unique ports for each component
- [ ] Use absolute paths for binaries (if `driver: process`)
- [ ] Build component binaries if needed (using flowctl-sdk)
- [ ] Validate configuration (`--dry-run`)
- [ ] Test with debug logging
- [ ] Verify data flow

## Getting Help

If you encounter issues during migration:

1. **Validate first**: `./bin/flowctl run --dry-run pipeline.yaml`
2. **Check examples**: See `examples/` directory for working configurations
3. **Read docs**:
   - [Configuration Guide](configuration.md)
   - [Getting Started](../examples/getting-started/README.md)
   - [Building Components](building-components.md)
4. **Ask for help**: Open an issue at https://github.com/withobsrvr/flowctl/issues

## Additional Resources

- **Configuration Reference**: [configuration.md](configuration.md)
- **Component Building**: [building-components.md](building-components.md)
- **Examples**: [../examples/README.md](../examples/README.md)
- **Real-world Demo**: https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline

---

**Ready to migrate?** Start with the [Migration Steps](#migration-steps) above!

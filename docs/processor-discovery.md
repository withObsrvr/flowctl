# Processor Discovery

Discover and inspect processors registered with the flowctl control plane at runtime.

## Overview

The `flowctl processors` command group allows you to:

- List all registered processors
- Find processors by input/output event types
- Inspect detailed processor information
- Filter by metadata (network, blockchain, etc.)

**Prerequisite**: These commands require a running pipeline with an active control plane.

## Commands

### processors list

List all processor components registered with the control plane.

```bash
flowctl processors list [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--include-unhealthy` | bool | false | Include unhealthy processors |
| `-e`, `--endpoint` | string | `localhost:8080` | Control plane endpoint |
| `-o`, `--output` | string | `table` | Output format: `table`, `yaml`, `json`, `wide` |

#### Examples

```bash
# List all healthy processors
./bin/flowctl processors list

# List all processors including unhealthy ones
./bin/flowctl processors list --include-unhealthy

# Output as JSON for scripting
./bin/flowctl processors list -o json

# List from remote control plane
./bin/flowctl processors list --endpoint remote-host:8080
```

#### Sample Output

```
NAME                    INPUT TYPE              OUTPUT TYPE              STATUS    ENDPOINT
ttp-processor-v1        stellar.ledger.v1       stellar.token.transfer   healthy   localhost:50052
contract-events-v1      stellar.ledger.v1       stellar.contract.event   healthy   localhost:50053
```

---

### processors find

Find processors that match specific criteria.

```bash
flowctl processors find [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--input` | string | | Filter by input event type |
| `--output` | string | | Filter by output event type |
| `--metadata` | key=value | | Filter by metadata (repeatable) |
| `--include-unhealthy` | bool | false | Include unhealthy processors |
| `-e`, `--endpoint` | string | `localhost:8080` | Control plane endpoint |

#### Examples

```bash
# Find processors that accept stellar.ledger.v1 events
./bin/flowctl processors find --input stellar.ledger.v1

# Find processors that produce token.transfer events
./bin/flowctl processors find --output stellar.token.transfer.v1

# Find processors for a specific network
./bin/flowctl processors find --metadata network=testnet

# Find processors for mainnet that process ledger data
./bin/flowctl processors find --input stellar.ledger.v1 --metadata network=mainnet

# Combine multiple filters
./bin/flowctl processors find \
  --input stellar.ledger.v1 \
  --output stellar.token.transfer.v1 \
  --metadata network=testnet
```

#### Use Cases

1. **Building Pipeline Chains**: Find processors that can consume output from another component
2. **Network-Specific Processing**: Filter processors by the Stellar network they support
3. **Capability Discovery**: Understand what transformations are available

---

### processors show

Show detailed information about a specific processor.

```bash
flowctl processors show <processor-id> [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-e`, `--endpoint` | string | `localhost:8080` | Control plane endpoint |
| `-o`, `--output` | string | `table` | Output format: `table`, `yaml`, `json` |

#### Examples

```bash
# Show details for a specific processor
./bin/flowctl processors show ttp-processor-v1

# Output as YAML
./bin/flowctl processors show ttp-processor-v1 -o yaml

# Show processor from remote control plane
./bin/flowctl processors show ttp-processor-v1 --endpoint remote-host:8080
```

#### Information Displayed

- **Component metadata**: name, version, description
- **Event types**: input and output types
- **Health status**: current health state
- **Endpoint**: gRPC endpoint address
- **Metrics**: processing statistics (if available)
- **Registration time**: when the processor registered

---

## Common Patterns

### Discovering Available Transformations

```bash
# 1. Start your pipeline
./bin/flowctl run my-pipeline.yaml &

# 2. List what's available
./bin/flowctl processors list

# 3. Find processors for your data type
./bin/flowctl processors find --input stellar.ledger.v1

# 4. Get details on a specific processor
./bin/flowctl processors show ttp-processor-v1
```

### Building Dynamic Pipelines

Use processor discovery to build pipelines dynamically:

```bash
# Find all processors that can transform ledger data
PROCESSORS=$(./bin/flowctl processors find --input stellar.ledger.v1 -o json)

# Parse and use in automation
echo "$PROCESSORS" | jq '.processors[].id'
```

### Debugging Data Flow

```bash
# Check if expected processors are registered
./bin/flowctl processors list

# Verify processor is receiving correct input type
./bin/flowctl processors show my-processor -o yaml

# Check for unhealthy processors
./bin/flowctl processors list --include-unhealthy
```

---

## Static vs Runtime Discovery

| Aspect | Static (flowctl init) | Runtime (processors commands) |
|--------|----------------------|------------------------------|
| **When** | Before pipeline runs | While pipeline is running |
| **What** | Pre-configured components | Registered processors |
| **Use Case** | Creating pipelines | Debugging, monitoring |
| **Source** | Docker Hub registry | Control plane registry |

**flowctl init** uses static component information from Docker Hub to generate pipeline configurations.

**processors commands** query the live control plane to see what's actually running.

---

## Event Types

Common event types in the Stellar ecosystem:

| Event Type | Description | Typical Producer |
|------------|-------------|------------------|
| `stellar.ledger.v1` | Raw Stellar ledger data | stellar-live-source |
| `stellar.token.transfer.v1` | Token transfer events | ttp-processor |
| `stellar.contract.event.v1` | Smart contract events | contract-events-processor |
| `stellar.payment.v1` | Payment operations | payment-extractor |

---

## Troubleshooting

### "Connection refused"

Control plane is not running. Start a pipeline first:

```bash
./bin/flowctl run my-pipeline.yaml
```

### "No processors found"

1. Verify pipeline is running with processors
2. Check processor health: `./bin/flowctl processors list --include-unhealthy`
3. Enable debug logging: `--log-level=debug`

### Processor shows as unhealthy

1. Check processor logs in pipeline output
2. Verify processor configuration (ports, endpoints)
3. Ensure processor's dependencies are available

---

## See Also

- [flowctl init Command](init-command.md)
- [Building Components](building-components.md)
- [Configuration Guide](configuration.md)
- [Architecture Overview](architecture.md)

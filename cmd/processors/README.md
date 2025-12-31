# Processors Command

The `flowctl processors` command group provides tools for managing and discovering processor components in the flowctl control plane.

## Overview

Processors are components that transform events from one type to another. They are the building blocks of dynamic topologies, accepting input events and producing output events based on their processing logic.

## Commands

### `flowctl processors list`

List all registered processor components.

```bash
# List all healthy processors
flowctl processors list

# Include unhealthy processors
flowctl processors list --include-unhealthy

# Connect to remote control plane
flowctl processors list --endpoint remote-host:8080
```

**Output:**
```
ID                INPUT TYPES              OUTPUT TYPES                    STATUS      ENDPOINT
──                ───────────              ────────────                    ──────      ────────
ttp-processor-v1  stellar.ledger.v1        stellar.token.transfer.v1       ✓ healthy   localhost:50051
aggregator-v1     stellar.token.transfer.v1 stellar.summary.v1             ✓ healthy   localhost:50052
```

**Status Indicators:**
- `✓ healthy` - Processor is running and healthy
- `⚠ degraded` - Processor is running but experiencing issues
- `✗ unhealthy` - Processor is not responding
- `⟳ starting` - Processor is starting up
- `⊗ stopping` - Processor is shutting down
- `? unknown` - Status is unknown

### `flowctl processors show <id>`

Show detailed information about a specific processor.

```bash
# Show processor details
flowctl processors show ttp-processor-v1
```

**Output:**
```
Processor: ttp-processor-v1
═══════════════════════════════════════════════════════

Basic Information:
  Name:        Token Transfer Processor
  Version:     1.0.0
  Description: Extracts token transfer events from Stellar ledgers

Event Types:
  Input Types:
    - stellar.ledger.v1
  Output Types:
    - stellar.token.transfer.v1

Connection:
  Endpoint:    localhost:50051

Health:
  Status:       ✓ healthy
  Last Heartbeat: 2025-12-07 10:30:15 (5s ago)
  Registered At:  2025-12-07 10:25:00

Metrics:
  events_processed: 1250
  events_per_second: 125.5
```

### `flowctl processors find`

Find processors by event types or metadata.

```bash
# Find processors that accept stellar.ledger.v1 events
flowctl processors find --input stellar.ledger.v1

# Find processors that produce token.transfer events
flowctl processors find --output stellar.token.transfer.v1

# Find processors for a specific network
flowctl processors find --metadata network=testnet

# Combine filters
flowctl processors find --input stellar.ledger.v1 --metadata network=mainnet
```

**Output:**
```
ID                INPUT TYPES              OUTPUT TYPES                    ENDPOINT
──                ───────────              ────────────                    ────────
ttp-processor-v1  stellar.ledger.v1        stellar.token.transfer.v1       localhost:50051

Found: 1 processor(s)
```

## Use Cases

### Discovering Available Processors

Before building a pipeline, discover what processors are available:

```bash
$ flowctl processors list
```

This shows all healthy processors registered with the control plane.

### Finding Compatible Processors

To find processors that can work with specific event types:

```bash
# What can process stellar.ledger.v1 events?
$ flowctl processors find --input stellar.ledger.v1

# What produces stellar.token.transfer.v1 events?
$ flowctl processors find --output stellar.token.transfer.v1
```

This is the foundation for automatic topology discovery (Shape 02).

### Inspecting Processor Details

To see full details about a processor:

```bash
$ flowctl processors show ttp-processor-v1
```

This shows event types, health status, metrics, and metadata.

### Filtering by Environment

Find processors for specific environments:

```bash
# Testnet processors
$ flowctl processors find --metadata network=testnet

# Mainnet processors
$ flowctl processors find --metadata network=mainnet

# Specific blockchain
$ flowctl processors find --metadata blockchain=stellar
```

## Flags

### Global Flags

- `--endpoint <host:port>` - Control plane endpoint (default: `localhost:8080`)
- `--include-unhealthy` - Include unhealthy processors in results

### Find-Specific Flags

- `--input <type>` - Filter by input event type
- `--output <type>` - Filter by output event type
- `--metadata <key=value>` - Filter by metadata (can specify multiple)

## Integration with Control Plane

The processors command connects to the flowctl control plane to query registered components. The control plane must be running for these commands to work.

### Starting the Control Plane

If you're running a pipeline:
```bash
$ flowctl run pipeline.yaml
# Control plane starts automatically
```

In another terminal:
```bash
$ flowctl processors list
# Shows processors from the running pipeline
```

### No Control Plane Running

If no control plane is running:
```bash
$ flowctl processors list
Error: failed to list processors: rpc error: code = Unavailable desc = connection error...
Hint: Make sure 'flowctl run' is running or the control plane is available
```

## Architecture

### Component Registration

When a processor starts, it automatically registers with the control plane:

1. Processor calls `RegisterComponent` RPC
2. Provides component info (ID, event types, endpoint, metadata)
3. Control plane stores in registry
4. Processor sends periodic heartbeats
5. Control plane removes stale processors

### Discovery Flow

The `find` command uses the control plane's discovery API:

1. Client calls `DiscoverComponents` RPC
2. Provides filters (type, event types, metadata)
3. Control plane searches registry
4. Returns matching components
5. Client filters results further if needed

### Health Monitoring

Processors maintain health status:

1. Processor sends heartbeat every 10s
2. Control plane tracks last heartbeat time
3. If no heartbeat for 30s → marked unhealthy
4. Health status visible in `list` and `show` commands

## Examples

### Example 1: Discovering a Processing Chain

Find processors to build a chain from ledger events to database:

```bash
# Step 1: What accepts ledger events?
$ flowctl processors find --input stellar.ledger.v1
ttp-processor-v1  stellar.ledger.v1  stellar.token.transfer.v1  localhost:50051

# Step 2: What accepts token.transfer events?
$ flowctl processors find --input stellar.token.transfer.v1
aggregator-v1     stellar.token.transfer.v1  stellar.summary.v1  localhost:50052
postgres-sink-v1  stellar.token.transfer.v1  -                   localhost:50053

# Result: ledger → ttp → postgres (2 hops)
```

This manual discovery will be automated in Shape 02!

### Example 2: Checking Processor Health

```bash
# List all processors
$ flowctl processors list
ID                STATUS
──                ──────
ttp-processor-v1  ✓ healthy
old-processor-v1  ✗ unhealthy

# Show details of unhealthy processor
$ flowctl processors show old-processor-v1
Health:
  Status:       ✗ unhealthy
  Last Heartbeat: 2025-12-07 10:20:00 (10m ago)
  # ^^ No heartbeat for 10 minutes, marked unhealthy
```

### Example 3: Environment-Specific Discovery

```bash
# Find testnet processors
$ flowctl processors find --metadata network=testnet
testnet-ttp-v1  stellar.ledger.v1  stellar.token.transfer.v1  testnet.example.com:50051

# Find mainnet processors
$ flowctl processors find --metadata network=mainnet
mainnet-ttp-v1  stellar.ledger.v1  stellar.token.transfer.v1  mainnet.example.com:50051
```

## Testing

Run tests:
```bash
go test -v ./cmd/processors/
```

Build and test CLI:
```bash
make build
./bin/flowctl processors --help
./bin/flowctl processors list --help
```

## Next Steps: Shape 02

The processors command provides the foundation for **Shape 02: Type-Based Discovery**.

With processor discovery working, we can now build:
- BFS chain discovery algorithm
- `flowctl topology suggest` command
- Automatic processor chain building

See `docs/shapes/02-type-based-discovery.md` for details.

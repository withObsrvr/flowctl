# Pitch: Make `get` and `logs` Commands Actually Work

## Problem
Two core CLI commands are complete stubs:

**`flowctl get pipelines`** (`cmd/get.go`)
```go
fmt.Println("Listing Pipeline resources...")
// Returns fake placeholder output
```

**`flowctl logs <component>`** (`cmd/logs.go`)
```go
fmt.Println("Fetching logs for component...")
// Returns fake placeholder output
```

**User experience**: Commands exist but don't do anything. Feels broken. Can't debug running pipelines.

## Appetite
**1 week** - Two commands, similar backend work for both.

## Solution (Fat-marker sketch)

### Architecture
```
CLI Command
    ↓
Control Plane Client (already exists!)
    ↓
Query running components
    ↓
Format and display results
```

### For `get` command:
1. Connect to control plane API (client already exists in `internal/api/client.go`)
2. Call `ListServices()` to get running components
3. Filter by resource type (pipeline, source, processor, sink)
4. Format as table (use a simple table library or just printf)
5. Show status, uptime, health

### For `logs` command:
1. Figure out which Docker container corresponds to component
2. Shell out to `docker logs` (or use Docker SDK)
3. Stream logs to stdout
4. Support `-f` flag for follow mode
5. Support `--tail` for last N lines

### Key Insight
The control plane API already exists! Just need to wire it up:
- `internal/api/client.go` - ✓ Already has `ListServices()`
- `internal/controlplane/embedded_server.go` - ✓ Already serving API

## Rabbit Holes to Avoid
- ❌ **Don't build log aggregation/storage** - Just fetch from containers
- ❌ **Don't add log filtering/search** - Pipe to grep if needed
- ❌ **Don't format/colorize logs** - Raw output is fine
- ❌ **Don't implement watch mode for get** - Static snapshot is enough
- ❌ **Don't add fancy ASCII art tables** - Simple columns are fine

## No-Gos (Explicitly Out of Scope)
- Log aggregation across multiple components
- Log persistence/archival
- Log analysis/parsing
- Structured log output (JSON mode)
- Log export to external systems
- Real-time log streaming dashboard
- Custom formatters per component type

## Done (Concrete Success Criteria)

### Must Demo This:

**Scenario 1: List Running Pipelines**
```bash
# Start a pipeline
./bin/flowctl apply examples/minimal.yaml

# List pipelines
./bin/flowctl get pipelines

# Output:
# NAME       STATUS    COMPONENTS    UPTIME
# minimal    Running   3             2m15s
#
# ✓ Shows actual running pipeline
# ✓ Accurate component count
# ✓ Real uptime
```

**Scenario 2: List All Components**
```bash
./bin/flowctl get components

# Output:
# NAME                TYPE        STATUS     UPTIME
# minimal-source      source      healthy    2m30s
# minimal-processor   processor   healthy    2m25s
# minimal-sink        sink        healthy    2m20s
#
# ✓ Shows all registered components
# ✓ Shows component types
# ✓ Shows health status from control plane
```

**Scenario 3: Fetch Component Logs**
```bash
./bin/flowctl logs minimal-processor

# Output:
# [2025-01-15 10:30:00] Starting processor...
# [2025-01-15 10:30:01] Registered with control plane
# [2025-01-15 10:30:02] Processing event: {"type": "data"}
# ...
#
# ✓ Shows actual Docker container logs
# ✓ Real timestamps
```

**Scenario 4: Follow Logs**
```bash
./bin/flowctl logs -f minimal-processor

# ✓ Streams logs in real-time
# ✓ Updates as new logs come in
# ✓ Ctrl-C stops cleanly
```

### Code Changes Expected:
- `cmd/get.go` - Implement actual backend (100-150 LOC)
- `cmd/logs.go` - Implement log fetching (80-100 LOC)
- `internal/cli/formatter.go` - Table formatting helper (50 LOC)
- Tests for both commands (150 LOC)

## Scope Line

```
═══════════════════════════════════════════
MUST HAVE
═══════════════════════════════════════════
✓ `get pipelines` shows actual running pipelines from control plane
✓ `get components` shows registered components with status
✓ `logs <component>` fetches real logs from Docker
✓ `-f` flag for log following works
✓ `--tail N` shows last N lines
✓ Empty state handling (no pipelines running)
✓ Clear error if control plane not reachable

───────────────────────────────────────────
NICE TO HAVE
───────────────────────────────────────────
○ `--since` flag for logs (last 5m, 1h, etc.)
○ Color-coded status (green=healthy, red=unhealthy)
○ `get all` shows everything
○ `-o json` flag for machine-readable output
○ Component health check status in output

───────────────────────────────────────────
COULD HAVE (cut first if behind)
───────────────────────────────────────────
○ `get events` - show component registration events
○ Auto-refresh for get command (watch mode)
○ Log timestamps in relative format ("2m ago")
○ Multi-component logs (logs across multiple components)
○ Log grep/filter built-in
```

## Technical Dependencies
- **Control Plane API**: Already exists, just needs connection
- **Docker SDK**: Already imported, just needs usage
- **Component Registry**: Already storing component metadata

## Implementation Path

### Day 1-2: Get Command
1. Wire up control plane client connection
2. Implement `get pipelines`
3. Implement `get components`
4. Basic table formatting

### Day 3-4: Logs Command
1. Map component ID to Docker container name
2. Fetch logs using Docker SDK
3. Implement `-f` follow mode
4. Implement `--tail` option

### Day 5: Polish
1. Error handling
2. Integration tests
3. Documentation
4. Edge cases (component crashed, control plane down, etc.)

## Hill Progress Indicators

**Left side (figuring it out):**
- Control plane client connects successfully
- Can query ListServices() and get results
- Can map component to Docker container
- Can fetch logs from Docker

**Right side (making it happen):**
- `get` command implemented and tested
- `logs` command implemented and tested
- Error cases handled gracefully
- Works with real running pipelines
- Documentation updated

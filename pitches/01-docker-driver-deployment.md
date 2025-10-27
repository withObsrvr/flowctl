# Pitch: Make Docker Driver Actually Deploy Things

## Problem
The Docker driver (`internal/drivers/docker/docker.go`) is completely stubbed out - it only translates pipeline YAML to Docker Compose but doesn't actually deploy anything. Users run `flowctl apply` expecting their pipeline to start, but nothing happens. The `Apply()`, `Delete()`, and `Status()` methods all return "not implemented" errors.

**Current pain**: Users must manually run `docker-compose up` after translation. The tool feels broken.

## Appetite
**2 weeks** - This is foundational infrastructure that needs to work properly.

## Solution (Fat-marker sketch)

### Core Flow
```
flowctl apply pipeline.yaml
  ↓
Translate to docker-compose.yml
  ↓
Use Docker SDK to:
  - docker compose up -d
  - Wait for containers to start
  - Register components with control plane
  ↓
Return status/confirmation
```

### Key Pieces
1. **Integration with Docker SDK** (already imported in go.mod)
   - Use `github.com/docker/docker/client` to manage compose projects
   - Don't shell out to `docker` CLI - use SDK directly

2. **State Tracking**
   - Store deployment metadata (project name, compose file path)
   - Track which pipelines are running
   - Simple file-based state or use existing BoltDB

3. **Basic Operations**
   - `Apply()`: Generate compose file → Start containers → Wait for healthy
   - `Delete()`: Stop containers → Remove volumes → Clean state
   - `Status()`: Query container status → Return component health

4. **Error Recovery**
   - Handle partial failures (some containers start, others don't)
   - Rollback on failure (stop what started)
   - Clear error messages about what went wrong

## Rabbit Holes to Avoid
- ❌ **Don't build sophisticated rollback logic** - Just stop containers on failure, don't try to restore previous state
- ❌ **Don't implement blue-green deployments** - Simple stop/start is enough
- ❌ **Don't add deployment history tracking** - Just current state
- ❌ **Don't handle container networking beyond compose** - Let compose do it
- ❌ **Don't implement custom health checks beyond what's in spec** - Use what's already in pipeline YAML

## No-Gos (Explicitly Out of Scope)
- Multi-host Docker Swarm deployments
- Container image building (assume images exist)
- Custom network driver configurations
- Volume backup/restore
- Secret encryption at rest
- Deployment strategies (rolling, canary, etc.)

## Done (Concrete Success Criteria)

### Must Demo This:
```bash
# Start with no containers running
docker ps  # Empty

# Apply a pipeline
./bin/flowctl apply examples/minimal.yaml
# ✓ Containers start automatically
# ✓ Shows "Pipeline deployed successfully"
# ✓ Lists running components with status

# Check status
./bin/flowctl get pipelines
# ✓ Shows "minimal" pipeline as "running"

# Delete the pipeline
./bin/flowctl delete pipeline minimal
# ✓ Containers stop
# ✓ Confirms deletion

docker ps  # Empty again
```

### File Changes Expected:
- `internal/drivers/docker/docker.go` - Actual implementation (200-300 LOC)
- `internal/drivers/docker/state.go` - Simple state tracking (100 LOC)
- `cmd/apply/apply.go` - Hook up to driver
- `cmd/delete.go` - Implement using driver
- Tests for docker driver (150+ LOC)

## Scope Line

```
═══════════════════════════════════════════
MUST HAVE
═══════════════════════════════════════════
✓ Apply generates compose file and starts containers
✓ Delete stops containers and cleans up
✓ Status returns actual container states
✓ Basic error handling with clear messages
✓ Works with examples/minimal.yaml end-to-end
✓ Integration test that actually starts/stops containers

───────────────────────────────────────────
NICE TO HAVE
───────────────────────────────────────────
○ Pretty status output (table format)
○ Progress indicators during startup
○ Parallel container startup where possible
○ Automatic retry on transient failures
○ Validate images exist before starting

───────────────────────────────────────────
COULD HAVE (cut first if behind)
───────────────────────────────────────────
○ Logs streaming during apply
○ Wait for health checks before returning
○ Resource usage stats in status
○ Apply updates existing deployment (vs delete/recreate)
○ Deployment events to control plane
```

## Risk: Docker Compose Library Complexity
The docker SDK is large. If parsing compose files gets hairy, **pivot to shelling out to `docker compose` CLI**. Less elegant but ships on time. SDK is preferred but CLI is the escape hatch.

## Hill Progress Indicators

**Left side (figuring it out):**
- Can create Docker client and query daemon
- Can start/stop containers via SDK
- Can track project state reliably
- Error handling strategy decided

**Right side (making it happen):**
- Apply() fully implemented
- Delete() fully implemented
- Status() fully implemented
- Tests passing
- Example works end-to-end

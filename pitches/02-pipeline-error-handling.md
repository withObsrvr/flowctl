# Pitch: Stop Silently Swallowing Pipeline Errors

## Problem
The core pipeline execution (`internal/core/pipeline.go`) has **TODO comments** where errors should be handled but aren't:
- Line 98: Sources fail silently (network errors ignored)
- Line 116: Processors fail silently (`continue` on error)
- Line 138: Sinks fail silently (write failures ignored)

**Result**: Pipelines appear to run but data gets lost. No visibility into failures. Users think everything is working when it's not.

## Appetite
**1 week** - Critical fix, but scope is bounded to one file.

## Solution (Fat-marker sketch)

### Error Handling Strategy
```
Component fails
  ↓
Log the error (already have logger)
  ↓
Increment failure counter
  ↓
Decide: Retry, Skip, or Stop Pipeline?
  ↓
Expose errors via control plane API
```

### Three Levels of Failure Handling

1. **Transient Errors** (retry)
   - Network timeouts
   - Temporary resource unavailable
   - Quick retry with exponential backoff

2. **Recoverable Errors** (skip + log)
   - Bad message format (processor can't parse)
   - Missing optional dependency
   - Continue pipeline, log warning

3. **Fatal Errors** (stop pipeline)
   - Component crashes repeatedly
   - Out of memory/disk
   - Configuration error
   - Stop entire pipeline, clear failure

### Implementation Approach
- Add `ErrorPolicy` to pipeline spec (new field in YAML)
  ```yaml
  spec:
    errorPolicy: "continue" # or "stop-on-error"
    maxRetries: 3
  ```
- Add error counters to component state
- Implement simple retry logic (3 tries with backoff)
- Surface errors in control plane API

## Rabbit Holes to Avoid
- ❌ **Don't build a sophisticated dead letter queue** - Just log and count
- ❌ **Don't implement circuit breakers per component** - Simple retry is enough
- ❌ **Don't add error classification ML** - Hard-code common error types
- ❌ **Don't build error dashboards** - Just expose via API, let users build their own
- ❌ **Don't handle every edge case** - Cover the common ones

## No-Gos (Explicitly Out of Scope)
- Error alerting/notifications (webhook, email, etc.)
- Error analytics/aggregation
- Custom error handler plugins
- Error replay/reprocessing queue
- Distributed tracing integration
- Error rate limiting

## Done (Concrete Success Criteria)

### Must Demo This:

**Scenario 1: Transient Network Failure**
```bash
# Start pipeline with flaky network
./bin/flowctl apply examples/dag-pipeline.yaml

# Network blip occurs
# ✓ See retry log messages
# ✓ Pipeline continues after retry succeeds
# ✓ Error counter shows in status

./bin/flowctl get pipelines
# Shows: "3 transient errors, 3 retries"
```

**Scenario 2: Fatal Component Crash**
```bash
# Start pipeline
./bin/flowctl apply examples/minimal.yaml

# Kill a component container
docker kill minimal-processor

# ✓ Pipeline stops gracefully
# ✓ Clear error: "Pipeline stopped: processor crashed after 3 retries"
# ✓ Other components clean up properly
```

**Scenario 3: Recoverable Processing Error**
```bash
# Pipeline with "continue" error policy
# Processor gets malformed message
# ✓ Logs warning
# ✓ Skips bad message
# ✓ Continues processing next message
# ✓ Error count incremented
```

### Code Changes Expected:
- `internal/core/pipeline.go` - Replace TODOs with actual error handling (100-150 LOC)
- `internal/model/pipeline.go` - Add ErrorPolicy field (10 LOC)
- `schemas/cue/schema.cue` - Add error policy to schema (20 LOC)
- Tests for each error scenario (200+ LOC)
- Update examples with error policy (5 files × 5 LOC)

## Scope Line

```
═══════════════════════════════════════════
MUST HAVE
═══════════════════════════════════════════
✓ Source errors logged and counted (no more silent ignore)
✓ Processor errors logged and counted
✓ Sink errors logged and counted
✓ Simple retry logic (3 tries, exponential backoff)
✓ Fatal errors stop the pipeline with clear message
✓ Error counts exposed in pipeline status
✓ Basic error policy in spec (continue vs stop)

───────────────────────────────────────────
NICE TO HAVE
───────────────────────────────────────────
○ Configurable retry count per component
○ Different error policies per component type
○ Error metrics exposed to Prometheus
○ Graceful degradation (some components fail, others continue)

───────────────────────────────────────────
COULD HAVE (cut first if behind)
───────────────────────────────────────────
○ Error pattern matching (regex on error messages)
○ Component restart on recoverable failure
○ Error history/log in control plane storage
○ Error webhooks for external monitoring
○ Per-message error context (which message failed)
```

## Integration with Existing Systems
- Leverage existing `logger` package (already imported)
- Use existing control plane API to expose errors
- Error counts can be simple fields on component status
- No new dependencies needed

## Hill Progress Indicators

**Left side (figuring it out):**
- Error taxonomy defined (transient, recoverable, fatal)
- Retry strategy decided and documented
- Error policy YAML schema designed
- Agreement on which errors stop pipeline vs continue

**Right side (making it happening):**
- Error handling in runSource() implemented
- Error handling in runProcessors() implemented
- Error handling in runSink() implemented
- Tests for each error type passing
- Examples updated with error policy
- Documentation on error handling written

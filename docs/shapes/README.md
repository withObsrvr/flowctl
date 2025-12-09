# flowctl Shapes: Development Roadmap

This directory contains Shape Up-style pitch documents for flowctl features. Each shape represents a fixed-time, variable-scope project.

---

## Completed Shapes ✅

### Phase 1: Foundation (Shapes 01-04)
- **Shape 01:** Processor Registry - Component registration and discovery
- **Shape 02:** Type-Based Discovery - BFS search over event types
- **Shape 03:** Intent-Based Topologies - High-level v2 YAML format
- **Shape 04:** Runtime Reconfiguration - Hot reload and updates

### Phase 2: Dynamic Topology (Shapes 05-08)
- **[Shape 05](./SHAPE-05-COMPLETE.md):** Integrated V2 Execution - Automatic v2 intent detection and execution
- **[Shape 06](./SHAPE-06-COMPLETE.md):** Processor Registry & Auto-Download - OCI image distribution
- **[Shape 07](./SHAPE-07-COMPLETE.md):** Init Command - Interactive wizard for new users
- **[Shape 08](./SHAPE-08-COMPLETE.md):** External Control Plane Mode - Service mesh architecture

**Result:** Complete dynamic topology vision with intent-based pipelines, automatic discovery, and auto-download.

---

## Pitched Shapes (Ready to Build) 📋

### Phase 3: Cleanup & Polish (Shape 16)

### [Shape 16: Command Cleanup - Remove Dead/Redundant Commands](./16-command-cleanup.md)
**Appetite:** 2 days | **Status:** Ready | **Dependencies:** None

**Problem:** Flowctl has 4 broken/redundant commands cluttering the CLI.

**Solution:** Remove `apply`, `new`, `context`, and `help` commands. Reduces codebase by ~500 lines and improves UX.

**Related Docs:**
- [Implementation Checklist](./16-command-cleanup-checklist.md) - Step-by-step guide
- [Detailed Rationale](./16-command-cleanup-rationale.md) - Analysis & decision record

---

### Phase 4: Orchestration & Observability (Shapes 09-11)

These shapes transform flowctl from a **component registry** to a **pipeline orchestrator** like Dagster.

### [Shape 09: Pipeline Run Tracking & Management API](./09-pipeline-run-tracking.md)
**Appetite:** 2-3 days | **Status:** Pitched | **Dependencies:** None

**Problem:** Control plane tracks components but not pipeline runs.

**Solution:** Add run tracking, lifecycle management, and persistent storage.

### [Shape 10: Pipeline Management CLI](./10-pipeline-management-cli.md)
**Appetite:** 1 day | **Status:** Pitched | **Depends On:** Shape 09

**Problem:** No user-facing CLI for pipeline management.

**Solution:** New `flowctl pipelines` command group with list, runs, info, stop subcommands.

### [Shape 11: Interactive Terminal UI](./11-interactive-tui.md)
**Appetite:** 2 days | **Status:** Pitched | **Depends On:** Shapes 09, 10

**Problem:** Static CLI, no live monitoring.

**Solution:** Interactive TUI with live dashboard using bubbletea framework.

---

## Historical Shapes (Original Dynamic Topology)

### [Shape 01: Processor Registry](01-processor-registry.md)
**Appetite:** 1 week (5-7 days)
**Foundation for everything else**

Processors auto-register with flowctl control plane, advertising:
- Input event types
- Output event types
- Endpoint and health URL
- Metadata (network, version, etc.)

**Deliverable:**
```bash
$ flowctl processors list
ID                    INPUT TYPES              OUTPUT TYPES                    STATUS
stellar-live-source   -                        stellar.ledger.v1               healthy
ttp-processor-v1      stellar.ledger.v1        stellar.token.transfer.v1       healthy
```

**Enables:** Discovery and chain building

---

### [Shape 02: Type-Based Discovery](02-type-based-discovery.md)
**Appetite:** 1 week (5-7 days)
**Depends on:** Shape 01

Flowctl discovers processor chains using BFS graph search over event types.

**Deliverable:**
```bash
$ flowctl topology suggest --from stellar.ledger.v1 --to postgres
Chain found:
  stellar-live-source → ttp-processor → postgres-sink
```

**Enables:** Automatic chain building

---

### [Shape 03: Intent-Based Topologies](03-intent-based-topologies.md)
**Appetite:** 2 weeks (10-14 days)
**Depends on:** Shapes 01, 02

Users write high-level intent, flowctl translates to executable pipeline.

**Deliverable:**
```yaml
# Write this (v2 intent)
spec:
  from: stellar.ledger.v1
  to: postgres

# Flowctl generates this (v1 implementation)
processors:
  - id: ttp-processor
    command: ["/path/to/ttp-processor"]
    # ... discovered from registry
```

**Enables:** Declarative topologies

---

### [Shape 04: Runtime Reconfiguration](04-runtime-reconfiguration.md)
**Appetite:** 2 weeks (10-14 days)
**Depends on:** Shapes 01, 02, 03

Add/remove/update processors without stopping pipeline.

**Deliverable:**
```bash
# Pipeline running: source → ttp → postgres
$ flowctl topology add aggregator --after ttp-processor

# Flowctl:
# 1. Starts aggregator
# 2. Creates new routes
# 3. Drains old route
# 4. Removes old route
# Result: source → ttp → aggregator → postgres (no downtime, no data loss)
```

**Achieves:** Full dynamic topology vision

---

## Timeline

**Sequential (safe approach):**
```
Week 1-2:   Shape 01 (Processor Registry)
Week 3-4:   Shape 02 (Type-Based Discovery)
Week 5-8:   Shape 03 (Intent-Based Topologies)
Week 9-12:  Shape 04 (Runtime Reconfiguration)
Total: ~3 months
```

**Parallel (aggressive approach):**
```
Week 1-2:   Shape 01 (Processor Registry) - BLOCKER
Week 3-4:   Shape 02 (Discovery) + start Shape 03 design
Week 5-6:   Shape 03 (Intent Topologies) - finish implementation
Week 7-8:   Shape 04 (Runtime Reconfig) - start with drain-and-restart
Week 9-10:  Shape 04 - iterate to hot reload if time permits
Total: ~2.5 months
```

**Recommended:** Start sequential, go parallel if ahead of schedule.

---

## Scope Line Across All Shapes

### Must Have (Non-negotiable)
Core functionality that proves the vision:
- ✅ Auto-registration of processors
- ✅ Discovery of processor chains by event type
- ✅ Intent → implementation translation
- ✅ Add/remove processors (at minimum with drain-and-restart)

### Nice to Have (Include if time)
Polish that improves UX:
- ⭐ Health monitoring integration
- ⭐ Interactive chain selection
- ⭐ Constraint filtering (network, blockchain)
- ⭐ Hot reload without restart

### Could Have (Cut if needed)
Features that can wait:
- 💡 Persistent registry
- 💡 Processor marketplace
- 💡 Visual topology builder
- 💡 Multi-path topologies

---

## Success Criteria

**Demo that proves we achieved the vision:**

```bash
# 1. Start flowctl control plane
$ flowctl serve
Control plane running on :8080

# 2. Start processors (they auto-register)
$ ./stellar-live-source &
$ ./ttp-processor &
$ ./postgres-sink &

# 3. Check registry
$ flowctl processors list
stellar-live-source   -                        stellar.ledger.v1               healthy
ttp-processor-v1      stellar.ledger.v1        stellar.token.transfer.v1       healthy
postgres-sink-v1      stellar.token.transfer.v1 -                              healthy

# 4. Write intent YAML
$ cat > pipeline.yaml <<EOF
apiVersion: flowctl/v2
kind: Topology
spec:
  from: stellar.ledger.v1
  to: postgres
EOF

# 5. Run pipeline (auto-discovers chain)
$ flowctl run pipeline.yaml
Discovered chain: stellar-live-source → ttp-processor → postgres-sink
Starting topology...
  ✓ All components running

# 6. Add processor at runtime
$ flowctl topology add aggregator --after ttp-processor
Reconfiguring topology...
  ✓ Topology updated: source → ttp → aggregator → postgres
  ✓ No events lost, no downtime
```

**If we can do this demo, we've achieved the vision.**

---

## Risk Assessment

### Shape 01 (Processor Registry)
**Risk:** Low
**Reason:** Simple data structure + gRPC registration
**Mitigation:** None needed

### Shape 02 (Type-Based Discovery)
**Risk:** Low
**Reason:** BFS is well-understood algorithm
**Mitigation:** Set max search depth to prevent infinite loops

### Shape 03 (Intent-Based Topologies)
**Risk:** Medium
**Reason:** Translation logic could get complex with constraints
**Mitigation:** Start with simple from→to, add constraints later

### Shape 04 (Runtime Reconfiguration)
**Risk:** High
**Reason:** Event draining and rollback are complex
**Mitigation:** Build drain-and-restart first, iterate to hot reload

---

## Decision Points

**After Shape 01:**
- ✅ Continue if: Registry works, processors auto-register
- ❌ Kill if: Can't reliably track processors or heartbeats fail

**After Shape 02:**
- ✅ Continue if: Can discover valid chains for common topologies
- ⚠️ Reassess if: Discovery takes >1s or produces invalid chains

**After Shape 03:**
- ✅ Continue to Shape 04 if: Intent translation works reliably
- ⚠️ Stop at Shape 03 if: Translation too fragile or error-prone

**During Shape 04:**
- ✅ Ship if: Drain-and-restart works without data loss
- ⭐ Iterate if: Hot reload feasible and time permits
- ❌ Rollback to Shape 03 if: Can't guarantee no data loss

---

## Files Structure

After all shapes complete:

```
flowctl/
├── internal/
│   ├── registry/
│   │   ├── client.go (existing - OCI images)
│   │   └── processor_registry.go (NEW - Shape 01)
│   ├── topology/
│   │   ├── matcher.go (NEW - Shape 02)
│   │   ├── builder.go (NEW - Shape 02)
│   │   ├── intent.go (NEW - Shape 03)
│   │   ├── translator.go (NEW - Shape 03)
│   │   ├── state_manager.go (NEW - Shape 04)
│   │   └── router.go (NEW - Shape 04)
│   └── controlplane/
│       └── server.go (MODIFIED - add registration RPCs)
├── cmd/
│   ├── processors.go (NEW - Shape 01)
│   ├── topology.go (NEW - Shape 02, 03)
│   └── run.go (MODIFIED - Shape 03)
└── proto/
    └── processor_registration.proto (NEW - Shape 01)
```

**Estimated total new code:** ~2500-3000 lines

---

## Notes

These shapes follow **Shape Up methodology**:

- ✅ **Fixed time, variable scope** - Each shape has hard deadline
- ✅ **Appetites, not estimates** - "We'll spend 1 week" not "This will take 1 week"
- ✅ **Fat-marker sketches** - Solution outlines, not detailed specs
- ✅ **Rabbit holes identified** - Explicit "don't build" lists
- ✅ **Clear done criteria** - Concrete examples that prove success
- ✅ **Scope lines** - Must/Nice/Could have prioritization
- ✅ **Ship or kill** - If stuck at 50% time, cut scope or abandon

**Most important:** These shapes are **bets**, not commitments. After each cycle, we decide whether to continue, pivot, or stop.

---

## Related Documents

- [Original PR #5637](https://github.com/stellar/go-stellar-sdk/pull/5637) - Vision and inspiration
- [ttp_processor_layout.png](../../ttp-processor-sdk/ttp_processor_layout.png) - Distributed topology diagram
- [GENERIC_EVENT_ENVELOPE.md](../../ttp-processor-demo/GENERIC_EVENT_ENVELOPE.md) - Event envelope architecture

---

## Questions or Feedback?

These shapes are living documents. If assumptions change or new information emerges, update the shapes before starting work.

**Better to spend 1 day reshaping than 2 weeks building the wrong thing.**

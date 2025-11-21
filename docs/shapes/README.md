# Shape Up Documents - Component Distribution Friction Reduction

This directory contains Shape Up documents for eliminating developer friction in flowctl component distribution.

## Problem Statement

**Current Friction:** Developers must build component binaries locally and reference them with absolute paths, making pipelines non-portable and creating team coordination problems.

**Goal:** Enable developers to reference components from OCI registries (like Docker Hub, GitHub Container Registry) so pipelines become portable, version-controlled, and shareable with zero friction.

---

## Phases Overview

### Phase 1: OCI Registry Support
**Appetite:** 1 week (5 days)
**Status:** Shaped, awaiting approval
**Document:** [phase1-oci-registry-support.md](./phase1-oci-registry-support.md)

**What it solves:** Users can reference components from container registries instead of local paths.

**Before:**
```yaml
command: ["/home/tillman/Documents/ttp-processor-demo/stellar-live-source-datalake/stellar-live-source-datalake"]
```

**After:**
```yaml
image: ghcr.io/withobsrvr/stellar-live-source-datalake:v1.2.3
```

**Key Features:**
- Parse `image:` field in pipeline YAML
- Pull images from public registries (Docker Hub, GHCR, GCR)
- Cache images locally to avoid re-downloading
- Support multiple drivers (local, Docker Compose, Kubernetes)
- Backwards compatible with existing `command:` field

---

### Phase 2: Publishing Automation
**Appetite:** 2-3 days (small batch)
**Status:** Shaped, awaiting approval
**Depends on:** Phase 1
**Document:** [phase2-publishing-automation.md](./phase2-publishing-automation.md)

**What it solves:** Component authors can publish images automatically via CI/CD instead of manual multi-platform builds.

**Developer Workflow:**
```bash
git tag v1.0.0
git push origin v1.0.0
# GitHub Actions automatically builds multi-arch images and pushes to registry
```

**Key Deliverables:**
- Component image specification document
- Dockerfile template (multi-stage, distroless)
- GitHub Actions workflow template
- Publishing guide documentation
- Example component repository

---

### Phase 3: Development Mode
**Appetite:** 1 week (5 days)
**Status:** Shaped, awaiting approval
**Depends on:** Phase 1
**Document:** [phase3-development-mode.md](./phase3-development-mode.md)

**What it solves:** Developers can use local binaries during active development without repeatedly building/pushing images.

**Production:**
```bash
flowctl apply pipeline.yaml  # Uses images
```

**Development:**
```bash
flowctl dev pipeline.yaml  # Uses local binaries (auto-detected overrides)
```

**Key Features:**
- Separate `flowctl-dev.yaml` override file
- Selective component override (mix local + images)
- Automatic rebuild before start
- Environment variable overrides
- Fast iteration cycles (< 30 seconds code change → running)

---

### Phase 4: Discovery & Management
**Appetite:** 1-2 weeks (5-10 days)
**Status:** Shaped, awaiting approval
**Depends on:** Phase 1, Phase 2
**Document:** [phase4-discovery-management.md](./phase4-discovery-management.md)

**What it solves:** Developers can discover, search, and manage components without manual GitHub/registry browsing.

**Discovery:**
```bash
flowctl components search kafka
# Found 3 components:
#   kafka-source          Stream from Apache Kafka        v2.1.0
#   kafka-sink            Write to Apache Kafka           v1.9.0
#   kafka-dlq-processor   Dead letter queue for Kafka     v0.5.0

flowctl components info kafka-source
# Full details: description, versions, configuration, examples
```

**Cache Management:**
```bash
flowctl cache list      # Show cached images
flowctl cache clean     # Remove old versions
flowctl cache prune     # Remove unused images
```

**Key Features:**
- Git-based component index (zero infrastructure)
- Search and filter components
- Inspect component details and versions
- Local cache management commands
- Pretty table formatting

---

## Timeline & Dependencies

```
Phase 1 (1 week)
  └─ OCI Registry Support
      ├─ Phase 2 (2-3 days) - Publishing Automation
      │
      ├─ Phase 3 (1 week) - Development Mode
      │
      └─ Phase 2 + Phase 3 complete
          └─ Phase 4 (1-2 weeks) - Discovery & Management

Total: 3-4 weeks for all phases
```

**Parallel execution possible:**
- Phase 2 and Phase 3 can run in parallel after Phase 1 completes
- Phase 4 requires Phase 2 (components must be published)

---

## Success Metrics

### Before (Current State - High Friction)
- **Portability:** ❌ Pipelines locked to specific machines (absolute paths)
- **Time to share pipeline:** ∞ (can't share due to local paths)
- **Time to run someone's pipeline:** 30+ min (build all components)
- **Version consistency:** ❌ Everyone has different builds
- **Discovery:** 10+ min (Google, GitHub, guesswork)
- **Publishing:** 30+ min (manual, error-prone)

### After (Target State - Zero Friction)
- **Portability:** ✅ Pipelines work on any machine
- **Time to share pipeline:** 0 sec (git commit YAML)
- **Time to run someone's pipeline:** 2 min (pull images, cached after)
- **Version consistency:** ✅ Exact same images via tags
- **Discovery:** 30 sec (`flowctl components search`)
- **Publishing:** 5 min (copy templates, push tag)

---

## Shape Up Principles Applied

### Fixed Time, Variable Scope
- Each phase has a clear appetite (1 week, 2-3 days)
- Scope lines explicitly define MUST HAVE vs COULD HAVE
- No deadline extensions - ship what's done or cut scope

### Fat-Marker Sketches
- Solutions show key concepts without over-specifying
- Room for implementation creativity
- Focus on user experience and outcomes

### Rabbit Holes Identified
- Each document lists potential rabbit holes
- Clear guidance on what NOT to build
- Time-saving decisions documented

### Uphill/Downhill Work
- Clear implementation plans (day-by-day)
- Dependencies identified
- Risks and mitigations documented

### Cool-Down After Each Phase
- 20% of cycle time (1-2 days between phases)
- Fix bugs, refactor, explore
- Prevent burnout

---

## How to Use These Documents

### For Planning
1. Review each phase document
2. Confirm appetites match available time
3. Approve scope lines (MUST HAVE vs NICE TO HAVE)
4. Agree on success criteria

### For Implementation
1. Follow day-by-day implementation plan
2. Use rabbit holes list to avoid time sinks
3. Check progress against "Done Looks Like" section
4. Cut COULD HAVE items if running behind

### For Retrospectives
1. Compare actuals vs estimates
2. Review what was cut and why
3. Identify new rabbit holes discovered
4. Update future shapes based on learnings

---

## Approval Status

| Phase | Status | Kick-off | Ship Deadline | Actual Shipped |
|-------|--------|----------|---------------|----------------|
| Phase 1 | Awaiting approval | TBD | TBD | - |
| Phase 2 | Awaiting approval | TBD | TBD | - |
| Phase 3 | Awaiting approval | TBD | TBD | - |
| Phase 4 | Awaiting approval | TBD | TBD | - |

---

## References

- [Shape Up Methodology](https://basecamp.com/shapeup)
- [Component Distribution Analysis](../component-distribution-analysis.md) - Research behind these decisions
- [Terraform Provider Registry](https://registry.terraform.io/)
- [OCI Distribution Spec](https://github.com/opencontainers/distribution-spec)
- [Krew Plugin Index](https://github.com/kubernetes-sigs/krew-index)

---

## Questions or Feedback?

Open an issue or discussion in the flowctl repository to discuss these shapes, suggest modifications, or ask questions about implementation approach.

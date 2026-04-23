# Flowctl Flagship Scope

## Purpose

This document defines the current product scope for `flowctl` as the flagship orchestration layer for production Stellar data pipelines.

Its purpose is to prevent scope drift and ensure we focus engineering effort on the runtime and operator experience that matter most.

---

## 1. Product position

`flowctl` is the **production orchestration and operations layer** for data pipeline components.

It is responsible for:
- pipeline definition
- component resolution
- component lifecycle management
- control plane coordination
- registration and health tracking
- run visibility
- operational UX for running pipelines

`flowctl` is **not** the prototyping layer.
That role belongs to **nebu**.

---

## 2. Primary user promise

A user should be able to:

1. create a pipeline
2. validate the pipeline
3. run the pipeline
4. observe component health and pipeline status
5. inspect active and historical runs
6. stop and restart safely
7. trust the runtime path being used

Embedded mode should support persistence for run history, with an explicit ephemeral option for one-off local runs.

The core experience should feel:
- obvious
- reliable
- testable
- production-oriented

---

## 3. Canonical workflow

The primary supported workflow is:

```bash
flowctl init
flowctl validate stellar-pipeline.yaml
flowctl run stellar-pipeline.yaml
flowctl status
flowctl pipelines active
flowctl pipelines run-info <run-id>
flowctl pipelines stop <run-id>
```

Optional:

```bash
flowctl dashboard
```

This workflow is the core product path and should receive the highest testing and documentation priority.

---

## 4. Supported scope

## 4.1 Canonical pipeline format
**Supported**
- `apiVersion: flowctl/v1`
- `kind: Pipeline`

This is the only canonical user-facing pipeline format.

### Implications
- docs should lead with v1 only
- examples should use v1 only
- validation and run paths should optimize for v1 only

---

## 4.2 Runtime backend
**Supported**
- `process`

This is the primary and recommended execution backend.

### Why
- simplest operational model
- easiest to test
- easiest to debug
- most aligned with current working code
- enough to establish product trust

---

## 4.3 Control plane model
**Supported**
- embedded control plane
- external control plane mode

### Embedded control plane
This is the default and easiest user path.
It should support:
- run tracking
- operator stop semantics
- persistent history by default
- an opt-out ephemeral mode for local experimentation

### External control plane
Supported for advanced or shared-infrastructure workflows, but secondary to the embedded mode in docs and examples.

---

## 4.4 Component model
**Supported**
- sources
- processors
- sinks

Each component must be resolvable via:
- `command`
- `image`
- or `type`

Registry-style `type` references are part of the intended production model and should be hardened.

---

## 4.5 Core commands
**Supported and first-class**
- `flowctl init`
- `flowctl validate`
- `flowctl run`
- `flowctl status`
- `flowctl pipelines`

**Supported if stable**
- `flowctl dashboard`

These commands define the flagship operator experience.

---

## 4.6 Validation
**Supported**
- structural pipeline validation
- schema/path checks required for the v1 runtime path
- CI-friendly failure behavior

Validation should be treated as part of the core product, not a secondary utility.

---

## 5. Experimental or secondary scope

## 5.1 Docker / container orchestration
**Experimental / not flagship-supported**

Until status, health, teardown, and lifecycle behavior are complete and tested, Docker/container orchestration should not be presented as equal to process execution.

### Product rule
If a runtime backend is incomplete, it should be:
- marked experimental
- hidden from prominent docs
- or removed from the supported promise

---

## 5.2 Translation and deployment generation
**Secondary**

Translation-related features may remain useful, but they are not part of the current flagship promise unless they directly support the core production runtime path.

---

## 5.3 Legacy compatibility
**Compatibility-only, not part of the flagship story**

Legacy config or parser support should exist only if:
- required for real users
- covered by tests
- clearly labeled

Otherwise it should be removed or quarantined.

---

## 5.4 Historical design experiments
**Out of current product scope**

Old topology, intent, or abandoned architecture explorations may remain in archived docs, but they should not shape:
- onboarding
- README messaging
- main examples
- current product language

---

## 6. Relationship to nebu

### nebu
Used for:
- prototyping
- quick iteration
- local experimentation
- processor design

### flowctl
Used for:
- production orchestration
- component lifecycle
- health and run tracking
- operational visibility
- standardized execution

### Promotion path
The intended path is:

```text
Prototype in nebu
    ↓
Stabilize contract/config/behavior
    ↓
Productionize as a flowctl component
    ↓
Run and operate via flowctl
```

This should become an explicit part of the ecosystem story.

---

## 7. Non-goals for the current flagship cycle

During this consolidation cycle, flowctl is **not** optimizing for:

- dynamic topology
- multiple equally mature runtime backends
- broad deployment abstraction
- speculative feature surfaces
- historical compatibility at the expense of clarity
- expanding command count without strengthening core reliability

---

## 8. Engineering priorities

Priority order:

1. repository test hygiene
2. process runtime reliability
3. operator commands and run visibility
4. documentation coherence
5. example pipelines and E2E tests
6. component promotion path from nebu to flowctl

---

## 9. Exit criteria for this focus cycle

This flagship focus cycle is complete when:

- `make test` is reliable
- the process runtime is clearly the supported path
- the core commands behave consistently
- the README and docs tell one clear story
- there are canonical working examples
- there are smoke/E2E tests for the golden workflow
- the nebu → flowctl promotion story is documented

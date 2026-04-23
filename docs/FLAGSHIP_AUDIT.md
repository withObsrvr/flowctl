# Flowctl Flagship Audit

## Objective

Refocus `flowctl` as the **production orchestration and operations layer** for Stellar data pipelines.

This audit answers:
- what exists
- what is core
- what is incomplete
- what should be removed, hidden, or deprioritized
- what must be hardened for user adoption

---

## 1. Product definition

### flowctl is

A production-oriented pipeline orchestrator that:
- loads declarative pipeline definitions
- resolves source/processor/sink components
- starts and supervises components
- runs an embedded or external control plane
- tracks component registration and health
- exposes operational status and run visibility

### flowctl is not

At this stage, flowctl should not be treated as:
- a dynamic topology engine
- a broad deployment abstraction layer with equal maturity across backends
- a place for half-supported orchestration models
- a prototype playground

### Relationship to nebu

- **nebu** is for prototyping and rapid processor iteration
- **flowctl** is for productionizing and operating those processors as components

---

## 2. Canonical user journey

The core user journey we need to make excellent:

```bash
flowctl init
flowctl validate stellar-pipeline.yaml
flowctl run stellar-pipeline.yaml
flowctl status
flowctl pipelines active
```

Optional:

```bash
flowctl dashboard
```

If this path is reliable, documented, and test-covered, flowctl earns its flagship position.

---

## 3. Current strengths

### 3.1 Core runtime exists

The following areas already provide the right foundation:
- `cmd/run.go`
- `internal/runner/loader.go`
- `internal/runner/pipeline_runner.go`
- `internal/controlplane/`
- `internal/api/`
- `internal/orchestrator/process.go`
- `internal/storage/`
- `internal/registry/`
- `schemas/cue/`

This is the right center of gravity for the product.

### 3.2 Canonical v1 pipeline format exists

`flowctl/v1` with `kind: Pipeline` is already functioning as the main pipeline format and should remain the only canonical user-facing format.

### 3.3 Operational commands exist

The repo already contains the commands expected of a production orchestrator:
- `run`
- `init`
- `validate`
- `status`
- `pipelines`
- `dashboard`

### 3.4 Test baseline is now healthy

`make test` now passes in the current Nix development environment.

This is a prerequisite for hardening the product further.

### 3.5 Operator flow now has live proof

The flagship operator path is now backed by repo-level smoke coverage for:
- `validate`
- `run`
- `status`
- `pipelines active`
- `pipelines run-info`
- `pipelines stop`

This materially improves confidence in the production story.

---

## 4. Current weaknesses

### 4.1 Supported runtime scope is unclear in some code/docs areas

`process` orchestration appears to be the most real and aligned path.
`docker/container` support exists but remains incomplete in several places.

This needs a clear product decision:
- process is supported
- docker/container is experimental or hidden until hardened

### 4.2 Legacy and historical residue remain

The repo still contains:
- legacy config support
- translator branches for legacy formats
- stale comments and fallback language
- large historical shape docs that do not reflect the current product

This creates cognitive drag and weakens the flagship story.

### 4.3 Docs are too internally historical

The docs contain too much design history relative to product guidance.
We need sharper public-facing docs and more clearly archived internal material.

### 4.4 Operator workflow docs and persistence still need polish

The core operator path is now materially stronger, but a few areas still need tightening:
- docs should fully reflect the current operator commands and flags
- embedded persistence should be documented clearly
- port collision handling should be documented as a first-class operator workflow
- health-port behavior should be made more explicit for component authors

---

## 5. Core keep / fix / cut decisions

### Keep
- `flowctl/v1` pipeline format
- process orchestration
- embedded control plane
- external control plane mode
- component resolution
- run tracking
- status/ops commands
- starter pipeline generation via `init`

### Fix
- E2E confidence
- command help consistency
- docs clarity
- status/dashboard/pipelines operator flow
- component resolution under real conditions

### Cut / hide / demote
- incomplete orchestration backends presented as mature
- non-core runtime paths that lack tests
- stale user-facing references to abandoned directions
- historical design material from the main onboarding path

---

## 6. Immediate goals

### Goal 1: make the repo trustworthy
- `make test` remains green
- examples are runnable
- docs match reality

### Goal 2: make the process runtime excellent
- `init -> validate -> run -> status` works cleanly
- component registration and run tracking are visible
- failure paths are understandable
- operator stop semantics are real, not cosmetic

### Goal 3: make the product story simple
- one canonical format
- one recommended runtime
- one operator story
- one relationship to nebu

---

## 7. Success criteria

Flowctl is “flagship ready” when:

1. a user can create a pipeline from docs and run it
2. validation catches common mistakes
3. components register and become visible via status
4. pipeline runs are tracked and queryable
5. shutdown behavior is reliable, including operator-initiated stop
6. the test suite is green and reproducible
7. the docs are clear and narrowly focused

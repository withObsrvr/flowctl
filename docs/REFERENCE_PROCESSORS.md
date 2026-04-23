# Flowctl Reference Processors

This document defines the initial reference processor set for `flowctl`.

It operationalizes the direction in:

- [`docs/FLAGSHIP_AUDIT.md`](./FLAGSHIP_AUDIT.md)
- [`docs/FLAGSHIP_SCOPE.md`](./FLAGSHIP_SCOPE.md)
- [`docs/BUILDING_PROCESSORS.md`](./BUILDING_PROCESSORS.md)

The goal is not to create a giant catalog first.
The goal is to create a **small, high-signal set** of processors that teach users how flowctl processors are built, configured, run, and operated.

---

## Why reference processors matter

Reference processors do four jobs at once:

1. **teach the processor model**
2. **prove the nebu → flowctl promotion path**
3. **support example and starter pipelines**
4. **give the flagship runtime real, documented building blocks**

Per the flagship scope, this is exactly the kind of work that strengthens `flowctl` as the production layer.

---

## Selection criteria

A processor should be considered a reference processor if it is:

- broadly useful
- easy to explain
- valuable in production pipelines
- a good example of one processor pattern
- backed by a nebu prototype or direct nebu equivalent when possible

We want the starter set to cover the most important processor shapes:

- **extract from ledgers**
- **filter typed events**
- **transform or normalize typed events**

---

## Starter set

## 1. contract-events-processor

**Role:** extractor

**Why it belongs in the first wave**
- already present in flowctl examples and metadata
- highly relevant to Soroban/Stellar users
- natural demonstration of `stellar.ledger.v1 -> stellar.contract.events.v1`
- directly supports flagship starter pipelines

**Input type**
- `stellar.ledger.v1`

**Output type**
- `stellar.contract.events.v1`

**Nebu precursor**
- `contract-events` in nebu / nebu-processor-registry

**Current in-repo signal**
- `docker/contract-events-processor/metadata.json`
- `examples/quickstart/testnet-duckdb-pipeline.yaml`

**What this reference should teach**
- consuming raw ledgers
- protobuf payload decoding
- emitting typed event batches
- using the processor in a full source → processor → sink pipeline

**Required example pipeline**
- Stellar source → contract-events-processor → DuckDB sink

---

## 2. token-transfer-processor

**Role:** extractor

**Why it belongs in the first wave**
- token transfers are one of the most understandable domain events
- great for demos, analytics, and onboarding
- maps cleanly from a proven nebu pattern into flowctl

**Input type**
- `stellar.ledger.v1`

**Output type**
- `stellar.token.transfer.v1`

**Nebu precursor**
- `token-transfer`
- related production lineage: `ttp-processor`

**What this reference should teach**
- extracting a high-value domain event from ledgers
- preserving metadata for ordering and sinks
- using shared protobuf contracts
- how a processor becomes the center of a pipeline story

**Required example pipeline**
- Stellar source → token-transfer-processor → Postgres or DuckDB sink

---

## 3. contract-filter-processor

**Role:** typed filter transform

**Why it belongs in the first wave**
- shows the simplest useful transform pattern
- teaches same-type input/output processors
- easy to compose after `contract-events-processor`
- mirrors the small-but-useful transform processors that made nebu approachable

**Input type**
- `stellar.contract.events.v1`

**Output type**
- `stellar.contract.events.v1`

**Nebu precursor**
- closest equivalents: `contract-filter`, `asset-filter`, `amount-filter`

**What this reference should teach**
- config-driven filtering
- zero-or-one output per input event or batch
- chaining processors together
- writing docs for common operator knobs like contract ID allowlists

**Required example pipeline**
- Stellar source → contract-events-processor → contract-filter-processor → sink

---

## Second wave

These should come after the starter set is solid.

## 4. contract-invocation-processor

**Role:** extractor

**Input type**
- `stellar.ledger.v1`

**Output type**
- `stellar.contract.invocation.v1`

**Nebu precursor**
- `contract-invocation`

**Why it matters**
- strong developer-facing value
- useful for app observability and protocol indexing
- good advanced extractor reference

---

## 5. amount-filter-processor

**Role:** typed filter transform

**Likely input/output types**
- `stellar.token.transfer.v1` → `stellar.token.transfer.v1`

**Nebu precursor**
- `amount-filter`

**Why it matters**
- simple, obvious, reusable transform
- great for example chaining after token transfer extraction

---

## 6. dedup-processor

**Role:** normalization / transform

**Likely input/output types**
- same-type passthrough with duplicate suppression

**Nebu precursor**
- `dedup`

**Why it matters**
- operationally useful
- demonstrates stateful or semi-stateful processor behavior
- valuable for real pipelines, not just tutorials

---

## Reference patterns we need to cover

The reference set should deliberately cover these patterns.

| Pattern | Reference processor |
|---|---|
| Ledger extractor | `contract-events-processor` |
| Ledger extractor | `token-transfer-processor` |
| Same-type filter | `contract-filter-processor` |
| Advanced extractor | `contract-invocation-processor` |
| Numeric filter | `amount-filter-processor` |
| Dedup / normalization | `dedup-processor` |

If we only ship extractors, users will not learn how to build transforms.
If we only ship filters, users will not learn how to decode ledgers.
We need both.

---

## Proposed implementation order

## Phase 1: flagship minimum

1. harden `contract-events-processor`
2. build `token-transfer-processor`
3. build `contract-filter-processor`
4. document all three together
5. add runnable pipelines for all three

This gives flowctl:
- one existing Soroban-centric extractor
- one high-signal transfer extractor
- one simple transform processor

That is enough to create a credible reference story.

## Phase 2: deepen the catalog

6. build `contract-invocation-processor`
7. build `amount-filter-processor`
8. build `dedup-processor`

This adds advanced extraction and transform examples without over-expanding the surface too early.

---

## Processor-by-processor deliverables

Each reference processor should ship with the same package of artifacts.

### Code
- standalone repo or example repo
- flowctl SDK-based implementation
- tests for extraction or transform logic

### Contract
- explicit input type
- explicit output type
- protobuf definitions or shared proto references
- versioned image tag

### Docs
- README
- config reference
- example pipeline YAML
- operator notes: ports, health, status expectations
- relation to the nebu precursor

### Ops
- container image publishing
- health endpoint
- visible registration metadata
- discoverable through `flowctl processors`

---

## Suggested example chains

These are the examples that should appear in docs and demos.

### Chain A: Soroban contract events

```text
stellar-live-source
  → contract-events-processor
  → duckdb-consumer
```

### Chain B: Soroban contract events with filter

```text
stellar-live-source
  → contract-events-processor
  → contract-filter-processor
  → postgres-consumer
```

### Chain C: Token transfers

```text
stellar-live-source
  → token-transfer-processor
  → duckdb-consumer
```

### Chain D: Token transfers with thresholding

```text
stellar-live-source
  → token-transfer-processor
  → amount-filter-processor
  → webhook or postgres sink
```

These chains are good because they are both:
- teachable
- operationally relevant

---

## What “reference quality” means

A processor is reference quality when a new user can:

1. understand what it does in under a minute
2. run it in a documented pipeline
3. inspect it with `flowctl status` and `flowctl processors`
4. read the config knobs without reading source
5. use it as a template for a new processor

If those are not true, it is not yet a reference processor.

---

## Relationship to nebu

These processors should usually start in nebu because nebu is the best place to:

- prove the event shape
- test the extraction logic quickly
- refine schemas with real data
- iterate without orchestration overhead

Then flowctl becomes the production wrapper and operations layer.

That means each reference processor should explicitly point back to its precursor, for example:

- `token-transfer` → `token-transfer-processor`
- `contract-events` → `contract-events-processor`
- `contract-filter` → `contract-filter-processor`

This promotion path is a core part of the flagship ecosystem story.

---

## Immediate next steps

1. keep `contract-events-processor` as the first-class existing reference
2. create a `token-transfer-processor` reference implementation
3. create a simple `contract-filter-processor` reference implementation
4. add example pipelines that chain them
5. keep all docs cross-linked from [`docs/BUILDING_PROCESSORS.md`](./BUILDING_PROCESSORS.md)

That is the smallest useful set that makes the story coherent.

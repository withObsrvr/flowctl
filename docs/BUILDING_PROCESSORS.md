# Building Processors for flowctl

This guide is the focused companion to [`docs/building-components.md`](./building-components.md).

It exists to make one part of the flagship story explicit:

**prototype processors in nebu, then graduate the successful ones into production-ready flowctl processors.**

That direction comes directly from:

- [`docs/FLAGSHIP_AUDIT.md`](./FLAGSHIP_AUDIT.md)
- [`docs/FLAGSHIP_SCOPE.md`](./FLAGSHIP_SCOPE.md)
- [`docs/flowctl-vs-nebu.md`](./flowctl-vs-nebu.md)

## The short version

Use this path:

```text
Prototype in nebu
  ↓
Validate schema, naming, and behavior
  ↓
Keep the protobuf contract stable
  ↓
Wrap the logic as a flowctl processor
  ↓
Run and operate it with flowctl
```

If you're starting from scratch, do **not** begin by over-designing a production component.
Start with a small, useful processor in nebu, prove it, then promote it.

---

## Why this is the recommended path

Per the flagship scope, `flowctl` is the **production orchestration and operations layer**.
It should be excellent at:

- running pipelines
- supervising components
- tracking health and registration
- exposing operator visibility

It should not be the place where processor ideas are first explored.

That exploratory work belongs to **nebu**.

The practical split is:

- **nebu**: fastest way to discover the right event shape and processor behavior
- **flowctl**: best place to run that processor in a monitored, production-oriented pipeline

---

## Reference documents

### In this repository

- [`docs/FLAGSHIP_AUDIT.md`](./FLAGSHIP_AUDIT.md)
- [`docs/FLAGSHIP_SCOPE.md`](./FLAGSHIP_SCOPE.md)
- [`docs/flowctl-vs-nebu.md`](./flowctl-vs-nebu.md)
- [`docs/building-components.md`](./building-components.md)
- [`docs/component-publishing-guide.md`](./component-publishing-guide.md)
- [`docs/processor-discovery.md`](./processor-discovery.md)
- [`docs/REFERENCE_PROCESSORS.md`](./REFERENCE_PROCESSORS.md)

### Nebu references

- nebu processor guide: `https://github.com/withObsrvr/nebu/blob/main/docs/BUILDING_PROCESSORS.md`
- nebu proto-first guide: `https://github.com/withObsrvr/nebu-processor-registry/blob/main/BUILDING_PROTO_PROCESSORS.md`
- nebu community registry: `https://github.com/withObsrvr/nebu-processor-registry`
- promotion guide: `https://github.com/withObsrvr/nebu-processor-registry/blob/main/docs/GRADUATING_TO_FLOWCTL.md`

### SDK and shared proto references

- flowctl SDK: `https://github.com/withObsrvr/flowctl-sdk`
- SDK quickstart: `https://github.com/withObsrvr/flowctl-sdk/blob/main/docs/QUICKSTART.md`
- shared protobuf definitions: `https://github.com/withObsrvr/flow-proto`

---

## What a flowctl processor is

A flowctl processor is a standalone component that:

- receives events from a source or another processor
- transforms, filters, enriches, or extracts events
- emits zero or more downstream events
- registers with the flowctl control plane
- exposes health and lifecycle behavior suitable for operations

In other words, the **business logic** may start life in nebu, but the flowctl version adds:

- control plane registration
- health checks
- heartbeats
- production configuration
- structured execution inside a pipeline

---

## The three processor patterns you should build first

The best reference processors for flowctl are the same ones that teach the most in nebu.

### 1. Ledger extractor processor
Consumes raw Stellar ledger events and emits typed domain events.

Examples:
- `contract-events-processor`
- `token-transfer-processor`
- `contract-invocation-processor`

This is the highest-value reference pattern because it shows how to:
- consume `stellar.ledger.v1`
- unmarshal protobuf payloads
- emit stable domain events
- preserve metadata needed by downstream sinks

### 2. Typed filter processor
Consumes a typed domain event and emits the same type, filtered.

Examples:
- `contract-filter-processor`
- `asset-filter-processor`
- `amount-filter-processor`

This pattern is important because it teaches:
- same-type input/output processors
- config-driven filtering
- easy chaining in pipelines

### 3. Typed enrichment or normalization processor
Consumes a typed event and emits a refined typed event.

Examples:
- `swap-normalizer-processor`
- `trade-extractor-processor`
- `dedup-processor`

This pattern matters once the basic extractor/filter story is solid.

---

## Promotion rule: proto-first, then wrapper

When a processor is likely to graduate into flowctl, build it in this order:

### Step 1: Define or reuse a protobuf contract
Use an existing event type from `flow-proto` when possible.
If the event type does not exist yet, define it clearly before you lock in the processor behavior.

### Step 2: Prove the logic in nebu
Use nebu to answer:
- is the event shape correct?
- are the names correct?
- does the processor emit what users actually need?
- are important metadata fields present?

### Step 3: Keep the event contract stable
Promotion should not reinvent the schema.
The flowctl processor should usually preserve:
- event type name
- payload protobuf type
- metadata fields
- config semantics where practical

### Step 4: Wrap the logic with the flowctl SDK
The wrapper should add:
- flowctl registration
- health checks
- runtime config via env/config
- operational logging

### Step 5: Add an operator-facing example pipeline
A reference processor is not finished until there is a copy-pasteable pipeline showing:
- source
- processor
- sink
- how to run it with `flowctl run`
- how to inspect it with `flowctl status` and `flowctl processors`

---

## Recommended repository layout for a flowctl processor

A good standalone flowctl processor repo should look like this:

```text
my-flowctl-processor/
├── cmd/my-flowctl-processor/main.go
├── internal/processor/
│   ├── processor.go
│   └── config.go
├── proto/
│   └── my_event.proto              # only if not using shared flow-proto types
├── README.md
├── Dockerfile
├── go.mod
├── go.sum
└── .github/workflows/release.yml
```

If the logic already exists in a nebu processor repo, try to keep the extraction logic in a reusable package and keep the flowctl-specific code as a thin wrapper.

---

## Minimal development workflow

## 1. Prototype in nebu

Start from the nebu guides:

- `nebu/docs/BUILDING_PROCESSORS.md`
- `nebu-processor-registry/BUILDING_PROTO_PROCESSORS.md`

Your goals in this phase:
- settle the protobuf/event shape
- test with bounded ledger ranges
- confirm the output is useful
- document examples

## 2. Select the flowctl event types

Prefer shared protobuf messages from `flow-proto`.

Typical pattern:

- input event type: `stellar.ledger.v1`
- output event type: one of:
  - `stellar.contract.events.v1`
  - `stellar.token.transfer.v1`
  - `stellar.contract.invocation.v1`

## 3. Implement the flowctl wrapper

At a high level, the processor does this:

1. starts with `flowctl-sdk/pkg/processor`
2. declares `InputType` and `OutputType`
3. unmarshals the incoming protobuf payload
4. calls your extraction or transform logic
5. marshals one or more outgoing protobuf payloads
6. returns the new events

A schematic example:

```go
package main

import (
    "context"
    "fmt"
    "log"

    flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
    stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
    sdkprocessor "github.com/withObsrvr/flowctl-sdk/pkg/processor"
    "google.golang.org/protobuf/proto"
)

func main() {
    proc := sdkprocessor.New(sdkprocessor.Config{
        Name:        "contract-events-processor",
        Description: "Extract Soroban contract events from Stellar ledgers",
        Version:     "0.1.0",
        InputType:   "stellar.ledger.v1",
        OutputType:  "stellar.contract.events.v1",
    })

    proc.SetProcessFunc(func(ctx context.Context, event *flowctlv1.Event) ([]*flowctlv1.Event, error) {
        var ledger stellarv1.RawLedger
        if err := proto.Unmarshal(event.Payload, &ledger); err != nil {
            return nil, fmt.Errorf("unmarshal ledger: %w", err)
        }

        batch, err := extractContractEvents(&ledger)
        if err != nil {
            return nil, fmt.Errorf("extract contract events: %w", err)
        }

        payload, err := proto.Marshal(batch)
        if err != nil {
            return nil, fmt.Errorf("marshal contract event batch: %w", err)
        }

        out := &flowctlv1.Event{
            Id:        event.Id + "-contract-events",
            Type:      "stellar.contract.events.v1",
            Timestamp: event.Timestamp,
            Payload:   payload,
        }

        return []*flowctlv1.Event{out}, nil
    })

    if err := proc.Run(); err != nil {
        log.Fatal(err)
    }
}
```

The exact SDK API may evolve; use the latest examples in `flowctl-sdk` as the canonical implementation reference.

---

## 4. Add pipeline examples immediately

Every reference processor should ship with at least one small pipeline example.

Example:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: contract-events-reference

spec:
  driver: process

  sources:
    - id: stellar-source
      type: stellar-live-source@v1.0.0
      config:
        network_passphrase: "Test SDF Network ; September 2015"
        backend_type: RPC
        rpc_endpoint: https://soroban-testnet.stellar.org
        start_ledger: 54000000

  processors:
    - id: contract-events
      type: contract-events-processor@v1.0.0
      inputs: ["stellar-source"]
      config:
        network_passphrase: "Test SDF Network ; September 2015"

  sinks:
    - id: duckdb-sink
      type: duckdb-consumer@v1.0.0
      inputs: ["contract-events"]
      config:
        database_path: ./reference.duckdb
```

---

## How to choose which processor graduates first

Promote a nebu processor into flowctl when all of these are true:

- the schema is stable enough to support downstream users
- the processor solves a common, repeatable pipeline need
- it is useful in docs, demos, and quickstarts
- it benefits from health monitoring and long-running operation
- it is not just a one-off query helper

Good candidates:
- contract events
- token transfers
- contract invocations
- typed filters used in many pipelines

Less urgent candidates:
- highly ad hoc transforms
- one-off analytics helpers
- processors that are mainly useful in shell pipelines only

---

## What a good reference processor must include

A reference processor is more than code.
It should include:

### Required
- stable name
- clear input and output event types
- README with a one-minute explanation
- at least one runnable flowctl pipeline example
- tests for the core transform/extraction logic
- release automation and image publishing
- health-port behavior that works under flowctl

### Strongly recommended
- matching nebu prototype or precursor
- protobuf schema docs
- migration notes from nebu to flowctl
- example sink integrations
- sample queries for DuckDB or Postgres consumers

---

## Suggested initial processor set

See [`docs/REFERENCE_PROCESSORS.md`](./REFERENCE_PROCESSORS.md) for the proposed starter set.

The short recommendation is:

### First wave
1. `contract-events-processor`
2. `token-transfer-processor`
3. `contract-filter-processor`

### Second wave
4. `contract-invocation-processor`
5. `amount-filter-processor`
6. `dedup-processor`

This gives flowctl both:
- **extractor references**
- **transform references**

That mirrors what made nebu useful for processor builders.

---

## Testing checklist

Before calling a processor a reference implementation, verify:

### Logic
- unit tests cover happy path and malformed inputs
- protobuf payloads unmarshal and marshal correctly
- output event type names are correct

### Runtime
- component registers with the control plane
- `flowctl status` shows it as healthy
- `flowctl processors list` shows expected input/output types
- shutdown is graceful

### Operator UX
- pipeline example works with `flowctl validate`
- pipeline example works with `flowctl run`
- logs are readable
- config errors fail clearly

### Documentation
- README includes copy-paste examples
- docs state input/output event types explicitly
- docs explain when to use nebu vs flowctl

---

## Publishing and distribution

When the processor is ready to distribute:

1. publish a container image
2. include metadata for discovery
3. document versioned references in pipeline examples
4. make sure `type:`-based resolution is possible where applicable

Use:
- [`docs/component-publishing-guide.md`](./component-publishing-guide.md)
- [`docs/component-image-spec.md`](./component-image-spec.md)

---

## One strong rule for this flagship cycle

For this cycle, optimize for:

- a few excellent processors
- excellent docs
- excellent runnable examples

Do **not** optimize for a huge catalog before the promotion path is clear.

The flagship story gets stronger when users can see:

```text
nebu prototype
  ↓
shared proto contract
  ↓
flowctl reference processor
  ↓
runnable example pipeline
  ↓
operator visibility with status and pipelines commands
```

That is the ecosystem loop we should make obvious.

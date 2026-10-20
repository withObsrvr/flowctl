# Flowctl programmable application and CLI roadmap

**Status:** Proposed implementation roadmap  
**Scope:** flowctl CLI, application services, control plane, resource contracts,
storage, process runtime, and operator-facing automation  
**Audience:** Maintainers and contributors planning work after `v0.0.12`

## Purpose

Flowctl should evolve from a collection of orchestration commands into a
coherent, programmable pipeline application. The CLI, a future web interface,
automation clients, and agents should invoke the same typed operations and
observe the same authoritative state.

The guiding principle is:

> Build flowctl as a system of identifiable resources, explicit operations,
> predictable state machines, and resumable events. Make the CLI the first and
> best client of that system.

This document turns broad CLI and application-design guidance into a roadmap
that fits flowctl's current architecture. It preserves the flagship scope:

- `flowctl/v1` pipeline definitions;
- process orchestration as the primary runtime;
- embedded control plane as the default local experience;
- external control plane as a secondary advanced mode;
- supervised gRPC delivery through sources, processors, and sinks;
- pipeline run history, health, failure context, and operator control.

It does not make arbitrary DAG execution, container orchestration, or a hosted
multi-tenant service prerequisites for improving the current product.

## Product boundary

Flowctl is responsible for:

- validating pipeline definitions;
- resolving component artifacts;
- launching and stopping components;
- wiring and supervising data-plane streams;
- recording pipeline run lifecycle and failure context;
- exposing current and historical operational state;
- providing synchronization primitives for operators and automation;
- serving the same capabilities through a typed control-plane API;
- supporting local embedded and external control-plane deployments.

Flowctl is not currently promising:

- arbitrary graph execution beyond the documented runtime shape;
- several equally mature runtime drivers;
- a Kubernetes operator;
- a public hosted control plane;
- multi-tenant authentication and billing;
- exactly-once processing without component-specific durable contracts;
- source or sink checkpoint ownership;
- automatic infinite restart;
- dynamic code plugins inside the flowctl process.

Nebu remains the local Unix-pipe prototyping and processor-development layer.
Flowctl owns production lifecycle and operational state.

## Existing foundations

Flowctl is not starting from zero. It already has:

- pipeline definitions and validation;
- pipeline run IDs and persistent run records;
- explicit run states;
- component registration and heartbeats;
- component health data;
- process and embedded-control-plane operation;
- external control-plane connectivity;
- supervised source, processor, and sink delivery;
- first-error cancellation across stream edges;
- bounded completion and sink acknowledgement checks;
- direct source-to-sink routing;
- structured, credential-free delivery failure records;
- chunk resources for bounded historical work;
- operator commands for active runs, run information, and stop;
- global context, namespace, output, configuration, and noninteractive flags;
- synthetic process-level integration coverage.

The next step is to make those pieces one consistent application contract.

## Core design principles

1. **The CLI is a client of application operations.** Cobra handlers should
   parse, call, and render—not implement orchestration or storage behavior.
2. **Embedded and remote modes share semantics.** A local operation and a
   control-plane RPC must apply the same validation, authorization boundary,
   state transition, and error classification.
3. **Resources have stable identities.** Pipelines, runs, components, chunks,
   and future operations use IDs that scripts never derive from display order.
4. **State machines are explicit.** Legal transitions, terminal states, and
   idempotent repeat behavior are documented and tested.
5. **Commands, queries, and events are distinct.** Mutations request work,
   queries read state, and events describe committed changes.
6. **State and event publication are coordinated.** Clients must not observe an
   event for a state change that was not committed, or miss committed changes
   because an event was emitted only in process memory.
7. **Machine output is versioned.** JSON, NDJSON, API, and event contracts have
   explicit compatibility rules.
8. **Human presentation is replaceable.** Tables and prose may evolve without
   becoming parsing contracts.
9. **Cancellation is not failure.** Operator stop, context cancellation,
   component failure, and successful completion remain distinct outcomes.
10. **Durability claims are precise.** Transport receipt, handler completion,
    sink acknowledgement, and durable commit must not be conflated.
11. **Process mode comes first.** New abstractions must prove themselves on the
    flagship runtime before being generalized to secondary drivers.
12. **No silent degradation.** Failed registration, delivery, persistence, or
    explicitly requested observability must be visible and actionable.

## Target architecture

The target architecture separates product semantics from adapters:

```text
CLI / future web UI / automation / agent
                  |
                  v
          typed application client
          /                   \
 embedded implementation      RPC implementation
          \                   /
           application services
                  |
        domain state machines
                  |
       storage + event journal
                  |
   process runtime / components / streams
```

Illustrative interfaces:

```go
type PipelineRuns interface {
    Start(ctx context.Context, req StartRunRequest) (PipelineRun, error)
    Get(ctx context.Context, id string) (PipelineRun, error)
    List(ctx context.Context, filter RunFilter) (RunPage, error)
    Stop(ctx context.Context, id string, reason string) (PipelineRun, error)
    Wait(ctx context.Context, id string, condition RunCondition) (PipelineRun, error)
}

type Events interface {
    List(ctx context.Context, filter EventFilter) (EventPage, error)
    Watch(ctx context.Context, filter EventFilter, cursor string) (EventStream, error)
}
```

These interfaces are examples, not an instruction to publish a large Go API
immediately. Begin internally and expose only contracts with demonstrated use.

## Resource model

### Pipeline definition

A pipeline definition is desired topology and configuration. Today it is a
`flowctl/v1` YAML document. It is not the same thing as a running pipeline.

Recommended identity:

- stable pipeline ID assigned when stored, if persistent definitions are added;
- human-readable name for discovery;
- immutable definition revision or digest;
- schema/API version;
- creation/update timestamps;
- source metadata without embedded secret values.

Persistent pipeline definitions are optional for the first phases. A run can
continue to embed the submitted definition while the run model is stabilized.

### Pipeline run

A run is one execution attempt with a stable run ID. It owns:

- pipeline name and definition revision/digest;
- lifecycle state;
- component instances;
- start/end timestamps;
- classified terminal result;
- structured failure context;
- operator stop reason;
- correlation/operation ID;
- aggregate progress that is carefully distinguished from durable sink state.

### Component instance

A component instance is a source, processor, or sink participating in one run.
Registration identity must distinguish the logical component from a process
instance or restart attempt.

### Chunk

A chunk represents a bounded historical range attempt. The existing chunk
model is a useful foundation, but its transition rules, retry ownership,
verification boundary, and relationship to a pipeline run must be formalized.

### Operation

An operation represents an accepted asynchronous mutation when a command cannot
truthfully return a final result immediately. Examples may include future
remote starts, controlled checkpoints, or long stop/drain operations.

Do not create operation resources for synchronous local work merely for
architectural symmetry.

### Event

An event is an immutable description of a committed lifecycle change. It is
not a log line and not the pipeline data stream.

## State machines

### Pipeline run state

The current run enum is a useful base:

```text
unknown -> starting -> running -> completed
                         |  |
                         |  +----> failed
                         +-------> stopping -> stopped
```

The exact addition of `stopping` should be made only with a protobuf/API
versioning decision. Until then, stopping may remain an operation/event while
the stored state transitions directly to `stopped`.

Required rules:

- `completed`, `failed`, and `stopped` are terminal;
- successful bounded delivery alone can produce `completed`;
- component/data-plane failure produces `failed`;
- operator cancellation produces `stopped`, not `failed`;
- terminal states cannot be overwritten by late heartbeats or shutdown updates;
- repeating stop on a stopped run is idempotent;
- stopping a completed or failed run returns the existing terminal state or a
  typed conflict, not a fabricated success;
- the first authoritative failure is retained;
- start must not report `running` until required components are registered and
  stream wiring has succeeded;
- registration-only mode remains explicit and cannot imply data delivery.

### Chunk state

Formalize legal transitions among planned, dispatched, running, completed,
verified, failed, retry-pending, and aborted. At minimum:

- only completed work can become verified;
- a failed attempt is immutable;
- retry creates or advances an attempt without erasing prior evidence;
- aborted is distinct from failed;
- verification records reference the exact attempt and range;
- overlapping active chunks for the same pipeline/component/range are rejected
  unless explicitly supported.

Use table-driven transition tests as the executable specification.

## Prioritized implementation plan

### Priority 1: Define application services and thin CLI adapters

#### Problem

Current Cobra handlers often create gRPC connections, use background contexts,
query the control plane, transform domain state, and render directly to global
stdout. This makes embedded and remote behavior harder to keep consistent.

#### Work

- Introduce internal typed services for run queries and mutations.
- Introduce a typed client interface implemented by embedded and gRPC clients.
- Move run-ID resolution, filtering, stop semantics, and state validation out
  of Cobra handlers.
- Thread `cmd.Context()` through every operation instead of creating
  `context.Background()` in command paths.
- Inject clocks and storage where state transition tests require determinism.
- Keep orchestration internals private until an external Go API is justified.
- Use `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` through renderers.

#### Acceptance criteria

- Pipeline list/get/stop behavior can be tested without Cobra and without a
  live TCP listener.
- The same service tests run against memory and persistent storage.
- Embedded and external clients pass shared contract tests.
- No command-specific business rule exists only in a renderer.

### Priority 2: Establish stable CLI exit codes and structured errors

#### Problem

The root command currently maps every error to exit code 1 and human prose.
Automation cannot distinguish invalid configuration, unavailable control plane,
not-found resources, conflicts, failed runs, authorization, or interruption.

#### Proposed exit classes

The exact values require a compatibility decision, but the contract should
distinguish:

- success;
- general execution failure;
- invalid usage/configuration;
- not found;
- unavailable dependency/control plane;
- conflict or illegal state transition;
- authentication/authorization failure when remote security exists;
- pipeline run failed;
- interruption (conventionally 130 for SIGINT).

#### Structured error

```json
{
  "_schema": "flowctl.error.v1",
  "code": "pipeline_run_failed",
  "exit_code": 7,
  "operation": "pipeline.run",
  "resource": {"type": "pipeline_run", "id": "..."},
  "message": "Pipeline delivery failed",
  "details": {"edge": "source -> sink", "cause": "unavailable"},
  "correlation_id": "..."
}
```

#### Work

- Create typed domain/application errors with stable string codes.
- Map gRPC status codes and local errors into the same taxonomy.
- Add a stable JSON error mode on stderr.
- Redact pipeline environment values and credentials.
- Preserve safe structured delivery failure details.
- Document whether component logs may precede the terminal JSON record; prefer a
  dedicated diagnostics stream/file when strict parsing is required.

#### Acceptance criteria

- Exit-code behavior is table-tested at the CLI boundary.
- Embedded and remote forms of the same failure yield the same code.
- interruption is never reported as pipeline failure;
- stdout remains a valid selected output format on errors.

### Priority 3: Stabilize machine-readable output

#### Problem

The root advertises `table|yaml|json|wide`, but individual commands do not
consistently honor the global output flag. Human tables and protobuf-derived
JSON are not yet defined as stable CLI contracts.

#### Work

- Inventory every command and supported output format.
- Centralize renderers for table, wide, JSON, YAML, and NDJSON.
- Define versioned envelopes such as:
  - `flowctl.pipeline_list.v1`;
  - `flowctl.pipeline_run.v1`;
  - `flowctl.component_list.v1`;
  - `flowctl.chunk_list.v1`;
  - `flowctl.status.v1`.
- Define pagination fields even if the first local implementation returns one
  page.
- Use stable enum strings rather than Go/protobuf debug formatting.
- Preserve exact IDs in machine output; truncation is human presentation only.
- Define null/omitted behavior for timestamps, metrics, and failures.
- Make empty results successful with an empty collection.
- Add JSON Schema or equivalent contract tests.

#### Compatibility strategy

Do not silently change existing JSON shapes. Introduce explicit versioned
formats or document a pre-1.0 migration and deprecation window.

#### Acceptance criteria

- Every read command honors documented output formats.
- JSON/YAML contain no ANSI escapes, progress lines, or logger output.
- Contract tests cover unknown fields and enum stability.
- human and machine renderers consume the same typed results.

### Priority 4: Implement `pipeline wait`

#### Goal

Replace arbitrary sleeps and polling loops with an explicit synchronization
primitive.

#### Proposed interface

```bash
flowctl pipeline wait <run-id> --for running --timeout 2m
flowctl pipeline wait <run-id> --for terminal --timeout 30m
flowctl pipeline wait <run-id> --for completed --output json
```

If the existing plural `pipelines` group remains canonical, use
`flowctl pipelines wait`. Settle singular/plural command organization before
adding aliases that become permanent.

#### Conditions

- `starting`, `running`, `completed`, `failed`, `stopped`, or `terminal`;
- optionally component-health readiness after its semantics are precise;
- never a vague `ready` condition without a documented definition.

#### Semantics

- returns 0 only when the requested condition is satisfied;
- a run reaching an incompatible terminal state returns a typed nonzero result;
- timeout differs from run failure and user interruption;
- deleting or losing the resource is not success;
- remote watch may be used later, but bounded polling is acceptable initially;
- polling includes backoff and honors context cancellation.

#### Acceptance criteria

- process integration tests cover success, failure, stopped, timeout, and
  interruption;
- scripts no longer require fixed sleeps in the flagship workflow;
- machine output returns the final observed resource.

### Priority 5: Add a durable lifecycle event model

#### Goal

Let clients understand how resources reached their current state and react to
changes without scraping logs or repeatedly listing resources.

#### Event envelope

```json
{
  "_schema": "flowctl.event.v1",
  "event_id": "...",
  "sequence": 42,
  "occurred_at": "2026-10-14T14:00:00Z",
  "type": "pipeline.run.failed",
  "resource": {"type": "pipeline_run", "id": "..."},
  "pipeline_id": "...",
  "run_id": "...",
  "operation_id": "...",
  "correlation_id": "...",
  "data": {}
}
```

#### Initial event names

- `pipeline.run.created`;
- `pipeline.run.starting`;
- `pipeline.run.running`;
- `pipeline.run.completed`;
- `pipeline.run.failed`;
- `pipeline.run.stop_requested`;
- `pipeline.run.stopped`;
- `component.registered`;
- `component.health_changed`;
- `delivery.edge_failed`;
- chunk lifecycle events matching committed transitions.

#### Durability rules

- Events describe committed state transitions.
- Event IDs and per-stream sequence numbers are stable.
- Storage commits state and its event atomically, or uses an outbox pattern that
  guarantees eventual publication without inventing events.
- Retention and compaction are explicit.
- Events contain bounded metadata and no secrets.
- Event history is operational metadata, not the high-volume pipeline data
  plane.

#### Acceptance criteria

- Restarting the control plane does not erase retained lifecycle history.
- Tests prove state/event atomicity under injected storage failure.
- Duplicate delivery is tolerated through stable event IDs.
- Event order is defined per resource or journal, not implied globally without
  evidence.

### Priority 6: Add event listing and follow

#### Proposed interface

```bash
flowctl pipeline events <run-id>
flowctl pipeline events <run-id> --follow
flowctl pipeline events <run-id> --after <cursor> --output ndjson
```

#### Snapshot-plus-watch contract

A client must be able to:

1. request the current resource and a corresponding event cursor;
2. subscribe after that cursor;
3. receive every subsequent committed event without a snapshot/watch race;
4. reconnect using the last acknowledged cursor.

The first implementation may use a gRPC server stream. A future browser-facing
adapter may translate the same application event stream to SSE or WebSocket.

#### Backpressure and retention

- Slow event clients must not block pipeline delivery.
- Use bounded subscriber buffers and disconnect lagging clients with a resumable
  cursor/error.
- Return a typed cursor-expired response when retention has removed history.
- Keep event payloads small and schema-versioned.

#### Acceptance criteria

- follow works across control-plane restart when history is retained;
- reconnect produces no gaps and consumers can deduplicate repeats;
- NDJSON stdout contains events only;
- Ctrl+C exits as interruption without changing run state.

### Priority 7: Make start/stop idempotent and operation-aware

#### Goal

Automation should be able to retry mutations safely after timeouts or uncertain
responses.

#### Work

- Accept an idempotency key for start and other creation-like operations.
- Store the key with the resulting run/operation for a bounded retention period.
- Repeating the same key and equivalent request returns the existing result.
- Reusing a key for a different request returns a conflict.
- Make stop naturally idempotent.
- Return an operation ID if a remote mutation is accepted before it reaches its
  final state.
- Add `operation get`/`wait` only when asynchronous operations actually exist.
- Never equate idempotent orchestration with exactly-once sink effects.

#### Acceptance criteria

- Tests inject response loss and prove a retry does not create a second run.
- idempotency keys are not logged with secret request content;
- local CLI defaults may generate a key for retryable remote calls while still
  exposing explicit control to automation.

### Priority 8: Add `plan` for pipeline definitions

#### Goal

Show resolution, validation, and operational consequences before starting
components.

#### Proposed interface

```bash
flowctl pipeline plan -f stellar-pipeline.yaml
flowctl pipeline plan -f stellar-pipeline.yaml --output json
```

#### Plan content

- parsed API version and pipeline identity;
- selected runtime driver and support level;
- resolved component references, versions, commands/images, and digests where
  available;
- source-to-processor-to-sink edges actually supported by the runtime;
- input/output event compatibility;
- ports, health endpoints, persistence mode, and control-plane mode;
- required secret names without values;
- bounded/unbounded source declarations;
- warnings for experimental drivers or registration-only mode;
- local mutations that start would perform;
- definition digest used to associate a run with the plan.

#### Non-goal

Do not call this Terraform-style apply/reconciliation until flowctl stores
persistent desired pipeline definitions and can safely converge actual state.
Initially, `plan` is an immutable launch preview.

#### Acceptance criteria

- plan performs no launches or durable mutations;
- secret values never appear;
- start and plan share the same resolver and validator;
- a stale plan digest is detected if start is asked to consume an approved
  plan artifact.

### Priority 9: Add capability introspection

#### Goal

Let clients discover what a control plane and component support without
guessing from version strings or help text.

#### Control-plane capabilities

Examples:

- API/protocol versions;
- supported pipeline schema versions;
- supported runtime drivers and maturity;
- event history/watch support;
- idempotency support;
- persistence mode;
- authentication mode;
- maximum message sizes and relevant bounded limits.

#### Component capabilities

Build on component metadata and SDK contracts:

- component kind and version;
- accepted and produced event types;
- bounded-stream semantics;
- health/readiness endpoints;
- supported operations such as checkpoint or drain, only when standardized;
- configuration schema with secret annotations;
- delivery/acknowledgement contract version.

#### Constraints

- Keep common operations typed. Do not replace `stop` or `wait` with an opaque
  generic method call.
- Extension methods require namespaced identifiers, typed arguments, side-effect
  descriptions, and permission requirements.
- Unknown capabilities are ignored by old clients.

#### Acceptance criteria

- The CLI can reject an unsupported operation before partially executing it.
- Capability results have a versioned machine format.
- Runtime support level is truthful; experimental Docker behavior is not
  advertised as equivalent to process mode.

### Priority 10: Add `flowctl doctor`

#### Default checks

- CLI version and executable path;
- configuration file selected and precedence sources;
- active context and namespace;
- persistence path and permissions;
- control-plane reachability and reported capabilities when configured;
- required ports and conflicting listeners for an embedded run;
- process runtime prerequisites;
- component cache integrity and executable permissions;
- pipeline component resolution without exposing secrets;
- schema/protocol compatibility among flowctl, flowctl-sdk, flow-proto, and
  components;
- stale runs whose owning process/control plane no longer exists;
- whether an advertised health endpoint is actually reachable.

#### Modes

```bash
flowctl doctor
flowctl doctor --pipeline stellar-pipeline.yaml
flowctl doctor --control-plane 127.0.0.1:8080
flowctl doctor --output json
```

Destructive repair must never be implicit. If repair commands are later added,
show an explicit plan and require confirmation or `--yes`.

#### Acceptance criteria

- Offline/local checks work without starting a pipeline.
- Remote checks use bounded timeouts.
- Results distinguish failure, warning, and informational findings.
- Suggested remediation uses commands that are tested and current.

### Priority 11: Make contexts, namespaces, and configuration real

#### Problem

Global flags exist, but their semantics and coverage must be consistent before
they can support local, shared, and future hosted control planes.

#### Work

- Define configuration precedence: command flag, environment, selected context,
  config file, default.
- Add `flowctl config view --effective` with redaction.
- Add `flowctl context list|get|use` only after context storage and endpoint/TLS
  semantics are fully specified.
- Define namespace behavior. If namespaces are not enforced by storage and API,
  stop presenting the flag as a production isolation mechanism.
- Keep credentials out of command arguments where process listings could expose
  them.
- Support TLS and authentication configuration consistently across status,
  pipelines, chunks, and future events commands.

#### Acceptance criteria

- All control-plane commands use the same resolved client configuration.
- Effective configuration identifies each value's source.
- namespaces are either enforced end-to-end or clearly marked reserved.
- machine output never contains secret material.

### Priority 12: Add stable audit metadata before multi-user operation

Flowctl does not need a full compliance subsystem for local embedded mode, but
state mutations should carry enough attribution to support future shared use:

- actor identity when authenticated;
- client name/version;
- operation and correlation IDs;
- request source (`cli`, API, automation);
- timestamp;
- target resource;
- redacted reason for stop/retry/checkpoint;
- resulting state/event ID.

Do not claim an authoritative audit log until events are durable, tamper policy
is defined, retention is configured, and authenticated identity exists.

## Command organization

Flowctl currently mixes top-level commands and a plural `pipelines` resource
group. Before expanding the command set, choose and document one grammar.

Recommended long-term form:

```text
flowctl pipeline validate
flowctl pipeline plan
flowctl pipeline run
flowctl pipeline list
flowctl pipeline inspect <run-id>
flowctl pipeline wait <run-id>
flowctl pipeline events <run-id>
flowctl pipeline stop <run-id>

flowctl component list
flowctl component inspect <id>

flowctl chunk list
flowctl chunk inspect <id>
```

However, compatibility matters. Existing `flowctl run`, `validate`, `status`,
and `pipelines ...` commands should remain aliases through a documented
deprecation period if commands move. Do not rename merely for aesthetic
consistency; make the change only when the new resource model adds value.

Global flags should behave uniformly:

- `--output` selects presentation for non-streaming results;
- `--context` selects client/control-plane configuration;
- `--namespace` is meaningful only if enforced;
- `--timeout` should represent an operation deadline, not silently change
  pipeline runtime duration;
- `--yes` suppresses confirmation but never validation;
- `--no-color` and `NO_COLOR` affect human presentation only;
- `--log-format` and `--log-level` affect diagnostics, never command data.

## API and contract strategy

### Canonical protobuf ownership

Flowctl currently has local control-plane protobufs while data-plane component
contracts live in `flow-proto`. Establish clear ownership:

- shared external contracts should live in a versioned contract module;
- generated code should not be the only specification;
- Buf or an equivalent compatibility check should detect breaking protobuf
  changes;
- field numbers are never reused;
- enum values are append-only;
- API versions and server capabilities are discoverable.

Do not move contracts only for repository tidiness. Plan compatibility,
imports, releases, generated code, and migration together.

### Browser-facing API

A future dashboard should call the same application services. Connect RPC,
gRPC-Web, or a small HTTP/SSE adapter can be evaluated after the service and
event contracts stabilize.

The browser API must not become an independent implementation of pipeline
state transitions.

### SDK boundary

Flowctl owns orchestration semantics. SDKs own component registration,
heartbeats, health, and stream-service helpers. Keep contract tests across:

- `flowctl`;
- `flowctl-sdk`;
- `flow-proto`;
- representative source, processor, and sink components;
- JavaScript/TypeScript SDK where supported.

## Observability model

### Logs

- Human logs go to stderr or component log files.
- Structured logs use stable field names but are not a substitute for events.
- Every log line should carry run, component, operation, and correlation IDs
  where applicable.
- Environment maps and credentials are never logged wholesale.

### Progress and metrics

Define each counter precisely:

- transport events received/sent;
- handler successes/failures;
- sink acknowledgements;
- durable destination watermark, when reported by the destination;
- bytes and rates with measurement interval.

Do not populate `events_processed` until its meaning and aggregation are
correct. Prefer omitted/unknown to a misleading zero.

### Health

Separate:

- liveness: process/control-plane responsiveness;
- readiness: able to accept/control work;
- data-plane health: active streams and components functioning;
- durability: sink-specific committed watermark.

A green control-plane heartbeat is not proof that a pipeline is delivering or
committing data.

### Status protocol

Flowctl itself can emit OSC 7501 for foreground `run`, `wait`, plan, component
resolution, and blocking permission/authentication states. OSC is terminal
presentation metadata, not a replacement for durable lifecycle events or JSON
results.

For headless foreground invocation, consider a versioned status-file stream
only if the durable control-plane event API does not already satisfy the use
case. Avoid two competing machine status contracts.

## Reliability requirements

### Supervision

- every delivery goroutine belongs to a supervised group;
- first non-cancellation failure cancels sibling edges;
- shutdown waits for bounded cleanup;
- no event is dropped to keep a run apparently healthy;
- fan-out applies backpressure or fails explicitly;
- sink close acknowledgement is verified;
- unexpected EOF from an unbounded source is failure;
- bounded completion is declared explicitly by the source contract.

### Persistence

- terminal state and first failure survive restart;
- writes are transactionally safe;
- storage migrations are versioned and tested;
- corrupted or incompatible state produces an actionable startup error;
- backup/export is available before destructive migrations;
- embedded persistence defaults are explicit and documented.

### Recovery

Flowctl must not claim checkpoint ownership it does not have. A recovery plan
should identify:

- source restart cursor;
- destination durable watermark;
- component-specific replay behavior;
- orchestration run/attempt identity;
- verification command proving convergence.

Automatic restart should be introduced only after those contracts are
machine-readable and retries are demonstrably safe.

## Security requirements

Before external/shared control-plane use is considered production-ready:

- TLS configuration is required or an explicit insecure-local mode is used;
- clients and components authenticate;
- mutation authorization is enforced server-side;
- secrets use references or protected environment injection, not stored plain
  in run definitions or events;
- APIs have message-size, rate, and concurrency bounds;
- component registration cannot impersonate another run/component;
- local state and sockets have restrictive permissions;
- structured failures, plans, and audit metadata are redacted;
- server-side validation never trusts CLI validation alone.

Secure local IPC may be considered for an always-running local control plane,
but a daemon should not be added until persistent background operation is a
real supported workflow.

## Developer experience

### `flowctl validate`

Validation should provide:

- stable diagnostic codes;
- source locations in YAML where practical;
- all independent validation errors in one run;
- human and versioned JSON output;
- strict mode for warnings;
- offline structural validation by default;
- an explicit resolution/connectivity mode for external checks.

### Schema discovery

Publish the canonical `flowctl/v1` pipeline schema and expose it through a
command such as:

```bash
flowctl schema pipeline --version v1 --output json
```

This supports editors, agents, CI, and form generation without scraping help.

### Shell completion

Generate Bash, Zsh, Fish, and PowerShell completion. Where latency permits,
complete contexts, local pipeline files, run IDs, component IDs, and supported
conditions. Dynamic completion must use short timeouts and fail silently rather
than hanging the shell.

### Examples

- Keep a tiny deterministic process-mode pipeline for first success.
- Validate every checked-in pipeline in CI.
- Exercise the canonical init/validate/plan/run/wait/inspect workflow.
- Keep network-dependent live Stellar tests separate and report exact failure.
- Clearly label experimental driver examples.

## Agent-friendly operation

Agents should interact through typed, discoverable contracts rather than parse
terminal prose. Required foundations:

- stable JSON/YAML schemas;
- typed errors and exit codes;
- capability discovery;
- pipeline schema discovery;
- plan output describing side effects;
- idempotency keys;
- wait and event cursors;
- noninteractive operation;
- explicit confirmation and permission requirements;
- bounded queries and pagination;
- correlation IDs;
- redacted diagnostics.

An MCP adapter can be built on the application client after these contracts
stabilize. Do not make MCP the domain model.

## TUI and dashboard guidance

The existing dashboard or a future Bubble Tea TUI should consume the same
queries, mutations, and event stream as the ordinary CLI. Its update loop must
not contain independent orchestration rules.

Build richer interfaces only after:

- stable snapshot queries exist;
- event cursors solve reconnect races;
- state transitions are centralized;
- machine errors are typed;
- terminal output does not leak into data contracts.

A small run dashboard is then a useful architecture test, not the architecture
itself.

## Explicit deferrals

Defer these until the flagship process runtime and contracts above are solid:

- arbitrary DAG scheduling;
- generalized resource-method invocation;
- Kubernetes/operator reconciliation;
- equally supported Docker and process drivers;
- hosted multi-tenant flowctl;
- full event sourcing as the primary state store;
- automatic infinite component restart;
- cross-region control-plane clustering;
- dynamic in-process plugins;
- a GUI-specific backend;
- billing and organization administration.

## Suggested pull-request sequence

Keep contract work separate from internal refactors and land one independently
verifiable capability per PR.

1. Internal run service and injected CLI renderers
2. Stable error taxonomy and exit codes
3. Machine-output inventory and versioned output schemas
4. Shared embedded/gRPC client contract tests
5. Explicit run and chunk state-transition validators
6. `pipeline wait`
7. Durable lifecycle event storage/outbox
8. Event list API and CLI
9. Event follow with cursors and reconnect tests
10. Idempotent start/stop semantics
11. Pipeline plan and definition digest
12. Capability discovery
13. `flowctl doctor`
14. Effective configuration/context commands
15. Schema discovery and dynamic completion
16. Optional dashboard/TUI migration to the shared client

Do not combine protobuf contract migration, command renaming, and runtime
refactoring in one PR.

## Verification matrix

| Area | Required verification |
|---|---|
| Go implementation | `go test ./...`, `go vet ./...`, `go build ./...` |
| State machine | table-driven legal/illegal/idempotent transition tests |
| Storage | memory/Bolt parity, migration, restart, and injected failure tests |
| CLI | exit-code table, stdout/stderr isolation, golden human output where useful |
| Machine output | schema tests, enum stability, unknown-field compatibility |
| Embedded/remote parity | shared client/service contract suite |
| Wait | completed, failed, stopped, timeout, not-found, interruption |
| Events | atomicity, ordering scope, duplicates, cursor resume, retention expiry |
| Process runtime | bounded success, every edge failure, fan-out, cancellation, no goroutine leak |
| Security | redaction, TLS/auth configuration, unauthorized mutation tests |
| Pipeline definitions | validate all checked-in examples and generated presets |
| Cross-repository | tagged flowctl-sdk/source/processor/sink process E2E |
| Live Stellar gate | bounded archive parity before unbounded tip/reconnect testing |

Tests requiring localhost listeners must run in an environment that permits
binding ephemeral ports. Network-dependent tests must report exact failures and
must not be silently treated as passing.

## Release gates

Before declaring the programmable resource model stable:

- publish compatibility policy for CLI JSON and control-plane protobufs;
- publish supported flowctl/flowctl-sdk/flow-proto version combinations;
- run the full synthetic process E2E;
- run a bounded real Stellar source-to-durable-sink parity test;
- inject source, processor, sink, persistence, and control-plane failures;
- prove restart retains terminal state and events;
- prove event reconnect from a cursor has no gaps;
- verify no secrets appear in plans, errors, events, or persisted run records;
- verify documentation and completion from release artifacts;
- retain a rollback path for storage migrations.

## Definition of success

This roadmap is successful when:

- the CLI, embedded mode, and remote client invoke the same application rules;
- pipelines, runs, components, chunks, operations, and events have clear
  identities and lifecycle semantics;
- automation can plan, start, wait, inspect, follow events, and stop without
  parsing human text or sleeping arbitrarily;
- retries do not create accidental duplicate runs;
- current state and event subscriptions can be combined without missing
  changes;
- failures remain structured, redacted, durable, and attributable to an edge;
- the process runtime remains the trustworthy flagship path;
- a dashboard can be built as another client without duplicating orchestration;
- nebu remains the simple prototyping layer and flowctl remains the production
  lifecycle owner.


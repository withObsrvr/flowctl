# flowctl modernization plan

**Role:** Pipeline supervisor and stream orchestrator
**Local checkout:** `/home/tillman/Documents/flowctl`
**Depends on:** Tagged error-aware `flowctl-sdk` release and a source that uses it

## Implementation notes (current checkout)

The runner now waits for supervised source/processor/sink edges and returns
transport failures as CLI errors. Bounded completion is inferred from the
source's `END_LEDGER` environment setting; an unbounded source EOF is an error.
The first failing edge cancels all streams. Fan-out sends synchronously, so a
slow sink applies backpressure instead of dropping events. The run is marked
failed on delivery error, completed on bounded completion, and stopped on
operator cancellation. Restart and checkpoints remain owned by the operator.

The process-driver synthetic source test in `test/delivery_process_test.go`
exercises nonzero CLI exit, failure before and after one event, EOF, bounded
completion, processor/sink failures, and operator cancellation. The live Stellar
process smoke completed successfully with 101 events at the sink; it requires
prebuilt flowctl and SDK processor binaries and network access.

## Direct wiring and durable failure records

Pipelines without processors now wire `source → sink` only for sinks that
list the source ID in `inputs`, and supervise delivery with the same
cancellation and completion semantics. Components that only register with
the control plane must opt out explicitly using
`spec.registrationOnly: true`. In this mode, `RUNNING` means registered, **not**
that events are being delivered; use it only for intentional monitor-only
pipelines. The operator smoke test uses this explicit mode.

On delivery failure the runner writes a structured, credential-free JSON
record (component, from/to, edge, operation, classified cause) to the existing
`PipelineRun.error` field. Both storage backends persist this field;
`flowctl pipelines run-info` already displays it. The first error record is
kept if further status updates occur. Component heartbeat health metadata
remains a live signal, not the authoritative historical failure reason.

## Remaining limitation

**Boundedness is inferred from `END_LEDGER`.** Sources with other finite
termination criteria are currently classified as unbounded and their normal
EOF fails the run. Introduce an explicit bounded-source setting before
supporting other finite source types.

## Objective

Make pipeline status reflect delivery reality. A failed source, processor,
transport edge, or sink must fail and cancel the run instead of being logged by
a detached goroutine while the pipeline appears healthy.

## Historical failure boundaries (before this implementation)

Previously `internal/runner/stream_orchestrator.go` launched forwarding
goroutines and `WireAll` returned after wiring, not after delivery completed.
Observed behavior included:

- source `Recv` errors are logged and discarded;
- processor `Recv` and `Send` errors are logged and discarded;
- sink `Send` errors may be counted and processing continues;
- sink backpressure timeout drops an event and processing continues;
- `CloseAndRecv` errors are ignored;
- goroutine termination is not delivered to `PipelineRunner`;
- a pipeline can remain apparently alive after its only data path exits.

## Required outcome

Introduce one orchestrator-owned terminal result mechanism. An
`errgroup.WithContext` or equivalent first-error-wins design should:

- attach every forwarding loop to the pipeline context;
- return component and edge identity with every failure;
- cancel all other loops on the first non-cancellation failure;
- wait for all loops to exit;
- return the terminal result to `PipelineRunner`;
- set run/component state to failed;
- cause `flowctl run` to exit nonzero;
- preserve successful bounded completion;
- treat unexpected EOF from an unbounded source as failure.

No event may be dropped to keep the pipeline alive. Backpressure must slow the
pipeline or fail it explicitly.

## Pull-request sequence

1. Add characterization tests for source, processor, and sink edge failures.
2. Add an orchestrator terminal-error channel/group and expose `Wait`.
3. Move source-to-processor forwarding under that lifecycle.
4. Move processor chaining and sink fan-out under the same lifecycle.
5. Remove log-and-continue delivery behavior.
6. Connect terminal result to `PipelineRunner`, run status, and CLI exit code.
7. Add process-driver E2E with a synthetic source that emits once then fails.
8. Document bounded completion, unbounded cancellation, and restart ownership.

## Error context

Every returned/logged delivery error should carry:

- pipeline/run ID;
- source and destination component IDs;
- edge type;
- operation (`recv`, `send`, `close`, registration);
- last event ID/type when safe;
- classified cause.

Never log credentials or complete environment maps.

## Required tests

- source fails before first event;
- source emits one event then fails;
- bounded source completes successfully;
- unbounded source ends unexpectedly;
- processor receive failure;
- processor send failure;
- one fan-out sink fails while another is healthy;
- sink buffer remains full;
- sink `CloseAndRecv` fails;
- operator cancellation returns interrupted rather than failed;
- all forwarding goroutines exit;
- CLI returns nonzero for pipeline failure.

## Verification

```bash
cd /home/tillman/Documents/flowctl
go test ./...
go vet ./...
go build ./...
```

Then run the process-driver smoke from:

```bash
cd /home/tillman/Documents/stellar-raw-ledger-origin
RUN_TIMEOUT=45s scripts/flowctl-process-e2e.sh
```

## Non-goals

- component-specific retry loops;
- source checkpoints;
- automatic infinite restart;
- changing Stellar event schemas;
- container-orchestrator redesign;
- dropping events as a congestion strategy.

## Definition of done

A source terminal error crosses gRPC, reaches `PipelineRunner`, cancels the
pipeline, marks the run failed, stops all forwarding goroutines, and makes
`flowctl run` exit nonzero with the failing edge identified.

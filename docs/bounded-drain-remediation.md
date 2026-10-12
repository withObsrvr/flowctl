# Bounded drain: flowctl runner remediation

**Priority:** P0 correctness; `RUN_STATUS_COMPLETED` is not proof of durable
sink handling with the current SDK.

**Affected release:** v0.0.11 with sinks using `flowctl-sdk` v0.1.4

**Implementation status:** strict per-stream acknowledgements, explicit source
boundedness, and SDK v0.1.5 process-driver drain/failure/stop tests are in this
checkout. Release is **blocked** pending rebuilt real sink binaries and the
120-ledger DuckLake telemetry gate below; no new flowctl tag is justified yet.
The process fixture is a serial 10 ms SDK handler, not a DuckLake commit test.
Run metric redesign is still pending; `events_processed` is not a commit count.

**Depends on:** [SDK consumer/processor remediation](https://github.com/withObsrvr/flowctl-sdk/blob/main/docs/bounded-drain-remediation.md) and upgraded/rebuilt component binaries.

## Reproduction and scope

See `obsrvr-stellar-components/docs/flowctl-bounded-drain-handoff.md` for the
120-ledger pubnet archive run at 62,080,000–62,080,119. The source completed,
flowctl recorded `RUN_STATUS_COMPLETED` and shut components down, but DuckLake
had committed only 32 contiguous watermarks; 88 events were outstanding. The
script correctly failed its external acknowledgement gate. The timed 30/60s
archive probes in that repository are **not** evidence of bounded completion,
sustained throughput, or RPC/tip performance. Treat local `/tmp/` logs as
non-durable evidence.

`internal/runner/stream_orchestrator.go` does wait for forwarding goroutines and
calls each sink's `CloseAndRecv` on upstream EOF. However, `fanOut` currently
accepts a response with `EventsFailed == 0` and no `Errors` as success even
when `EventsConsumed` is too small or the response is nil. `CloseAndRecv`
reflects what the sink **claims**, not what a gRPC `Send` has durably stored.
The v0.1.4 consumer claims accepted/queued events and responds before its
handlers finish. `internal/runner/pipeline_runner.go` therefore selects a nil
terminal error, marks `COMPLETED`, and stops the processes, canceling pending
sink callbacks. Its `eventsCount` is not incremented by delivery, explaining
`events_processed: 0`; it is not a commit count.

## Required flowctl contract

- In each sink stream count only events actually routed to that sink and
  successfully passed to its `Send` (after `inputs`/event-type filtering).
  Count *per stream*, not by source or across all fan-out sinks. This is a
  transport-accepted count, **not** a durable-write count.
- After upstream EOF, half-close every sink stream and wait for its final
  `CloseAndRecv` before declaring delivery complete. Require a **non-nil**
  response, zero `EventsFailed`, empty `Errors`, and `EventsConsumed` equal to
  that stream's sent count. A short count, excess count, failed/unknown status,
  gRPC error, or cancellation before all acks is not `COMPLETED`. Include sink
  ID, expected and acknowledged counts, and operation in the failure context
  without logging payloads or credentials. A sink receiving zero matching
  events must acknowledge zero.
- Require the SDK contract that `EventsConsumed` counts **finished successful
  handlers**; until upgraded SDK-based sinks satisfy it, count comparison
  alone cannot fix the false-success race. A durable sink must return from its
  handler only after commit. No retries/drop-to-survive logic in flowctl.
- The terminal result must reach `PipelineRunner` **before** `COMPLETED` is
  persisted or components are stopped. Errors mark `FAILED` and exit nonzero;
  intentional stop while work is in flight is `STOPPED` (unless a real error
  was already captured), never `COMPLETED`. Wait for all forwarding loops to
  exit on either path. Preserve the first error when stop and delivery failure
  race.
- Do not label gRPC sends as "processed" or "committed". Decide explicit
  metrics for source-emitted, processor-output, sink transport-accepted, sink
  handler-completed, and application-durable watermarks. Do not aggregate a
  fan-out sink count as distinct ledgers. The authoritative durable range
  remains the sink/catalog watermark proof, not the runner's current
  `events_processed` value.

## Work and tests

1. **P0 — Fail closed on final sink responses.** Update `fanOut` and its tests
   in `internal/runner/stream_orchestrator.go` and
   `internal/runner/stream_orchestrator_test.go`. Existing `fakeSink` success
   cases return `(nil, nil)` or an empty response and must be changed to
   acknowledge the actual matching sends. Add missing response, short count,
   excess count, failed count, gRPC error, zero-event, filtered-event,
   multi-source, and fan-out cases; ensure one failed sink fails the run.
2. **P0 — Test the *real SDK* drain semantics across process boundaries.** The
   existing `test/testdata/delivery_component.go` fixture replies to EOF with
   an empty `ConsumeResponse` and does not simulate a slow callback. Replace
   or extend it with an upgraded SDK-backed consumer, a bounded source of 120
   numbered events, and a 10 ms/event serial durable stand-in. Assert no
   `COMPLETED` status or component shutdown before all 120 are acknowledged;
   verify contiguous, distinct IDs and absence of leaked workers. Inject a
   handler failure at N and cancellation during drain: expect `FAILED` and
   `STOPPED` respectively, never a success claim for unfinished work. Check
   that CLI exit status and stored run status/error match the outcome.
3. **P1 — Clarify boundedness and metrics.** `WireAll` currently treats any
   nonempty `END_LEDGER` as bounded, including `"0"`, which the archive probe
   uses for unbounded streaming. Require a documented, explicit source
   boundedness signal rather than inferring it from the mere presence of an
   endpoint value; test ordinary bounded EOF, unbounded unexpected EOF, and
   the `END_LEDGER=0` case. Define and implement useful run metrics without
   conflating transport acceptance with durable commit.
4. **P1 — Update user-facing docs.** Until fixes are released, qualify the
   completion promise in `README.md` and `docs/modernization-plan.md`: v0.0.11
   plus v0.1.4 cannot establish complete bounded ingestion. Document the new
   acknowledgement contract and telemetry semantics after they are verified.

## Release gate and sequencing

1. Release the SDK fix, then update `obsrvr-stellar-components`' `go.mod` and
   `vendor/` (currently v0.1.4) and rebuild the actual processor and sink.
   Do not test the new flowctl against stale vendored binaries.
2. Add flowctl's strict ack checks and process-driver tests; run
   `go test ./...`, `go vet ./...`, and `go build ./...`.
3. Run `obsrvr-stellar-components/scripts/ducklake-telemetry-gate.sh` with the
   documented `TELEMETRY_GATE_START_LEDGER=62080000`,
   `TELEMETRY_GATE_END_LEDGER=62080119`, and `TELEMETRY_GATE_TIMEOUT=300`
   using the rebuilt binaries. Choose a dedicated
   `TELEMETRY_GATE_RUNTIME_DIR` **under `/tmp`** as required by the script. Record a successful bounded process exit,
   sink ack count **120**, and offline `ingest_watermarks` min/max/count/
   distinct/gap proof of all 120 ledgers *before* releasing a new flowctl tag.
   Retain pipeline, server and sink logs/catalog checks outside `/tmp` for
   review. Recheck cancellation/failure paths, not only the happy path.

A source-only archive benchmark (~12 ledgers/s on this workstation) is not a
substitute for this E2E gate. Longer sustained catch-up/tip measurements remain
separate operational work after correctness is proven.

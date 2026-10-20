# AGENTS.md

Instructions for AI agents working in this repository. Humans: see
[CONTRIBUTING.md](CONTRIBUTING.md) — this file is a subset written for automated tools.

## What this is

flowctl is a control plane for Stellar blockchain data pipelines. It runs sources,
processors and sinks as separate processes or containers, wires them together over
gRPC, and tracks their health and run history.

It is open source and must stay fully usable without any hosted OBSRVR service.

## Setup

```bash
nix develop          # or: direnv allow
make test
```

## Commands

| Task | Command |
|---|---|
| Build | `make build` → `./bin/flowctl` |
| Test (all) | `make test` |
| Test (one) | `go test -v ./internal/core -run TestPipelineDAG` |
| Test (race) | `go test -race ./...` |
| Format | `make fmt` |
| Lint | `make lint` |
| Regenerate protobuf | `make generate` |
| Multi-platform build | `make build-all` |

Run a pipeline locally:

```bash
./bin/flowctl validate examples/getting-started/pipeline.yaml
./bin/flowctl run examples/getting-started/pipeline.yaml
```

The sandbox needs Docker or nerdctl installed:

```bash
./bin/flowctl sandbox start --pipeline examples/sandbox-pipeline.yaml --services examples/sandbox.yaml
./bin/flowctl sandbox status
./bin/flowctl sandbox logs
./bin/flowctl sandbox stop
```

## Architecture

```
Sources → Processors → Sinks
    ↓          ↓         ↓
    └──────────┴─────────┘
               ↓
         Control Plane
         (gRPC API)
```

Components register with the control plane at startup and heartbeat for health
monitoring. Two execution models: simple (linear) and DAG (complex topologies).

| Path | Holds |
|---|---|
| `cmd/` | Cobra commands. Adapters only — no business logic. |
| `internal/core/` | Pipeline execution, DAG resolution |
| `internal/runner/` | Pipeline lifecycle, orchestration, supervision |
| `internal/api/` | Control plane gRPC service |
| `internal/config/` | Kubernetes-style YAML config, TLS config |
| `internal/translator/` | Pipeline YAML → docker-compose / local |
| `internal/storage/` | BoltDB and in-memory backends |
| `internal/cliout/` | Output rendering: table, json, ndjson |
| `internal/exitcode/` | Exit code vocabulary and error mapping |
| `internal/sanitize/` | Control-character stripping for untrusted strings |
| `proto/` | gRPC service and message definitions |
| `schemas/cue/schema.cue` | Pipeline config schema (authoritative) |

## Rules that are easy to break by accident

These cause real bugs or standard violations. Check them before proposing a change.

1. **stdout is results only.** Logs, progress, hints and prompts go to stderr, so
   `flowctl … | jq` always works. Use `internal/cliout` for results and
   `internal/utils/logger` for everything else. Never add a bare `fmt.Println` that
   prints status to stdout.

2. **Every machine-readable output carries a schema identifier.** When you add a
   command that emits JSON or NDJSON, give it a schema name (`flowctl.<thing>.v1`) and
   add a golden test. Changing an existing shape is a breaking change.

3. **Exit codes are a contract.** Return errors wrapped by `internal/exitcode` —
   2 usage, 3 not found, 4 unauthorized, 5 conflict, 6 timeout, 7 dependency
   unavailable, 8 data validation. Do not add a bare `os.Exit(1)`.

4. **Anything from a registered component is untrusted.** Service names, metadata and
   relayed errors arrive from processes flowctl did not start. Pass them through
   `internal/sanitize` before they reach a terminal, a log, or persisted state. A
   crafted service name can otherwise inject terminal escape sequences.

5. **Standalone ingestion never depends on the control plane.** A running pipeline must
   keep processing and checkpointing when the control plane is unreachable. Losing the
   control plane degrades observability, never delivery. Treat any change to
   `internal/runner` or `pkg/console/heartbeat` that violates this as a bug.

6. **The two orchestrators must behave identically.** Process and container
   orchestration must produce the same lifecycle, run states and exit behaviour. A fix
   to one usually needs the same fix in the other, and a test covering both.

7. **Never default to pubnet.** The Stellar network is always explicit configuration.

8. **Protobuf changes keep field numbers and wire types**, and `reserve` removed numbers
   and names. `buf breaking` runs in CI.

## Testing expectations

- `go test -race` must pass; the runner is heavily concurrent.
- A bug fix needs a test that fails without the fix.
- Parsers of external input need fuzz tests (`internal/config`, `internal/translator`).
- Prefer real captured data. Invented data is for injecting failures only.

## Contracts and decisions

- Contracts flowctl owns are listed in [`docs/engineering-profile.md`](docs/engineering-profile.md),
  along with the current conformance gaps. Read it before changing a public surface.
- Decisions that change a contract, the architecture or the security posture need an
  ADR in `docs/adr/`. Copy `docs/adr/0000-template.md`. Do not edit an accepted ADR —
  supersede it.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

## Things to leave alone unless asked

- `proto/` package naming. Renaming `flowctl` → `flowctl.v1` is a wire-breaking change
  that needs coordination with flowctl-sdk and every deployed component. It is tracked
  as GAP-7 with an ADR pending.
- The control plane's lack of authentication. It is a known gap (GAP-12) with a design
  decision outstanding, not an oversight to patch ad hoc.

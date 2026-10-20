# flowctl — Engineering Profile

Profile for [OBSRVR Engineering Standard](https://github.com/withObsrvr/ways-of-working) **v2.0**.

| | |
|---|---|
| Repo | `github.com/withobsrvr/flowctl` |
| Owner | Tillman Mosley III (sole maintainer) |
| Current release | `v0.0.12` (pre-1.0) |
| Licence | MPL-2.0 *(claimed in README; `LICENSE` file missing — see GAP-1)* |
| Profile last reviewed | 2026-10-20 |
| Next review | 2027-01-20 (quarterly, CONF-2) |

---

## 1. System classes and applicable sections

flowctl is **two system classes at once**: the `flowctl` binary is a CLI tool, and the
control plane it embeds (and serves standalone via `flowctl server`) is a network service.
Both sets of sections apply; where they overlap, the stricter reading wins.

| Class | Applies |
|---|---|
| Always (§1) | **2, 7, 9, 12** |
| CLI tool | **3, 4, 6, 8, 10, 11** |
| Network service | **4, 5, 6, 8, 10** |

Union: **§2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12.** Every section of the standard applies to
this repo except §13 (this document) and §14 (conformance process).

§11 applies narrowly. flowctl orchestrates components; it does not decode XDR itself.
STL-1 binds the *components* flowctl runs, and flowctl's obligation is to carry and
surface their spec declarations (see rule FC-3 below), not to make them.

---

## 2. Contracts owned (VER-1)

Every contract below is owned by Tillman Mosley III until the team grows. No contract has
a recorded supported-consumer range or deprecation policy yet — closing that is GAP-5.

| Contract | Source of truth | Version | Consumers | Status |
|---|---|---|---|---|
| CLI grammar, flags and arguments | `cmd/**`, cobra definitions | v0.0.12 | Operators, CI jobs, `obsrvr` umbrella CLI | Unstable; pre-1.0 |
| CLI exit codes | *none — not implemented* | — | Any script branching on failure | **Absent** (GAP-3) |
| CLI `--output json` / NDJSON shapes | *none — flag declared, never read* | — | Scripts, agents | **Absent** (GAP-2) |
| Pipeline config format (`apiVersion: flowctl/v1`) | `schemas/cue/schema.cue` | `flowctl/v1` | Pipeline authors, `translate`, `run`, `validate` | Live; enforcement inconsistent (GAP-6) |
| Control plane gRPC API | `proto/control_plane.proto` (package `flowctl`) | unversioned | Components, SDK, `flowctl` subcommands | Live; package not version-qualified (GAP-7) |
| Event envelope | `proto/common.proto` → `Event` | `schema_version` field, per-event | Sources, processors, sinks | Live; missing required fields (GAP-4) |
| Component service contracts | `proto/source.proto`, `processor.proto`, `sink.proto` | unversioned | flowctl-sdk, all components | Live |
| Embedded run-history store | `internal/storage/bolt.go` buckets `services`, `pipeline_runs`, `chunk_runs` | **no format version recorded** | `flowctl pipelines`, `chunks`, `status` | Live; upgrade-unsafe (GAP-8) |
| Translated deployment output | `internal/translator`, `internal/generator` | tracks CLI version | docker-compose / local runners | Live |

**Not owned by this repo:** the `stellar.v1.RawLedger` schema (see the
stellar-raw-ledger-origin profile), and any CAP/SEP decoding, which belongs to the
processors flowctl runs.

---

## 3. Product-specific rules

These tighten the standard for flowctl. They are rules, not aspirations — a change that
breaks one needs an ADR.

- **FC-1 Standalone ingestion never depends on the control plane.** A pipeline that has
  started MUST continue to run, process and checkpoint when the control plane is
  unreachable. Loss of the control plane degrades observability, never delivery. This is
  the load-bearing rule of the whole repo (P-8) and every change to
  `internal/runner` and `pkg/console/heartbeat` is reviewed against it.
- **FC-2 Events carry provenance.** Every event crossing a component boundary MUST carry
  its type and version, the network, the ledger it derives from, a correlation ID and a
  resumable cursor (DATA-6, DATA-7). Carried in the envelope, not in free-form metadata.
- **FC-3 flowctl relays component spec declarations verbatim.** When a component declares
  the CAP or SEP it implements (STL-1), flowctl MUST surface that declaration unaltered
  through registration, `processors show` and machine-readable output. flowctl MUST NOT
  infer, default or normalise it.
- **FC-4 Two orchestrators, one behaviour.** The process and container orchestrators MUST
  produce the same component lifecycle, the same run states and the same exit behaviour.
  Any divergence is a bug, and a fix needs a test covering both (extends TEST-2.5 parity
  to the orchestrator axis).
- **FC-5 The control plane is a trust boundary.** It accepts registrations from processes
  flowctl did not start. Everything it receives — service names, metadata, relayed errors
  — is untrusted input and MUST be sanitised before it reaches a terminal (CLI-22), a log
  or persisted state.
- **FC-6 Run IDs are stable and public.** A run ID, once issued, identifies that run for
  the life of the store and appears in every surface that mentions the run (CLI-4).

---

## 4. Exceptions (§14)

**None recorded. The repo has no `docs/adr/` directory and no ADRs.**

Under CONF-1 this matters: an unmet MUST without a live exception is non-conformance, not
a deferral. Every ✗ in §5 below is currently counted as non-conformance.

Three candidate exceptions are worth recording rather than rushing to fix. Each still
needs an ADR written and accepted before it is a real exception:

| Candidate | Rule | Argument | Proposed expiry |
|---|---|---|---|
| No OSC terminal signalling | CLI-17–19 | SHOULDs, not MUSTs; the protocol is a draft at rev 0.4 and the shared package CLI-17 requires does not exist yet in any OBSRVR repo. Build it once, adopt everywhere. | 2027-04-20 |
| Makefile instead of justfile | BUILD-1 | The standard grandfathers existing Makefiles until the repo is next reworked. `nix develop` + `make test` already gets clone-to-tests in two commands. | 2027-04-20 |
| No reproducible-build verification | BUILD-3 | Depends on the release pipeline rework in GAP-11; fixing it piecemeal before signing lands produces a false sense of assurance. | 2027-01-20 |

CLI-22 (escape injection) is explicitly **not** a candidate for exception — see GAP-9.

---

## 5. Conformance status

Assessed 2026-10-20 against standard v2.0. ✓ met · ◐ partial · ✗ unmet.

| § | Area | Status | Summary |
|---|---|---|---|
| 2 | Principles | ✓ | Architecture follows P-2, P-3, P-5, P-8 well. Headless core in `internal/`, adapters in `cmd/`, components as separate processes over documented gRPC. |
| 3 | CLI | ✗ | Weakest area. CLI-3, 5, 6, 7, 9, 14, 15, 22 unmet; CLI-1, 8, 11, 12, 13 partial. |
| 4 | Data contracts | ◐ | Proto and CUE are authoritative (DATA-5 ✓). Event envelope lacks network, ledger, correlation ID and cursor (DATA-6, DATA-7 ✗). |
| 5 | Network APIs | ✗ | No health service, readiness or metrics (API-13). Errors carry no stable code or request ID (API-3). mTLS available but off by default (API-11 ◐). |
| 6 | Versioning | ✗ | No buf, no `reserved`, no breaking check (VER-5). Changelog has no released versions despite 12 tags (VER-11). Tags unsigned (VER-12). No ADRs (VER-8). |
| 7 | Security | ✗ | Control plane has no authn/authz (SEC-1–4). No `govulncheck` or secret scanning in CI (SEC-6, SEC-9). No signing, SBOM-on-release or provenance (SEC-10). No `LICENSE` or `SECURITY.md` (SEC-11, SEC-12). |
| 8 | Observability | ✗ | Logs are console-encoded, not JSON, and carry no service/version/trace/network/ledger fields (OBS-1 ◐). No OpenTelemetry (OBS-2). flowctl exports no metrics at all (OBS-3). |
| 9 | Testing | ◐ | 24 test files with reasonable unit coverage of DAG, storage, runner and config. No real-chain fixtures (TEST-1), no fuzzing (TEST-3), no `-race` (TEST-4). TEST-2 levels 3–8 absent. Merge gate missing lint, gofmt, govulncheck, secret scanning, buf breaking. |
| 10 | Builds | ◐ | `flake.nix` and `flake.lock` present and current ✓. No `toolchain` line in `go.mod`; CI uses `setup-go`, not the flake (BUILD-2). No `-trimpath`, and `BUILD_DATE` embeds wall-clock time (BUILD-3). No `USER` directive in any Dockerfile (BUILD-4). |
| 11 | Stellar | ◐ | Correctly delegated to components. No pubnet default anywhere ✓ (BUILD-7). FC-3 relay path not yet machine-readable, pending GAP-2. |
| 12 | Documentation | ✗ | Missing `LICENSE`, `SECURITY.md`, `AGENTS.md`, `CONTRIBUTING.md` (DOC-1). CLI reference hand-maintained in a 24KB README (DOC-2). `docs/` mixes tutorials, reference and internal planning (DOC-3). No ADRs (DOC-4). |

### Gap register

Ordered by the standard's own adoption sequence (§14), not by severity alone.

| ID | Gap | Rule | Phase |
|---|---|---|---|
| GAP-1 | No `LICENSE` file; README claims MPL-2.0. Also missing `SECURITY.md`, `AGENTS.md`, `CONTRIBUTING.md` | DOC-1, SEC-11, SEC-12 | 1 |
| GAP-2 | `--output` declared at `cmd/root.go:56` and read nowhere. No JSON, no NDJSON, no schema identifier on any output | CLI-6 | 2 |
| GAP-3 | Only exit codes 0 and 1. Clean SIGINT/SIGTERM shutdown still exits 0, not 130/143 | CLI-15, CLI-11 | 2 |
| GAP-4 | `Event` envelope has no network, ledger, correlation ID or cursor, and no stated delivery semantics | DATA-6, DATA-7, FC-2 | 3 |
| GAP-5 | No contract owners, supported ranges or change classification recorded; no ADR directory | VER-1, VER-3, VER-8, DOC-4 | 1 |
| GAP-6 | Three `apiVersion` validation paths disagree: CUE enforces `^flowctl/v[0-9]+$`, but `internal/runner/loader.go:141` and `internal/translator/parsers/yaml_parser.go:48` only check non-empty, so `flowctl run` silently accepts `apiVersion: v99` | VER-7 | 2 |
| GAP-7 | Proto package is `flowctl`, not `flowctl.v1`. No buf, no lint, no `buf breaking`, no `reserved` statements | DATA-5, VER-5, API-1 | 2 |
| GAP-8 | BoltDB store records no format version, so an upgrade cannot detect or migrate an old store | VER-7 | 3 |
| GAP-9 | No control-character sanitisation of component-supplied strings before they reach the terminal. A registered component can inject OSC sequences via its service name or relayed errors | CLI-22, FC-5 | 2 |
| GAP-10 | Merge gate missing `gofmt`, `golangci-lint`/`staticcheck`, `govulncheck`, secret scanning, `-race`, `buf breaking`. The Makefile has a `lint` target CI never calls | TEST-4, SEC-6, SEC-9, §9 merge gate | 2 |
| GAP-11 | Tags unsigned and lightweight; no Sigstore signing, no SBOM attached to releases, no SLSA provenance; builds non-reproducible (`BUILD_DATE` is wall-clock, no `-trimpath`) | VER-12, SEC-10, BUILD-3 | 5 |
| GAP-12 | Control plane has no authentication, authorization, actor identity or audit log of state-changing calls. Anything that can reach the port can register, read and stop runs | SEC-1, SEC-2, SEC-3, SEC-4 | 4 |
| GAP-13 | No gRPC health service, readiness endpoint or metrics listener; no problem-details-equivalent error codes or request IDs | API-13, API-3 | 3 |
| GAP-14 | No metrics at all: no last-processed-ledger, lag-behind-tip, backlog, checkpoint or gap-count signals. Blocks Lake's freshness and completeness SLOs downstream | OBS-3, OBS-4 | 3 |
| GAP-15 | `viper.AutomaticEnv()` at `cmd/root.go:98` has no `SetEnvPrefix`, so it binds bare `OUTPUT`, `CONTEXT`, `NAMESPACE`, `LOG_LEVEL` instead of `FLOWCTL_*`. Config path hardcodes `$HOME/.config/flowctl`, ignoring `$XDG_CONFIG_HOME`. No `config show` | CLI-9 | 2 |
| GAP-16 | `--output` reused for two unrelated meanings: `init --output <file>` and `processors find --output <event-type>` | CLI-3 | 2 |
| GAP-17 | No colour suppression: no `NO_COLOR`, no `TERM=dumb`, no TTY detection. `logger.go:42` hardcodes a colour encoder | CLI-7 | 2 |
| GAP-18 | Progress and banners written to stdout alongside results | CLI-5 | 2 |
| GAP-19 | No real-chain fixtures; no fuzz tests over YAML/CUE config parsing; no golden, subprocess, parity, interruption-replay or benchmark tests | TEST-1, TEST-2, TEST-3 | 3 |
| GAP-20 | No version-matrix tests for rolling producer/consumer deploys *(the gap named in §13 of the standard)* | VER-9 | 3 |
| GAP-21 | No `toolchain` line in `go.mod`; CI uses `actions/setup-go` rather than the repo flake | BUILD-2 | 5 |
| GAP-22 | No `USER` directive in any Dockerfile — `docker/duckdb-consumer` runs as root on `debian:bookworm-slim`. No OCI annotations | BUILD-4 | 5 |
| GAP-23 | No MCP server or JSON help for agent operation | CLI-14 | 6 |

### Next gap to close

**GAP-1**, then **GAP-10**.

GAP-1 is an afternoon: a `LICENSE` file matching the README's MPL-2.0 claim, plus
`SECURITY.md`, `AGENTS.md` and `CONTRIBUTING.md`. It is the only gap that is currently a
licensing defect rather than an engineering one, and it unblocks the OSPS Baseline L1 work
in phase 5.

GAP-10 is the highest value-per-hour change in the register: adding `gofmt`,
`golangci-lint`, `govulncheck`, secret scanning and `-race` to `.github/workflows/pr-tests.yml`
closes four MUSTs in one PR and stops the rest of the register from growing.

After those, phase 2 is best done as one piece of work rather than gap by gap. An
`internal/cliout` package that owns exit codes (GAP-3), JSON/NDJSON emission with schema
identifiers (GAP-2), stream separation (GAP-18), colour suppression (GAP-17) and
control-character stripping (GAP-9) resolves five gaps behind a single interface, and
every command then converts onto it incrementally. GAP-15 and GAP-16 are small enough to
ride along in the same PR.

---

## 6. Revision history

| Date | Change |
|---|---|
| 2026-10-20 | Initial profile against standard v2.0. Conformance assessed from a repo audit at `3c954ab`. |

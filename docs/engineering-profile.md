# flowctl — Engineering Profile

Profile for [OBSRVR Engineering Standard](https://github.com/withObsrvr/ways-of-working) **v2.0**.

| | |
|---|---|
| Repo | `github.com/withobsrvr/flowctl` |
| Owner | Tillman Mosley III (sole maintainer) |
| Current release | `v0.0.12` (pre-1.0) |
| Licence | MPL-2.0 (`LICENSE`) |
| Profile last reviewed | 2026-10-20 |
| Conformance assessed at | branch `chore/engineering-standard-conformance` |
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
| CLI exit codes | `internal/exitcode` | v1 ([ADR 0001](adr/0001-cli-output-and-exit-code-contract.md)) | Any script branching on failure | Live |
| CLI `--output json` / NDJSON shapes | `internal/cliout` schema ids | `flowctl.<noun>.v1` | Scripts, agents | Live — every record-producing command converted |
| Pipeline config format (`apiVersion: flowctl/v1`) | `schemas/cue/schema.cue` | `flowctl/v1` | Pipeline authors, `translate`, `run`, `validate` | Live; enforced by `internal/apiversion` on every path |
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
  or persisted state. Commands rendering through `internal/cliout` get this at the point a
  cell is formatted; commands not yet converted call `sanitize.Message` on each
  control-plane response, and MUST drop that call only when they move to `cliout`.
- **FC-6 Run IDs are stable and public.** A run ID, once issued, identifies that run for
  the life of the store and appears in every surface that mentions the run (CLI-4).

---

## 4. Exceptions (§14)

### Recorded

| Rule | ADR | Reason | Owner | Expiry |
|---|---|---|---|---|
| DATA-5 (versioned schema name) | [0002](adr/0002-defer-protobuf-package-versioning.md) | Renaming the protobuf package `flowctl` → `flowctl.v1` breaks the wire for every deployed component at once; it needs a coordinated release with flowctl-sdk and the version-matrix tests of VER-9. `buf lint`/`buf breaking` are in CI meanwhile. | Tillman Mosley III | 2027-04-20 |

### Candidates, not yet recorded

Each needs an accepted ADR before it counts as an exception under CONF-1. Until then
the underlying rule is simply unmet.

| Candidate | Rule | Argument | Proposed expiry |
|---|---|---|---|
| No OSC terminal signalling | CLI-17–19 | SHOULDs, not MUSTs; the protocol is a draft at rev 0.4 and the shared package CLI-17 requires does not exist yet in any OBSRVR repo. Build it once, adopt everywhere. | 2027-04-20 |
| Makefile instead of justfile | BUILD-1 | The standard grandfathers existing Makefiles until the repo is next reworked. `nix develop` + `make test` already gets clone-to-tests in two commands. | 2027-04-20 |

CLI-22 (escape injection) was a candidate in the first draft of this profile and is
not one any more: it is closed.

---

## 5. Conformance status

Assessed 2026-10-20 against standard v2.0. ✓ met · ◐ partial · ✗ unmet.

| § | Area | Status | Summary |
|---|---|---|---|
| 2 | Principles | ✓ | Architecture follows P-2, P-3, P-5, P-8 well. Headless core in `internal/`, adapters in `cmd/`, components as separate processes over documented gRPC. |
| 3 | CLI | ◐ | CLI-3, 5, 6, 7, 11, 15, 22 met across every command. CLI-8 now holds too: `init` refuses to prompt when stdin is not a terminal. CLI-1 (grammar), 12 (operation ids, `wait`, `cancel`) and 13 (`doctor`) partial; CLI-14, 17–21 unmet. |
| 4 | Data contracts | ◐ | Proto and CUE are authoritative (DATA-5 ✓). Event envelope lacks network, ledger, correlation ID and cursor (DATA-6, DATA-7 ✗). |
| 5 | Network APIs | ✗ | No health service, readiness or metrics (API-13). Errors carry no stable code or request ID (API-3). mTLS available but off by default (API-11 ◐). |
| 6 | Versioning | ◐ | `buf lint`/`buf breaking` in CI, ADRs in place, changelog rewritten, VER-7 enforced. Tags still unsigned until the next release runs the new workflow (VER-12). Version matrix still missing (VER-9). |
| 7 | Security | ◐ | `govulncheck` and `gitleaks` now gate the merge (SEC-6, SEC-9); signing and SBOM are wired into the release workflow and take effect on the next tag (SEC-10); `LICENSE` and `SECURITY.md` published (SEC-11, SEC-12). The control plane still has no authn/authz — the largest remaining gap (SEC-1–4, GAP-12). |
| 8 | Observability | ✗ | Logs are now JSON off-terminal with `service`, `version` and RFC 3339 UTC timestamps (OBS-1 ◐; still no `trace_id`/`network`/`ledger`). No OpenTelemetry (OBS-2). flowctl still exports no metrics (OBS-3) — the second-largest gap. |
| 9 | Testing | ◐ | Merge gate now complete except change classification, which a PR template prompts for. `-race` runs in CI (TEST-4) and found a real race on first run. Fuzz tests cover the sanitiser and `apiVersion` (TEST-3, partial). Still no real-chain fixtures (TEST-1) and no parity, replay or benchmark tests (TEST-2 levels 5–8). |
| 10 | Builds | ◐ | Go version pinned by the `go` directive in `go.mod` (a bare `toolchain` line is redundant once the two match, and `go mod tidy` — which CI enforces — removes it) and matched by the flake; builds use `-trimpath` and embed only the commit date, verified by `make verify-repro` in the release gate (BUILD-3). Containers run non-root with OCI annotations (BUILD-4). CI still uses `setup-go` rather than the flake (BUILD-2, GAP-21). |
| 11 | Stellar | ◐ | Correctly delegated to components. No pubnet default anywhere ✓ (BUILD-7). FC-3 relay path not yet machine-readable, pending GAP-2. |
| 12 | Documentation | ◐ | DOC-1 met. ADRs in place (DOC-4). CLI reference still hand-maintained in the README (DOC-2) and `docs/` still mixes tutorials, reference and internal planning (DOC-3). |

### Gap register

Ordered by the standard's own adoption sequence (§14). **Closed** entries are kept so
the register reads as a history rather than only a backlog.

| ID | Gap | Rule | Phase | Status |
|---|---|---|---|---|
| GAP-1 | No `LICENSE`, `SECURITY.md`, `AGENTS.md`, `CONTRIBUTING.md` | DOC-1, SEC-11, SEC-12 | 1 | **Closed** |
| GAP-2 | `--output` declared and read nowhere; no JSON, NDJSON or schema identifiers | CLI-6 | 2 | **Closed** — every record-producing command emits json/ndjson with a schema id. `dashboard` and `run`/`server` produce no records and reject or ignore machine formats accordingly |
| GAP-3 | Only exit codes 0 and 1; clean signal shutdown exited 0 | CLI-15, CLI-11 | 2 | **Closed** |
| GAP-4 | `Event` envelope has no network, ledger, correlation ID or cursor | DATA-6, DATA-7, FC-2 | 3 | Open — wire-breaking; pair with GAP-7 |
| GAP-5 | No contract owners or change classification; no ADR directory | VER-1, VER-3, VER-8, DOC-4 | 1 | **Closed** — owners in §2, ADRs in `docs/adr/`, classification in the PR template |
| GAP-6 | Three disagreeing `apiVersion` validators; `flowctl run` accepted `v99` | VER-7 | 2 | **Closed** — `internal/apiversion`, with a test asserting the Go pattern matches the CUE schema |
| GAP-7 | Proto package is `flowctl`, not `flowctl.v1`; no buf | DATA-5, VER-5, API-1 | 2 | **Partial** — `buf lint`/`buf breaking`/generated-code check in CI; rename deferred under [ADR 0002](adr/0002-defer-protobuf-package-versioning.md) |
| GAP-8 | BoltDB store records no format version, so an upgrade cannot migrate | VER-7 | 3 | Open |
| GAP-9 | No control-character sanitisation of component-supplied strings | CLI-22, FC-5 | 2 | **Closed** — `internal/sanitize` for formatted values plus `sanitize.Message` at every command's control-plane ingress, fuzzed, and verified end-to-end against a service registered under a hostile name |
| GAP-10 | Merge gate missing gofmt, staticcheck, govulncheck, secret scanning, `-race`, buf | §9 merge gate | 2 | **Closed** — found a real data race on its first run |
| GAP-11 | Unsigned tags, no SBOM or provenance, non-reproducible builds | VER-12, SEC-10, BUILD-3 | 5 | **Partial** — `-trimpath`, commit-date stamping and `make verify-repro` done; signing and SBOM wired into the release workflow but unproven until the next tag |
| GAP-12 | Control plane has no authentication, authorization, actor identity or audit log | SEC-1–4 | 4 | Open — **largest remaining gap** |
| GAP-13 | No gRPC health service, readiness endpoint, or stable error codes/request IDs | API-13, API-3 | 3 | Open |
| GAP-14 | No metrics: no last-ledger, lag, backlog, checkpoint or gap-count signals | OBS-3, OBS-4 | 3 | Open — blocks Lake's freshness/completeness SLOs |
| GAP-15 | `viper.AutomaticEnv` with no prefix; no XDG support; no `config show` | CLI-9 | 2 | **Closed** |
| GAP-16 | `--output` reused for two unrelated meanings | CLI-3 | 2 | **Closed** — renamed to `--file` and `--output-type` |
| GAP-17 | No colour suppression: no `NO_COLOR`, `TERM=dumb` or TTY detection | CLI-7 | 2 | **Closed** — logger, `cliout`, and `status` (which emitted raw ANSI and a screen-clear unconditionally). `dashboard` relies on lipgloss's own detection. |
| GAP-18 | Progress and banners written to stdout alongside results | CLI-5 | 2 | **Closed** — no bare `fmt.Print*` to stdout remains outside one TTY-gated screen clear; `run`, the live monitor and the component downloaders all write progress to stderr |
| GAP-19 | No real-chain fixtures, fuzzing, golden, subprocess, parity or replay tests | TEST-1, TEST-2, TEST-3 | 3 | **Partial** — fuzz tests for the sanitiser and `apiVersion`; golden tests for the output envelopes; TEST-1 and TEST-2 levels 5–8 still open |
| GAP-20 | No version-matrix tests for rolling deploys *(the gap §13 of the standard names)* | VER-9 | 3 | Open |
| GAP-21 | CI uses `setup-go` rather than the repo flake | BUILD-2 | 5 | **Partial** — the flake now provides Go 1.27.3, which satisfies the `go.mod` minimum, so the dev shell no longer lags and needs no toolchain download. CI still resolves its own Go from `go-version-file` rather than entering the flake, which is the remaining half. |
| GAP-22 | Containers run as root; no OCI annotations | BUILD-4 | 5 | **Closed** |
| GAP-23 | No MCP server or JSON help for agent operation | CLI-14 | 6 | Open |
| GAP-24 | Packages lack package comments (`ST1000` excluded in `staticcheck.conf`) | §9 linters | 2 | Open — mechanical, ~20 packages |
| GAP-25 | `grpc.DialContext`/`WithBlock` deprecated (`SA1019` excluded) | §9 linters | 2 | Open — `NewClient` never blocks, so call sites relying on `WithBlock` change behaviour |
| GAP-26 | `nix build .#default` fails: the derivation runs protobuf generation without the protoc plugins in its `nativeBuildInputs` | BUILD-1 | 5 | Open — pre-existing and unrelated to the Go bump. `nix develop`, the path BUILD-1 documents, is unaffected. The generation step can probably be dropped from the derivation: the generated code is committed and CI checks it is current. |

### Next gap to close

**GAP-13 and GAP-14 together** — the gRPC health service and the first metrics.

These are what make flowctl operable rather than merely runnable. GAP-14 is the one with
consequences outside this repo: without last-ledger, lag-behind-tip, backlog and
gap-count signals, Obsrvr Lake cannot publish the freshness and completeness SLOs OBS-4
requires of it. They belong together because both are served from the same place — a
metrics listener alongside the health service, on a port that is not the gRPC port
(API-13).

After that, in order:

1. **GAP-12** — control plane authn/authz. The biggest single gap, and the one needing a
   design decision first: workload identity versus per-caller keys, and where the
   read/operate split falls. That decision is an ADR, not a patch.
2. **GAP-4 with GAP-7** — the event envelope fields and the `flowctl.v1` rename. Both are
   wire-breaking, so they should cost one coordinated migration rather than two.
3. **GAP-19 and GAP-20** — real-ledger fixtures and the version-matrix tests. VER-9 is
   the gap §13 of the standard names for this repo, and it gates any confident rollout
   of the migration above.

## 6. Revision history

| Date | Change |
|---|---|
| 2026-10-20 | Initial profile against standard v2.0. Conformance assessed from a repo audit at `3c954ab`. |
| 2026-10-20 | Updated after the conformance branch: GAP-1, 3, 5, 6, 9, 10, 15, 16, 17, 22 closed; GAP-2, 7, 11, 18, 19, 21 partial; GAP-24 and GAP-25 added for the two linter exclusions. |
| 2026-10-20 | Output conversion finished: GAP-2 and GAP-18 closed. CLI-8 met by the `init` TTY guard. Next gap is now GAP-13/GAP-14. |

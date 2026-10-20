# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

flowctl is pre-1.0: per SemVer, a minor bump may break. Breaking changes are
called out below and, where they change a contract, carry an ADR in `docs/adr/`.

## [Unreleased]

### Added

- **Machine-readable output on every command that produces records.**
  `--output json` emits one JSON document and `--output ndjson` one object per
  line, each carrying a schema identifier.
  ([ADR 0001](docs/adr/0001-cli-output-and-exit-code-contract.md))

  | Command | Schema |
  |---|---|
  | `version` | `flowctl.version.v1` |
  | `list` | `flowctl.service_list.v1` / `flowctl.service.v1` |
  | `config show` | `flowctl.config_setting_list.v1` / `flowctl.config_setting.v1` |
  | `status` | `flowctl.pipeline_status.v1` |
  | `pipelines list` | `flowctl.pipeline_list.v1` / `flowctl.pipeline.v1` |
  | `pipelines runs` / `active` / `run-info` / `stop` | `flowctl.pipeline_run_list.v1` / `flowctl.pipeline_run.v1` |
  | `chunks list` / `show` | `flowctl.chunk_run_list.v1` / `flowctl.chunk_run.v1` |
  | `processors list` / `find` / `show` | `flowctl.processor_list.v1` / `flowctl.processor.v1` |
  | `sandbox status` | `flowctl.sandbox_container_list.v1` / `flowctl.sandbox_container.v1` |
  | `validate` | `flowctl.validation_result.v1` |
  | `translate` | `flowctl.translation.v1` |
  | `init` | `flowctl.pipeline_init.v1` |

  Machine output carries full values where the table condenses: every event
  type rather than "first (+2)", raw counts, RFC 3339 timestamps, and status as
  a stable lowercase string. The table layout is explicitly not a contract.

  `dashboard` is an interactive terminal UI and rejects a machine format,
  pointing at `flowctl status` instead. `run` and `server` are long-running and
  produce no record; their progress now goes to stderr.
- **Exit codes.** Failures now exit with a code that says why: 2 usage, 3 not
  found, 4 not permitted, 5 conflict, 6 timed out, 7 dependency unavailable,
  8 data validation. A clean shutdown on SIGINT or SIGTERM exits 130 or 143.
- `flowctl config show` prints every setting with the source it came from —
  flag, environment variable, config file or default.
- `LICENSE` (MPL-2.0), `SECURITY.md`, `CONTRIBUTING.md` and `AGENTS.md`.
- `docs/engineering-profile.md`: the contracts this repo owns and its
  conformance gaps against the OBSRVR Engineering Standard v2.0.
- `docs/adr/`: architecture decision records, with a MADR template.
- Merge gate in CI: `gofmt`, `go vet`, `staticcheck`, `go test -race`,
  `govulncheck`, `gitleaks` secret scanning, `buf lint` and `buf breaking`.
- `make test-race`, `make vulncheck` and `make verify-repro`.

### Changed

- **BREAKING: `flowctl init --output <file>` is now `--file` / `-f`.** `-o` and
  `--output` select an output *format* everywhere in flowctl, and reusing the
  name for a filename is forbidden by CLI-3. No alias is kept: a local
  `--output` would shadow the global one and silently write to a file named
  `json`.
- **BREAKING: `flowctl processors find --output <type>` is now `--output-type`.**
  Same reason, same absence of an alias.
- **BREAKING: `flowctl translate --output <format>` is now `--target` / `-t`.**
  It selected a deployment target, not an encoding — the third instance of the
  same collision. `flowctl translate -f p.yaml -t docker-compose`.
- **BREAKING: `flowctl list --table` / `-t` removed.** Table is the default;
  use `--output json` for machine-readable output.
- **BREAKING: `--output yaml` and `--output wide` are now errors.** They were
  advertised and never implemented; they used to be silently ignored.
- **BREAKING: failures no longer always exit 1.** A script testing `$? -eq 1`
  should test `$? -ne 0` instead.
- **BREAKING: `apiVersion` is validated consistently.** The CUE schema required
  `flowctl/v1` but the Go loaders accepted any non-empty string, so
  `apiVersion: v99` was rejected by `flowctl validate` and silently run as v1 by
  `flowctl run`. All three paths now share one validator and reject anything
  that is not `flowctl/v1`, with an actionable error (VER-7). Pipelines using a
  bare `v1` must change to `flowctl/v1`.
- Progress messages, hints and prompts now go to stderr, leaving stdout for
  results so that `flowctl … | jq` always works. That includes the `flowctl run`
  startup banner, the `--show-status` live monitor, and component download
  progress, none of which were results.
- `flowctl init` refuses to start its wizard when stdin is not a terminal,
  instead of reading EOF for every answer and silently building a pipeline from
  defaults (CLI-8). Use `--preset`, or `--non-interactive` with `--network` and
  `--destination`. In table mode it now prints just the created path on stdout,
  so `flowctl run "$(flowctl init --preset testnet-duckdb)"` works.
- `flowctl status --watch` is a usage error when combined with a machine format,
  since redrawing a terminal has no meaning for a consumer reading one document.
- `flowctl validate` always emits a result document in machine mode, including
  when the file fails to load — a caller never has to handle two shapes.
- Colour is suppressed when output is not a terminal, when `NO_COLOR` is set,
  or when `TERM=dumb`.
- Logs are JSON unless attached to a colour-capable terminal, carry `service`
  and `version` on every line, and timestamp in RFC 3339 UTC.
- Environment variables are read with a `FLOWCTL_` prefix. Previously viper's
  `AutomaticEnv` ran with no prefix, binding bare `OUTPUT`, `CONTEXT`,
  `NAMESPACE` and `LOG_LEVEL` from the ambient environment.
- Configuration honours `$XDG_CONFIG_HOME`, and a project `flowctl.yaml` in the
  working directory layers over the user file.
- Release builds are reproducible: `-trimpath`, and the only embedded timestamp
  is the commit's. `make verify-repro` builds twice and compares hashes.
- Container images run as a non-root user and carry OCI source, revision and
  version annotations.

### Fixed

- **Data race in the control plane.** `GetComponentStatus` and `ListComponents`
  returned pointers into the service map that the health janitor mutates, so
  gRPC serialised a struct while the janitor was writing to it. Both now return
  a clone. Found by switching on `go test -race`.
- **Terminal escape injection.** Service names, metadata, event-type strings and
  relayed error messages come from processes flowctl did not start and were
  printed to the terminal unescaped, letting a crafted name emit its own OSC
  sequences — set the window title, plant a misleading hyperlink, or write to
  the clipboard. Every display path is now covered: commands rendering through
  the new output layer sanitise each formatted cell, and the commands not yet
  converted — `status`, `pipelines`, `chunks`, `processors` and `dashboard` —
  sanitise each control-plane response on arrival. JSON output is deliberately
  unaffected and still round-trips the original bytes, since `encoding/json`
  escapes control characters anyway.
- `flowctl status` wrote raw ANSI colour and a screen-clear escape
  unconditionally, so `flowctl status > file` embedded escape codes and
  `flowctl status | grep RUNNING` could fail to match. Colour now follows the
  same rule as everywhere else: off when stdout is not a terminal, when
  `NO_COLOR` is set, or when `TERM=dumb`.
- **SIGINT during startup killed the process.** The signal handler was installed
  only once the pipeline was running, so an interrupt during startup bypassed
  graceful shutdown and orphaned child components. It is now installed before
  anything starts.
- Errors from `src.Events` and `snk.Write` in the simple pipeline were silently
  discarded behind a `// TODO: Handle error`. A dropped sink write is data loss;
  both are now logged.
- A malformed config file is now a usage error instead of being ignored.
- `parseFileMode` had a comparison that could never be true (`byte < 0`). The
  validation was still correct — the upper bound caught every bad input — but
  the dead half has been removed.
- `flowctl pipelines` sliced `run.RunId[:8]` for its short-id column, which
  panics on any run id shorter than eight bytes. The id arrives over the wire
  from the control plane, which is a trust boundary; it now slices by rune count
  and tolerates short input.
- `flowctl validate` exited via a bare `os.Exit(1)`, bypassing the exit-code
  contract. It now exits 8 when validation fails and 3 when the file is missing.
- `flowctl translate` reported an incomplete pipeline — a missing image, say —
  as a generic failure. It is a data validation failure and now exits 8.

### Removed

- Dead code: eleven unused helpers in `internal/translator/utils.go`, unused
  test mocks, an unused `sinkChan` field, and a vestigial builder stage in the
  duckdb-consumer Dockerfile that downloaded a Go toolchain and discarded it.

---

## [0.0.12] and earlier

Released before this changelog was maintained. See the
[git history](https://github.com/withObsrvr/flowctl/commits/main) and the
[release list](https://github.com/withObsrvr/flowctl/releases).

Notable breaking changes in that period:

- Removed `flowctl apply` — use `flowctl translate`.
- Removed `flowctl new` — use `flowctl init`.
- Removed `flowctl context` — the feature was never implemented; use the
  `--context` flag.
- Removed `flowctl help` — use `--help`.

| Old command | New alternative |
|---|---|
| `flowctl apply -f file.yaml --translate-only` | `flowctl translate -f file.yaml` |
| `flowctl new pipeline foo` | `flowctl init` |
| `flowctl context --show` | `--context` flag on commands |
| `flowctl help <cmd>` | `flowctl <cmd> --help` |

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

flowctl is pre-1.0: per SemVer, a minor bump may break. Breaking changes are
called out below and, where they change a contract, carry an ADR in `docs/adr/`.

## [Unreleased]

### Added

- **Machine-readable output.** `--output json` emits one JSON document and
  `--output ndjson` one object per line, each carrying a schema identifier such
  as `flowctl.service_list.v1`. Converted so far: `version`, `list`,
  `config show`. Other commands still print their own tables; they are being
  converted incrementally. ([ADR 0001](docs/adr/0001-cli-output-and-exit-code-contract.md))
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
  results so that `flowctl … | jq` always works.
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
- **Terminal escape injection.** Service names, metadata and relayed errors come
  from processes flowctl did not start and were printed to the terminal
  unescaped, letting a crafted name emit its own OSC sequences — set the window
  title, plant a misleading hyperlink, or write to the clipboard. Untrusted
  strings are now stripped of control characters before display (CLI-22). JSON
  output is unaffected and still round-trips the original bytes.
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

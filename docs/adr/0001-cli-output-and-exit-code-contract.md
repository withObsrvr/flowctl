---
status: accepted
date: 2026-10-20
decision-makers: Tillman Mosley III
consulted: 
informed: 
---

# 0001. Adopt an exit-code vocabulary and a machine-readable output contract

## Context and problem statement

flowctl advertised a global `--output table|yaml|json|wide` flag at `cmd/root.go` that no
command ever read. Every command printed hand-formatted tables with `fmt.Printf`, mixing
results, progress and hints on stdout. Errors all exited 1.

That makes flowctl unusable from a script or an agent: there is nothing to parse and
nothing to branch on. It also violates four MUSTs of the OBSRVR Engineering Standard
v2.0 — CLI-5 (stdout carries results only), CLI-6 (JSON and NDJSON with a schema
identifier), CLI-15 (the exit-code table) and CLI-22 (escape-sequence sanitisation) —
recorded as gaps GAP-2, GAP-3, GAP-9 and GAP-18 in `docs/engineering-profile.md`.

The standard treats JSON output and exit codes as public contracts, as binding as an
API. flowctl therefore needs to define them deliberately, once, rather than letting each
command invent its own.

## Decision drivers

- A script must be able to branch on *why* a command failed, not just that it did.
- `flowctl … | jq` must always work, with no flag juggling.
- The shape of machine-readable output must be versioned from the first release that
  emits it, so later changes are classifiable (VER-3).
- Component-supplied strings reach the terminal; they are untrusted input.
- The conversion must be incremental. Converting 14 commands in one change is not
  reviewable.

## Considered options

1. **Per-command JSON marshalling** — each command marshals its own struct with
   `encoding/json`.
2. **One output package all commands render through**, owning format selection, schema
   identifiers, stream separation, colour suppression and sanitisation.
3. **Adopt an existing CLI framework's output layer** (e.g. kubectl's printers).

## Decision outcome

Chosen option: **Option 2** — three small internal packages:

- `internal/exitcode` — the exit-code vocabulary and error→code mapping.
- `internal/sanitize` — control-character stripping for untrusted strings.
- `internal/cliout` — a `Printer` that renders results as table, JSON or NDJSON,
  writes results to stdout and everything else to stderr, and suppresses colour.

Exit codes follow CLI-15 exactly, with no domain-specific additions yet:

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Operation failed |
| 2 | Usage error |
| 3 | Resource not found |
| 4 | Not authenticated or not permitted |
| 5 | Conflict or failed precondition |
| 6 | Timed out waiting |
| 7 | Dependency unavailable |
| 8 | Data validation failed |
| 130 / 143 | Interrupted (SIGINT / SIGTERM) after clean shutdown |

Codes 10–63 are reserved for domain-specific additions and must be documented here
when first used.

Machine-readable output is enveloped. `--output json` emits one document:

```json
{"schema": "flowctl.service_list.v1", "items": [ … ]}
```

`--output ndjson` emits one object per line, each carrying its own schema identifier,
flushed per line. Schema names are `flowctl.<noun>.v1`, singular for an object and
`<noun>_list` for a collection. Human table output is explicitly *not* a contract and
may change at any time.

Option 1 was rejected because it guarantees divergence: fourteen commands would produce
fourteen envelope conventions, and sanitisation would be forgotten in most of them.
Option 3 was rejected as far more dependency weight than a tool this size needs, and it
would not give us the schema-identifier requirement for free anyway.

### Consequences

- Good: one place to change when the standard's output rules change; one place that
  can be tested for CLI-5, CLI-7 and CLI-22 compliance for every command at once.
- Good: `exitcode.Of` maps gRPC status codes and `context.DeadlineExceeded`
  centrally, so "control plane unreachable" becomes exit 7 everywhere without each
  command knowing.
- Bad: commands not yet converted still print ad-hoc output. The conversion is
  incremental and the gap stays open until it finishes. Converted commands are listed
  in `docs/engineering-profile.md`.
- Bad: `--output yaml|wide` were advertised and are not implemented. They are now
  rejected with a usage error rather than silently ignored, which is a visible
  behaviour change for anyone who passed them.

### Contract impact

| | |
|---|---|
| Affected interfaces | CLI exit codes; `--output` on all converted commands; `init --output`; `processors find --output` |
| Change class | **Breaking** |
| Migration window | None. flowctl is pre-1.0 (v0.0.12); VER-2 permits a minor bump to break. |
| Rollback behaviour | Revert the PR. No persisted state changes format. |
| Tests proving it | Golden tests per schema in `internal/cliout`; exit-code mapping tests in `internal/exitcode`; injection tests in `internal/sanitize`. |

Specific breaks, all recorded in `CHANGELOG.md`:

- Commands that previously exited 1 for every failure now exit 2–8 where the cause is
  known. A script testing `$? -eq 1` must be updated to test `$? -ne 0`.
- `flowctl init --output <file>` is renamed to `--file`/`-f`, because CLI-3 forbids
  reusing a shared option name for a different meaning.
- `flowctl processors find --output <type>` is renamed to `--output-type` for the same
  reason.
- Neither rename keeps a deprecated alias. A local `--output` shadows the global one,
  so an alias would make `--output json` silently mean "write to a file named json" or
  "filter for the event type json" — the precise confusion CLI-3 exists to prevent, and
  worse than a clean break. VER-2 permits breaking on a minor bump before 1.0.
- `--output yaml` and `--output wide` now error instead of being ignored.

## More information

- OBSRVR Engineering Standard v2.0 §3, rules CLI-3, CLI-5, CLI-6, CLI-7, CLI-15, CLI-22.
- `docs/engineering-profile.md` gaps GAP-2, GAP-3, GAP-9, GAP-16, GAP-17, GAP-18.
- Superseded by: none.

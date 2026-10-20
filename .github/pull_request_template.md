## What and why

<!-- What changes, and what problem it solves. Link the issue or ADR. -->

## Contract classification (VER-3)

<!--
Required on any PR that touches a contract. flowctl's contracts are listed in
docs/engineering-profile.md §2: CLI flags and arguments, exit codes, the
--output json/ndjson shapes, the pipeline config format, the gRPC API, the
event envelope, and the on-disk run store.

Delete this section only if the PR touches none of them.
-->

- [ ] **None** — this PR touches no contract.
- [ ] **Additive** — new optional field, new flag, new subcommand. Consumers that
      ignore unknown fields are unaffected.
- [ ] **Deprecating** — something is scheduled for removal. Notice, migration note
      and removal date below.
- [ ] **Breaking** — removed or renamed field, changed type, changed exit code.
      Needs an ADR and a migration window.
- [ ] **Semantic** — same wire format, different meaning. Treated as breaking:
      version bump, parity test, replay decision.

ADR: <!-- docs/adr/NNNN-....md, or "n/a" -->

Migration notes: <!-- what a consumer must do, or "n/a" -->

## Testing

<!--
- A bug fix needs a test that fails without the fix.
- Concurrent code needs `go test -race`.
- Parsers of external input need fuzz tests.
- Prefer real captured data; invented data is for injecting failures only.
-->

- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make test-race` passes

## Checklist

- [ ] Commit messages follow Conventional Commits
- [ ] `CHANGELOG.md` updated if this is user-visible
- [ ] Docs updated (README, `docs/`, `--help` text)
- [ ] No secrets, credentials or customer data in the diff

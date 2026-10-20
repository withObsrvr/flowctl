# Contributing to flowctl

Thanks for your interest. flowctl is a control plane for Stellar data pipelines; it is
open source and meant to stay usable without any hosted OBSRVR service.

## Before you start

- **Found a security problem?** Do not open an issue. See [SECURITY.md](SECURITY.md).
- **Large or cross-cutting change?** Open an issue describing the problem before writing
  code. A change that alters a contract — CLI flags, JSON output, exit codes, config
  format, the gRPC API, or event schemas — needs an
  [ADR](docs/adr/) accepted first. See "Changing a contract" below.
- **Small fix?** Just open a PR.

## Development setup

Two commands from clone to passing tests:

```bash
nix develop     # or: direnv allow
make test
```

Without Nix, you need Go (the version in `go.mod`) and `protoc` with the Go plugins
(`make install-proto-tools`). The Nix shell is what CI uses, so it is the supported path.

Useful targets:

```bash
make build      # build ./bin/flowctl
make test       # run the test suite
make fmt        # gofmt the tree
make lint       # golangci-lint
make generate   # regenerate protobuf code
```

## Making a change

1. Branch from `main`.
2. Write the change and a test that fails without it. For a bug fix this is required —
   the test must demonstrate the bug.
3. Run `make fmt && make test` before pushing. CI runs `gofmt`, `go vet`,
   `golangci-lint`, `go test -race`, `govulncheck`, secret scanning and `buf lint`;
   all of them block the merge.
4. Open a PR describing what changed and why. Link the issue or ADR.

### Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) — release
notes and version bumps are generated from them.

```
feat(cli): add --output ndjson to pipelines list
fix(runner): stop leaking a goroutine when a sink disconnects
docs: document the control plane trust boundary
chore(deps): bump grpc to 1.79.2
```

Types in use: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`,
`style`. Breaking changes get a `!` (`feat(cli)!: …`) and a `BREAKING CHANGE:` footer.

### Changing a contract

flowctl's contracts are listed in [`docs/engineering-profile.md`](docs/engineering-profile.md).
They include CLI flags and arguments, exit codes, `--output json`/`ndjson` shapes, the
pipeline config format, the gRPC API, the event envelope, and the on-disk run store.

Every change to one must be classified in the PR description:

| Class | Example | What it needs |
|---|---|---|
| **Additive** | New optional field, new flag, new subcommand | Minor release. Consumers must ignore unknown fields. |
| **Deprecating** | A flag or field scheduled for removal | Deprecation notice, migration note, removal date |
| **Breaking** | Removed or renamed field, changed type, changed exit code | Major release (pre-1.0: minor), an ADR, a migration window |
| **Semantic** | Same wire format, different meaning | Treated as breaking: version bump, parity test, replay decision |

Protobuf changes must keep field numbers and wire types, and must `reserve` removed
numbers and names. `buf breaking` runs in CI against the last release.

### Architecture Decision Records

Decisions that change a contract, the architecture, or the security posture go in
`docs/adr/NNNN-title.md` in [MADR](https://adr.github.io/madr/) form. Copy
`docs/adr/0000-template.md`. ADRs are not edited after acceptance — supersede them with
a new one.

## Testing expectations

- Concurrent code must pass `go test -race`.
- Parsers of external input (pipeline YAML, CUE, anything off the wire) need fuzz tests.
- A bug fix needs a test that fails without the fix.
- Prefer real captured data over invented data. Invented data is for injecting failures
  — timeouts, malformed input, storage errors — not for showing that a feature works.

## Code style

- `gofmt` is enforced; `golangci-lint` config lives in `.golangci.yml`.
- Logs go to stderr, structured, via `internal/utils/logger`. Results go to stdout via
  `internal/cliout`. Never print results with bare `fmt.Println` — it breaks
  `flowctl … | jq`.
- Anything that arrives from a registered component is untrusted. Sanitize it with
  `internal/sanitize` before it reaches a terminal, a log, or persisted state.
- Return errors wrapped with an exit code (`internal/exitcode`) so the CLI exits with
  something a script can branch on.

## Licence

flowctl is [MPL-2.0](LICENSE). By contributing you agree your contribution is licensed
under it. There is no CLA.

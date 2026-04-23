# flowctl Onboarding Plan

This document turns the onboarding discussion into a concrete implementation plan for making `flowctl` easier to install and easier to succeed with on first run.

## Goals

1. **Install without cloning the repo**
2. **Make the first successful run obvious and fast**
3. **Give the website and README one canonical story**
4. **Remove stale references to the deleted `flowctl quickstart` command**
5. **Align `flowctl` positioning with `nebu`**

## Product framing

- **nebu** = fastest path for querying Stellar data and prototyping processors
- **flowctl** = fastest path for standing up an operable Stellar pipeline

The onboarding path for `flowctl` should optimize for:

```bash
flowctl init --non-interactive --network testnet --destination duckdb
flowctl run stellar-pipeline.yaml
flowctl status
flowctl pipelines active
```

Then, optionally:

```bash
duckdb stellar-pipeline.duckdb "SELECT COUNT(*) FROM contract_events"
```

## Phase 1: Distribution and installability

### Deliverables

- [x] Add version-aware build metadata to `Makefile`
- [x] Add `make install`
- [x] Add release packaging config via `.goreleaser.yaml`
- [x] Add GitHub release workflow
- [x] Add `docs/install.sh` for curl-based installation

### Success criteria

Users can install `flowctl` via one of:

```bash
go install github.com/withobsrvr/flowctl@latest
curl -sSL https://flowctl.withobsrvr.com/install.sh | sh
```

## Phase 2: Canonical first-run journey

### Deliverables

- [x] Rewrite the top of `README.md` around install + first pipeline
- [x] Make DuckDB the clearly recommended first-run path
- [ ] Improve `flowctl init` success output with clearer next steps
- [x] Improve `flowctl run` startup output with clearer readiness state
- [x] Add a preset flag such as `flowctl init --preset testnet-duckdb`

### Success criteria

A new user can copy/paste from the README and get from install to a running local pipeline with no repo checkout and no Docker daemon required for the DuckDB path.

## Phase 3: Public docs and website

### Deliverables

- [x] Replace stale `docs/index.html` landing page with install + first pipeline content
- [x] Add a dedicated quickstart HTML page mirroring the canonical README flow
- [ ] Add troubleshooting sections for:
  - port already in use
  - component download failures
  - missing DuckDB CLI
  - control plane port/address mismatch

### Success criteria

The docs site and README tell the same story and use the same commands.

## Phase 4: Remove stale quickstart-command residue

### Deliverables

- [x] Update `docs/configuration.md` to remove references to the old quickstart format as a current path
- [x] Update `scripts/quickstart.sh` wording so it does not imply a deleted command exists
- [x] Update `docs/index.html` so it no longer advertises `flowctl quickstart run`
- [ ] Continue grepping and cleaning stale references in lower-priority docs

### Success criteria

A user should not encounter documentation that tells them to run a command that does not exist.

## Phase 5: Optional distribution polish

### Possible follow-ups

- [ ] Homebrew tap
- [ ] Better release notes automation
- [ ] Checksums/signature verification in installer
- [ ] Package-manager specific install docs

## Initial implementation scope

This first implementation batch focuses on the highest leverage changes:

1. release/install scaffolding
2. README onboarding rewrite
3. docs landing page rewrite
4. removal of the most visible stale quickstart-command references
5. smoke coverage for the canonical onboarding path in CI

That should be enough to materially improve first impressions and make `flowctl` feel like a shipped product instead of a repo-first developer tool.

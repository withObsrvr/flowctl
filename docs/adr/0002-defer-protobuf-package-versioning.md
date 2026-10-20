---
status: accepted
date: 2026-10-20
decision-makers: Tillman Mosley III
consulted: 
informed: 
---

# 0002. Defer the protobuf package rename to `flowctl.v1`

## Context and problem statement

flowctl's protobuf files declare `package flowctl`, with no version qualifier. The
OBSRVR Engineering Standard v2.0 requires every record type to have a named, versioned
schema (DATA-5), and buf's own style guidance — which VER-5 adopts through
`buf breaking` — expects a version suffix on the package.

Renaming `flowctl` to `flowctl.v1` changes the fully-qualified name of every message
and the gRPC path of every method (`/flowctl.ControlPlane/Register` becomes
`/flowctl.v1.ControlPlane/Register`). It is a hard wire break: every deployed component,
every build of flowctl-sdk, and every third-party processor stops talking to the control
plane at the moment one side is upgraded.

This is recorded as gap GAP-7.

## Decision drivers

- The rename is correct and should happen before 1.0.
- flowctl has no version-matrix tests for rolling deploys (GAP-20, and the gap the
  standard itself names for this repo under VER-9). We cannot currently prove a
  coordinated upgrade works.
- flowctl-sdk and the published processor images are versioned and released separately;
  a wire break needs their release trains lined up.
- The value of the rename is forward-compatibility hygiene. Nothing is broken today by
  the unversioned name.

## Considered options

1. **Rename now**, in the same PR as the rest of the conformance work.
2. **Rename now behind a dual-registration shim** — serve both the old and new service
   paths for a deprecation window.
3. **Defer**, add `buf` lint and breaking-change checking against the current package
   name, and do the rename as its own coordinated change before 1.0.

## Decision outcome

Chosen option: **Option 3, defer.**

`buf.yaml` and `buf.gen.yaml` land now, and CI runs `buf lint` and `buf breaking`
against the last release. The `PACKAGE_VERSION_SUFFIX` lint rule is explicitly excluded,
with a comment pointing at this ADR, so the exclusion is visible to anyone reading the
config rather than silently absent.

Option 1 was rejected because we would be making a wire-breaking change at the same
time as a large CLI change, with no version-matrix test to catch the fallout. Option 2
was rejected as the worst of both: the shim is real work, it doubles the surface to
test, and it would itself need removing later — all to avoid a coordinated release we
have to do anyway.

### Consequences

- Good: `buf lint` and `buf breaking` start protecting the proto files immediately,
  which is the larger share of the value.
- Good: the rename gets done once, deliberately, with the version-matrix tests that
  VER-9 requires already in place.
- Bad: the repo stays non-conformant with DATA-5's versioned-schema requirement until
  then. This exception is the reason that is acceptable rather than an oversight.
- Neutral: `reserve` statements and field-number discipline are enforced from now
  regardless of the package name.

### Contract impact

| | |
|---|---|
| Affected interfaces | `proto/common.proto`, `control_plane.proto`, `source.proto`, `processor.proto`, `sink.proto` |
| Change class | None yet — this ADR defers a Breaking change |
| Migration window | To be set by the superseding ADR; must precede 1.0 |
| Rollback behaviour | N/A |
| Tests proving it | `buf breaking` in CI against the last release tag |

## Exception record (CONF-1)

| | |
|---|---|
| Rule | DATA-5 (versioned schema name) |
| Reason | Wire-breaking; requires coordinated release with flowctl-sdk and deployed components |
| Risk | Low. The unversioned name causes no failure today; it forecloses a clean v2 later. |
| Compensating control | `buf lint` and `buf breaking` in CI from this PR; the rename is a release blocker for 1.0 |
| Owner | Tillman Mosley III |
| Expiry | 2027-04-20 |

## More information

- OBSRVR Engineering Standard v2.0, DATA-5, VER-5, VER-9, CONF-1.
- `docs/engineering-profile.md` gaps GAP-7, GAP-20.
- The rename should also carry the event-envelope fields in GAP-4 (network, ledger,
  correlation ID, cursor), since both are wire-breaking and should cost one migration
  rather than two.

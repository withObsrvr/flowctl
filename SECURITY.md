# Security Policy

## Reporting a vulnerability

**Do not open a public issue for a security vulnerability.**

Report it privately through either channel:

- **GitHub Security Advisories** — [open a private advisory](https://github.com/withObsrvr/flowctl/security/advisories/new)
  on this repository. This is preferred: it gives us a private fork to develop and test
  the fix in, and it issues the CVE.
- **Email** — security@withobsrvr.com. If you want to encrypt, say so in a first
  contactless message and we will send a key.

Please include, as far as you can determine it:

- the flowctl version (`flowctl version`) and how it was installed;
- the component involved — CLI, embedded control plane, standalone `flowctl server`,
  sandbox, or the translation layer;
- what an attacker gains, and what access they need to get it;
- a reproduction, ideally a pipeline YAML plus the commands you ran.

## What to expect

| Stage | Target |
|---|---|
| Acknowledgement of your report | 3 business days |
| Initial assessment and severity | 10 business days |
| Fix released, or a dated plan if it needs longer | 90 days from acknowledgement |

We will tell you when we disagree that something is a vulnerability, and why. We credit
reporters in the advisory unless you ask us not to.

flowctl is maintained by a very small team. If you have not heard from us within the
acknowledgement window, please escalate by emailing tillman@withobsrvr.com directly —
it means something went wrong with routing, not that we are ignoring you.

## Scope

In scope:

- The `flowctl` binary and everything under this repository.
- The control plane gRPC API, embedded or standalone.
- The pipeline configuration parser and the CUE schema, including
  denial-of-service and injection through crafted pipeline YAML.
- The translation layer's generated output (`docker-compose`, local runner scripts),
  including command or argument injection from pipeline configuration.
- Terminal escape-sequence injection through values that reach a user's terminal.

Out of scope:

- Vulnerabilities in the components flowctl runs. Report those to the component's own
  repository. flowctl orchestrates arbitrary user-supplied container images and
  processes, and runs them with the privileges you give it; that is by design.
- The absence of authentication on the control plane. **This is a known, documented
  limitation, not a vulnerability report we need** — see below.
- Findings that require an attacker to already have local code execution as the user
  running flowctl.

## Known limitations

These are deliberate, documented gaps, tracked in
[`docs/engineering-profile.md`](docs/engineering-profile.md). Reporting them is not
necessary — but reporting a *bypass* of a control we claim to have is.

- **The control plane gRPC API has no authentication or authorization** (gap GAP-12).
  Any process that can reach the port can register services, read run state, and stop
  runs. Do not expose the control plane port to an untrusted network. Bind it to
  localhost, or put it behind mTLS (see "Secure Communication with TLS" in the README)
  and a trusted network boundary.
- **Released binaries are not yet signed, and carry no SBOM or provenance** (gap GAP-11).
  Verify downloads against `checksums.txt` on the release until that lands.

## Supported versions

flowctl is pre-1.0. Only the latest released minor version receives security fixes.
Once 1.0 ships, this table will state a support window per the OBSRVR Engineering
Standard (VER-6).

| Version | Supported |
|---|---|
| Latest release | Yes |
| Anything older | No — upgrade |

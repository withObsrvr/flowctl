#!/usr/bin/env bash
#
# Run govulncheck and fail on any reachable vulnerability that is not covered by
# an OpenVEX "not_affected" statement (SEC-9, SEC-12).
#
# govulncheck has no suppression mechanism of its own, and a blanket "|| true"
# would turn the gate into decoration. Instead every exemption must be a
# statement in .vex/flowctl.openvex.json carrying a justification and an impact
# statement, so the reason is reviewable and shows up in a diff.
#
# Exits 0 if the only findings are VEXed, 1 otherwise.

set -euo pipefail

VEX_FILE="${VEX_FILE:-.vex/flowctl.openvex.json}"
GOVULNCHECK_VERSION="${GOVULNCHECK_VERSION:-latest}"
TARGET="${1:-./...}"

if ! command -v jq >/dev/null 2>&1; then
    echo "error: jq is required" >&2
    exit 1
fi

echo "Running govulncheck on ${TARGET}..."
raw=$(go run "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" \
        -format json "${TARGET}" 2>/dev/null) || true

if [ -z "${raw}" ]; then
    echo "error: govulncheck produced no output" >&2
    exit 1
fi

# A finding at the "symbol" level is one govulncheck believes is reachable.
# Anything less specific is informational and does not gate the merge.
found=$(printf '%s\n' "${raw}" \
    | jq -r 'select(.finding != null)
             | select(.finding.trace[0].function != null)
             | .finding.osv' \
    | sort -u)

if [ -z "${found}" ]; then
    echo "No reachable vulnerabilities."
    exit 0
fi

if [ -f "${VEX_FILE}" ]; then
    exempt=$(jq -r '.statements[]
                    | select(.status == "not_affected" or .status == "fixed")
                    | .vulnerability.name' "${VEX_FILE}" | sort -u)
else
    exempt=""
fi

unexempt=$(comm -23 <(printf '%s\n' "${found}") <(printf '%s\n' "${exempt}"))

if [ -n "${exempt}" ]; then
    echo
    echo "Covered by ${VEX_FILE}:"
    comm -12 <(printf '%s\n' "${found}") <(printf '%s\n' "${exempt}") | sed 's/^/  - /'
fi

if [ -n "${unexempt}" ]; then
    echo
    echo "Reachable vulnerabilities with no VEX statement:" >&2
    printf '%s\n' "${unexempt}" | sed 's/^/  - /' >&2
    echo >&2
    echo "Upgrade the dependency, or — only if flowctl is genuinely not affected —" >&2
    echo "add a justified statement to ${VEX_FILE}." >&2
    echo >&2
    echo "Full report:" >&2
    go run "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" "${TARGET}" >&2 || true
    exit 1
fi

echo
echo "All reachable vulnerabilities are covered by ${VEX_FILE}."

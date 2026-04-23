#!/usr/bin/env bash
set -euo pipefail

echo "scripts/smoke-readmes.sh is deprecated; use scripts/smoke-onboarding.sh" >&2
exec "$(dirname "$0")/smoke-onboarding.sh" "$@"

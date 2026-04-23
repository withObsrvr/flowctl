#!/usr/bin/env bash
set -euo pipefail

NETWORK="testnet"
DESTINATION="duckdb"
OUTPUT="stellar-pipeline.yaml"
RUN_PIPELINE=0

usage() {
  cat <<'EOF'
flowctl quickstart helper

Usage:
  ./scripts/quickstart.sh [options]

Options:
  --network <testnet|mainnet>         Network to use (default: testnet)
  --destination <duckdb|postgres>     Destination to use (default: duckdb)
  --output <file>                     Output pipeline file (default: stellar-pipeline.yaml)
  --run                              Run the pipeline after generating and validating it
  -h, --help                         Show this help

Examples:
  ./scripts/quickstart.sh
  ./scripts/quickstart.sh --run
  ./scripts/quickstart.sh --destination postgres --output postgres-pipeline.yaml
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --network)
      NETWORK="$2"
      shift 2
      ;;
    --destination)
      DESTINATION="$2"
      shift 2
      ;;
    --output)
      OUTPUT="$2"
      shift 2
      ;;
    --run)
      RUN_PIPELINE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "${DESTINATION}" != "duckdb" && "${DESTINATION}" != "postgres" ]]; then
  echo "Unsupported destination: ${DESTINATION} (supported: duckdb, postgres)" >&2
  exit 1
fi

if [[ ! -f "go.mod" || ! -f "Makefile" ]]; then
  echo "Run this script from the repository root." >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required but was not found in PATH." >&2
  exit 1
fi

if ! command -v make >/dev/null 2>&1; then
  echo "make is required but was not found in PATH." >&2
  exit 1
fi

if [[ ! -x "./bin/flowctl" ]]; then
  echo "==> Building flowctl"
  make build
else
  echo "==> Using existing ./bin/flowctl"
fi

echo "==> flowctl version"
./bin/flowctl version

echo "==> Generating starter pipeline: ${OUTPUT}"
./bin/flowctl init --non-interactive \
  --network "${NETWORK}" \
  --destination "${DESTINATION}" \
  --output "${OUTPUT}"

echo "==> Validating ${OUTPUT}"
./bin/flowctl validate "${OUTPUT}"

cat <<EOF

Quickstart complete.

Generated pipeline: ${OUTPUT}
Network:            ${NETWORK}
Destination:        ${DESTINATION}

Next steps:
  ./bin/flowctl run ${OUTPUT}

In another terminal:
  ./bin/flowctl status
  ./bin/flowctl pipelines active
EOF

if [[ "${DESTINATION}" == "duckdb" ]]; then
  cat <<EOF

DuckDB note:
  This path does not require a local Docker daemon.
  Components are downloaded automatically on first run.
EOF
fi

if [[ ${RUN_PIPELINE} -eq 1 ]]; then
  echo
  echo "==> Running pipeline"
  exec ./bin/flowctl run "${OUTPUT}"
fi

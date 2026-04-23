#!/usr/bin/env bash
set -euo pipefail

TMP_DIR=$(mktemp -d)
DUCKDB_PID=""
POSTGRES_PID=""
POSTGRES_CONTAINER="flowctl-smoke-postgres"

stop_component_processes() {
  pkill -f '/\.flowctl/components/(duckdb-consumer|postgres-consumer|contract-events-processor|stellar-live-source)/.*/component' >/dev/null 2>&1 || true
}

cleanup() {
  if [[ -n "${DUCKDB_PID}" ]]; then kill "${DUCKDB_PID}" >/dev/null 2>&1 || true; fi
  if [[ -n "${POSTGRES_PID}" ]]; then kill "${POSTGRES_PID}" >/dev/null 2>&1 || true; fi
  stop_component_processes
  docker rm -f "${POSTGRES_CONTAINER}" >/dev/null 2>&1 || true
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}

need go
need make

component_image_available() {
  local image_ref="$1"

  if command -v docker >/dev/null 2>&1; then
    docker manifest inspect "$image_ref" >/dev/null 2>&1
    return $?
  fi

  return 1
}

wait_for_control_plane() {
  local port="$1"
  local pid="$2"
  local log_path="$3"
  local timeout_seconds="${4:-120}"
  local start_time
  start_time=$(date +%s)

  while true; do
    if ./bin/flowctl status --control-plane-address 127.0.0.1 --control-plane-port "$port" >/dev/null 2>&1; then
      return 0
    fi

    if ! kill -0 "$pid" >/dev/null 2>&1; then
      echo "flowctl run exited before control plane on port $port became reachable" >&2
      tail -n 80 "$log_path" >&2 || true
      return 1
    fi

    if (( $(date +%s) - start_time >= timeout_seconds )); then
      echo "timed out waiting for control plane on port $port" >&2
      tail -n 80 "$log_path" >&2 || true
      return 1
    fi

    sleep 2
  done
}

wait_for_duckdb_unlock() {
  local db_path="$1"
  local timeout_seconds="${2:-120}"
  local start_time
  start_time=$(date +%s)

  while true; do
    if command -v lsof >/dev/null 2>&1; then
      if ! lsof "$db_path" >/dev/null 2>&1; then
        return 0
      fi
    elif command -v fuser >/dev/null 2>&1; then
      if ! fuser "$db_path" >/dev/null 2>&1; then
        return 0
      fi
    else
      if ! pgrep -f '/\.flowctl/components/duckdb-consumer/.*/component' >/dev/null 2>&1; then
        return 0
      fi
    fi

    if (( $(date +%s) - start_time >= timeout_seconds )); then
      echo "timed out waiting for DuckDB lock to clear: $db_path" >&2
      return 1
    fi

    sleep 2
  done
}

wait_for_log_pattern() {
  local file_path="$1"
  local pattern="$2"
  local pid="$3"
  local timeout_seconds="${4:-120}"
  local start_time
  start_time=$(date +%s)

  while true; do
    if [[ -f "$file_path" ]] && grep -q "$pattern" "$file_path"; then
      return 0
    fi

    if ! kill -0 "$pid" >/dev/null 2>&1; then
      echo "flowctl run exited before log pattern '$pattern' appeared" >&2
      tail -n 80 "$file_path" >&2 || true
      return 1
    fi

    if (( $(date +%s) - start_time >= timeout_seconds )); then
      echo "timed out waiting for log pattern '$pattern' in $file_path" >&2
      tail -n 80 "$file_path" >&2 || true
      return 1
    fi

    sleep 2
  done
}

wait_for_postgres_events() {
  local container_name="$1"
  local pid="$2"
  local log_path="$3"
  local timeout_seconds="${4:-180}"
  local start_time
  start_time=$(date +%s)

  while true; do
    local pg_count
    pg_count=$(docker exec "$container_name" psql -U postgres -d stellar_events -tAc "SELECT COUNT(*) FROM contract_events;" 2>/dev/null || echo 0)
    if [[ -n "${pg_count}" && "${pg_count}" != "0" ]]; then
      return 0
    fi

    if ! kill -0 "$pid" >/dev/null 2>&1; then
      echo "flowctl run exited before PostgreSQL received events" >&2
      tail -n 80 "$log_path" >&2 || true
      return 1
    fi

    if (( $(date +%s) - start_time >= timeout_seconds )); then
      echo "timed out waiting for PostgreSQL events" >&2
      tail -n 80 "$log_path" >&2 || true
      return 1
    fi

    sleep 2
  done
}

query_duckdb() {
  local db_path="$1"
  local sql="$2"
  local attempt output

  for attempt in $(seq 1 30); do
    if command -v duckdb >/dev/null 2>&1; then
      if output=$(duckdb -readonly -csv "$db_path" "$sql" 2>/dev/null); then
        printf '%s\n' "$output" | tail -n 1 | tr -d '[:space:]'
        return
      fi
    elif command -v python3 >/dev/null 2>&1; then
      if output=$(python3 - <<PY 2>/dev/null
import duckdb
print(duckdb.connect(r'''$db_path''', read_only=True).execute(r'''$sql''').fetchone()[0])
PY
); then
        printf '%s\n' "$output" | tail -n 1 | tr -d '[:space:]'
        return
      fi
    else
      echo "need duckdb CLI or python3 with duckdb module" >&2
      exit 1
    fi

    sleep 1
  done

  echo "failed to query DuckDB after waiting for file lock to clear: $db_path" >&2
  return 1
}

if [[ ! -f "go.mod" || ! -f "Makefile" ]]; then
  echo "run from repository root" >&2
  exit 1
fi

echo "==> Build"
make build >/dev/null

echo "==> Validate README/example pipelines"
./bin/flowctl validate examples/quickstart/testnet-duckdb-pipeline.yaml >/dev/null
./bin/flowctl validate examples/quickstart/testnet-postgres-pipeline.yaml >/dev/null
./bin/flowctl validate schemas/cue/test-pipeline.yaml >/dev/null

if ./bin/flowctl validate schemas/cue/invalid-pipeline.yaml >/dev/null 2>&1; then
  echo "expected schemas/cue/invalid-pipeline.yaml to fail validation" >&2
  exit 1
fi

echo "==> Generate quickstart pipeline"
./scripts/quickstart.sh --output "${TMP_DIR}/stellar-pipeline.yaml" >/dev/null
perl -0pi -e "s#\./stellar-pipeline\.duckdb#${TMP_DIR}/stellar-pipeline.duckdb#g" "${TMP_DIR}/stellar-pipeline.yaml"

echo "==> DuckDB quickstart smoke"
nohup ./bin/flowctl run --control-plane-port 9090 --no-persistence "${TMP_DIR}/stellar-pipeline.yaml" >"${TMP_DIR}/duckdb.log" 2>&1 &
DUCKDB_PID=$!
wait_for_control_plane 9090 "$DUCKDB_PID" "${TMP_DIR}/duckdb.log" 180
./bin/flowctl processors list --endpoint 127.0.0.1:9090 >/dev/null
./bin/flowctl processors find --input stellar.ledger.v1 --endpoint 127.0.0.1:9090 >/dev/null
./bin/flowctl processors find --output stellar.contract.events.v1 --endpoint 127.0.0.1:9090 >/dev/null
wait_for_log_pattern "${TMP_DIR}/duckdb.log" "Stored [0-9][0-9]* contract events" "$DUCKDB_PID" 180
kill "${DUCKDB_PID}" >/dev/null 2>&1 || true
wait "${DUCKDB_PID}" || true
DUCKDB_PID=""
stop_component_processes
wait_for_duckdb_unlock "${TMP_DIR}/stellar-pipeline.duckdb" 180

count=$(query_duckdb "${TMP_DIR}/stellar-pipeline.duckdb" "SELECT COUNT(*) FROM contract_events;")
if [[ -z "${count}" || "${count}" == "0" ]]; then
  echo "duckdb quickstart produced no contract events" >&2
  tail -n 40 "${TMP_DIR}/duckdb.log" >&2 || true
  exit 1
fi

echo "==> PostgreSQL quickstart smoke"
if command -v docker >/dev/null 2>&1; then
  if ! component_image_available "docker.io/withobsrvr/postgres-consumer:v1.0.0"; then
    echo "postgres component image is not publicly available; skipping postgres quickstart smoke"
    echo "smoke tests passed"
    exit 0
  fi
  docker rm -f "${POSTGRES_CONTAINER}" >/dev/null 2>&1 || true
  docker run --name "${POSTGRES_CONTAINER}" -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:16 >/dev/null
  for _ in $(seq 1 30); do
    if docker exec "${POSTGRES_CONTAINER}" pg_isready -U postgres >/dev/null 2>&1; then break; fi
    sleep 2
  done
  docker exec "${POSTGRES_CONTAINER}" createdb -U postgres stellar_events >/dev/null 2>&1 || true

  ./bin/flowctl init --non-interactive --network testnet --destination postgres -o "${TMP_DIR}/postgres-pipeline.yaml" >/dev/null
  nohup ./bin/flowctl run --control-plane-port 9091 --no-persistence "${TMP_DIR}/postgres-pipeline.yaml" >"${TMP_DIR}/postgres.log" 2>&1 &
  POSTGRES_PID=$!
  wait_for_postgres_events "${POSTGRES_CONTAINER}" "$POSTGRES_PID" "${TMP_DIR}/postgres.log" 240
  kill "${POSTGRES_PID}" >/dev/null 2>&1 || true
  wait "${POSTGRES_PID}" || true
  POSTGRES_PID=""
  stop_component_processes

  pg_count=$(docker exec "${POSTGRES_CONTAINER}" psql -U postgres -d stellar_events -tAc "SELECT COUNT(*) FROM contract_events;")
  if [[ -z "${pg_count}" || "${pg_count}" == "0" ]]; then
    echo "postgres quickstart produced no contract events" >&2
    tail -n 40 "${TMP_DIR}/postgres.log" >&2 || true
    exit 1
  fi
else
  echo "docker not available; skipping postgres quickstart smoke"
fi

echo "smoke tests passed"

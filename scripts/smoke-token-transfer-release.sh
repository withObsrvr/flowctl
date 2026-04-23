#!/usr/bin/env bash
set -euo pipefail

# Smoke-test a released token-transfer-processor binary against local flowctl.
#
# What this verifies:
# - released processor artifact downloads and runs
# - flowctl validate works
# - flowctl run works
# - component registration works
# - flowctl status works
# - flowctl processors list works
# - flowctl pipelines active works (with retry)
# - flowctl pipelines run-info works
# - flowctl pipelines stop works
# - sink receives real stellar.token.transfer.v1 events
#
# Usage:
#   ./scripts/smoke-token-transfer-release.sh [v0.1.3]
#
# Optional env:
#   FLOWCTL_SDK_VERSION=v0.1.0
#   FLOW_PROTO_VERSION=v0.1.3
#   START_LEDGER=60200000
#   END_LEDGER=60200100
#   RPC_ENDPOINT=https://archive-rpc.lightsail.network
#   NETWORK_PASSPHRASE='Public Global Stellar Network ; September 2015'

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
RELEASE_TAG="${1:-v0.1.3}"
WORK_DIR=$(mktemp -d /tmp/flowctl-token-transfer-smoke-XXXXXX)
START_LEDGER="${START_LEDGER:-60200000}"
END_LEDGER="${END_LEDGER:-60200100}"
RPC_ENDPOINT="${RPC_ENDPOINT:-https://archive-rpc.lightsail.network}"
NETWORK_PASSPHRASE="${NETWORK_PASSPHRASE:-Public Global Stellar Network ; September 2015}"
FLOWCTL_BIN="$ROOT_DIR/bin/flowctl"
FLOWCTL_SDK_VERSION="${FLOWCTL_SDK_VERSION:-v0.1.0}"
FLOW_PROTO_VERSION="${FLOW_PROTO_VERSION:-v0.1.3}"
LOG_DIR="$WORK_DIR/flowctl-logs"

cleanup() {
  if [[ -n "${FLOWCTL_PID:-}" ]] && kill -0 "$FLOWCTL_PID" 2>/dev/null; then
    kill -INT "$FLOWCTL_PID" 2>/dev/null || true
    sleep 2
    kill -KILL "$FLOWCTL_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

wait_for_contains() {
  local timeout="$1"
  local needle="$2"
  shift 2
  local start now out
  start=$(date +%s)
  while true; do
    if out=$("$@" 2>&1); then
      if grep -q "$needle" <<<"$out"; then
        printf '%s\n' "$out"
        return 0
      fi
    fi
    now=$(date +%s)
    if (( now - start >= timeout )); then
      echo "$out" >&2
      return 1
    fi
    sleep 1
  done
}

free_port() {
  perl -MIO::Socket::INET -e '$s=IO::Socket::INET->new(Listen=>1,LocalAddr=>"127.0.0.1",LocalPort=>0,Proto=>"tcp") or die $!; print $s->sockport; close($s);'
}

require_cmd gh
require_cmd go
require_cmd tar
require_cmd perl

mkdir -p "$WORK_DIR" "$LOG_DIR"
CONTROL_PLANE_PORT=$(free_port)
SOURCE_PORT=$(free_port)
PROCESSOR_PORT=$(free_port)
SINK_PORT=$(free_port)
SOURCE_HEALTH_PORT=$(free_port)
PROCESSOR_HEALTH_PORT=$(free_port)
SINK_HEALTH_PORT=$(free_port)

echo "work dir: $WORK_DIR"
echo "control plane port: $CONTROL_PLANE_PORT"

cat > "$WORK_DIR/go.mod" <<EOF
module token-transfer-smoke

go 1.26.1

require (
	github.com/stellar/go-stellar-sdk v0.5.0
	github.com/withObsrvr/flowctl-sdk ${FLOWCTL_SDK_VERSION}
	github.com/withObsrvr/flow-proto ${FLOW_PROTO_VERSION}
	google.golang.org/protobuf v1.36.11
)
EOF

cat > "$WORK_DIR/source.go" <<'EOF'
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	ledgerbackend "github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/xdr"
	flowctlpkg "github.com/withObsrvr/flowctl-sdk/pkg/flowctl"
	"github.com/withObsrvr/flowctl-sdk/pkg/source"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	stellarv1 "github.com/withObsrvr/flow-proto/go/gen/stellar/v1"
	"google.golang.org/protobuf/proto"
)

func main() {
	componentID := getenv("FLOWCTL_COMPONENT_ID", getenv("COMPONENT_ID", "stellar-source"))
	port := getenv("PORT", ":50051")
	healthPort := mustInt(getenv("HEALTH_PORT", "8088"))
	rpcEndpoint := getenv("RPC_ENDPOINT", "https://archive-rpc.lightsail.network")
	startLedger := uint32(mustInt(getenv("START_LEDGER", "60200000")))
	endLedger := uint32(mustInt(getenv("END_LEDGER", "60200100")))
	flowctlEndpoint := getenv("FLOWCTL_ENDPOINT", "127.0.0.1:8080")
	enableFlowctl := getenv("ENABLE_FLOWCTL", "false") == "true"

	config := source.DefaultConfig()
	config.ID = componentID
	config.Name = "Smoke Stellar Source"
	config.Description = "Fetches a tiny bounded ledger range from Stellar RPC"
	config.Version = "smoke"
	config.Endpoint = port
	config.HealthPort = healthPort
	config.OutputEventTypes = []string{"stellar.ledger.v1"}
	config.FlowctlConfig = &flowctlpkg.Config{
		Enabled:           enableFlowctl,
		Endpoint:          flowctlEndpoint,
		ServiceID:         componentID,
		ServiceType:       flowctlpkg.ServiceTypeSource,
		HeartbeatInterval: flowctlpkg.DefaultHeartbeatInterval,
		Metadata:          map[string]string{"rpc_endpoint": rpcEndpoint},
		OutputEventTypes:  []string{"stellar.ledger.v1"},
	}

	src, err := source.New(config)
	if err != nil {
		log.Fatalf("create source: %v", err)
	}

	backend := ledgerbackend.NewRPCLedgerBackend(ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: rpcEndpoint,
		BufferSize:   4,
	})
	defer backend.Close()

	ctx := context.Background()
	if err := backend.PrepareRange(ctx, ledgerbackend.BoundedRange(startLedger, endLedger)); err != nil {
		log.Fatalf("prepare range: %v", err)
	}

	err = src.OnProduce(func(ctx context.Context, req *flowctlv1.StreamRequest) (<-chan *flowctlv1.Event, error) {
		ch := make(chan *flowctlv1.Event, 4)
		go func() {
			defer close(ch)
			for seq := startLedger; seq <= endLedger; seq++ {
				ledger, err := backend.GetLedger(ctx, seq)
				if err != nil {
					log.Printf("get ledger %d: %v", seq, err)
					return
				}
				xdrBytes, err := ledger.MarshalBinary()
				if err != nil {
					log.Printf("marshal xdr %d: %v", seq, err)
					return
				}
				payload, err := proto.Marshal(&stellarv1.RawLedger{
					Sequence:           seq,
					LedgerCloseMetaXdr: xdrBytes,
					Network:            "public",
				})
				if err != nil {
					log.Printf("marshal proto %d: %v", seq, err)
					return
				}
				event := &flowctlv1.Event{
					Id:          fmt.Sprintf("ledger-%d", seq),
					Type:        "stellar.ledger.v1",
					Payload:     payload,
					Metadata:    map[string]string{"ledger_sequence": fmt.Sprintf("%d", seq)},
					ContentType: "application/protobuf",
					StellarCursor: &flowctlv1.StellarCursor{
						LedgerSequence: uint64(seq),
					},
				}
				select {
				case <-ctx.Done():
					return
				case ch <- event:
					log.Printf("emitted ledger %d", seq)
				}
			}
			log.Printf("finished emitting ledgers %d-%d", startLedger, endLedger)
		}()
		return ch, nil
	})
	if err != nil {
		log.Fatalf("register producer: %v", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := src.Start(runCtx); err != nil {
		log.Fatalf("start source: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	_ = src.Stop()
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustInt(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		panic(err)
	}
	return n
}

var _ xdr.LedgerCloseMeta
EOF

cat > "$WORK_DIR/sink.go" <<'EOF'
package main

import (
	"context"
	"log"
	"os"
	"sync/atomic"

	"github.com/withObsrvr/flowctl-sdk/pkg/consumer"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func main() {
	componentID := getenv("FLOWCTL_COMPONENT_ID", getenv("COMPONENT_ID", "token-transfer-sink"))
	countFile := getenv("COUNT_FILE", "./sink-count.txt")
	var count atomic.Int64

	consumer.Run(consumer.ConsumerConfig{
		ConsumerName: "Token Transfer Smoke Sink",
		ComponentID:  componentID,
		InputTypes:   []string{"stellar.token.transfer.v1"},
		OnEvent: func(ctx context.Context, event *flowctlv1.Event) error {
			n := count.Add(1)
			f, err := os.OpenFile(countFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := f.WriteString(event.Id + "\n"); err != nil {
				return err
			}
			log.Printf("received token transfer batch event %s (count=%d, payload=%d bytes)", event.Id, n, len(event.Payload))
			return nil
		},
	})
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
EOF

cat > "$WORK_DIR/pipeline.yaml" <<EOF
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: token-transfer-smoke
spec:
  driver: process
  sources:
    - id: stellar-source
      command: ["$WORK_DIR/stellar-source"]
      env:
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:$CONTROL_PLANE_PORT"
        COMPONENT_ID: "stellar-source"
        PORT: ":$SOURCE_PORT"
        HEALTH_PORT: "$SOURCE_HEALTH_PORT"
        RPC_ENDPOINT: "$RPC_ENDPOINT"
        START_LEDGER: "$START_LEDGER"
        END_LEDGER: "$END_LEDGER"
  processors:
    - id: token-transfer
      command: ["$WORK_DIR/token-transfer-processor"]
      inputs: ["stellar-source"]
      env:
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:$CONTROL_PLANE_PORT"
        COMPONENT_ID: "token-transfer"
        NETWORK_PASSPHRASE: "$NETWORK_PASSPHRASE"
        PORT: ":$PROCESSOR_PORT"
        HEALTH_PORT: "$PROCESSOR_HEALTH_PORT"
  sinks:
    - id: token-transfer-sink
      command: ["$WORK_DIR/token-transfer-sink"]
      inputs: ["token-transfer"]
      env:
        ENABLE_FLOWCTL: "true"
        FLOWCTL_ENDPOINT: "127.0.0.1:$CONTROL_PLANE_PORT"
        COMPONENT_ID: "token-transfer-sink"
        PORT: ":$SINK_PORT"
        HEALTH_PORT: "$SINK_HEALTH_PORT"
        COUNT_FILE: "$WORK_DIR/sink-count.txt"
EOF

echo "==> downloading released processor $RELEASE_TAG"
cd "$WORK_DIR"
gh release download "$RELEASE_TAG" -R withObsrvr/flowctl-token-transfer-processor -p 'token-transfer-processor-linux-amd64.tar.gz'
tar -xzf token-transfer-processor-linux-amd64.tar.gz
chmod +x token-transfer-processor
./token-transfer-processor --version

echo "==> building smoke source and sink"
go mod tidy
GOFLAGS='' go build -o stellar-source source.go
GOFLAGS='' go build -o token-transfer-sink sink.go

echo "==> building flowctl"
cd "$ROOT_DIR"
make build >/dev/null

echo "==> validating pipeline"
"$FLOWCTL_BIN" validate "$WORK_DIR/pipeline.yaml"

echo "==> starting pipeline"
cd "$WORK_DIR"
rm -f sink-count.txt run.log status.out processors.out active.out runinfo.out stop.out
"$FLOWCTL_BIN" run --show-status=false --no-persistence --log-dir "$LOG_DIR" --control-plane-port "$CONTROL_PLANE_PORT" "$WORK_DIR/pipeline.yaml" > run.log 2>&1 &
FLOWCTL_PID=$!
echo "flowctl pid: $FLOWCTL_PID"

echo "==> waiting for all components to register"
STATUS_OUT=$(wait_for_contains 120 'Components: 3 total (3 healthy)' "$FLOWCTL_BIN" status --control-plane-addr "127.0.0.1:$CONTROL_PLANE_PORT")
printf '%s\n' "$STATUS_OUT" | tee status.out

echo "==> checking processors list"
"$FLOWCTL_BIN" processors list --endpoint "127.0.0.1:$CONTROL_PLANE_PORT" > processors.out
cat processors.out

echo "==> waiting for active pipeline run"
ACTIVE_OUT=$(wait_for_contains 60 'token-transfer-smoke' "$FLOWCTL_BIN" pipelines active --control-plane-address 127.0.0.1 --control-plane-port "$CONTROL_PLANE_PORT")
printf '%s\n' "$ACTIVE_OUT" | tee active.out
RUN_ID=$(printf '%s\n' "$ACTIVE_OUT" | awk 'NF && $1 != "RUN" && $1 != "Found" && $1 != "No" {print $1; exit}')
if [[ -z "$RUN_ID" ]]; then
  echo "failed to parse run id from active output" >&2
  exit 1
fi

echo "==> checking run-info for $RUN_ID"
"$FLOWCTL_BIN" pipelines run-info "$RUN_ID" --control-plane-address 127.0.0.1 --control-plane-port "$CONTROL_PLANE_PORT" > runinfo.out
cat runinfo.out

echo "==> waiting for sink to receive events"
for i in $(seq 1 180); do
  [[ -s sink-count.txt ]] && break
  sleep 1
done
if [[ ! -s sink-count.txt ]]; then
  echo "sink did not receive any events" >&2
  tail -n 200 run.log >&2 || true
  find "$LOG_DIR" -maxdepth 1 -type f -name '*.log' -print -exec tail -n 100 {} \; >&2 || true
  exit 1
fi
cat sink-count.txt

echo "==> stopping pipeline"
"$FLOWCTL_BIN" pipelines stop "$RUN_ID" --control-plane-address 127.0.0.1 --control-plane-port "$CONTROL_PLANE_PORT" > stop.out
cat stop.out

wait "$FLOWCTL_PID" || true

echo "==> final artifacts"
echo "work dir: $WORK_DIR"
echo "logs: $WORK_DIR/run.log"
echo "component logs: $LOG_DIR"
echo "smoke test passed"

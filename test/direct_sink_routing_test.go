package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Each source must reach only sinks which declare it in inputs. Event-type
// filtering alone is insufficient when both sources emit the same type.
func TestProcessDirectSinkInputs(t *testing.T) {
	if testing.Short() {
		t.Skip("process-driver integration")
	}
	root := findRepoRoot(t)
	dir := t.TempDir()
	flowctl := filepath.Join(dir, "flowctl")
	fixture := filepath.Join(dir, "delivery-component")
	buildGoBinary(t, root, flowctl, ".")
	buildGoBinary(t, root, fixture, filepath.Join(root, "test", "testdata", "delivery_component.go"))
	pipeline := fmt.Sprintf(`apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: direct-routing
spec:
  driver: process
  sources:
    - id: source-a
      command: [%q]
      env:
        DELIVERY_ROLE: source
        DELIVERY_MODE: bounded
        END_LEDGER: "2"
    - id: source-b
      command: [%q]
      env:
        DELIVERY_ROLE: source
        DELIVERY_MODE: bounded
        END_LEDGER: "2"
  sinks:
    - id: sink-a
      command: [%q]
      inputs: [source-a]
      env:
        DELIVERY_ROLE: sink
    - id: sink-b
      command: [%q]
      inputs: [source-b]
      env:
        DELIVERY_ROLE: sink
`, fixture, fixture, fixture, fixture)
	file := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(file, []byte(pipeline), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, flowctl, "run", "--no-persistence", "--show-status=false", "--control-plane-port=0", file)
	cmd.Dir = dir // process driver writes component logs to its working directory
	out, err := cmd.CombinedOutput()
	if err != nil || ctx.Err() != nil {
		t.Fatalf("pipeline did not complete: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "RUN_STATUS_COMPLETED") {
		t.Fatalf("run not completed:\n%s", out)
	}
	for _, tc := range []struct{ sink, want, reject string }{
		{"sink-a", "source-a", "source-b"},
		{"sink-b", "source-b", "source-a"},
	} {
		contents, err := os.ReadFile(filepath.Join(dir, "logs", tc.sink+".log"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), "DELIVERY_RECEIVED id="+tc.want) || strings.Contains(string(contents), "DELIVERY_RECEIVED id="+tc.reject) {
			t.Fatalf("%s received wrong source:\n%s", tc.sink, contents)
		}
	}
}

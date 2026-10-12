package test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/withobsrvr/flowctl/internal/storage"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercises the CLI, process driver, control-plane registration, and gRPC
// forwarding with a source that returns a non-EOF error across the wire.
func TestProcessDeliveryTerminalResult(t *testing.T) {
	if testing.Short() {
		t.Skip("process-driver integration")
	}
	root := findRepoRoot(t)
	dir := t.TempDir()
	flowctl := filepath.Join(dir, "flowctl")
	fixture := filepath.Join(dir, "delivery-component")
	buildGoBinary(t, root, flowctl, ".")
	buildGoBinary(t, root, fixture, filepath.Join(root, "test", "testdata", "delivery_component.go"))
	for _, tc := range []struct {
		mode, processorMode, sinkMode string
		bounded, failed, direct       bool
		want, edge                    string
	}{
		{"fail-before", "", "", false, true, false, "synthetic source failure", "source -> processor"},
		{"fail-after", "", "", false, true, false, "synthetic source failure", "source -> processor"},
		{"bounded", "", "", true, false, false, "RUN_STATUS_COMPLETED", ""},
		{"unbounded-eof", "", "", false, true, false, "unexpected EOF", "source -> processor"},
		{"bounded", "processor-fail", "", true, true, false, "synthetic processor failure", "processor -> sinks"},
		{"bounded", "", "sink-fail", true, true, false, "synthetic sink failure", "processor -> sink"},
		{"wait", "", "", false, false, false, "RUN_STATUS_STOPPED", ""},
		{"fail-before", "", "", false, true, true, "synthetic source failure", "source -> sinks"},
		{"fail-after", "", "", false, true, true, "synthetic source failure", "source -> sinks"},
		{"unbounded-eof", "", "", false, true, true, "unexpected EOF", "source -> sinks"},
		{"bounded", "", "", true, false, true, "RUN_STATUS_COMPLETED", ""},
		{"bounded", "", "sink-fail", true, true, true, "synthetic sink failure", "source -> sink"},
	} {
		t.Run(fmt.Sprintf("%s-%s-%s-direct=%t", tc.mode, tc.processorMode, tc.sinkMode, tc.direct), func(t *testing.T) {
			sourceEnv := fmt.Sprintf("        DELIVERY_MODE: %s\n", tc.mode)
			if tc.bounded {
				sourceEnv += "        END_LEDGER: \"2\"\n"
			}
			processorSection := fmt.Sprintf(`  processors:
    - id: processor
      command: [%q]
      inputs: [source]
      env:
        DELIVERY_ROLE: processor
        DELIVERY_MODE: %q
`, fixture, tc.processorMode)
			sinkInput := "processor"
			if tc.direct {
				processorSection = ""
				sinkInput = "source"
			}
			pipeline := fmt.Sprintf(`apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: delivery-test
spec:
  driver: process
  sources:
    - id: source
      command: [%q]
      env:
        DELIVERY_ROLE: source
%s%s  sinks:
    - id: sink
      command: [%q]
      inputs: [%s]
      env:
        DELIVERY_ROLE: sink
        DELIVERY_MODE: %q
`, fixture, sourceEnv, processorSection, fixture, sinkInput, tc.sinkMode)
			file := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-")+".yaml")
			db := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-")+".db")
			if err := os.WriteFile(file, []byte(pipeline), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, flowctl, "run", "--show-status=false", "--db-path", db, "--control-plane-port=0", "--log-dir", filepath.Join(dir, tc.mode+"-logs"), file)
			cmd.Dir = root
			var out []byte
			var err error
			if tc.mode == "wait" {
				var buf bytes.Buffer
				cmd.Stdout, cmd.Stderr = &buf, &buf
				if err = cmd.Start(); err != nil {
					t.Fatal(err)
				}
				time.Sleep(2 * time.Second)
				if err = cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
				err = cmd.Wait()
				out = buf.Bytes()
			} else {
				out, err = cmd.CombinedOutput()
			}
			if ctx.Err() != nil {
				t.Fatalf("run timed out: %s", out)
			}
			if tc.failed && err == nil {
				t.Fatalf("expected nonzero CLI exit: %s", out)
			}
			if !tc.failed && err != nil {
				t.Fatalf("expected successful completion: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("missing %q in output:\n%s", tc.want, out)
			}
			if tc.failed && !strings.Contains(string(out), tc.edge) {
				t.Fatalf("missing failing edge:\n%s", out)
			}
			if tc.failed && !strings.Contains(string(out), "RUN_STATUS_FAILED") {
				t.Fatalf("run was not marked failed:\n%s", out)
			}
			store := storage.NewBoltDBStorage(&storage.BoltOptions{Path: db})
			if err := store.Open(); err != nil {
				t.Fatal(err)
			}
			runs, err := store.ListPipelineRuns(context.Background(), "delivery-test", flowctlpb.RunStatus_RUN_STATUS_UNKNOWN, 10)
			_ = store.Close()
			if err != nil || len(runs) != 1 {
				t.Fatalf("stored runs=%d err=%v", len(runs), err)
			}
			if tc.failed {
				if (!strings.Contains(runs[0].Run.Error, `"from":"source"`) && !strings.Contains(runs[0].Run.Error, `"from":"processor"`)) || !strings.Contains(runs[0].Run.Error, `"cause":`) || strings.Contains(runs[0].Run.Error, "synthetic source failure") {
					t.Fatalf("missing or unsafe persisted structured failure: %s", runs[0].Run.Error)
				}
			} else if runs[0].Run.Error != "" {
				t.Fatalf("unexpected persisted error: %s", runs[0].Run.Error)
			}
		})
	}
}

package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withobsrvr/flowctl/internal/exitcode"
	"github.com/withobsrvr/flowctl/internal/storage"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
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
		{"drain", "", "drain", true, false, true, "RUN_STATUS_COMPLETED", ""},
		{"drain", "", "drain-fail", true, true, true, "failed 1", "source -> sink"},
		{"drain-stop", "", "drain", true, false, true, "RUN_STATUS_STOPPED", ""},
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
				sourceEnv += "        END_LEDGER: \"2\"\n        FLOWCTL_SOURCE_BOUNDED: \"true\"\n"
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
			cmd.Dir = filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-"))
			if err := os.MkdirAll(cmd.Dir, 0700); err != nil {
				t.Fatal(err)
			}
			var out []byte
			var err error
			if tc.mode == "wait" || tc.mode == "drain-stop" {
				var buf bytes.Buffer
				cmd.Stdout, cmd.Stderr = &buf, &buf
				if err = cmd.Start(); err != nil {
					t.Fatal(err)
				}
				if tc.mode == "drain-stop" {
					logPath := filepath.Join(cmd.Dir, "logs", "sink.log")
					deadline := time.Now().Add(10 * time.Second)
					for time.Now().Before(deadline) {
						contents, _ := os.ReadFile(logPath)
						if strings.Contains(string(contents), "DELIVERY_RECEIVED id=") {
							break
						}
						time.Sleep(10 * time.Millisecond)
					}
				} else {
					time.Sleep(2 * time.Second)
				}
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
			signalled := tc.mode == "wait" || tc.mode == "drain-stop"
			switch {
			case tc.failed:
				if err == nil {
					t.Fatalf("expected nonzero CLI exit: %s", out)
				}
			case signalled:
				// CLI-11/CLI-15: a clean shutdown on SIGINT exits 130, so a
				// supervisor can tell "interrupted, resume from checkpoint"
				// apart from "finished".
				if got := exitCodeOf(err); got != exitcode.Interrupted {
					t.Fatalf("expected exit %d after SIGINT, got %d (%v)\n%s",
						exitcode.Interrupted, got, err, out)
				}
			case err != nil:
				t.Fatalf("expected successful completion: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("missing %q in output:\n%s", tc.want, out)
			}
			if tc.failed && !strings.Contains(string(out), tc.edge) {
				t.Fatalf("missing failing edge:\n%s", out)
			}
			if tc.mode == "drain-stop" {
				contents, readErr := os.ReadFile(filepath.Join(cmd.Dir, "logs", "sink.log"))
				if readErr != nil || strings.Contains(string(contents), "DELIVERY_RECEIVED id=119") {
					t.Fatalf("drain did not stop in flight: %v %s", readErr, contents)
				}
			}
			if tc.mode == "drain" && !tc.failed {
				contents, readErr := os.ReadFile(filepath.Join(cmd.Dir, "logs", "sink.log"))
				if readErr != nil {
					t.Fatal(readErr)
				}
				seen := make(map[string]bool)
				for _, line := range strings.Split(string(contents), "\n") {
					if i := strings.Index(line, "DELIVERY_RECEIVED id="); i >= 0 {
						seen[strings.TrimSpace(line[i+len("DELIVERY_RECEIVED id="):])] = true
					}
				}
				if len(seen) != 120 {
					t.Fatalf("sink did not finish all 120: %d unique\n%s", len(seen), contents)
				}
				for i := 0; i < 120; i++ {
					if !seen[fmt.Sprint(i)] {
						t.Fatalf("missing %d", i)
					}
				}
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
			if (tc.failed && runs[0].Run.Status != flowctlpb.RunStatus_RUN_STATUS_FAILED) || (tc.mode == "drain-stop" && runs[0].Run.Status != flowctlpb.RunStatus_RUN_STATUS_STOPPED) || (!tc.failed && tc.mode != "wait" && tc.mode != "drain-stop" && runs[0].Run.Status != flowctlpb.RunStatus_RUN_STATUS_COMPLETED) {
				t.Fatalf("wrong stored status: %s", runs[0].Run.Status)
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

// exitCodeOf reports the process exit code behind an *exec.ExitError, or -1 if
// err is not one. A nil error means the process exited 0.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return ee.ExitCode()
	}
	return -1
}

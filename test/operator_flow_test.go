package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestFlagshipOperatorFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping operator smoke test in short mode")
	}

	repoRoot := findRepoRoot(t)
	tempDir := t.TempDir()

	flowctlBin := filepath.Join(tempDir, "flowctl")
	buildGoBinary(t, repoRoot, flowctlBin, ".")

	componentSrc := filepath.Join(tempDir, "test_component.go")
	if err := os.WriteFile(componentSrc, []byte(testComponentProgram), 0644); err != nil {
		t.Fatalf("failed to write test component source: %v", err)
	}

	componentBin := filepath.Join(tempDir, "test-component")
	buildGoBinary(t, repoRoot, componentBin, componentSrc)

	controlPlanePort := freeTCPPort(t)
	pipelinePath := filepath.Join(tempDir, "operator-pipeline.yaml")
	pipelineYAML := fmt.Sprintf(`apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: operator-smoke
spec:
  driver: process
  registrationOnly: true
  sources:
    - id: test-source
      command: [%q]
      env:
        TEST_COMPONENT_ROLE: source
  sinks:
    - id: test-sink
      command: [%q]
      inputs: [test-source]
      env:
        TEST_COMPONENT_ROLE: sink
`, componentBin, componentBin)
	if err := os.WriteFile(pipelinePath, []byte(pipelineYAML), 0644); err != nil {
		t.Fatalf("failed to write pipeline yaml: %v", err)
	}

	validateOut := runCommand(t, repoRoot, 10*time.Second, flowctlBin, "validate", pipelinePath)
	assertContains(t, validateOut, "Pipeline validation passed")

	runCmd := exec.Command(flowctlBin,
		"run",
		"--show-status=false",
		"--no-persistence",
		"--control-plane-port", fmt.Sprintf("%d", controlPlanePort),
		pipelinePath,
	)
	runCmd.Dir = repoRoot
	var runOutput bytes.Buffer
	runCmd.Stdout = &runOutput
	runCmd.Stderr = &runOutput
	if err := runCmd.Start(); err != nil {
		t.Fatalf("failed to start flowctl run: %v", err)
	}

	runDone := make(chan error, 1)
	go func() {
		runDone <- runCmd.Wait()
	}()

	t.Cleanup(func() {
		if runCmd.Process == nil {
			return
		}
		if runCmd.ProcessState != nil && runCmd.ProcessState.Exited() {
			return
		}

		_ = runCmd.Process.Signal(os.Interrupt)

		select {
		case <-time.After(10 * time.Second):
			_ = runCmd.Process.Kill()
			select {
			case <-runDone:
			default:
			}
		case <-runDone:
		}
	})

	endpoint := fmt.Sprintf("127.0.0.1:%d", controlPlanePort)

	statusOut := waitForCommand(t, 20*time.Second, func() (string, error) {
		out, err := runCommandE(repoRoot, 5*time.Second, flowctlBin,
			"status",
			"--control-plane-addr", endpoint,
		)
		if err == nil && (!strings.Contains(out, "test-source") || !strings.Contains(out, "test-sink")) {
			err = fmt.Errorf("components not registered yet")
		}
		return out, err
	})
	assertContains(t, statusOut, "test-source")
	assertContains(t, statusOut, "test-sink")
	assertContains(t, statusOut, "HEALTH_STATUS_HEALTHY")

	activeOut := waitForCommand(t, 10*time.Second, func() (string, error) {
		out, err := runCommandE(repoRoot, 5*time.Second, flowctlBin,
			"pipelines",
			"active",
			"--control-plane-address", "127.0.0.1",
			"--control-plane-port", fmt.Sprintf("%d", controlPlanePort),
		)
		if err == nil && !strings.Contains(out, "operator-smoke") {
			err = fmt.Errorf("pipeline run not visible yet")
		}
		return out, err
	})
	assertContains(t, activeOut, "operator-smoke")
	assertContains(t, activeOut, "running")

	shortRunID := parseFirstRunID(t, activeOut)
	if len(shortRunID) != 8 {
		t.Fatalf("expected shortened run id from pipelines active, got %q", shortRunID)
	}

	// Assert against --output json, not the table. The human layout is
	// explicitly not a contract (ADR 0001); the schema is.
	runInfo := runCommandJSON(t, repoRoot, 5*time.Second, flowctlBin,
		"pipelines",
		"run-info", shortRunID,
		"--control-plane-address", "127.0.0.1",
		"--control-plane-port", fmt.Sprintf("%d", controlPlanePort),
		"--output", "json",
	)
	if got := runInfo["schema"]; got != "flowctl.pipeline_run.v1" {
		t.Errorf("run-info schema = %v", got)
	}
	data := runInfo["data"].(map[string]any)
	if got := data["pipeline_name"]; got != "operator-smoke" {
		t.Errorf("pipeline_name = %v", got)
	}
	if got := data["status"]; got != "running" {
		t.Errorf("status = %v", got)
	}
	if !strings.HasPrefix(data["run_id"].(string), shortRunID) {
		t.Errorf("run_id %v does not start with the short id %q", data["run_id"], shortRunID)
	}
	components, _ := data["component_ids"].([]any)
	joined := fmt.Sprint(components...)
	if !strings.Contains(joined, "test-source") || !strings.Contains(joined, "test-sink") {
		t.Errorf("component_ids = %v", components)
	}

	// The human path must still render; its exact spacing is not asserted.
	runInfoOut := runCommand(t, repoRoot, 5*time.Second, flowctlBin,
		"pipelines",
		"run-info", shortRunID,
		"--control-plane-address", "127.0.0.1",
		"--control-plane-port", fmt.Sprintf("%d", controlPlanePort),
	)
	assertContains(t, runInfoOut, "operator-smoke")
	assertContains(t, runInfoOut, "running")

	stopped := runCommandJSON(t, repoRoot, 5*time.Second, flowctlBin,
		"pipelines",
		"stop", shortRunID,
		"--control-plane-address", "127.0.0.1",
		"--control-plane-port", fmt.Sprintf("%d", controlPlanePort),
		"--output", "json",
	)
	if got := stopped["schema"]; got != "flowctl.pipeline_run.v1" {
		t.Errorf("stop schema = %v", got)
	}
	if got := stopped["data"].(map[string]any)["status"]; got == "running" {
		t.Errorf("run still reports %v after stop", got)
	}

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("flowctl run exited with error after stop: %v\nOutput:\n%s", err, runOutput.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("flowctl run did not exit after stop request\nOutput:\n%s", runOutput.String())
	}

	activeAfterStop, err := runCommandE(repoRoot, 5*time.Second, flowctlBin,
		"pipelines",
		"active",
		"--control-plane-address", "127.0.0.1",
		"--control-plane-port", fmt.Sprintf("%d", controlPlanePort),
	)
	if err == nil {
		assertContains(t, activeAfterStop, "No active pipeline runs")
	} else if !strings.Contains(activeAfterStop, "no control plane is reachable") {
		t.Fatalf("unexpected pipelines active result after stop\nError: %v\nOutput:\n%s", err, activeAfterStop)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("failed to locate repository root")
		}
		wd = parent
	}
}

func buildGoBinary(t *testing.T, repoRoot, output, target string) {
	t.Helper()

	cmd := exec.Command("go", "build", "-o", output, target)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build %s: %v\nOutput:\n%s", target, err, string(out))
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func runCommand(t *testing.T, repoRoot string, timeout time.Duration, bin string, args ...string) string {
	t.Helper()
	out, err := runCommandE(repoRoot, timeout, bin, args...)
	if err != nil {
		t.Fatalf("command failed: %s %s\nError: %v\nOutput:\n%s", bin, strings.Join(args, " "), err, out)
	}
	return out
}

func runCommandE(repoRoot string, timeout time.Duration, bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		return out.String(), err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return out.String(), err
	case <-time.After(timeout):
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-done:
			return out.String(), fmt.Errorf("command timed out after %s: %w", timeout, err)
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			return out.String(), fmt.Errorf("command timed out after %s", timeout)
		}
	}
}

func waitForCommand(t *testing.T, timeout time.Duration, fn func() (string, error)) string {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastOut string
	var lastErr error

	for time.Now().Before(deadline) {
		out, err := fn()
		lastOut = out
		lastErr = err
		if err == nil {
			return out
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for command success after %s: %v\nLast output:\n%s", timeout, lastErr, lastOut)
	return ""
}

func parseFirstRunID(t *testing.T, output string) string {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "RUN ID") || strings.HasSuffix(line, "active pipeline run(s)") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[0]
		}
	}

	t.Fatalf("failed to parse run id from output:\n%s", output)
	return ""
}

func assertContains(t *testing.T, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("expected output to contain %q\nOutput:\n%s", want, output)
	}
}

const testComponentProgram = `package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	endpoint := os.Getenv("FLOWCTL_ENDPOINT")
	componentID := os.Getenv("FLOWCTL_COMPONENT_ID")
	role := os.Getenv("TEST_COMPONENT_ROLE")
	if endpoint == "" || componentID == "" || role == "" {
		log.Fatalf("missing FLOWCTL_ENDPOINT, FLOWCTL_COMPONENT_ID, or TEST_COMPONENT_ROLE")
	}

	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to dial control plane: %v", err)
	}
	defer conn.Close()

	client := flowctlv1.NewControlPlaneServiceClient(conn)

	componentType := flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE
	inputTypes := []string{}
	outputTypes := []string{"test.event"}
	if role == "sink" {
		componentType = flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER
		inputTypes = []string{"test.event"}
		outputTypes = nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.RegisterComponent(ctx, &flowctlv1.RegisterRequest{
		ComponentId: componentID,
		Component: &flowctlv1.ComponentInfo{
			Id:               componentID,
			Name:             componentID,
			Type:             componentType,
			Endpoint:         "",
			InputEventTypes:  inputTypes,
			OutputEventTypes: outputTypes,
			Metadata: map[string]string{
				"test_role": role,
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to register component: %v", err)
	}

	sendHeartbeat := func() {
		hbCtx, hbCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer hbCancel()
		_, err := client.Heartbeat(hbCtx, &flowctlv1.HeartbeatRequest{
			ServiceId: resp.ServiceId,
			Status:    flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY,
			Metrics: map[string]string{
				"role": role,
			},
		})
		if err != nil {
			log.Printf("failed to send heartbeat: %v", err)
		}
	}

	sendHeartbeat()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			sendHeartbeat()
		case <-sigCh:
			return
		}
	}
}
`

// runCommandJSON runs a command capturing stdout and stderr separately, and
// returns the parsed stdout document.
//
// Keeping the streams apart is the point: it asserts CLI-5 as a side effect.
// If any progress line leaked onto stdout, the parse here would fail.
func runCommandJSON(t *testing.T, repoRoot string, timeout time.Duration, bin string, args ...string) map[string]any {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start %s: %v", bin, err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("command failed: %s %s\nError: %v\nStderr:\n%s", bin, strings.Join(args, " "), err, stderr.String())
		}
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		t.Fatalf("command timed out: %s %s", bin, strings.Join(args, " "))
	}

	var doc map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\nStdout:\n%s\nStderr:\n%s",
			err, stdout.String(), stderr.String())
	}
	return doc
}

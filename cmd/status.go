package cmd

import (
	"context"
	"fmt"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
	"github.com/withobsrvr/flowctl/internal/sanitize"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/runner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	statusControlPlaneAddress string
	statusControlPlanePort    int
	statusControlPlaneAddr    string
	statusIncludeUnhealthy    bool
	statusWatch               bool
	statusWatchInterval       int
)

var statusCmd = &cobra.Command{
	Use:   "status [pipeline-file]",
	Short: "Show pipeline component status",
	Long: `Query the control plane to show real-time status of pipeline components.

This command connects to a running control plane and displays the current
state of all registered components including:
- Component health status
- Last heartbeat time
- Metrics (if available)
- Event types

Examples:
  # Show status for currently running pipeline
  flowctl status

  # Show status from specific control plane
  flowctl status --control-plane-address localhost --control-plane-port 9090

  # Legacy combined endpoint form (still supported)
  flowctl status --control-plane-addr localhost:9090

  # Include unhealthy components
  flowctl status --include-unhealthy

  # Watch status (refresh every 5 seconds)
  flowctl status --watch --interval 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Optional: Load pipeline file to get metadata
		var pipelineName string
		if len(args) > 0 {
			pipeline, err := runner.LoadPipelineFromFile(args[0])
			if err == nil {
				pipelineName = pipeline.Metadata.Name
			}
		}

		endpoint := resolveStatusControlPlaneEndpoint()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(
			ctx,
			endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return formatControlPlaneError(err, endpoint)
		}
		defer conn.Close()

		client := flowctlv1.NewControlPlaneServiceClient(conn)

		p := cliout.FromContext(cmd.Context())

		// Watch mode redraws a terminal; it has no meaning for a program
		// consuming one JSON document (CLI-6).
		if statusWatch && p.Machine() {
			return exitcode.Errorf(exitcode.Usage,
				"--watch cannot be combined with --output %s; poll the command instead", p.Format())
		}

		// If watch mode, loop
		if statusWatch {
			for {
				rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := displayStatus(rpcCtx, p, client, pipelineName)
				rpcCancel()
				if err != nil {
					return formatControlPlaneError(err, endpoint)
				}
				time.Sleep(time.Duration(statusWatchInterval) * time.Second)
				if colorEnabled() {
					fmt.Print("\033[H\033[2J") // Clear screen
				}
			}
		}

		// Single status display
		rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer rpcCancel()
		if err := displayStatus(rpcCtx, p, client, pipelineName); err != nil {
			return formatControlPlaneError(err, endpoint)
		}
		return nil
	},
}

func displayStatus(ctx context.Context, p *cliout.Printer, client flowctlv1.ControlPlaneServiceClient, pipelineName string) error {
	// Query all components
	resp, err := client.ListComponents(ctx, &flowctlv1.ListComponentsRequest{
		IncludeUnhealthy: statusIncludeUnhealthy,
	})
	if err != nil {
		return fmt.Errorf("failed to list components: %w", err)
	}

	// Both renderings need the health counts; only the human one groups by type.
	counts := map[string]int{}
	for _, comp := range resp.Components {
		counts[statusHealthName(comp.GetStatus())]++
	}

	// The worst component health decides the pipeline's overall status.
	overall := "running"
	switch {
	case counts["unhealthy"] > 0:
		overall = "unhealthy"
	case counts["degraded"] > 0:
		overall = "degraded"
	case counts["starting"] > 0:
		overall = "starting"
	}

	if p.Machine() {
		// One document carrying the summary and every component, so a caller
		// does not have to make a second call to get the detail.
		components := make([]map[string]any, 0, len(resp.Components))
		for _, comp := range resp.Components {
			components = append(components, statusComponentRecord(comp))
		}
		data := cliout.Row{
			"status":          overall,
			"component_count": len(resp.Components),
			"healthy":         counts["healthy"],
			"degraded":        counts["degraded"],
			"unhealthy":       counts["unhealthy"],
			"starting":        counts["starting"],
			"components":      components,
		}
		if pipelineName != "" {
			data["pipeline"] = pipelineName
		}
		return p.Object(cliout.Object{Schema: "flowctl.pipeline_status.v1", Data: data})
	}

	if len(resp.Components) == 0 {
		p.Info("No components registered with control plane")
		p.Info("Start a pipeline with: flowctl run pipeline.yaml")
		return nil
	}

	// Human rendering below. This grouped layout is not a contract and may
	// change freely; the record above is the contract. Untrusted strings are
	// stripped here because they do not pass through a cliout cell formatter.
	resp = sanitize.Message(resp)

	var sources, processors, sinks []*flowctlv1.ComponentStatusResponse
	for _, comp := range resp.Components {
		switch comp.GetComponent().GetType() {
		case flowctlv1.ComponentType_COMPONENT_TYPE_SOURCE:
			sources = append(sources, comp)
		case flowctlv1.ComponentType_COMPONENT_TYPE_PROCESSOR:
			processors = append(processors, comp)
		case flowctlv1.ComponentType_COMPONENT_TYPE_CONSUMER:
			sinks = append(sinks, comp)
		}
	}

	out := p.Out()
	if pipelineName != "" {
		fmt.Fprintf(out, "Pipeline: %s\n", sanitize.Line(pipelineName))
	} else {
		fmt.Fprintln(out, "Pipeline Status")
	}

	fmt.Fprintf(out, "Status: %s\n", colorizeStatus(strings.ToUpper(overall)))
	fmt.Fprintf(out, "Components: %d total (%d healthy", len(resp.Components), counts["healthy"])
	for _, kind := range []string{"starting", "degraded", "unhealthy"} {
		if counts[kind] > 0 {
			fmt.Fprintf(out, ", %d %s", counts[kind], kind)
		}
	}
	fmt.Fprintln(out, ")")
	fmt.Fprintln(out)

	for _, group := range []struct {
		title string
		comps []*flowctlv1.ComponentStatusResponse
	}{
		{"SOURCES", sources},
		{"PROCESSORS", processors},
		{"CONSUMERS", sinks},
	} {
		if len(group.comps) == 0 {
			continue
		}
		fmt.Fprintf(out, "%s (%d)\n", group.title, len(group.comps))
		for _, comp := range group.comps {
			printComponentStatus(out, comp)
		}
		fmt.Fprintln(out)
	}

	return nil
}

// statusHealthName renders a health status as a stable lowercase string. These
// values appear in --output json and are part of the schema.
func statusHealthName(status flowctlv1.HealthStatus) string {
	name := strings.TrimPrefix(status.String(), "HEALTH_STATUS_")
	if name == "" || name == "UNSPECIFIED" {
		return "unknown"
	}
	return strings.ToLower(name)
}

// statusComponentRecord is one component inside the status document.
func statusComponentRecord(comp *flowctlv1.ComponentStatusResponse) map[string]any {
	info := comp.GetComponent()
	rec := map[string]any{
		"id":     info.GetId(),
		"type":   strings.ToLower(strings.TrimPrefix(info.GetType().String(), "COMPONENT_TYPE_")),
		"status": statusHealthName(comp.GetStatus()),
	}
	if v := info.GetEndpoint(); v != "" {
		rec["endpoint"] = v
	}
	if len(info.GetInputEventTypes()) > 0 {
		rec["input_event_types"] = info.GetInputEventTypes()
	}
	if len(info.GetOutputEventTypes()) > 0 {
		rec["output_event_types"] = info.GetOutputEventTypes()
	}
	if ts := comp.GetRegisteredAt(); ts != nil {
		rec["registered_at"] = ts.AsTime()
	}
	if ts := comp.GetLastHeartbeat(); ts != nil {
		rec["last_heartbeat"] = ts.AsTime()
	}
	if len(comp.GetMetrics()) > 0 {
		rec["metrics"] = comp.GetMetrics()
	}
	return rec
}

func printComponentStatus(out io.Writer, comp *flowctlv1.ComponentStatusResponse) {
	statusStr := colorizeHealth(comp.Status.String())

	// Calculate uptime
	var uptime string
	if comp.RegisteredAt != nil {
		duration := time.Since(comp.RegisteredAt.AsTime())
		uptime = formatDuration(duration)
	}

	// Get last heartbeat
	var lastHeartbeat string
	if comp.LastHeartbeat != nil {
		duration := time.Since(comp.LastHeartbeat.AsTime())
		if duration < 5*time.Second {
			lastHeartbeat = "just now"
		} else {
			lastHeartbeat = fmt.Sprintf("%s ago", formatDuration(duration))
		}
	} else {
		lastHeartbeat = "never"
	}

	fmt.Fprintf(out, "  %s (%s)\n", comp.Component.Id, statusStr)
	fmt.Fprintf(out, "    Endpoint: %s\n", comp.Component.Endpoint)
	if uptime != "" {
		fmt.Fprintf(out, "    Uptime: %s\n", uptime)
	}
	fmt.Fprintf(out, "    Last heartbeat: %s\n", lastHeartbeat)

	// Print event types
	if len(comp.Component.InputEventTypes) > 0 {
		fmt.Fprintf(out, "    Input types: %s\n", strings.Join(comp.Component.InputEventTypes, ", "))
	}
	if len(comp.Component.OutputEventTypes) > 0 {
		fmt.Fprintf(out, "    Output types: %s\n", strings.Join(comp.Component.OutputEventTypes, ", "))
	}

	// Print metrics if available
	if len(comp.Metrics) > 0 {
		fmt.Fprintln(out, "    Metrics:")

		// Sort metric keys for consistent output
		keys := make([]string, 0, len(comp.Metrics))
		for k := range comp.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := comp.Metrics[key]
			fmt.Fprintf(out, "      %s: %s\n", key, value)
		}
	}
}

// colorEnabled reports whether ANSI may be written to stdout. CLI-7 requires
// colour off when stdout is not a terminal, when NO_COLOR is set, or when
// TERM=dumb — otherwise `flowctl status > file` embeds escape codes in the file
// and `flowctl status | grep RUNNING` fails to match.
//
// Resolved once: stdout cannot change underneath a single invocation, and
// re-checking per cell would cost a syscall per field in watch mode.
var colorEnabled = sync.OnceValue(func() bool {
	return cliout.ShouldColor(os.Stdout)
})

func colorize(code, text string) string {
	if !colorEnabled() {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

func colorizeStatus(status string) string {
	switch status {
	case "RUNNING":
		return colorize("32", status) // Green
	case "STARTING":
		return colorize("36", status) // Cyan
	case "DEGRADED":
		return colorize("33", status) // Yellow
	case "UNHEALTHY":
		return colorize("31", status) // Red
	default:
		return status
	}
}

func colorizeHealth(health string) string {
	if strings.Contains(health, "HEALTHY") && !strings.Contains(health, "UNHEALTHY") {
		return colorize("32", health) // Green
	} else if strings.Contains(health, "DEGRADED") {
		return colorize("33", health) // Yellow
	} else if strings.Contains(health, "UNHEALTHY") {
		return colorize("31", health) // Red
	} else if strings.Contains(health, "STARTING") {
		return colorize("36", health) // Cyan
	}
	return health
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func resolveStatusControlPlaneEndpoint() string {
	if statusControlPlaneAddr != "" {
		return statusControlPlaneAddr
	}
	return fmt.Sprintf("%s:%d", statusControlPlaneAddress, statusControlPlanePort)
}

func formatControlPlaneError(err error, endpoint string) error {
	msg := err.Error()
	if strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "failed to connect") || strings.Contains(msg, "Error while dialing") {
		return fmt.Errorf("no control plane is reachable at %s\nstart a pipeline with: flowctl run <pipeline.yaml>\nor specify a different endpoint with --control-plane-address/--control-plane-port", endpoint)
	}
	return err
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVar(&statusControlPlaneAddress, "control-plane-address", "127.0.0.1", "control plane address")
	statusCmd.Flags().IntVar(&statusControlPlanePort, "control-plane-port", 8080, "control plane port")
	statusCmd.Flags().StringVar(&statusControlPlaneAddr, "control-plane-addr", "", "legacy combined control plane endpoint host:port")
	statusCmd.Flags().BoolVar(&statusIncludeUnhealthy, "include-unhealthy", true, "include unhealthy components")
	statusCmd.Flags().BoolVar(&statusWatch, "watch", false, "watch mode (refresh periodically)")
	statusCmd.Flags().IntVar(&statusWatchInterval, "interval", 5, "watch interval in seconds")
}

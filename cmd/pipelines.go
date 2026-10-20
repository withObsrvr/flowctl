package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
	"github.com/withobsrvr/flowctl/internal/sanitize"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Schema identifiers for this command group's machine-readable output.
const (
	pipelineListSchema    = "flowctl.pipeline_list.v1"
	pipelineSchema        = "flowctl.pipeline.v1"
	pipelineRunListSchema = "flowctl.pipeline_run_list.v1"
	pipelineRunSchema     = "flowctl.pipeline_run.v1"
)

// runRow converts a pipeline run into a record. Machine output carries full
// ids, raw counts and real timestamps; the table shortens and humanises them.
func runRow(run *flowctlpb.PipelineRun) cliout.Row {
	row := cliout.Row{
		"run_id":        run.GetRunId(),
		"pipeline_name": run.GetPipelineName(),
		"status":        formatRunStatus(run.GetStatus()),
	}
	if ts := run.GetStartTime(); ts != nil {
		row["started_at"] = ts.AsTime()
		row["_started"] = formatRunTime(ts.AsTime())
	}
	if ts := run.GetEndTime(); ts != nil {
		row["ended_at"] = ts.AsTime()
	}
	row["_duration"] = formatRunDuration(run)
	if m := run.GetMetrics(); m != nil {
		row["events_processed"] = m.GetEventsProcessed()
		row["_events"] = formatEventCount(m.GetEventsProcessed())
		if m.GetEventsPerSecond() > 0 {
			row["events_per_second"] = m.GetEventsPerSecond()
		}
	}
	if ids := run.GetComponentIds(); len(ids) > 0 {
		row["component_ids"] = ids
	}
	if v := run.GetError(); v != "" {
		row["error"] = v
	}
	row["_short_id"] = shortID(run.GetRunId())
	return row
}

// shortID is the abbreviated run id shown in tables. It slices by rune count
// and tolerates a short id: the control plane is a trust boundary, and
// run.RunId[:8] panics on anything shorter than eight bytes.
func shortID(id string) string {
	r := []rune(id)
	if len(r) <= 8 {
		return id
	}
	return string(r[:8])
}

// runListColumns is the table layout for a list of runs.
func runListColumns(withPipeline bool) []cliout.Column {
	cols := []cliout.Column{{Header: "Run ID", Key: "_short_id"}}
	if withPipeline {
		cols = append(cols, cliout.Column{Header: "Pipeline", Key: "pipeline_name"})
	}
	cols = append(cols,
		cliout.Column{Header: "Status", Key: "status"},
		cliout.Column{Header: "Started", Key: "_started"},
		cliout.Column{Header: "Duration", Key: "_duration"},
		cliout.Column{Header: "Events", Key: "_events"},
	)
	return cols
}

var (
	pipelinesControlPlaneAddr string
	pipelinesControlPlanePort int
	pipelinesLimit            int32
)

// pipelinesCmd represents the pipelines command group
var pipelinesCmd = &cobra.Command{
	Use:   "pipelines",
	Short: "Manage pipeline runs",
	Long: `View and manage pipeline runs tracked by the control plane.

Examples:
  # List all known pipelines
  flowctl pipelines list

  # View run history for a pipeline
  flowctl pipelines runs my-pipeline

  # Get details about a specific run
  flowctl pipelines run-info abc123

  # Stop a running pipeline
  flowctl pipelines stop abc123

  # List all currently running pipelines
  flowctl pipelines active`,
}

// pipelinesListCmd lists all known pipelines
var pipelinesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all known pipelines",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		resp, err := client.ListPipelineRuns(cmd.Context(), &flowctlpb.ListPipelineRunsRequest{
			Limit: 1000, // all runs, so they can be grouped by pipeline
		})
		if err != nil {
			return err
		}

		// Group by pipeline name, keeping the most recent run of each.
		latest := make(map[string]*flowctlpb.PipelineRun)
		counts := make(map[string]int)
		for _, run := range resp.Runs {
			name := run.GetPipelineName()
			counts[name]++
			prev, ok := latest[name]
			if !ok || run.GetStartTime().AsTime().After(prev.GetStartTime().AsTime()) {
				latest[name] = run
			}
		}

		// Sorted so the output is stable between runs; map iteration is not.
		names := make([]string, 0, len(latest))
		for name := range latest {
			names = append(names, name)
		}
		sort.Strings(names)

		rows := make([]cliout.Row, 0, len(names))
		for _, name := range names {
			last := latest[name]
			row := cliout.Row{
				"pipeline_name": name,
				"run_count":     counts[name],
				"last_run_id":   last.GetRunId(),
				"last_status":   formatRunStatus(last.GetStatus()),
			}
			if ts := last.GetStartTime(); ts != nil {
				row["last_run_at"] = ts.AsTime()
				row["_last_run"] = formatRunTime(ts.AsTime())
			}
			rows = append(rows, row)
		}

		p := cliout.FromContext(cmd.Context())
		if err := p.List(cliout.List{
			Schema:     pipelineListSchema,
			ItemSchema: pipelineSchema,
			Columns: []cliout.Column{
				{Header: "Name", Key: "pipeline_name"},
				{Header: "Last Run", Key: "_last_run"},
				{Header: "Status", Key: "last_status"},
				{Header: "Runs", Key: "run_count"},
			},
			Rows:  rows,
			Empty: "No pipelines found.",
		}); err != nil {
			return err
		}

		if !p.Machine() && len(rows) > 0 {
			p.Info("\n%d pipeline(s) found", len(rows))
		}
		return nil
	},
}

// pipelinesRunsCmd shows run history for a pipeline
var pipelinesRunsCmd = &cobra.Command{
	Use:   "runs <pipeline-name>",
	Short: "Show run history for a pipeline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pipelineName := args[0]

		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		resp, err := client.ListPipelineRuns(cmd.Context(), &flowctlpb.ListPipelineRunsRequest{
			PipelineName: pipelineName,
			Limit:        pipelinesLimit,
		})
		if err != nil {
			return err
		}

		rows := make([]cliout.Row, 0, len(resp.Runs))
		for _, run := range resp.Runs {
			rows = append(rows, runRow(run))
		}

		p := cliout.FromContext(cmd.Context())
		if err := p.List(cliout.List{
			Schema:     pipelineRunListSchema,
			ItemSchema: pipelineRunSchema,
			Columns:    runListColumns(false),
			Rows:       rows,
			Empty:      fmt.Sprintf("No runs found for pipeline %s.", sanitize.Line(pipelineName)),
		}); err != nil {
			return err
		}

		if !p.Machine() && len(rows) > 0 {
			p.Info("\nShowing last %d run(s). Use --limit to see more.", len(rows))
		}
		return nil
	},
}

// pipelinesRunInfoCmd shows detailed info for a run
var pipelinesRunInfoCmd = &cobra.Command{
	Use:   "run-info <run-id>",
	Short: "Show detailed information about a pipeline run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		runID, err := resolveRunID(ctx, client, args[0])
		if err != nil {
			return err
		}

		run, err := client.GetPipelineRun(ctx, &flowctlpb.GetPipelineRunRequest{
			RunId: runID,
		})
		if err != nil {
			return exitcode.Errorf(exitcode.NotFound, "pipeline run %q not found: %v", runID, err)
		}

		row := runRow(run)
		if m := run.GetMetrics(); m != nil && m.GetEventsPerSecond() > 0 {
			row["_throughput"] = fmt.Sprintf("%.1f events/sec", m.GetEventsPerSecond())
		}

		return cliout.FromContext(cmd.Context()).Object(cliout.Object{
			Schema: pipelineRunSchema,
			Fields: []cliout.Field{
				{Label: "Run ID", Key: "run_id"},
				{Label: "Pipeline", Key: "pipeline_name"},
				{Label: "Status", Key: "status"},
				{Label: "Started", Key: "started_at"},
				{Label: "Duration", Key: "_duration"},
				{Label: "Events", Key: "_events"},
				{Label: "Throughput", Key: "_throughput"},
				{Label: "Components", Key: "component_ids", Format: joinList},
				{Label: "Error", Key: "error"},
			},
			Data: row,
		})
	},
}

// joinList renders a string slice for a detail view. The record keeps the slice.
func joinList(v any) string {
	items, ok := v.([]string)
	if !ok || len(items) == 0 {
		return "-"
	}
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = cliout.Value(s)
	}
	return strings.Join(out, ", ")
}

// pipelinesStopCmd stops a running pipeline
var pipelinesStopCmd = &cobra.Command{
	Use:   "stop <run-id>",
	Short: "Stop a running pipeline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		runID, err := resolveRunID(ctx, client, args[0])
		if err != nil {
			return err
		}

		p := cliout.FromContext(cmd.Context())
		p.Info("Stopping pipeline run %s...", runID)

		run, err := client.StopPipelineRun(ctx, &flowctlpb.StopPipelineRunRequest{
			RunId: runID,
		})
		if err != nil {
			return err
		}

		return p.Object(cliout.Object{
			Schema: pipelineRunSchema,
			Fields: []cliout.Field{
				{Label: "Run ID", Key: "run_id"},
				{Label: "Pipeline", Key: "pipeline_name"},
				{Label: "Status", Key: "status"},
			},
			Data: runRow(run),
		})
	},
}

// pipelinesActiveCmd lists currently running pipelines
var pipelinesActiveCmd = &cobra.Command{
	Use:   "active",
	Short: "List currently running pipelines",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		resp, err := client.ListPipelineRuns(cmd.Context(), &flowctlpb.ListPipelineRunsRequest{
			Status: flowctlpb.RunStatus_RUN_STATUS_RUNNING,
			Limit:  100,
		})
		if err != nil {
			return err
		}

		rows := make([]cliout.Row, 0, len(resp.Runs))
		for _, run := range resp.Runs {
			rows = append(rows, runRow(run))
		}

		return cliout.FromContext(cmd.Context()).List(cliout.List{
			Schema:     pipelineRunListSchema,
			ItemSchema: pipelineRunSchema,
			Columns:    runListColumns(true),
			Rows:       rows,
			Empty:      "No active pipeline runs.",
		})
	},
}

// Helper functions

func connectToControlPlane() (flowctlpb.ControlPlaneClient, *grpc.ClientConn, error) {
	endpoint := fmt.Sprintf("%s:%d", pipelinesControlPlaneAddr, pipelinesControlPlanePort)

	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, formatPipelinesControlPlaneError(endpoint, err)
	}

	client := flowctlpb.NewControlPlaneClient(conn)
	return client, conn, nil
}

func formatRunTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}

func formatRunDuration(run *flowctlpb.PipelineRun) string {
	start := run.GetStartTime().AsTime()
	var end time.Time

	if run.EndTime != nil {
		end = run.EndTime.AsTime()
	} else {
		end = time.Now()
	}

	duration := end.Sub(start)

	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	case duration < time.Hour:
		return fmt.Sprintf("%dm %ds", int(duration.Minutes()), int(duration.Seconds())%60)
	default:
		return fmt.Sprintf("%dh %dm", int(duration.Hours()), int(duration.Minutes())%60)
	}
}

func formatRunStatus(status flowctlpb.RunStatus) string {
	switch status {
	case flowctlpb.RunStatus_RUN_STATUS_STARTING:
		return "starting"
	case flowctlpb.RunStatus_RUN_STATUS_RUNNING:
		return "running"
	case flowctlpb.RunStatus_RUN_STATUS_COMPLETED:
		return "completed"
	case flowctlpb.RunStatus_RUN_STATUS_FAILED:
		return "failed"
	case flowctlpb.RunStatus_RUN_STATUS_STOPPED:
		return "stopped"
	default:
		return "unknown"
	}
}

func formatEventCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	} else {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
}

func resolveRunID(ctx context.Context, client flowctlpb.ControlPlaneClient, runID string) (string, error) {
	if runID == "" {
		return "", fmt.Errorf("run id is required")
	}

	// Fast path: exact lookup succeeds.
	run, err := client.GetPipelineRun(ctx, &flowctlpb.GetPipelineRunRequest{RunId: runID})
	if err == nil {
		// The id comes back over the wire and is echoed into later output, so
		// it is sanitised like any other control-plane string (CLI-22).
		return sanitize.Line(run.RunId), nil
	}

	// Fallback: allow a unique prefix so operators can use the short IDs shown in tables.
	resp, listErr := client.ListPipelineRuns(ctx, &flowctlpb.ListPipelineRunsRequest{Limit: -1})
	if listErr != nil {
		return "", fmt.Errorf("failed to resolve run id %q: %w", runID, listErr)
	}

	// Everything below is written by components flowctl did not start and is
	// printed to a terminal, so strip control characters first (CLI-22).
	// Remove this when the command is converted to internal/cliout, which
	// sanitises at the point it formats a cell.
	resp = sanitize.Message(resp)

	matches := make([]string, 0)
	for _, candidate := range resp.Runs {
		if strings.HasPrefix(candidate.RunId, runID) {
			matches = append(matches, candidate.RunId)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("pipeline run not found: %s", runID)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("run id prefix %q is ambiguous (%d matches); provide more characters", runID, len(matches))
	}
}

func formatPipelinesControlPlaneError(endpoint string, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "Error while dialing") {
		return fmt.Errorf("no control plane is reachable at %s\nstart a pipeline with: flowctl run <pipeline.yaml>\nor specify a different endpoint with --control-plane-address/--control-plane-port", endpoint)
	}
	return fmt.Errorf("failed to connect to control plane at %s: %w", endpoint, err)
}

func init() {
	rootCmd.AddCommand(pipelinesCmd)

	// Add subcommands
	pipelinesCmd.AddCommand(pipelinesListCmd)
	pipelinesCmd.AddCommand(pipelinesRunsCmd)
	pipelinesCmd.AddCommand(pipelinesRunInfoCmd)
	pipelinesCmd.AddCommand(pipelinesStopCmd)
	pipelinesCmd.AddCommand(pipelinesActiveCmd)

	// Add flags
	pipelinesCmd.PersistentFlags().StringVar(&pipelinesControlPlaneAddr, "control-plane-address", "127.0.0.1", "Control plane address")
	pipelinesCmd.PersistentFlags().IntVar(&pipelinesControlPlanePort, "control-plane-port", 8080, "Control plane port")
	pipelinesRunsCmd.Flags().Int32Var(&pipelinesLimit, "limit", 10, "Maximum number of runs to show")
}

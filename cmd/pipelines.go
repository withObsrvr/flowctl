package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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

		// Get all runs
		resp, err := client.ListPipelineRuns(context.Background(), &flowctlpb.ListPipelineRunsRequest{
			Limit: 1000, // Get all runs to group by pipeline
		})
		if err != nil {
			return fmt.Errorf("failed to list pipeline runs: %w", err)
		}

		// Group by pipeline name
		pipelineMap := make(map[string][]*flowctlpb.PipelineRun)
		for _, run := range resp.Runs {
			pipelineMap[run.PipelineName] = append(pipelineMap[run.PipelineName], run)
		}

		if len(pipelineMap) == 0 {
			fmt.Println("No pipelines found")
			return nil
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tLAST RUN\tSTATUS\tRUNS")
		for name, runs := range pipelineMap {
			// Find most recent run
			var lastRun *flowctlpb.PipelineRun
			for _, run := range runs {
				if lastRun == nil || run.StartTime.AsTime().After(lastRun.StartTime.AsTime()) {
					lastRun = run
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
				name,
				formatRunTime(lastRun.StartTime.AsTime()),
				formatRunStatus(lastRun.Status),
				len(runs))
		}
		w.Flush()

		fmt.Printf("\n%d pipeline(s) found\n", len(pipelineMap))
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

		resp, err := client.ListPipelineRuns(context.Background(), &flowctlpb.ListPipelineRunsRequest{
			PipelineName: pipelineName,
			Limit:        pipelinesLimit,
		})
		if err != nil {
			return fmt.Errorf("failed to list pipeline runs: %w", err)
		}

		if len(resp.Runs) == 0 {
			fmt.Printf("No runs found for pipeline: %s\n", pipelineName)
			return nil
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "RUN ID\tSTATUS\tSTARTED\tDURATION\tEVENTS")
		for _, run := range resp.Runs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				run.RunId[:8],
				formatRunStatus(run.Status),
				formatRunTime(run.StartTime.AsTime()),
				formatRunDuration(run),
				formatEventCount(run.Metrics.EventsProcessed))
		}
		w.Flush()

		fmt.Printf("\nShowing last %d run(s). Use --limit to see more.\n", len(resp.Runs))
		return nil
	},
}

// pipelinesRunInfoCmd shows detailed info for a run
var pipelinesRunInfoCmd = &cobra.Command{
	Use:   "run-info <run-id>",
	Short: "Show detailed information about a pipeline run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runID := args[0]

		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		run, err := client.GetPipelineRun(context.Background(), &flowctlpb.GetPipelineRunRequest{
			RunId: runID,
		})
		if err != nil {
			return fmt.Errorf("failed to get pipeline run: %w", err)
		}

		// Print run details
		fmt.Printf("Run ID:        %s\n", run.RunId)
		fmt.Printf("Pipeline:      %s\n", run.PipelineName)
		fmt.Printf("Status:        %s\n", formatRunStatus(run.Status))
		fmt.Printf("Started:       %s\n", run.StartTime.AsTime().Format("2006-01-02 15:04:05 MST"))
		fmt.Printf("Duration:      %s\n", formatRunDuration(run))
		fmt.Printf("Events:        %s\n", formatEventCount(run.Metrics.EventsProcessed))

		if run.Metrics.EventsPerSecond > 0 {
			fmt.Printf("Throughput:    %.1f events/sec\n", run.Metrics.EventsPerSecond)
		}

		if len(run.ComponentIds) > 0 {
			fmt.Printf("\nComponents:\n")
			for _, compID := range run.ComponentIds {
				fmt.Printf("  - %s\n", compID)
			}
		}

		if run.Error != "" {
			fmt.Printf("\nError: %s\n", run.Error)
		}

		return nil
	},
}

// pipelinesStopCmd stops a running pipeline
var pipelinesStopCmd = &cobra.Command{
	Use:   "stop <run-id>",
	Short: "Stop a running pipeline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runID := args[0]

		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		fmt.Printf("Stopping pipeline run %s...\n", runID)

		run, err := client.StopPipelineRun(context.Background(), &flowctlpb.StopPipelineRunRequest{
			RunId: runID,
		})
		if err != nil {
			return fmt.Errorf("failed to stop pipeline run: %w", err)
		}

		fmt.Printf("✓ Pipeline stopped (status: %s)\n", formatRunStatus(run.Status))
		return nil
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

		resp, err := client.ListPipelineRuns(context.Background(), &flowctlpb.ListPipelineRunsRequest{
			Status: flowctlpb.RunStatus_RUN_STATUS_RUNNING,
			Limit:  100,
		})
		if err != nil {
			return fmt.Errorf("failed to list active pipeline runs: %w", err)
		}

		if len(resp.Runs) == 0 {
			fmt.Println("No active pipeline runs")
			return nil
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "RUN ID\tPIPELINE\tSTARTED\tDURATION\tEVENTS\tSTATUS")
		for _, run := range resp.Runs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				run.RunId[:8],
				run.PipelineName,
				formatRunTime(run.StartTime.AsTime()),
				formatRunDuration(run),
				formatEventCount(run.Metrics.EventsProcessed),
				formatRunStatus(run.Status))
		}
		w.Flush()

		fmt.Printf("\n%d active pipeline run(s)\n", len(resp.Runs))
		return nil
	},
}

// Helper functions

func connectToControlPlane() (flowctlpb.ControlPlaneClient, *grpc.ClientConn, error) {
	endpoint := fmt.Sprintf("%s:%d", pipelinesControlPlaneAddr, pipelinesControlPlanePort)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to control plane at %s: %w", endpoint, err)
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
	start := run.StartTime.AsTime()
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

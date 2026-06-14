package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
)

var (
	chunksRunID       string
	chunksComponentID string
	chunksStatus      string
	chunksLimit       int32
)

var chunksCmd = &cobra.Command{
	Use:   "chunks",
	Short: "Inspect historical range chunk runs",
	Long: `Inspect historical range chunk runs tracked by the control plane.

Chunks are the durable control-plane records for bounded ledger work units such
as Bronze backfill/repair ranges. Data-plane components report chunk status,
row counts, verification results, and typed failure classes.`,
}

var chunksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List chunk runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		status, err := parseChunkStatus(chunksStatus)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		runID := chunksRunID
		if runID != "" {
			runID, err = resolveRunID(ctx, client, runID)
			if err != nil {
				return err
			}
		}

		resp, err := client.ListChunkRuns(ctx, &flowctlpb.ListChunkRunsRequest{
			PipelineRunId: runID,
			ComponentId:   chunksComponentID,
			Status:        status,
			Limit:         chunksLimit,
		})
		if err != nil {
			return fmt.Errorf("failed to list chunk runs: %w", err)
		}

		if len(resp.Chunks) == 0 {
			fmt.Println("No chunk runs found")
			return nil
		}

		sort.Slice(resp.Chunks, func(i, j int) bool {
			if resp.Chunks[i].ChunkStart == resp.Chunks[j].ChunkStart {
				return resp.Chunks[i].Attempt < resp.Chunks[j].Attempt
			}
			return resp.Chunks[i].ChunkStart < resp.Chunks[j].ChunkStart
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CHUNK ID\tCOMPONENT\tRANGE\tATTEMPT\tSTATUS\tFAILURE\tPHASE\tROWS")
		for _, chunk := range resp.Chunks {
			fmt.Fprintf(w, "%s\t%s\t%d-%d\t%d\t%s\t%s\t%s\t%d\n",
				shortChunkID(chunk.ChunkId),
				chunk.ComponentId,
				chunk.ChunkStart,
				chunk.ChunkEnd,
				chunk.Attempt,
				formatChunkStatus(chunk.Status),
				formatFailureClass(chunk.FailureClass),
				chunk.Phase,
				totalRows(chunk.RowCounts),
			)
		}
		w.Flush()

		fmt.Printf("\n%d chunk run(s) found\n", len(resp.Chunks))
		return nil
	},
}

var chunksShowCmd = &cobra.Command{
	Use:   "show <chunk-id>",
	Short: "Show detailed information about a chunk run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := connectToControlPlane()
		if err != nil {
			return err
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		chunk, err := client.GetChunkRun(ctx, &flowctlpb.GetChunkRunRequest{ChunkId: args[0]})
		if err != nil {
			return fmt.Errorf("failed to get chunk run: %w", err)
		}

		fmt.Printf("Chunk ID:      %s\n", chunk.ChunkId)
		fmt.Printf("Run ID:        %s\n", chunk.PipelineRunId)
		fmt.Printf("Component:     %s\n", chunk.ComponentId)
		fmt.Printf("Range:         %d-%d\n", chunk.ChunkStart, chunk.ChunkEnd)
		fmt.Printf("Attempt:       %d\n", chunk.Attempt)
		fmt.Printf("Status:        %s\n", formatChunkStatus(chunk.Status))
		fmt.Printf("Phase:         %s\n", chunk.Phase)
		if chunk.FailureClass != flowctlpb.FailureClass_FAILURE_CLASS_UNKNOWN {
			fmt.Printf("Failure:       %s\n", formatFailureClass(chunk.FailureClass))
		}
		if chunk.Error != "" {
			fmt.Printf("Error:         %s\n", chunk.Error)
		}
		if chunk.RecommendedAction != "" {
			fmt.Printf("Next action:   %s\n", chunk.RecommendedAction)
		}
		if chunk.StartedAt != nil {
			fmt.Printf("Started:       %s\n", chunk.StartedAt.AsTime().Format("2006-01-02 15:04:05 MST"))
		}
		if chunk.CompletedAt != nil {
			fmt.Printf("Completed:     %s\n", chunk.CompletedAt.AsTime().Format("2006-01-02 15:04:05 MST"))
		}
		if chunk.VerifiedAt != nil {
			fmt.Printf("Verified:      %s\n", chunk.VerifiedAt.AsTime().Format("2006-01-02 15:04:05 MST"))
		}
		if len(chunk.RowCounts) > 0 {
			fmt.Println("\nRow counts:")
			keys := make([]string, 0, len(chunk.RowCounts))
			for k := range chunk.RowCounts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("  %s: %d\n", k, chunk.RowCounts[k])
			}
		}
		if len(chunk.Verification) > 0 {
			fmt.Println("\nVerification:")
			keys := make([]string, 0, len(chunk.Verification))
			for k := range chunk.Verification {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("  %s: %s\n", k, chunk.Verification[k])
			}
		}
		return nil
	},
}

func parseChunkStatus(value string) (flowctlpb.ChunkStatus, error) {
	if value == "" {
		return flowctlpb.ChunkStatus_CHUNK_STATUS_UNKNOWN, nil
	}
	key := "CHUNK_STATUS_" + strings.ToUpper(strings.ReplaceAll(value, "-", "_"))
	status, ok := flowctlpb.ChunkStatus_value[key]
	if !ok {
		return flowctlpb.ChunkStatus_CHUNK_STATUS_UNKNOWN, fmt.Errorf("unknown chunk status %q", value)
	}
	return flowctlpb.ChunkStatus(status), nil
}

func formatChunkStatus(status flowctlpb.ChunkStatus) string {
	return strings.ToLower(strings.TrimPrefix(status.String(), "CHUNK_STATUS_"))
}

func formatFailureClass(failure flowctlpb.FailureClass) string {
	if failure == flowctlpb.FailureClass_FAILURE_CLASS_UNKNOWN {
		return "-"
	}
	return strings.ToLower(strings.TrimPrefix(failure.String(), "FAILURE_CLASS_"))
}

func shortChunkID(chunkID string) string {
	if len(chunkID) <= 18 {
		return chunkID
	}
	return chunkID[:18]
}

func totalRows(rows map[string]int64) int64 {
	var total int64
	for _, count := range rows {
		total += count
	}
	return total
}

func init() {
	rootCmd.AddCommand(chunksCmd)
	chunksCmd.AddCommand(chunksListCmd)
	chunksCmd.AddCommand(chunksShowCmd)

	chunksCmd.PersistentFlags().StringVar(&pipelinesControlPlaneAddr, "control-plane-address", "127.0.0.1", "Control plane address")
	chunksCmd.PersistentFlags().IntVar(&pipelinesControlPlanePort, "control-plane-port", 8080, "Control plane port")
	chunksListCmd.Flags().StringVar(&chunksRunID, "run", "", "Pipeline run ID or unique prefix")
	chunksListCmd.Flags().StringVar(&chunksComponentID, "component", "", "Component ID filter")
	chunksListCmd.Flags().StringVar(&chunksStatus, "status", "", "Chunk status filter (planned, dispatched, running, completed, verified, failed, retry-pending, aborted)")
	chunksListCmd.Flags().Int32Var(&chunksLimit, "limit", 100, "Maximum number of chunks to show")
}

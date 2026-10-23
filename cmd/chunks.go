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
	flowctlpb "github.com/withobsrvr/flowctl/proto"
)

// Schema identifiers for this command group's machine-readable output.
const (
	chunkListSchema = "flowctl.chunk_run_list.v1"
	chunkSchema     = "flowctl.chunk_run.v1"
)

// chunkRow converts a chunk run into a record. Machine output carries the
// numbers and maps as themselves; the table condenses them.
func chunkRow(chunk *flowctlpb.ChunkRun) cliout.Row {
	row := cliout.Row{
		"chunk_id":        chunk.GetChunkId(),
		"pipeline_run_id": chunk.GetPipelineRunId(),
		"component_id":    chunk.GetComponentId(),
		"chunk_start":     chunk.GetChunkStart(),
		"chunk_end":       chunk.GetChunkEnd(),
		"attempt":         chunk.GetAttempt(),
		"status":          formatChunkStatus(chunk.GetStatus()),
		"phase":           chunk.GetPhase(),
		"total_rows":      totalRows(chunk.GetRowCounts()),
	}
	if chunk.GetFailureClass() != flowctlpb.FailureClass_FAILURE_CLASS_UNKNOWN {
		row["failure_class"] = formatFailureClass(chunk.GetFailureClass())
	}
	if v := chunk.GetError(); v != "" {
		row["error"] = v
	}
	if v := chunk.GetRecommendedAction(); v != "" {
		row["recommended_action"] = v
	}
	if ts := chunk.GetStartedAt(); ts != nil {
		row["started_at"] = ts.AsTime()
	}
	if ts := chunk.GetCompletedAt(); ts != nil {
		row["completed_at"] = ts.AsTime()
	}
	if ts := chunk.GetVerifiedAt(); ts != nil {
		row["verified_at"] = ts.AsTime()
	}
	if len(chunk.GetRowCounts()) > 0 {
		row["row_counts"] = chunk.GetRowCounts()
	}
	if len(chunk.GetVerification()) > 0 {
		row["verification"] = chunk.GetVerification()
	}
	return row
}

// chunkRange renders the ledger range for a table cell.
func chunkRange(row cliout.Row) string {
	return fmt.Sprintf("%v-%v", row["chunk_start"], row["chunk_end"])
}

// sortedCounts renders an int64 map for a detail view, keys sorted so output
// is stable between runs.
func sortedCounts(v any) string {
	m, ok := v.(map[string]int64)
	if !ok || len(m) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", cliout.Value(k), m[k]))
	}
	return strings.Join(parts, ", ")
}

// sortedStrings renders a string map for a detail view, keys sorted.
func sortedStrings(v any) string {
	m, ok := v.(map[string]string)
	if !ok || len(m) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, cliout.Value(k)+"="+cliout.Value(m[k]))
	}
	return strings.Join(parts, ", ")
}

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
			return err
		}

		// No sanitize.Message here: cliout strips control characters as it
		// formats each cell, and machine output keeps the original bytes.

		sort.Slice(resp.Chunks, func(i, j int) bool {
			if resp.Chunks[i].ChunkStart == resp.Chunks[j].ChunkStart {
				return resp.Chunks[i].Attempt < resp.Chunks[j].Attempt
			}
			return resp.Chunks[i].ChunkStart < resp.Chunks[j].ChunkStart
		})

		rows := make([]cliout.Row, 0, len(resp.Chunks))
		for _, chunk := range resp.Chunks {
			rows = append(rows, chunkRow(chunk))
		}

		p := cliout.FromContext(cmd.Context())
		if err := p.List(cliout.List{
			Schema:     chunkListSchema,
			ItemSchema: chunkSchema,
			Columns: []cliout.Column{
				{Header: "Chunk ID", Key: "chunk_id"},
				{Header: "Component", Key: "component_id"},
				{Header: "Range", Key: "_range"},
				{Header: "Attempt", Key: "attempt"},
				{Header: "Status", Key: "status"},
				{Header: "Failure", Key: "failure_class"},
				{Header: "Phase", Key: "phase"},
				{Header: "Rows", Key: "total_rows"},
			},
			Rows:  withRange(rows),
			Empty: "No chunk runs found.",
		}); err != nil {
			return err
		}

		if !p.Machine() && len(rows) > 0 {
			p.Info("\n%d chunk run(s) found", len(rows))
		}
		return nil
	},
}

// withRange adds the derived display-only range cell. It is not part of the
// record: machine consumers get chunk_start and chunk_end as numbers.
func withRange(rows []cliout.Row) []cliout.Row {
	for _, row := range rows {
		row["_range"] = chunkRange(row)
	}
	return rows
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
			return exitcode.Errorf(exitcode.NotFound,
				"chunk run %q not found: %v", args[0], err)
		}

		// No sanitize.Message here: cliout strips control characters as it
		// formats each cell, and machine output keeps the original bytes.

		row := chunkRow(chunk)
		row["_range"] = chunkRange(row)

		return cliout.FromContext(cmd.Context()).Object(cliout.Object{
			Schema: chunkSchema,
			Fields: []cliout.Field{
				{Label: "Chunk ID", Key: "chunk_id"},
				{Label: "Run ID", Key: "pipeline_run_id"},
				{Label: "Component", Key: "component_id"},
				{Label: "Range", Key: "_range"},
				{Label: "Attempt", Key: "attempt"},
				{Label: "Status", Key: "status"},
				{Label: "Phase", Key: "phase"},
				{Label: "Failure", Key: "failure_class"},
				{Label: "Error", Key: "error"},
				{Label: "Next action", Key: "recommended_action"},
				{Label: "Started", Key: "started_at"},
				{Label: "Completed", Key: "completed_at"},
				{Label: "Verified", Key: "verified_at"},
				{Label: "Row counts", Key: "row_counts", Format: sortedCounts},
				{Label: "Verification", Key: "verification", Format: sortedStrings},
			},
			Data: row,
		})
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

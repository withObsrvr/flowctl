package api

import (
	"context"
	"testing"

	"github.com/withobsrvr/flowctl/internal/storage"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
)

func TestFirstRunErrorPersists(t *testing.T) {
	store := storage.NewMemoryStorage()
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := NewControlPlaneServer(store)
	ctx := context.Background()
	_, err := server.CreatePipelineRun(ctx, &flowctlpb.CreatePipelineRunRequest{RunId: "run-1", PipelineName: "delivery"})
	if err != nil {
		t.Fatal(err)
	}
	first := `{"component":"source","edge":"source-sink","cause":"UNEXPECTED_EOF"}`
	for _, value := range []string{first, `{"component":"sink"}`} {
		_, err = server.UpdatePipelineRun(ctx, &flowctlpb.UpdatePipelineRunRequest{RunId: "run-1", Status: flowctlpb.RunStatus_RUN_STATUS_FAILED, Error: value})
		if err != nil {
			t.Fatal(err)
		}
	}
	run, err := server.GetPipelineRun(ctx, &flowctlpb.GetPipelineRunRequest{RunId: "run-1"})
	if err != nil || run.Error != first {
		t.Fatalf("run error=%q err=%v", run.GetError(), err)
	}
}

package api

import (
	"context"
	"testing"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/storage"
)

func TestDeliveryFailureHealthSurvivesHeartbeat(t *testing.T) {
	store := storage.NewMemoryStorage()
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := NewControlPlaneServer(store)
	ctx := context.Background()
	_, err := server.RegisterComponent(ctx, &flowctlv1.RegisterRequest{ComponentId: "source", Component: &flowctlv1.ComponentInfo{Id: "source"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = server.Heartbeat(ctx, &flowctlv1.HeartbeatRequest{ServiceId: "source", Status: flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY, Metrics: map[string]string{"delivery_run_id": "run-1"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = server.Heartbeat(ctx, &flowctlv1.HeartbeatRequest{ServiceId: "source", Status: flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetComponentStatus(ctx, &flowctlv1.ComponentStatusRequest{ServiceId: "source"})
	if err != nil || resp.Status != flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY {
		t.Fatalf("status=%v err=%v", resp, err)
	}
	saved, err := store.GetService(ctx, "source")
	if err != nil || saved.Status.Status != flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY {
		t.Fatalf("stored status=%v err=%v", saved, err)
	}
}

package cmd

import "testing"

func TestResolveInitPreset(t *testing.T) {
	tests := []struct {
		preset      string
		wantNetwork string
		wantDest    string
		wantErr     bool
	}{
		{preset: "testnet-duckdb", wantNetwork: "testnet", wantDest: "duckdb"},
		{preset: "testnet-postgres", wantNetwork: "testnet", wantDest: "postgres"},
		{preset: "mainnet-duckdb", wantNetwork: "mainnet", wantDest: "duckdb"},
		{preset: "mainnet-postgres", wantNetwork: "mainnet", wantDest: "postgres"},
		{preset: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		network, dest, err := resolveInitPreset(tt.preset)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for preset %q", tt.preset)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for preset %q: %v", tt.preset, err)
		}
		if network != tt.wantNetwork || dest != tt.wantDest {
			t.Fatalf("preset %q resolved to %s/%s, want %s/%s", tt.preset, network, dest, tt.wantNetwork, tt.wantDest)
		}
	}
}

func TestGenerateIntentUsesArchiveRPCForMainnet(t *testing.T) {
	intent := generateIntent("stellar-pipeline", "stellar-ledger", "duckdb", "mainnet", "123", "", "stellar")

	spec, ok := intent["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec missing or wrong type")
	}

	sources, ok := spec["sources"].([]map[string]interface{})
	if !ok || len(sources) != 1 {
		t.Fatalf("expected one source, got %#v", spec["sources"])
	}

	config, ok := sources[0]["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("source config missing or wrong type")
	}

	got, _ := config["rpc_endpoint"].(string)
	want := "https://archive-rpc.lightsail.network"
	if got != want {
		t.Fatalf("expected mainnet rpc_endpoint %q, got %q", want, got)
	}
}

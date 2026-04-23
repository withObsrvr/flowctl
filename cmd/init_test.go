package cmd

import "testing"

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

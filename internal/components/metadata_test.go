package components

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMetadataBuildsEnvMappingFromConfigSchema(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "metadata.json")

	content := `{
  "name": "duckdb-consumer",
  "version": "1.0.0",
  "type": "sink",
  "config_schema": {
    "database_path": {
      "type": "string",
      "env": "DUCKDB_PATH"
    }
  }
}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test metadata: %v", err)
	}

	metadata, err := LoadMetadata(path)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}

	if got := metadata.EnvMapping["database_path"]; got != "DUCKDB_PATH" {
		t.Fatalf("expected database_path to map to DUCKDB_PATH, got %q", got)
	}
}

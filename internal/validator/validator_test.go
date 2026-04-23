package validator

import (
	"strings"
	"testing"

	"github.com/withobsrvr/flowctl/internal/model"
)

func TestValidatorRejectsKnownInvalidPipeline(t *testing.T) {
	pipeline := &model.Pipeline{
		APIVersion: "flowctl/v1",
		Kind:       "Pipeline",
		Metadata: model.Metadata{Name: "invalid-pipeline"},
		Spec: model.Spec{
			Driver: "unsupported-driver",
			Sources: []model.Component{{
				ID:          "source1",
				Image:       "source:latest",
				HealthCheck: "/health",
			}},
			Processors: []model.Component{{
				ID:       "processor1",
				Image:    "processor:latest",
				Replicas: -1,
			}},
			Sinks: []model.Component{{
				ID:     "sink1",
				Image:  "sink:latest",
				Inputs: []string{},
			}},
		},
	}

	result := NewValidator(pipeline).Validate()
	if result.Valid {
		t.Fatalf("expected invalid pipeline to fail validation")
	}

	joined := result.Format()
	for _, want := range []string{
		"Invalid driver",
		"health_check but no health_port",
		"must have at least one input",
		"replicas cannot be negative",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected validation output to contain %q, got:\n%s", want, joined)
		}
	}
}

func TestValidatorAcceptsQuickstartStylePipeline(t *testing.T) {
	pipeline := &model.Pipeline{
		APIVersion: "flowctl/v1",
		Kind:       "Pipeline",
		Metadata: model.Metadata{Name: "quickstart"},
		Spec: model.Spec{
			Driver: "process",
			Sources: []model.Component{{ID: "stellar-source", Type: "stellar-live-source@v1.0.0"}},
			Processors: []model.Component{{
				ID:     "contract-events",
				Type:   "contract-events-processor@v1.0.0",
				Inputs: []string{"stellar-source"},
			}},
			Sinks: []model.Component{{
				ID:     "duckdb-sink",
				Type:   "duckdb-consumer@v1.0.0",
				Inputs: []string{"contract-events"},
			}},
		},
	}

	result := NewValidator(pipeline).Validate()
	if !result.Valid {
		t.Fatalf("expected quickstart pipeline to be valid, got:\n%s", result.Format())
	}
}

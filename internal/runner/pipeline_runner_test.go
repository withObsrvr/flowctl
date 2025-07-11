package runner

import (
	"testing"
	"time"

	"github.com/withobsrvr/flowctl/internal/model"
)

func TestNewPipelineRunner(t *testing.T) {
	// Create a test pipeline
	pipeline := &model.Pipeline{
		APIVersion: "v1",
		Kind:       "Pipeline",
		Metadata: model.Metadata{
			Name: "test-pipeline",
		},
		Spec: model.Spec{
			Sources: []model.Component{
				{
					ID:   "test-source",
					Type: "test-source",
				},
			},
			Processors: []model.Component{
				{
					ID:   "test-processor",
					Type: "test-processor",
				},
			},
			Sinks: []model.Component{
				{
					ID:   "test-sink",
					Type: "test-sink",
				},
			},
		},
	}

	// Create runner config
	config := Config{
		OrchestratorType:    "process",
		ControlPlanePort:    8084, // Use different port
		ControlPlaneAddress: "127.0.0.1",
		ShowStatus:          false,
		LogDir:              "test-logs",
		HeartbeatTTL:        30 * time.Second,
		JanitorInterval:     10 * time.Second,
	}

	// Create pipeline runner
	runner, err := NewPipelineRunner(pipeline, config)
	if err != nil {
		t.Fatalf("Failed to create pipeline runner: %v", err)
	}

	// Test that runner is created with correct config
	if runner.config.OrchestratorType != "process" {
		t.Errorf("Expected orchestrator type 'process', got '%s'", runner.config.OrchestratorType)
	}

	if runner.config.ControlPlanePort != 8084 {
		t.Errorf("Expected control plane port 8084, got %d", runner.config.ControlPlanePort)
	}

	if runner.pipeline.Metadata.Name != "test-pipeline" {
		t.Errorf("Expected pipeline name 'test-pipeline', got '%s'", runner.pipeline.Metadata.Name)
	}

	// Test control plane endpoint
	expectedEndpoint := "http://127.0.0.1:8084"
	if runner.GetControlPlaneEndpoint() != expectedEndpoint {
		t.Errorf("Expected endpoint %s, got %s", expectedEndpoint, runner.GetControlPlaneEndpoint())
	}

	// Test that orchestrator is set
	if runner.orchestrator == nil {
		t.Error("Orchestrator should be set")
	}

	if runner.orchestrator.GetType() != "process" {
		t.Errorf("Expected orchestrator type 'process', got '%s'", runner.orchestrator.GetType())
	}
}

func TestLoadPipelineFromBytes(t *testing.T) {
	testYAML := `apiVersion: v1
kind: Pipeline
metadata:
  name: test-pipeline
spec:
  sources:
    - id: test-source
      type: test-source
  processors:
    - id: test-processor
      type: test-processor
      inputs:
        - test-source
  sinks:
    - id: test-sink
      type: test-sink
      inputs:
        - test-processor`

	pipeline, err := LoadPipelineFromBytes([]byte(testYAML))
	if err != nil {
		t.Fatalf("Failed to load pipeline from bytes: %v", err)
	}

	if pipeline.Metadata.Name != "test-pipeline" {
		t.Errorf("Expected pipeline name 'test-pipeline', got '%s'", pipeline.Metadata.Name)
	}

	if len(pipeline.Spec.Sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(pipeline.Spec.Sources))
	}

	if len(pipeline.Spec.Processors) != 1 {
		t.Errorf("Expected 1 processor, got %d", len(pipeline.Spec.Processors))
	}

	if len(pipeline.Spec.Sinks) != 1 {
		t.Errorf("Expected 1 sink, got %d", len(pipeline.Spec.Sinks))
	}
}

func TestValidatePipeline(t *testing.T) {
	// Test valid pipeline
	validPipeline := &model.Pipeline{
		APIVersion: "v1",
		Kind:       "Pipeline",
		Metadata: model.Metadata{
			Name: "test-pipeline",
		},
		Spec: model.Spec{
			Sources: []model.Component{
				{
					ID:   "test-source",
					Type: "test-source",
				},
			},
		},
	}

	if err := validatePipeline(validPipeline); err != nil {
		t.Errorf("Valid pipeline should pass validation: %v", err)
	}

	// Test invalid pipeline - missing apiVersion
	invalidPipeline := &model.Pipeline{
		Kind: "Pipeline",
		Metadata: model.Metadata{
			Name: "test-pipeline",
		},
		Spec: model.Spec{
			Sources: []model.Component{
				{
					ID:   "test-source",
					Type: "test-source",
				},
			},
		},
	}

	if err := validatePipeline(invalidPipeline); err == nil {
		t.Error("Pipeline missing apiVersion should fail validation")
	}

	// Test invalid pipeline - wrong kind
	invalidKindPipeline := &model.Pipeline{
		APIVersion: "v1",
		Kind:       "WrongKind",
		Metadata: model.Metadata{
			Name: "test-pipeline",
		},
		Spec: model.Spec{
			Sources: []model.Component{
				{
					ID:   "test-source",
					Type: "test-source",
				},
			},
		},
	}

	if err := validatePipeline(invalidKindPipeline); err == nil {
		t.Error("Pipeline with wrong kind should fail validation")
	}

	// Test invalid pipeline - no sources
	noSourcesPipeline := &model.Pipeline{
		APIVersion: "v1",
		Kind:       "Pipeline",
		Metadata: model.Metadata{
			Name: "test-pipeline",
		},
		Spec: model.Spec{
			Sources: []model.Component{},
		},
	}

	if err := validatePipeline(noSourcesPipeline); err == nil {
		t.Error("Pipeline with no sources should fail validation")
	}
}
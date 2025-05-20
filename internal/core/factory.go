package core

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/model"
)

// PipelineType represents the type of pipeline implementation to use
type PipelineType string

const (
	// PipelineTypeSimple uses the original simple pipeline implementation
	PipelineTypeSimple PipelineType = "simple"
	
	// PipelineTypeDAG uses the new DAG-based pipeline implementation
	PipelineTypeDAG PipelineType = "dag"
	
	// EnvPipelineType is the environment variable to control which pipeline type to use
	EnvPipelineType = "FLOWCTL_PIPELINE_TYPE"
)

// NewPipelineFromConfig creates a pipeline instance based on configuration
func NewPipelineFromConfig(cfg *config.Config) (*Pipeline, error) {
	return NewPipeline(cfg)
}

// NewDAGPipelineFromConfig creates a DAG-based pipeline instance based on configuration
func NewDAGPipelineFromConfig(cfg *config.Config, pipeline *model.Pipeline) (*DAGPipeline, error) {
	return NewDAGPipeline(cfg, pipeline)
}

// DeterminePipelineType determines which pipeline implementation to use
func DeterminePipelineType() PipelineType {
	// Check environment variable
	pipelineType := os.Getenv(EnvPipelineType)
	if pipelineType != "" {
		if strings.ToLower(pipelineType) == string(PipelineTypeDAG) {
			return PipelineTypeDAG
		}
		if strings.ToLower(pipelineType) == string(PipelineTypeSimple) {
			return PipelineTypeSimple
		}
	}
	
	// Default to DAG-based implementation
	return PipelineTypeDAG
}

// PipelineInterface defines a common interface for different pipeline implementations
type PipelineInterface interface {
	// Start begins processing data through the pipeline
	Start(ctx context.Context) error
	
	// Stop gracefully shuts down the pipeline
	Stop() error
}

// CreatePipeline creates a pipeline using the appropriate implementation
func CreatePipeline(cfg *config.Config, pipeline *model.Pipeline) (PipelineInterface, error) {
	pipelineType := DeterminePipelineType()
	
	switch pipelineType {
	case PipelineTypeSimple:
		return NewPipelineFromConfig(cfg)
	case PipelineTypeDAG:
		return NewDAGPipelineFromConfig(cfg, pipeline)
	default:
		return nil, fmt.Errorf("unknown pipeline type: %s", pipelineType)
	}
}
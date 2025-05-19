package drivers

import (
	"context"
	
	"github.com/withobsrvr/flowctl/internal/model"
)

// Driver represents a driver for deploying and managing pipelines
type Driver interface {
	// Apply deploys a pipeline 
	Apply(ctx context.Context, pipeline *model.Pipeline, options map[string]string) error
	
	// Delete removes a deployed pipeline
	Delete(ctx context.Context, pipelineName string, namespace string, options map[string]string) error
	
	// Status retrieves the status of a deployed pipeline
	Status(ctx context.Context, pipelineName string, namespace string, options map[string]string) (*PipelineStatus, error)
}

// PipelineStatus represents the status of a deployed pipeline
type PipelineStatus struct {
	Name      string                      `json:"name"`
	Namespace string                      `json:"namespace"`
	State     string                      `json:"state"` // running, failed, pending
	Components map[string]ComponentStatus `json:"components"`
	Message   string                      `json:"message,omitempty"`
}

// ComponentStatus represents the status of a pipeline component
type ComponentStatus struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"` // source, processor, sink
	State     string            `json:"state"` // running, failed, pending, stopped
	Replicas  int               `json:"replicas"`
	Ready     int               `json:"ready"`
	Message   string            `json:"message,omitempty"`
	Endpoints map[string]string `json:"endpoints,omitempty"`
}

// Factory is a factory function for creating a driver
type Factory func() Driver

// drivers is a registry of available drivers
var drivers = make(map[string]Factory)

// Register registers a driver factory with the registry
func Register(name string, factory Factory) {
	drivers[name] = factory
}

// Get returns a driver by name
func Get(name string) (Driver, bool) {
	factory, ok := drivers[name]
	if !ok {
		return nil, false
	}
	return factory(), true
}

// List returns a list of registered driver names
func List() []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	return names
}
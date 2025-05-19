package translator

import (
	"fmt"

	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
)

// BasicValidator implements basic validation logic
type BasicValidator struct{}

// NewSchemaValidatorImpl creates a new schema validator implementation
func NewSchemaValidatorImpl() *BasicValidator {
	return &BasicValidator{}
}

// Validate checks if a pipeline configuration is valid
func (v *BasicValidator) Validate(pipeline *model.Pipeline) error {
	logger.Debug("Validating pipeline configuration")

	// Check required fields
	if pipeline.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	
	if pipeline.Kind != "Pipeline" {
		return fmt.Errorf("kind must be 'Pipeline'")
	}
	
	if pipeline.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	
	// Check components
	if len(pipeline.Spec.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}
	
	// Validate that each component has required fields
	for _, source := range pipeline.Spec.Sources {
		if err := v.ValidateSource(source); err != nil {
			return err
		}
	}
	
	for _, processor := range pipeline.Spec.Processors {
		if err := v.ValidateProcessor(processor); err != nil {
			return err
		}
	}
	
	for _, sink := range pipeline.Spec.Sinks {
		if err := v.ValidateSink(sink); err != nil {
			return err
		}
	}
	
	// Validate connections between components
	// (Check that all input references exist)
	componentIDs := make(map[string]bool)
	
	// Collect all component IDs
	for _, src := range pipeline.Spec.Sources {
		componentIDs[src.ID] = true
	}
	
	// Check processor inputs
	for _, proc := range pipeline.Spec.Processors {
		componentIDs[proc.ID] = true
		for _, input := range proc.Inputs {
			if !componentIDs[input] {
				return fmt.Errorf("processor %s references unknown input %s", proc.ID, input)
			}
		}
	}
	
	// Check sink inputs
	for _, sink := range pipeline.Spec.Sinks {
		for _, input := range sink.Inputs {
			if !componentIDs[input] {
				return fmt.Errorf("sink %s references unknown input %s", sink.ID, input)
			}
		}
	}

	// Validate cross-field invariants
	// Driver is optional, but if specified must be valid
	if pipeline.Spec.Driver != "" {
		validDrivers := []string{"docker", "kubernetes", "k8s", "nomad", "local"}
		isValid := false
		for _, d := range validDrivers {
			if pipeline.Spec.Driver == d {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid driver: %s. Valid values: docker, kubernetes, k8s, nomad, local", pipeline.Spec.Driver)
		}
	}

	return nil
}

// ValidateSource validates a source component
func (v *BasicValidator) ValidateSource(source model.Component) error {
	if source.ID == "" {
		return fmt.Errorf("source id is required")
	}
	if source.Image == "" {
		return fmt.Errorf("source image is required for %s", source.ID)
	}
	
	// Source-specific validations can go here
	if source.HealthCheck != "" && source.HealthPort == 0 {
		return fmt.Errorf("source %s has health_check but no health_port specified", source.ID)
	}

	return nil
}

// ValidateProcessor validates a processor component
func (v *BasicValidator) ValidateProcessor(processor model.Component) error {
	if processor.ID == "" {
		return fmt.Errorf("processor id is required")
	}
	if processor.Image == "" {
		return fmt.Errorf("processor image is required for %s", processor.ID)
	}
	if len(processor.Inputs) == 0 {
		return fmt.Errorf("processor %s must have at least one input", processor.ID)
	}

	// Processor-specific validations can go here
	if processor.Replicas < 0 {
		return fmt.Errorf("processor %s replicas cannot be negative", processor.ID)
	}

	return nil
}

// ValidateSink validates a sink component
func (v *BasicValidator) ValidateSink(sink model.Component) error {
	if sink.ID == "" {
		return fmt.Errorf("sink id is required")
	}
	if sink.Image == "" {
		return fmt.Errorf("sink image is required for %s", sink.ID)
	}
	if len(sink.Inputs) == 0 {
		return fmt.Errorf("sink %s must have at least one input", sink.ID)
	}

	// Sink-specific validations can go here
	if sink.Replicas < 0 {
		return fmt.Errorf("sink %s replicas cannot be negative", sink.ID)
	}

	return nil
}

// ValidateConfig validates a legacy configuration
func (v *BasicValidator) ValidateConfig(cfg *model.Config) error {
	if cfg.Version == "" {
		return fmt.Errorf("version is required")
	}

	if cfg.Source.Type == "" {
		return fmt.Errorf("source type is required")
	}

	if len(cfg.Processors) == 0 {
		return fmt.Errorf("at least one processor is required")
	}

	if cfg.Sink.Type == "" {
		return fmt.Errorf("sink type is required")
	}

	return nil
}
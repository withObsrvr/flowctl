package translator

import (
	"fmt"

	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
)

// SchemaValidator validates configuration against schemas
type SchemaValidator interface {
	// Validate checks if configuration is valid
	Validate(cfg *config.Config) error

	// ValidateSource specifically validates source configuration
	ValidateSource(src config.SourceConfig) error

	// ValidateProcessor validates processor configuration
	ValidateProcessor(proc config.ProcessorConfig) error

	// ValidateSink validates sink configuration
	ValidateSink(sink config.SinkConfig) error
}

// BasicValidator implements a simple validation logic
type BasicValidator struct{}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator() SchemaValidator {
	return &BasicValidator{}
}

// Validate checks if configuration is valid
func (v *BasicValidator) Validate(cfg *config.Config) error {
	logger.Debug("Validating pipeline configuration")

	// Basic structure validation
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate source
	if err := v.ValidateSource(cfg.Source); err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	// Validate processors
	if len(cfg.Processors) == 0 {
		return fmt.Errorf("at least one processor must be defined")
	}
	for i, proc := range cfg.Processors {
		if err := v.ValidateProcessor(proc); err != nil {
			return fmt.Errorf("invalid processor at index %d: %w", i, err)
		}
	}

	// Validate sink
	if err := v.ValidateSink(cfg.Sink); err != nil {
		return fmt.Errorf("invalid sink: %w", err)
	}

	return nil
}

// ValidateSource specifically validates source configuration
func (v *BasicValidator) ValidateSource(src config.SourceConfig) error {
	if src.Type == "" {
		return fmt.Errorf("source type must be specified")
	}

	// Additional source-specific validations can be added here
	return nil
}

// ValidateProcessor validates processor configuration
func (v *BasicValidator) ValidateProcessor(proc config.ProcessorConfig) error {
	if proc.Name == "" {
		return fmt.Errorf("processor name must be specified")
	}

	if proc.Plugin == "" {
		return fmt.Errorf("processor plugin must be specified")
	}

	// Additional processor-specific validations can be added here
	return nil
}

// ValidateSink validates sink configuration
func (v *BasicValidator) ValidateSink(sink config.SinkConfig) error {
	if sink.Type == "" {
		return fmt.Errorf("sink type must be specified")
	}

	// Additional sink-specific validations can be added here
	return nil
}
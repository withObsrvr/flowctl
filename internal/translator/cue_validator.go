package translator

import (
	"os"
	"path/filepath"

	"github.com/withobsrvr/flowctl/internal/interfaces"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// CueValidator implements the Validator interface using CUE schema
type CueValidator struct {
	schemaPath string
}

// NewCueValidator creates a new CUE schema validator
func NewCueValidator() interfaces.Validator {
	// Locate the schema file relative to the binary
	schemaPath := findSchemaPath()
	return &CueValidator{
		schemaPath: schemaPath,
	}
}

// findSchemaPath locates the CUE schema directory
func findSchemaPath() string {
	// Possible locations for the schema directory
	possiblePaths := []string{
		// From binary in repo root
		"./schemas/cue",
		// From binary in bin/
		"../schemas/cue",
		// Absolute path for production deployments
		"/opt/flowctl/schemas/cue",
	}

	for _, path := range possiblePaths {
		schemaFile := filepath.Join(path, "schema.cue")
		if _, err := os.Stat(schemaFile); err == nil {
			logger.Info("Found CUE schema at", zap.String("path", schemaFile))
			return path
		} else {
			logger.Debug("Schema not found at", zap.String("path", schemaFile), zap.Error(err))
		}
	}

	// Default to the first path, even if it doesn't exist (we'll fail later)
	logger.Warn("Could not find CUE schema, using default path")
	return possiblePaths[0]
}

// Validate validates a Pipeline configuration
// Currently uses basic YAML validation. CUE schema validation may be added in the future.
func (v *CueValidator) Validate(pipeline *model.Pipeline) error {
	logger.Debug("Using basic YAML validator")
	basicValidator := NewSchemaValidatorImpl()
	return basicValidator.Validate(pipeline)
}

// ValidateSource validates a source component (delegates to the basic validator)
func (v *CueValidator) ValidateSource(source model.Component) error {
	basicValidator := NewSchemaValidatorImpl()
	return basicValidator.ValidateSource(source)
}

// ValidateProcessor validates a processor component (delegates to the basic validator)
func (v *CueValidator) ValidateProcessor(processor model.Component) error {
	basicValidator := NewSchemaValidatorImpl()
	return basicValidator.ValidateProcessor(processor)
}

// ValidateSink validates a sink component (delegates to the basic validator)
func (v *CueValidator) ValidateSink(sink model.Component) error {
	basicValidator := NewSchemaValidatorImpl()
	return basicValidator.ValidateSink(sink)
}

// ValidateConfig validates a legacy configuration
func (v *CueValidator) ValidateConfig(cfg *model.Config) error {
	basicValidator := NewSchemaValidatorImpl()
	return basicValidator.ValidateConfig(cfg)
}
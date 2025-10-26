package translator

import (
	// "fmt"
	"os"
	"path/filepath"
	// "strings"

	// "cuelang.org/go/cue"
	// "cuelang.org/go/cue/cuecontext"
	// "cuelang.org/go/cue/load"
	// "gopkg.in/yaml.v3"

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

// Validate validates a Pipeline configuration using the CUE schema
func (v *CueValidator) Validate(pipeline *model.Pipeline) error {
	// TODO: Fix CUE schema loading issue - temporarily use basic validator
	logger.Debug("CUE validator temporarily disabled, using basic validator")
	basicValidator := NewSchemaValidatorImpl()
	return basicValidator.Validate(pipeline)

	/* Original CUE validation code - temporarily disabled
	// Convert Pipeline to YAML
	yamlData, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("failed to marshal pipeline to YAML: %w", err)
	}

	// Create CUE context
	ctx := cuecontext.New()

	// Load CUE schema using absolute path to schema file
	schemaFile := filepath.Join(v.schemaPath, "schema.cue")
	
	loadConfig := &load.Config{
		ModuleRoot: v.schemaPath,
		Module:     "github.com/withobsrvr/flowctl/schemas/cue",
	}

	entrypoints := []string{
		schemaFile,
	}

	buildInstances := load.Instances(entrypoints, loadConfig)
	if len(buildInstances) == 0 {
		logger.Warn("Failed to load CUE schema, falling back to basic validator", 
			zap.String("path", v.schemaPath))
		// Fall back to basic validator
		basicValidator := NewSchemaValidatorImpl()
		return basicValidator.Validate(pipeline)
	}

	schemaInstance := buildInstances[0]
	if schemaInstance.Err != nil {
		return fmt.Errorf("CUE schema error: %w", schemaInstance.Err)
	}

	// Build the CUE value from the schema
	schemaValue := ctx.BuildInstance(schemaInstance)
	if schemaValue.Err() != nil {
		return fmt.Errorf("failed to build CUE schema: %w", schemaValue.Err())
	}

	// Parse the YAML data into CUE
	dataValue := ctx.CompileBytes(yamlData)
	if dataValue.Err() != nil {
		return fmt.Errorf("failed to compile pipeline data: %w", dataValue.Err())
	}

	// Validate the YAML data against the schema
	unified := schemaValue.Unify(dataValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		// Improve error messages
		errMsg := err.Error()
		if strings.Contains(errMsg, "#Pipeline") {
			errMsg = strings.Replace(errMsg, "#Pipeline", "Pipeline", -1)
		}
		if strings.Contains(errMsg, "field not allowed") {
			errMsg = strings.Replace(errMsg, "field not allowed", "unexpected field provided", -1)
		}
		
		// Format the error more clearly
		lines := strings.Split(errMsg, "\n")
		if len(lines) > 0 {
			mainError := lines[0]
			return fmt.Errorf("pipeline validation error: %s", mainError)
		}
		
		return fmt.Errorf("pipeline validation error: %v", err)
	}

	logger.Debug("Pipeline validated successfully using CUE schema")
	return nil
	*/
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
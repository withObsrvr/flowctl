package translator

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/withobsrvr/flowctl/internal/interfaces"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// translatorImpl implements the interfaces.Translator interface
type translatorImpl struct {
	options    model.TranslationOptions
	parsers    map[string]interfaces.Parser
	generators map[model.Format]interfaces.Generator
	validator  interfaces.Validator
}

// NewTranslator creates a new translator with given options
func NewTranslator(opts model.TranslationOptions) interfaces.Translator {
	// We'll initialize parsers and generators separately
	t := &translatorImpl{
		options:    opts,
		parsers:    make(map[string]interfaces.Parser),
		generators: make(map[model.Format]interfaces.Generator),
	}

	// Initialize parsers and generators during creation
	t.registerParsersAndGenerators()

	return t
}

// registerParsersAndGenerators sets up the available parsers and generators
func (t *translatorImpl) registerParsersAndGenerators() {
	// Register parsers - we'll implement factory functions in separate files
	t.parsers[".yaml"] = NewYAMLParser()
	t.parsers[".yml"] = NewYAMLParser()
	t.parsers[".json"] = NewJSONParser()

	// Register generators - we'll implement factory functions in separate files
	t.generators[model.DockerCompose] = NewDockerComposeGenerator()
	t.generators[model.Local] = NewLocalGenerator()

	// Set validator
	t.validator = NewSchemaValidator()
}

// Translate converts a pipeline to a target format
func (t *translatorImpl) Translate(ctx context.Context, pipeline *model.Pipeline, format model.Format) ([]byte, error) {
	logger.Debug("Translating pipeline", zap.String("format", string(format)))

	// Validate the pipeline
	if err := t.validator.Validate(pipeline); err != nil {
		return nil, fmt.Errorf("invalid pipeline configuration: %w", err)
	}

	// Get the appropriate generator
	gen, ok := t.generators[format]
	if !ok {
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}

	// Generate the output
	result, err := gen.Generate(pipeline, t.options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate %s output: %w", format, err)
	}

	return result, nil
}

// TranslateFromReader reads pipeline config from io.Reader and translates it
func (t *translatorImpl) TranslateFromReader(ctx context.Context, r io.Reader, format model.Format) ([]byte, error) {
	// Determine the parser to use
	p := t.parsers[".yaml"] // Default to YAML

	// Parse the input
	pipeline, err := p.ParseReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// Translate the parsed config
	return t.Translate(ctx, pipeline, format)
}

// TranslateFromFile translates a pipeline config file to target format
func (t *translatorImpl) TranslateFromFile(ctx context.Context, filePath string, format model.Format) ([]byte, error) {
	logger.Debug("Translating file", zap.String("file", filePath), zap.String("format", string(format)))

	// Get file extension
	ext := filepath.Ext(filePath)
	p, ok := t.parsers[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the file
	pipeline, err := p.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// Translate the parsed config
	result, err := t.Translate(ctx, pipeline, format)
	if err != nil {
		return nil, err
	}

	// If output path is specified in options, write the result to file
	if t.options.OutputPath != "" {
		outputFile := t.options.OutputPath
		if err := os.WriteFile(outputFile, result, 0644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %w", outputFile, err)
		}
		logger.Info("Translated pipeline written to file", zap.String("path", outputFile))
	}

	return result, nil
}

// ValidFormats returns all supported output formats
func (t *translatorImpl) ValidFormats() []model.Format {
	formats := make([]model.Format, 0, len(t.generators))
	for format := range t.generators {
		formats = append(formats, format)
	}
	return formats
}
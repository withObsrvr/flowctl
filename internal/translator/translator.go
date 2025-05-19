package translator

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Format represents a target output format
type Format string

const (
	// DockerCompose format generates Docker Compose YAML
	DockerCompose Format = "docker-compose"
	// Kubernetes format generates Kubernetes manifests
	Kubernetes Format = "kubernetes"
	// Nomad format generates Nomad job specifications
	Nomad Format = "nomad"
	// Local format for local execution (no container orchestration)
	Local Format = "local"
)

// TranslationOptions holds configuration for the translation process
type TranslationOptions struct {
	// OutputVersion is the version of the output format (e.g., "v1" for Kubernetes)
	OutputVersion string
	// ResourcePrefix is a prefix for resource names
	ResourcePrefix string
	// Labels are additional labels/tags to add to resources
	Labels map[string]string
	// EnvVars are environment variables to inject into all components
	EnvVars map[string]string
	// RegistryPrefix is a prefix for container image references
	RegistryPrefix string
	// OutputPath is the path where output files should be written
	OutputPath string
}

// Translator is the interface for pipeline translation
type Translator interface {
	// Translate converts a pipeline config to a target format
	Translate(ctx context.Context, cfg *config.Config, format Format) ([]byte, error)

	// TranslateFromReader reads pipeline config from io.Reader and translates it
	TranslateFromReader(ctx context.Context, r io.Reader, format Format) ([]byte, error)

	// TranslateFromFile translates a pipeline config file to target format
	TranslateFromFile(ctx context.Context, filePath string, format Format) ([]byte, error)

	// ValidFormats returns all supported output formats
	ValidFormats() []Format
}

// translatorImpl implements the Translator interface
type translatorImpl struct {
	options    TranslationOptions
	parsers    map[string]Parser
	generators map[Format]Generator
	validator  SchemaValidator
}

// NewTranslator creates a new translator with given options
func NewTranslator(opts TranslationOptions) Translator {
	return &translatorImpl{
		options: opts,
		parsers: map[string]Parser{
			".yaml": &YAMLParser{},
			".yml":  &YAMLParser{},
			".json": &JSONParser{},
		},
		generators: map[Format]Generator{
			DockerCompose: &DockerComposeGenerator{},
			Kubernetes:    &KubernetesGenerator{},
			Nomad:         &NomadGenerator{},
			Local:         &LocalGenerator{},
		},
		validator: NewSchemaValidator(),
	}
}

// Translate converts a pipeline config to a target format
func (t *translatorImpl) Translate(ctx context.Context, cfg *config.Config, format Format) ([]byte, error) {
	logger.Debug("Translating pipeline config", zap.String("format", string(format)))

	// Validate the config
	if err := t.validator.Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid pipeline configuration: %w", err)
	}

	// Get the appropriate generator
	generator, ok := t.generators[format]
	if !ok {
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}

	// Generate the output
	result, err := generator.Generate(cfg, t.options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate %s output: %w", format, err)
	}

	return result, nil
}

// TranslateFromReader reads pipeline config from io.Reader and translates it
func (t *translatorImpl) TranslateFromReader(ctx context.Context, r io.Reader, format Format) ([]byte, error) {
	// Get the right parser based on expected content
	parser := t.parsers[".yaml"] // Default to YAML

	// Parse the input
	cfg, err := parser.ParseReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// Translate the parsed config
	return t.Translate(ctx, cfg, format)
}

// TranslateFromFile translates a pipeline config file to target format
func (t *translatorImpl) TranslateFromFile(ctx context.Context, filePath string, format Format) ([]byte, error) {
	logger.Debug("Translating file", zap.String("file", filePath), zap.String("format", string(format)))

	// Get file extension
	ext := filepath.Ext(filePath)
	parser, ok := t.parsers[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the file
	cfg, err := parser.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// Translate the parsed config
	result, err := t.Translate(ctx, cfg, format)
	if err != nil {
		return nil, err
	}

	// If output path is specified, write the result to file
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
func (t *translatorImpl) ValidFormats() []Format {
	formats := make([]Format, 0, len(t.generators))
	for format := range t.generators {
		formats = append(formats, format)
	}
	return formats
}
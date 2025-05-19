package interfaces

import (
	"context"
	"io"

	"github.com/withobsrvr/flowctl/internal/model"
)

// Parser defines the interface for parsing pipeline configurations
type Parser interface {
	// Parse parses input data into a Pipeline structure
	Parse(data []byte) (*model.Pipeline, error)

	// ParseReader parses from an io.Reader
	ParseReader(r io.Reader) (*model.Pipeline, error) 
}

// Generator is the interface for generating target-specific output
type Generator interface {
	// Generate produces configuration for the target platform
	Generate(pipeline *model.Pipeline, opts model.TranslationOptions) ([]byte, error)

	// Validate checks if the pipeline can be translated to this format
	Validate(pipeline *model.Pipeline) error
}

// Validator validates pipeline configurations against a schema
type Validator interface {
	// Validate checks if a pipeline configuration is valid
	Validate(pipeline *model.Pipeline) error

	// ValidateSource validates a source component
	ValidateSource(source model.Component) error

	// ValidateProcessor validates a processor component
	ValidateProcessor(processor model.Component) error

	// ValidateSink validates a sink component
	ValidateSink(sink model.Component) error
}

// Translator is the interface for pipeline translation
type Translator interface {
	// Translate converts a pipeline to a target format
	Translate(ctx context.Context, pipeline *model.Pipeline, format model.Format) ([]byte, error)

	// TranslateFromReader reads pipeline config from io.Reader and translates it
	TranslateFromReader(ctx context.Context, r io.Reader, format model.Format) ([]byte, error)

	// TranslateFromFile translates a pipeline config file to target format
	TranslateFromFile(ctx context.Context, filePath string, format model.Format) ([]byte, error)

	// ValidFormats returns all supported output formats
	ValidFormats() []model.Format
}
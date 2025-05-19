package translator

import (
	"github.com/withobsrvr/flowctl/internal/generator"
	"github.com/withobsrvr/flowctl/internal/interfaces"
)

// NewYAMLParser creates a new YAML parser
func NewYAMLParser() interfaces.Parser {
	return NewYAMLParserImpl()
}

// NewJSONParser creates a new JSON parser
func NewJSONParser() interfaces.Parser {
	return NewJSONParserImpl()
}

// NewDockerComposeGenerator creates a Docker Compose generator
func NewDockerComposeGenerator() interfaces.Generator {
	return generator.NewDockerComposeGenerator()
}

// NewKubernetesGenerator creates a Kubernetes generator
func NewKubernetesGenerator() interfaces.Generator {
	return generator.NewKubernetesGenerator()
}

// NewNomadGenerator creates a Nomad generator
func NewNomadGenerator() interfaces.Generator {
	return generator.NewNomadGenerator()
}

// NewLocalGenerator creates a local execution generator
func NewLocalGenerator() interfaces.Generator {
	return generator.NewLocalGenerator()
}

// NewSchemaValidator creates a schema validator
func NewSchemaValidator() interfaces.Validator {
	// Create and return a CUE validator that uses the canonical schema in schemas/cue/schema.cue
	// Note: The CUE validator has built-in fallback to the basic validator
	// when CUE schemas aren't available (handled in CueValidator.Validate)
	return NewCueValidator()
}
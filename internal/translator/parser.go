package translator

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/withobsrvr/flowctl/internal/interfaces"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// YAMLParser implements Parser for YAML files
type YAMLParser struct {
	validator interfaces.Validator
}

// NewYAMLParserImpl creates a new YAML parser implementation
func NewYAMLParserImpl() *YAMLParser {
	// We'll use a placeholder for validator and set it properly later
	// This breaks the dependency cycle
	return &YAMLParser{}
}

// SetValidator sets the validator (used to break dependency cycle)
func (p *YAMLParser) SetValidator(v interfaces.Validator) {
	p.validator = v
}

// Parse parses YAML input data into a Pipeline structure
func (p *YAMLParser) Parse(data []byte) (*model.Pipeline, error) {
	logger.Debug("Parsing YAML pipeline configuration")

	// Try to parse as new Pipeline format
	var pipeline model.Pipeline
	if err := yaml.Unmarshal(data, &pipeline); err == nil {
		// If we can unmarshal as Pipeline and it has the expected fields, use it
		if pipeline.APIVersion != "" && pipeline.Kind == "Pipeline" {
			// Only validate if we have a validator
			if p.validator != nil {
				if err := p.validator.Validate(&pipeline); err != nil {
					logger.Error("Pipeline validation failed", zap.Error(err))
					return nil, err
				}
			}

			logger.Debug("Successfully parsed pipeline",
				zap.String("name", pipeline.Metadata.Name),
				zap.Int("sources", len(pipeline.Spec.Sources)),
				zap.Int("processors", len(pipeline.Spec.Processors)),
				zap.Int("sinks", len(pipeline.Spec.Sinks)))

			return &pipeline, nil
		}
	}

	// Otherwise, try legacy format and convert
	logger.Debug("Attempting to parse as legacy config format")
	cfg, err := p.ParseLegacy(data)
	if err != nil {
		logger.Error("Failed to parse as legacy format", zap.Error(err))
		return nil, fmt.Errorf("unable to parse pipeline configuration: %w", err)
	}

	// Convert legacy config to Pipeline
	pipeline = convertConfigToPipeline(cfg)
	
	logger.Debug("Successfully converted legacy config to pipeline format",
		zap.String("version", cfg.Version))

	return &pipeline, nil
}

// ParseReader parses YAML from an io.Reader
func (p *YAMLParser) ParseReader(r io.Reader) (*model.Pipeline, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	return p.Parse(data)
}

// ParseLegacy parses YAML into a legacy Config structure
func (p *YAMLParser) ParseLegacy(data []byte) (*model.Config, error) {
	var cfg model.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &cfg, nil
}

// JSONParser implements Parser for JSON files
type JSONParser struct {
	validator interfaces.Validator
}

// NewJSONParserImpl creates a new JSON parser implementation
func NewJSONParserImpl() *JSONParser {
	// We'll use a placeholder for validator and set it properly later
	return &JSONParser{}
}

// SetValidator sets the validator (used to break dependency cycle)
func (p *JSONParser) SetValidator(v interfaces.Validator) {
	p.validator = v
}

// Parse parses JSON input data into a Pipeline structure
func (p *JSONParser) Parse(data []byte) (*model.Pipeline, error) {
	logger.Debug("Parsing JSON pipeline configuration")

	// Try to parse as new Pipeline format first
	var pipeline model.Pipeline
	if err := json.Unmarshal(data, &pipeline); err == nil {
		// If we can unmarshal as Pipeline and it has the expected fields, use it
		if pipeline.APIVersion != "" && pipeline.Kind == "Pipeline" {
			// Only validate if we have a validator
			if p.validator != nil {
				if err := p.validator.Validate(&pipeline); err != nil {
					logger.Error("Pipeline validation failed", zap.Error(err))
					return nil, err
				}
			}

			logger.Debug("Successfully parsed pipeline",
				zap.String("name", pipeline.Metadata.Name),
				zap.Int("sources", len(pipeline.Spec.Sources)),
				zap.Int("processors", len(pipeline.Spec.Processors)),
				zap.Int("sinks", len(pipeline.Spec.Sinks)))

			return &pipeline, nil
		}
	}

	// Otherwise, try legacy format and convert
	logger.Debug("Attempting to parse as legacy config format")
	cfg, err := p.ParseLegacy(data)
	if err != nil {
		logger.Error("Failed to parse as legacy format", zap.Error(err))
		return nil, fmt.Errorf("unable to parse pipeline configuration: %w", err)
	}

	// Convert legacy config to Pipeline
	pipeline = convertConfigToPipeline(cfg)
	
	logger.Debug("Successfully converted legacy config to pipeline format",
		zap.String("version", cfg.Version))

	return &pipeline, nil
}

// ParseReader parses JSON from an io.Reader
func (p *JSONParser) ParseReader(r io.Reader) (*model.Pipeline, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	return p.Parse(data)
}

// ParseLegacy parses JSON into a legacy Config structure
func (p *JSONParser) ParseLegacy(data []byte) (*model.Config, error) {
	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &cfg, nil
}

// Helper function to convert legacy Config to Pipeline
func convertConfigToPipeline(cfg *model.Config) model.Pipeline {
	// Create a basic Pipeline from Config
	pipeline := model.Pipeline{
		APIVersion: "flowctl/v1",
		Kind:       "Pipeline",
		Metadata: model.Metadata{
			Name: "pipeline", // Default name
		},
		Spec: model.Spec{
			Sources:    make([]model.Component, 0, 1),
			Processors: make([]model.Component, 0, len(cfg.Processors)),
			Sinks:      make([]model.Component, 0, 1),
		},
	}

	// Convert Source
	source := model.Component{
		ID:   cfg.Source.Type,
		Type: cfg.Source.Type,
	}

	// Extract common fields from params
	extractComponentFieldsFromParams(&source, cfg.Source.Params)
	pipeline.Spec.Sources = append(pipeline.Spec.Sources, source)

	// Convert Processors
	for _, proc := range cfg.Processors {
		processor := model.Component{
			ID:     proc.Name,
			Type:   proc.Plugin,
			Inputs: []string{cfg.Source.Type}, // Default to source as input
		}
		
		// Extract common fields from params
		extractComponentFieldsFromParams(&processor, proc.Params)
		pipeline.Spec.Processors = append(pipeline.Spec.Processors, processor)
	}

	// Convert Sink
	sink := model.Component{
		ID:   cfg.Sink.Type,
		Type: cfg.Sink.Type,
	}
	
	// Set default input to last processor if available
	if len(cfg.Processors) > 0 {
		sink.Inputs = []string{cfg.Processors[len(cfg.Processors)-1].Name}
	} else {
		// Otherwise use source as input
		sink.Inputs = []string{cfg.Source.Type}
	}
	
	// Extract common fields from params
	extractComponentFieldsFromParams(&sink, cfg.Sink.Params)
	pipeline.Spec.Sinks = append(pipeline.Spec.Sinks, sink)

	return pipeline
}

// Helper to extract common component fields from a params map
func extractComponentFieldsFromParams(component *model.Component, params map[string]any) {
	// Extract image
	if img, ok := params["image"].(string); ok {
		component.Image = img
	}
	
	// Extract command
	if cmd, ok := params["command"].([]string); ok {
		component.Command = cmd
	} else if cmdList, ok := params["command"].([]interface{}); ok {
		component.Command = make([]string, len(cmdList))
		for i, c := range cmdList {
			component.Command[i] = fmt.Sprintf("%v", c)
		}
	}
	
	// Extract environment variables
	if env, ok := params["env"].(map[string]string); ok {
		component.Env = env
	} else if envMap, ok := params["env"].(map[string]interface{}); ok {
		component.Env = make(map[string]string)
		for k, v := range envMap {
			component.Env[k] = fmt.Sprintf("%v", v)
		}
	}
	
	// Extract ports
	if ports, ok := params["ports"].([]int); ok {
		component.Ports = make([]model.Port, len(ports))
		for i, p := range ports {
			component.Ports[i] = model.Port{
				ContainerPort: p,
			}
		}
	} else if portList, ok := params["ports"].([]interface{}); ok {
		component.Ports = make([]model.Port, len(portList))
		for i, p := range portList {
			if pInt, ok := p.(int); ok {
				component.Ports[i] = model.Port{
					ContainerPort: pInt,
				}
			} else {
				component.Ports[i] = model.Port{
					ContainerPort: 0, // Default
				}
				// Try to parse as number
				if pStr, ok := p.(string); ok {
					var port int
					if _, err := fmt.Sscanf(pStr, "%d", &port); err == nil {
						component.Ports[i].ContainerPort = port
					}
				}
			}
		}
	}
	
	// Extract health check
	if health, ok := params["health_check"].(string); ok {
		component.HealthCheck = health
	}
	
	// Extract health port
	if port, ok := params["health_port"].(int); ok {
		component.HealthPort = port
	} else if portStr, ok := params["health_port"].(string); ok {
		var port int
		if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil {
			component.HealthPort = port
		}
	}
}
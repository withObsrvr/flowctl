package generators

import (
	"fmt"
	"io"
	"strings"

	"github.com/withobsrvr/flowctl/internal/translator/models"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// DockerComposeGenerator generates Docker Compose files from Pipeline models
type DockerComposeGenerator struct{}

// NewDockerComposeGenerator creates a new Docker Compose generator
func NewDockerComposeGenerator() *DockerComposeGenerator {
	return &DockerComposeGenerator{}
}

// DockerComposeConfig represents a Docker Compose configuration
type DockerComposeConfig struct {
	Version  string                          `yaml:"version"`
	Services map[string]DockerComposeService `yaml:"services"`
	Networks map[string]DockerComposeNetwork `yaml:"networks,omitempty"`
	Volumes  map[string]DockerComposeVolume  `yaml:"volumes,omitempty"`
}

// DockerComposeService represents a service in Docker Compose
type DockerComposeService struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name,omitempty"`
	DependsOn     []string          `yaml:"depends_on,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
	Deploy        *DeployConfig     `yaml:"deploy,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
}

// DeployConfig represents deploy config in Docker Compose
type DeployConfig struct {
	Replicas int `yaml:"replicas,omitempty"`
}

// DockerComposeNetwork represents a network in Docker Compose
type DockerComposeNetwork struct {
	Driver string            `yaml:"driver,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

// DockerComposeVolume represents a volume in Docker Compose
type DockerComposeVolume struct {
	Driver string            `yaml:"driver,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

// Generate generates a Docker Compose configuration from a Pipeline
func (g *DockerComposeGenerator) Generate(pipeline *models.Pipeline, writer io.Writer) error {
	logger.Debug("Generating Docker Compose file",
		zap.String("pipeline", pipeline.Metadata.Name))

	// Create Docker Compose config
	composeConfig := DockerComposeConfig{
		Version:  "3.8",
		Services: make(map[string]DockerComposeService),
		Networks: map[string]DockerComposeNetwork{
			"flow-network": {
				Driver: "bridge",
				Labels: map[string]string{
					"com.obsrvr.flow.pipeline": pipeline.Metadata.Name,
				},
			},
		},
		Volumes: make(map[string]DockerComposeVolume),
	}

	// Configure shared volumes if needed
	composeConfig.Volumes["flow-data"] = DockerComposeVolume{
		Driver: "local",
		Labels: map[string]string{
			"com.obsrvr.flow.pipeline": pipeline.Metadata.Name,
		},
	}

	// Process sources
	for _, source := range pipeline.Spec.Sources {
		service := createDockerComposeService(source, pipeline.Metadata.Name)
		composeConfig.Services[sanitizeServiceName(source.ID)] = service
	}

	// Process processors
	for _, processor := range pipeline.Spec.Processors {
		service := createDockerComposeService(processor, pipeline.Metadata.Name)
		
		// Add dependencies
		for _, input := range processor.Inputs {
			service.DependsOn = append(service.DependsOn, sanitizeServiceName(input))
		}
		
		composeConfig.Services[sanitizeServiceName(processor.ID)] = service
	}

	// Process sinks
	for _, sink := range pipeline.Spec.Sinks {
		service := createDockerComposeService(sink, pipeline.Metadata.Name)
		
		// Add dependencies
		for _, input := range sink.Inputs {
			service.DependsOn = append(service.DependsOn, sanitizeServiceName(input))
		}
		
		composeConfig.Services[sanitizeServiceName(sink.ID)] = service
	}

	// Generate and write the YAML
	data, err := yaml.Marshal(composeConfig)
	if err != nil {
		logger.Error("Failed to marshal Docker Compose config", zap.Error(err))
		return fmt.Errorf("failed to generate Docker Compose YAML: %w", err)
	}

	_, err = writer.Write(data)
	if err != nil {
		logger.Error("Failed to write Docker Compose config", zap.Error(err))
		return fmt.Errorf("failed to write Docker Compose YAML: %w", err)
	}

	logger.Info("Successfully generated Docker Compose file",
		zap.String("pipeline", pipeline.Metadata.Name),
		zap.Int("services", len(composeConfig.Services)))
	return nil
}

// createDockerComposeService creates a Docker Compose service from a pipeline component
func createDockerComposeService(component models.Component, pipelineName string) DockerComposeService {
	service := DockerComposeService{
		Image:         component.Image,
		ContainerName: fmt.Sprintf("flow-%s-%s", pipelineName, component.ID),
		Restart:       "unless-stopped",
		Networks:      []string{"flow-network"},
		Environment:   make(map[string]string),
		Labels: map[string]string{
			"com.obsrvr.flow.pipeline":   pipelineName,
			"com.obsrvr.flow.component":  component.ID,
			"com.obsrvr.flow.type":       component.Type,
		},
	}

	// Add environment variables for configuration
	if component.Config != nil {
		for key, value := range component.Config {
			envKey := fmt.Sprintf("FLOW_CONFIG_%s", strings.ToUpper(key))
			service.Environment[envKey] = fmt.Sprintf("%v", value)
		}
	}

	// Add any explicit environment variables
	if component.Env != nil {
		for key, value := range component.Env {
			service.Environment[key] = value
		}
	}

	// Add component ID for service discovery
	service.Environment["FLOW_COMPONENT_ID"] = component.ID

	// Process ports
	if len(component.Ports) > 0 {
		for _, port := range component.Ports {
			portStr := ""
			if port.HostPort > 0 {
				portStr = fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort)
			} else {
				portStr = fmt.Sprintf("%d", port.ContainerPort)
			}
			
			if port.Protocol != "" {
				portStr = fmt.Sprintf("%s/%s", portStr, port.Protocol)
			}
			
			service.Ports = append(service.Ports, portStr)
		}
	}

	// Process volumes
	if len(component.Volumes) > 0 {
		for _, vol := range component.Volumes {
			if vol.HostPath != "" {
				// Named volume or bind mount
				volumeStr := fmt.Sprintf("%s:%s", vol.HostPath, vol.MountPath)
				service.Volumes = append(service.Volumes, volumeStr)
			} else {
				// Reference to a named volume
				volumeStr := fmt.Sprintf("%s:%s", vol.Name, vol.MountPath)
				service.Volumes = append(service.Volumes, volumeStr)
			}
		}
	}

	// Always add the shared data volume
	service.Volumes = append(service.Volumes, "flow-data:/flow/data")

	return service
}

// sanitizeServiceName sanitizes a component ID to be used as a Docker Compose service name
func sanitizeServiceName(id string) string {
	// Replace invalid characters with underscores
	return strings.ReplaceAll(id, "-", "_")
}
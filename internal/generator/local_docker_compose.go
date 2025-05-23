package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// DockerComposeLocalGenerator implements Generator for local execution using Docker Compose profiles
type DockerComposeLocalGenerator struct{}

// NewLocalGenerator creates a new local execution generator using Docker Compose
func NewLocalGenerator() *DockerComposeLocalGenerator {
	return &DockerComposeLocalGenerator{}
}

// DockerComposeServiceWithProfiles extends DockerComposeService to add profile support
type DockerComposeServiceWithProfiles struct {
	DockerComposeService
	Profiles []string `yaml:"profiles,omitempty"`
}

// LocalExecutionOptions contains runtime options for local execution
type LocalExecutionOptions struct {
	LogDirectory string
	EnvFile      string
}

// DockerComposeConfigWithProfiles extends DockerComposeConfig to use services with profiles
type DockerComposeConfigWithProfiles struct {
	Version  string                                `yaml:"version"`
	Services map[string]DockerComposeServiceWithProfiles `yaml:"services"`
	Networks map[string]DockerComposeNetwork       `yaml:"networks,omitempty"`
	Volumes  map[string]interface{}                `yaml:"volumes,omitempty"`
}

// Generate produces Docker Compose configuration for local execution
func (g *DockerComposeLocalGenerator) Generate(pipeline *model.Pipeline, opts model.TranslationOptions) ([]byte, error) {
	logger.Debug("Generating Docker Compose configuration for local execution")

	// Validate the pipeline for local execution
	if err := g.Validate(pipeline); err != nil {
		return nil, err
	}

	// Extract or generate pipeline name
	pipelineName := opts.ResourcePrefix
	if pipelineName == "" {
		pipelineName = pipeline.Metadata.Name
		if pipelineName == "" {
			pipelineName = "flowctl-pipeline"
		}
	}

	// Create Docker Compose config
	composeConfig := DockerComposeConfigWithProfiles{
		Version:  "3.8", // Using a more modern version for profile support
		Services: make(map[string]DockerComposeServiceWithProfiles),
		Networks: map[string]DockerComposeNetwork{
			"local-network": {
				External: false,
			},
		},
		Volumes: map[string]interface{}{
			"local-data": map[string]interface{}{
				"labels": map[string]string{
					"com.obsrvr.flow.pipeline": pipelineName,
					"com.obsrvr.flow.local": "true",
				},
			},
			"local-logs": map[string]interface{}{
				"labels": map[string]string{
					"com.obsrvr.flow.pipeline": pipelineName,
					"com.obsrvr.flow.local": "true",
				},
			},
		},
	}

	// Create a logs directory mapping
	logsDir := "logs/" + pipelineName
	if opts.OutputPath != "" {
		logsDir = filepath.Join(filepath.Dir(opts.OutputPath), "logs", pipelineName)
	}

	// Local execution options
	localOpts := LocalExecutionOptions{
		LogDirectory: logsDir,
		EnvFile:      ".env." + pipelineName,
	}

	// Process sources
	for _, src := range pipeline.Spec.Sources {
		service := createLocalDockerComposeService(src, pipelineName, localOpts, "source")
		composeConfig.Services[sanitizeLocalServiceName(src.ID)] = service
	}

	// Process processors
	for _, proc := range pipeline.Spec.Processors {
		service := createLocalDockerComposeService(proc, pipelineName, localOpts, "processor")
		
		// Add dependencies
		if len(proc.Inputs) > 0 {
			service.DependsOn = make([]string, len(proc.Inputs))
			for i, input := range proc.Inputs {
				service.DependsOn[i] = sanitizeLocalServiceName(input)
			}
		}
		
		composeConfig.Services[sanitizeLocalServiceName(proc.ID)] = service
	}

	// Process sinks
	for _, sink := range pipeline.Spec.Sinks {
		service := createLocalDockerComposeService(sink, pipelineName, localOpts, "sink")
		
		// Add dependencies
		if len(sink.Inputs) > 0 {
			service.DependsOn = make([]string, len(sink.Inputs))
			for i, input := range sink.Inputs {
				service.DependsOn[i] = sanitizeLocalServiceName(input)
			}
		}
		
		composeConfig.Services[sanitizeLocalServiceName(sink.ID)] = service
	}

	// Generate combined environment file with global and component-specific variables
	envFilePath := localOpts.EnvFile
	if opts.OutputPath != "" {
		envFilePath = filepath.Join(filepath.Dir(opts.OutputPath), localOpts.EnvFile)
	}
	
	if err := generateEnvFile(pipeline, pipelineName, envFilePath, localOpts); err != nil {
		logger.Warn("Failed to write environment file", zap.Error(err))
	}

	// Generate the Docker Compose YAML
	composeData, err := yaml.Marshal(composeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Docker Compose config: %w", err)
	}

	// Add helpful comments to the YAML output
	headerComments := fmt.Sprintf(`# Docker Compose configuration for local execution
# Generated by flowctl
#
# Pipeline: %s
#
# Usage:
# - Start all components: docker compose --profile local up -d
# - View logs: docker compose logs -f
# - Stop all components: docker compose down
#
# Logs directory: %s
# Environment file: %s
#
`, pipelineName, logsDir, localOpts.EnvFile)

	result := headerComments + string(composeData)
	return []byte(result), nil
}

// Validate checks if the pipeline can be translated to local execution
func (g *DockerComposeLocalGenerator) Validate(pipeline *model.Pipeline) error {
	logger.Debug("Validating pipeline for local execution")

	// Check basic requirements
	if len(pipeline.Spec.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}

	// Check component-specific requirements
	for _, src := range pipeline.Spec.Sources {
		if src.ID == "" {
			return fmt.Errorf("source id is required")
		}
		if src.Image == "" && len(src.Command) == 0 {
			return fmt.Errorf("source %s must have either an image or a command", src.ID)
		}
	}

	for _, proc := range pipeline.Spec.Processors {
		if proc.ID == "" {
			return fmt.Errorf("processor id is required")
		}
		if proc.Image == "" && len(proc.Command) == 0 {
			return fmt.Errorf("processor %s must have either an image or a command", proc.ID)
		}
		if len(proc.Inputs) == 0 {
			return fmt.Errorf("processor %s must have at least one input", proc.ID)
		}
	}

	for _, sink := range pipeline.Spec.Sinks {
		if sink.ID == "" {
			return fmt.Errorf("sink id is required")
		}
		if sink.Image == "" && len(sink.Command) == 0 {
			return fmt.Errorf("sink %s must have either an image or a command", sink.ID)
		}
		if len(sink.Inputs) == 0 {
			return fmt.Errorf("sink %s must have at least one input", sink.ID)
		}
	}

	return nil
}

// createLocalDockerComposeService creates a Docker Compose service for local execution
func createLocalDockerComposeService(component model.Component, pipelineName string, 
	opts LocalExecutionOptions, componentType string) DockerComposeServiceWithProfiles {

	sanitizedID := sanitizeLocalServiceName(component.ID)
	containerName := fmt.Sprintf("local-%s-%s", pipelineName, sanitizedID)
	
	// Create base service
	service := DockerComposeServiceWithProfiles{
		DockerComposeService: DockerComposeService{
			Image:         component.Image,
			ContainerName: containerName,
			Command:       component.Command,
			Restart:       "unless-stopped",
			Networks:      []string{"local-network"},
			Environment:   make(map[string]string),
			Labels: map[string]string{
				"com.obsrvr.flow.pipeline":  pipelineName,
				"com.obsrvr.flow.component": component.ID,
				"com.obsrvr.flow.type":      componentType,
				"com.obsrvr.flow.local":     "true",
			},
		},
		Profiles:    []string{"local"},
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

	// Add component ID and type for service discovery
	service.Environment["FLOW_COMPONENT_ID"] = component.ID
	service.Environment["FLOW_COMPONENT_TYPE"] = componentType

	// Add pipeline name
	service.Environment["FLOW_PIPELINE_NAME"] = pipelineName

	// Create log volume mapping
	logPath := filepath.Join("/var/log/flowctl", pipelineName)
	logFile := fmt.Sprintf("%s/%s.log", logPath, sanitizedID)
	service.Volumes = append(service.Volumes, fmt.Sprintf("local-logs:%s", logPath))

	// Set up container logging
	service.Environment["FLOW_LOG_FILE"] = logFile

	// Process ports
	if len(component.Ports) > 0 {
		for _, port := range component.Ports {
			portStr := ""
			if port.HostPort > 0 {
				portStr = fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort)
			} else {
				portStr = fmt.Sprintf("%d:%d", port.ContainerPort, port.ContainerPort)
			}
			
			if port.Protocol != "" && strings.ToLower(port.Protocol) != "tcp" {
				portStr = fmt.Sprintf("%s/%s", portStr, strings.ToLower(port.Protocol))
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

	// Add shared data volume
	service.Volumes = append(service.Volumes, "local-data:/flow/data")

	// Add health check if specified
	if component.HealthCheck != "" {
		healthPort := component.HealthPort
		
		// Default to a reasonable port if not specified
		if healthPort == 0 {
			// Try to use the first port if available
			if len(component.Ports) > 0 {
				healthPort = component.Ports[0].ContainerPort
			} else {
				healthPort = 8080 // Default fallback
			}
		}

		// Format health check URL
		healthCheckUrl := component.HealthCheck
		if !strings.HasPrefix(healthCheckUrl, "http") {
			healthCheckUrl = fmt.Sprintf("http://localhost:%d%s", healthPort, healthCheckUrl)
		}

		service.HealthCheck = &DockerHealthCheck{
			Test:        []string{"CMD-SHELL", fmt.Sprintf("curl -f %s || exit 1", healthCheckUrl)},
			Interval:    "10s",
			Timeout:     "5s",
			Retries:     3,
			StartPeriod: "20s",
		}
	}

	return service
}

// sanitizeLocalServiceName sanitizes a component ID to be used as a Docker Compose service name
// This version uses underscores instead of dashes for local generator
func sanitizeLocalServiceName(id string) string {
	// Replace invalid characters with underscores
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, id)
	
	// Ensure the name starts with a letter or underscore
	if len(sanitized) > 0 && !((sanitized[0] >= 'a' && sanitized[0] <= 'z') || (sanitized[0] >= 'A' && sanitized[0] <= 'Z') || sanitized[0] == '_') {
		sanitized = "_" + sanitized
	}
	
	return sanitized
}

// generateEnvFile generates an environment file for the pipeline
func generateEnvFile(pipeline *model.Pipeline, pipelineName, envFilePath string, opts LocalExecutionOptions) error {
	// Create environment variables map
	envVars := map[string]string{
		"FLOW_PIPELINE_NAME": pipelineName,
		"FLOW_LOG_DIR": opts.LogDirectory,
	}
	
	// Add component-specific environment variables
	for _, src := range pipeline.Spec.Sources {
		prefix := fmt.Sprintf("FLOW_SOURCE_%s_", strings.ToUpper(sanitizeLocalServiceName(src.ID)))
		envVars[prefix+"ID"] = src.ID
		
		if src.Env != nil {
			for key, value := range src.Env {
				envVars[key] = value
			}
		}
	}
	
	for _, proc := range pipeline.Spec.Processors {
		prefix := fmt.Sprintf("FLOW_PROCESSOR_%s_", strings.ToUpper(sanitizeLocalServiceName(proc.ID)))
		envVars[prefix+"ID"] = proc.ID
		
		if proc.Env != nil {
			for key, value := range proc.Env {
				envVars[key] = value
			}
		}
	}
	
	for _, sink := range pipeline.Spec.Sinks {
		prefix := fmt.Sprintf("FLOW_SINK_%s_", strings.ToUpper(sanitizeLocalServiceName(sink.ID)))
		envVars[prefix+"ID"] = sink.ID
		
		if sink.Env != nil {
			for key, value := range sink.Env {
				envVars[key] = value
			}
		}
	}
	
	// Build env file content
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Environment variables for %s\n", pipelineName))
	sb.WriteString("# Generated by flowctl\n\n")
	
	sb.WriteString("# Global environment variables\n")
	for key, value := range envVars {
		sb.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(envFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for env file: %w", err)
	}
	
	// Write env file
	if err := os.WriteFile(envFilePath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}
	
	return nil
}
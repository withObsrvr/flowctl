package translator

import (
	"fmt"
	"strings"

	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"gopkg.in/yaml.v3"
)

// Generator is the interface for generating target-specific output
type Generator interface {
	// Generate produces configuration for the target platform
	Generate(cfg *config.Config, opts TranslationOptions) ([]byte, error)

	// Validate checks if the config can be translated to this format
	Validate(cfg *config.Config) error
}

// DockerComposeGenerator generates Docker Compose configuration
type DockerComposeGenerator struct{}

// DockerComposeConfig represents the structure of a Docker Compose file
type DockerComposeConfig struct {
	Version  string                          `yaml:"version"`
	Services map[string]DockerComposeService `yaml:"services"`
	Networks map[string]DockerComposeNetwork `yaml:"networks,omitempty"`
	Volumes  map[string]interface{}          `yaml:"volumes,omitempty"`
}

// DockerComposeService represents a service in Docker Compose
type DockerComposeService struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Command       []string          `yaml:"command,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	DependsOn     []string          `yaml:"depends_on,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
	HealthCheck   *DockerHealthCheck `yaml:"healthcheck,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
}

// DockerHealthCheck represents a Docker health check configuration
type DockerHealthCheck struct {
	Test        []string `yaml:"test"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
}

// DockerComposeNetwork represents a network in Docker Compose
type DockerComposeNetwork struct {
	External bool `yaml:"external,omitempty"`
}

// Generate produces Docker Compose configuration
func (g *DockerComposeGenerator) Generate(cfg *config.Config, opts TranslationOptions) ([]byte, error) {
	logger.Debug("Generating Docker Compose configuration")

	// Validate the config for Docker Compose compatibility
	if err := g.Validate(cfg); err != nil {
		return nil, err
	}

	// Initialize Docker Compose configuration
	composeConfig := DockerComposeConfig{
		Version:  "3.8", // Use a recent version
		Services: make(map[string]DockerComposeService),
		Networks: map[string]DockerComposeNetwork{
			"pipeline": {External: false}, // Default pipeline network
		},
	}

	// Add source as a service
	sourceName := sanitizeServiceName(cfg.Source.Type)
	sourceService := DockerComposeService{
		Image:         getImage(cfg.Source.Params, opts.RegistryPrefix),
		ContainerName: fmt.Sprintf("%s-%s", opts.ResourcePrefix, sourceName),
		Environment:   getEnvironment(cfg.Source.Params, opts.EnvVars),
		Networks:      []string{"pipeline"},
		Restart:       "unless-stopped",
		Labels: map[string]string{
			"com.obsrvr.component": "source",
			"com.obsrvr.type":      cfg.Source.Type,
		},
	}

	// Apply common source parameters
	applyCommonParameters(&sourceService, cfg.Source.Params)

	composeConfig.Services[sourceName] = sourceService

	// Add processors as services
	for i, proc := range cfg.Processors {
		procName := sanitizeServiceName(proc.Name)
		procService := DockerComposeService{
			Image:         getImage(proc.Params, opts.RegistryPrefix),
			ContainerName: fmt.Sprintf("%s-%s", opts.ResourcePrefix, procName),
			Environment:   getEnvironment(proc.Params, opts.EnvVars),
			Networks:      []string{"pipeline"},
			DependsOn:     []string{sourceName}, // Processors depend on source
			Restart:       "unless-stopped",
			Labels: map[string]string{
				"com.obsrvr.component": "processor",
				"com.obsrvr.type":      proc.Plugin,
				"com.obsrvr.order":     fmt.Sprintf("%d", i),
			},
		}

		// Apply common processor parameters
		applyCommonParameters(&procService, proc.Params)

		composeConfig.Services[procName] = procService
	}

	// Add sink as a service
	sinkName := sanitizeServiceName(cfg.Sink.Type)
	sinkService := DockerComposeService{
		Image:         getImage(cfg.Sink.Params, opts.RegistryPrefix),
		ContainerName: fmt.Sprintf("%s-%s", opts.ResourcePrefix, sinkName),
		Environment:   getEnvironment(cfg.Sink.Params, opts.EnvVars),
		Networks:      []string{"pipeline"},
		DependsOn:     getProcessorNames(cfg.Processors), // Sink depends on all processors
		Restart:       "unless-stopped",
		Labels: map[string]string{
			"com.obsrvr.component": "sink",
			"com.obsrvr.type":      cfg.Sink.Type,
		},
	}

	// Apply common sink parameters
	applyCommonParameters(&sinkService, cfg.Sink.Params)

	composeConfig.Services[sinkName] = sinkService

	// Marshal the configuration to YAML
	data, err := yaml.Marshal(composeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Docker Compose configuration: %w", err)
	}

	return data, nil
}

// Validate checks if the config can be translated to Docker Compose
func (g *DockerComposeGenerator) Validate(cfg *config.Config) error {
	logger.Debug("Validating configuration for Docker Compose compatibility")

	// Basic validation that pipeline components are defined
	if cfg.Source.Type == "" {
		return fmt.Errorf("source type must be specified")
	}

	if len(cfg.Processors) == 0 {
		return fmt.Errorf("at least one processor must be defined")
	}

	if cfg.Sink.Type == "" {
		return fmt.Errorf("sink type must be specified")
	}

	return nil
}

// Helper functions

// sanitizeServiceName sanitizes a name to be used as a Docker Compose service name
func sanitizeServiceName(name string) string {
	// Replace any non-alphanumeric character with a dash
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)

	// Ensure name starts with a letter
	if len(sanitized) > 0 && !((sanitized[0] >= 'a' && sanitized[0] <= 'z') || (sanitized[0] >= 'A' && sanitized[0] <= 'Z')) {
		sanitized = "svc-" + sanitized
	}

	return sanitized
}

// getImage gets the image from parameters or defaults to a placeholder
func getImage(params map[string]interface{}, registryPrefix string) string {
	if img, ok := params["image"].(string); ok && img != "" {
		// Add registry prefix if provided
		if registryPrefix != "" && !strings.Contains(img, "/") {
			return registryPrefix + "/" + img
		}
		return img
	}
	return "busybox:latest" // Default placeholder image
}

// getEnvironment combines specific environment variables with global ones
func getEnvironment(params map[string]interface{}, globalEnv map[string]string) map[string]string {
	env := make(map[string]string)

	// Copy global environment variables
	for k, v := range globalEnv {
		env[k] = v
	}

	// Add component-specific environment variables
	if envMap, ok := params["env"].(map[string]interface{}); ok {
		for k, v := range envMap {
			if strVal, ok := v.(string); ok {
				env[k] = strVal
			} else {
				// Convert other types to string
				env[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return env
}

// getProcessorNames returns a list of processor service names
func getProcessorNames(processors []config.ProcessorConfig) []string {
	names := make([]string, len(processors))
	for i, p := range processors {
		names[i] = sanitizeServiceName(p.Name)
	}
	return names
}

// applyCommonParameters applies common parameters to a service
func applyCommonParameters(service *DockerComposeService, params map[string]interface{}) {
	// Handle command
	if cmd, ok := params["command"].([]interface{}); ok && len(cmd) > 0 {
		service.Command = make([]string, len(cmd))
		for i, c := range cmd {
			service.Command[i] = fmt.Sprintf("%v", c)
		}
	}

	// Handle ports
	if ports, ok := params["ports"].([]interface{}); ok && len(ports) > 0 {
		service.Ports = make([]string, len(ports))
		for i, p := range ports {
			if port, ok := p.(int); ok {
				service.Ports[i] = fmt.Sprintf("%d:%d", port, port)
			} else {
				service.Ports[i] = fmt.Sprintf("%v", p)
			}
		}
	}

	// Handle volumes
	if vols, ok := params["volumes"].([]interface{}); ok && len(vols) > 0 {
		service.Volumes = make([]string, len(vols))
		for i, v := range vols {
			service.Volumes[i] = fmt.Sprintf("%v", v)
		}
	}

	// Handle health check
	if health, ok := params["health_check"].(string); ok && health != "" {
		service.HealthCheck = &DockerHealthCheck{
			Test:        []string{"CMD", "wget", "--quiet", "--tries=1", "--spider", health},
			Interval:    "10s",
			Timeout:     "5s",
			Retries:     3,
			StartPeriod: "5s",
		}
	}
}

// KubernetesGenerator generates Kubernetes manifests
// This is a placeholder for future implementation
type KubernetesGenerator struct{}

// Generate produces Kubernetes manifests
func (g *KubernetesGenerator) Generate(cfg *config.Config, opts TranslationOptions) ([]byte, error) {
	return nil, fmt.Errorf("Kubernetes generator not yet implemented")
}

// Validate checks if the config can be translated to Kubernetes
func (g *KubernetesGenerator) Validate(cfg *config.Config) error {
	return fmt.Errorf("Kubernetes validation not yet implemented")
}

// NomadGenerator generates Nomad job specifications
// This is a placeholder for future implementation
type NomadGenerator struct{}

// Generate produces Nomad job specifications
func (g *NomadGenerator) Generate(cfg *config.Config, opts TranslationOptions) ([]byte, error) {
	return nil, fmt.Errorf("Nomad generator not yet implemented")
}

// Validate checks if the config can be translated to Nomad
func (g *NomadGenerator) Validate(cfg *config.Config) error {
	return fmt.Errorf("Nomad validation not yet implemented")
}
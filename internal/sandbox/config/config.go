package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

// SandboxConfig represents the configuration for sandbox services
type SandboxConfig struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

// ServiceConfig represents configuration for a single service
type ServiceConfig struct {
	Image       string            `yaml:"image"`
	Ports       []PortMapping     `yaml:"ports,omitempty"`
	Environment map[string]string `yaml:"env,omitempty"`
	Volumes     []VolumeMapping   `yaml:"volumes,omitempty"`
	Command     []string          `yaml:"command,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	HealthCheck *HealthCheck      `yaml:"health_check,omitempty"`
	Build       *BuildConfig      `yaml:"build,omitempty"`
}

// PortMapping represents a port mapping between host and container
type PortMapping struct {
	Host      int    `yaml:"host"`
	Container int    `yaml:"container"`
	Protocol  string `yaml:"protocol,omitempty"`
}

// VolumeMapping represents a volume mount
type VolumeMapping struct {
	Host      string `yaml:"host"`
	Container string `yaml:"container"`
	ReadOnly  bool   `yaml:"readonly,omitempty"`
}

// HealthCheck represents a health check configuration
type HealthCheck struct {
	Test        []string `yaml:"test"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
}

// BuildConfig represents build configuration for custom images
type BuildConfig struct {
	Context    string            `yaml:"context"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Args       map[string]string `yaml:"args,omitempty"`
	Target     string            `yaml:"target,omitempty"`
}

// LoadSandboxConfig loads sandbox configuration from a YAML file
func LoadSandboxConfig(configPath string) (*SandboxConfig, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return getDefaultConfig(), nil
	}

	// Read file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config SandboxConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// getDefaultConfig returns a default sandbox configuration
func getDefaultConfig() *SandboxConfig {
	return &SandboxConfig{
		Services: map[string]ServiceConfig{
			"redis": {
				Image: "redis:7",
				Ports: []PortMapping{
					{Host: 6379, Container: 6379},
				},
			},
		},
	}
}

// validateConfig validates the sandbox configuration
func validateConfig(config *SandboxConfig) error {
	if config.Services == nil || len(config.Services) == 0 {
		return fmt.Errorf("no services defined")
	}

	for name, service := range config.Services {
		if service.Image == "" && service.Build == nil {
			return fmt.Errorf("service %s: either image or build must be specified", name)
		}

		// Validate port mappings
		for _, port := range service.Ports {
			if port.Host <= 0 || port.Container <= 0 {
				return fmt.Errorf("service %s: invalid port mapping %d:%d", name, port.Host, port.Container)
			}
		}

		// Validate volume mappings
		for _, volume := range service.Volumes {
			if volume.Host == "" || volume.Container == "" {
				return fmt.Errorf("service %s: invalid volume mapping %s:%s", name, volume.Host, volume.Container)
			}
		}

		// Validate dependencies
		for _, dep := range service.DependsOn {
			if _, exists := config.Services[dep]; !exists {
				return fmt.Errorf("service %s: dependency %s not found", name, dep)
			}
		}
	}

	return nil
}
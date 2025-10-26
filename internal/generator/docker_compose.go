package generator

import (
	"fmt"
	"strings"

	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"gopkg.in/yaml.v3"
)

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

// NewDockerComposeGenerator creates a new Docker Compose generator
func NewDockerComposeGenerator() *DockerComposeGenerator {
	return &DockerComposeGenerator{}
}

// Generate produces Docker Compose configuration
func (g *DockerComposeGenerator) Generate(pipeline *model.Pipeline, opts model.TranslationOptions) ([]byte, error) {
	logger.Debug("Generating Docker Compose configuration")

	// Validate the pipeline for Docker Compose compatibility
	if err := g.Validate(pipeline); err != nil {
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

	// Add sources as services
	for _, src := range pipeline.Spec.Sources {
		serviceName := sanitizeServiceName(src.ID)
		service := DockerComposeService{
			Image:         getImageRef(src.Image, opts.RegistryPrefix),
			ContainerName: getContainerName(serviceName, opts.ResourcePrefix),
			Command:       src.Command,
			Environment:   mergeEnvs(src.Env, opts.EnvVars),
			Networks:      []string{"pipeline"},
			Restart:       "unless-stopped",
			Labels: map[string]string{
				"com.obsrvr.component": "source",
				"com.obsrvr.pipeline": pipeline.Metadata.Name,
			},
		}

		// Add labels from pipeline metadata
		for k, v := range pipeline.Metadata.Labels {
			service.Labels[k] = v
		}

		// Add ports
		if len(src.Ports) > 0 {
			service.Ports = make([]string, len(src.Ports))
			for i, p := range src.Ports {
				if p.HostPort > 0 {
					service.Ports[i] = fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort)
				} else {
					service.Ports[i] = fmt.Sprintf("%d:%d", p.ContainerPort, p.ContainerPort)
				}
			}
		}

		// Add volumes
		if len(src.Volumes) > 0 {
			service.Volumes = make([]string, len(src.Volumes))
			for i, v := range src.Volumes {
				if v.HostPath != "" {
					service.Volumes[i] = fmt.Sprintf("%s:%s", v.HostPath, v.MountPath)
				} else {
					service.Volumes[i] = fmt.Sprintf("%s:%s", v.Name, v.MountPath)
				}
			}
		}

		// Add health check
		if src.HealthCheck != "" {
			service.HealthCheck = &DockerHealthCheck{
				Test:        []string{"CMD", "wget", "--quiet", "--tries=1", "--spider", src.HealthCheck},
				Interval:    "10s",
				Timeout:     "5s",
				Retries:     3,
				StartPeriod: "5s",
			}
		}

		composeConfig.Services[serviceName] = service
	}

	// Add processors as services
	for i, proc := range pipeline.Spec.Processors {
		serviceName := sanitizeServiceName(proc.ID)
		service := DockerComposeService{
			Image:         getImageRef(proc.Image, opts.RegistryPrefix),
			ContainerName: getContainerName(serviceName, opts.ResourcePrefix),
			Command:       proc.Command,
			Environment:   mergeEnvs(proc.Env, opts.EnvVars),
			Networks:      []string{"pipeline"},
			Restart:       "unless-stopped",
			Labels: map[string]string{
				"com.obsrvr.component": "processor",
				"com.obsrvr.pipeline": pipeline.Metadata.Name,
				"com.obsrvr.order":     fmt.Sprintf("%d", i),
			},
		}

		// Set dependencies based on inputs
		if len(proc.Inputs) > 0 {
			service.DependsOn = make([]string, len(proc.Inputs))
			for i, input := range proc.Inputs {
				service.DependsOn[i] = sanitizeServiceName(input)
			}
		} else {
			// Default to depending on all sources
			service.DependsOn = make([]string, len(pipeline.Spec.Sources))
			for i, src := range pipeline.Spec.Sources {
				service.DependsOn[i] = sanitizeServiceName(src.ID)
			}
		}

		// Add labels from pipeline metadata
		for k, v := range pipeline.Metadata.Labels {
			service.Labels[k] = v
		}

		// Add ports
		if len(proc.Ports) > 0 {
			service.Ports = make([]string, len(proc.Ports))
			for i, p := range proc.Ports {
				if p.HostPort > 0 {
					service.Ports[i] = fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort)
				} else {
					service.Ports[i] = fmt.Sprintf("%d:%d", p.ContainerPort, p.ContainerPort)
				}
			}
		}

		// Add volumes
		if len(proc.Volumes) > 0 {
			service.Volumes = make([]string, len(proc.Volumes))
			for i, v := range proc.Volumes {
				if v.HostPath != "" {
					service.Volumes[i] = fmt.Sprintf("%s:%s", v.HostPath, v.MountPath)
				} else {
					service.Volumes[i] = fmt.Sprintf("%s:%s", v.Name, v.MountPath)
				}
			}
		}

		// Add health check
		if proc.HealthCheck != "" {
			service.HealthCheck = &DockerHealthCheck{
				Test:        []string{"CMD", "wget", "--quiet", "--tries=1", "--spider", proc.HealthCheck},
				Interval:    "10s",
				Timeout:     "5s",
				Retries:     3,
				StartPeriod: "5s",
			}
		}

		composeConfig.Services[serviceName] = service
	}

	// Add sinks as services
	for _, sink := range pipeline.Spec.Sinks {
		serviceName := sanitizeServiceName(sink.ID)
		service := DockerComposeService{
			Image:         getImageRef(sink.Image, opts.RegistryPrefix),
			ContainerName: getContainerName(serviceName, opts.ResourcePrefix),
			Command:       sink.Command,
			Environment:   mergeEnvs(sink.Env, opts.EnvVars),
			Networks:      []string{"pipeline"},
			Restart:       "unless-stopped",
			Labels: map[string]string{
				"com.obsrvr.component": "sink",
				"com.obsrvr.pipeline": pipeline.Metadata.Name,
			},
		}

		// Set dependencies based on inputs
		if len(sink.Inputs) > 0 {
			service.DependsOn = make([]string, len(sink.Inputs))
			for i, input := range sink.Inputs {
				service.DependsOn[i] = sanitizeServiceName(input)
			}
		} else {
			// Default to depending on all processors, or sources if no processors
			if len(pipeline.Spec.Processors) > 0 {
				service.DependsOn = make([]string, len(pipeline.Spec.Processors))
				for i, proc := range pipeline.Spec.Processors {
					service.DependsOn[i] = sanitizeServiceName(proc.ID)
				}
			} else {
				service.DependsOn = make([]string, len(pipeline.Spec.Sources))
				for i, src := range pipeline.Spec.Sources {
					service.DependsOn[i] = sanitizeServiceName(src.ID)
				}
			}
		}

		// Add labels from pipeline metadata
		for k, v := range pipeline.Metadata.Labels {
			service.Labels[k] = v
		}

		// Add ports
		if len(sink.Ports) > 0 {
			service.Ports = make([]string, len(sink.Ports))
			for i, p := range sink.Ports {
				if p.HostPort > 0 {
					service.Ports[i] = fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort)
				} else {
					service.Ports[i] = fmt.Sprintf("%d:%d", p.ContainerPort, p.ContainerPort)
				}
			}
		}

		// Add volumes
		if len(sink.Volumes) > 0 {
			service.Volumes = make([]string, len(sink.Volumes))
			for i, v := range sink.Volumes {
				if v.HostPath != "" {
					service.Volumes[i] = fmt.Sprintf("%s:%s", v.HostPath, v.MountPath)
				} else {
					service.Volumes[i] = fmt.Sprintf("%s:%s", v.Name, v.MountPath)
				}
			}
		}

		// Add health check
		if sink.HealthCheck != "" {
			service.HealthCheck = &DockerHealthCheck{
				Test:        []string{"CMD", "wget", "--quiet", "--tries=1", "--spider", sink.HealthCheck},
				Interval:    "10s",
				Timeout:     "5s",
				Retries:     3,
				StartPeriod: "5s",
			}
		}

		composeConfig.Services[serviceName] = service
	}

	// Add pipelines as services
	for _, pipelineComp := range pipeline.Spec.Pipelines {
		serviceName := sanitizeServiceName(pipelineComp.ID)
		service := DockerComposeService{
			Image:         getImageRef(pipelineComp.Image, opts.RegistryPrefix),
			ContainerName: getContainerName(serviceName, opts.ResourcePrefix),
			Command:       pipelineComp.Command,
			Environment:   mergeEnvs(pipelineComp.Env, opts.EnvVars),
			Networks:      []string{"pipeline"},
			Restart:       getRestartPolicy(pipelineComp.RestartPolicy, "pipeline"),
			Labels: map[string]string{
				"com.obsrvr.component": "pipeline",
				"com.obsrvr.pipeline": pipeline.Metadata.Name,
			},
		}

		// Add args to command if specified
		if len(pipelineComp.Args) > 0 {
			if service.Command == nil {
				service.Command = pipelineComp.Args
			} else {
				service.Command = append(service.Command, pipelineComp.Args...)
			}
		}

		// Set dependencies based on DependsOn field
		if len(pipelineComp.DependsOn) > 0 {
			service.DependsOn = make([]string, len(pipelineComp.DependsOn))
			for i, dep := range pipelineComp.DependsOn {
				service.DependsOn[i] = sanitizeServiceName(dep)
			}
		}

		// Add labels from pipeline metadata
		for k, v := range pipeline.Metadata.Labels {
			service.Labels[k] = v
		}

		// Add ports
		if len(pipelineComp.Ports) > 0 {
			service.Ports = make([]string, len(pipelineComp.Ports))
			for i, p := range pipelineComp.Ports {
				if p.HostPort > 0 {
					service.Ports[i] = fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort)
				} else {
					service.Ports[i] = fmt.Sprintf("%d:%d", p.ContainerPort, p.ContainerPort)
				}
			}
		}

		// Add volumes
		if len(pipelineComp.Volumes) > 0 {
			service.Volumes = make([]string, len(pipelineComp.Volumes))
			for i, v := range pipelineComp.Volumes {
				volString := ""
				if v.HostPath != "" {
					// Bind mount: prefer ContainerPath, fall back to MountPath
					targetPath := v.ContainerPath
					if targetPath == "" {
						targetPath = v.MountPath
					}
					volString = fmt.Sprintf("%s:%s", v.HostPath, targetPath)
				} else if v.MountPath != "" || v.ContainerPath != "" {
					// Named volume: prefer MountPath, fall back to ContainerPath
					targetPath := v.MountPath
					if targetPath == "" {
						targetPath = v.ContainerPath
					}
					volString = fmt.Sprintf("%s:%s", v.Name, targetPath)
				}

				if v.ReadOnly {
					volString += ":ro"
				}

				if volString != "" {
					service.Volumes[i] = volString
				}
			}
		}

		// Add health check
		if pipelineComp.HealthCheck != "" {
			service.HealthCheck = &DockerHealthCheck{
				Test:        []string{"CMD", "wget", "--quiet", "--tries=1", "--spider", pipelineComp.HealthCheck},
				Interval:    "10s",
				Timeout:     "5s",
				Retries:     3,
				StartPeriod: "5s",
			}
		}

		composeConfig.Services[serviceName] = service
	}

	// Marshal the configuration to YAML
	data, err := yaml.Marshal(composeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Docker Compose configuration: %w", err)
	}

	return data, nil
}

// Validate checks if the pipeline can be translated to Docker Compose
func (g *DockerComposeGenerator) Validate(pipeline *model.Pipeline) error {
	logger.Debug("Validating pipeline for Docker Compose compatibility")

	// Check required fields
	if pipeline.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}

	if pipeline.Kind != "Pipeline" {
		return fmt.Errorf("kind must be 'Pipeline'")
	}

	// At least one component is required (source or pipeline)
	if len(pipeline.Spec.Sources) == 0 && len(pipeline.Spec.Pipelines) == 0 {
		return fmt.Errorf("at least one source or pipeline component is required")
	}

	// Check that all components have required fields
	for _, src := range pipeline.Spec.Sources {
		if src.ID == "" {
			return fmt.Errorf("source id is required")
		}
		if src.Image == "" {
			return fmt.Errorf("source image is required for %s", src.ID)
		}
	}

	for _, proc := range pipeline.Spec.Processors {
		if proc.ID == "" {
			return fmt.Errorf("processor id is required")
		}
		if proc.Image == "" {
			return fmt.Errorf("processor image is required for %s", proc.ID)
		}
	}

	for _, sink := range pipeline.Spec.Sinks {
		if sink.ID == "" {
			return fmt.Errorf("sink id is required")
		}
		if sink.Image == "" {
			return fmt.Errorf("sink image is required for %s", sink.ID)
		}
	}

	// Check pipeline components
	for _, pipelineComp := range pipeline.Spec.Pipelines {
		if pipelineComp.ID == "" {
			return fmt.Errorf("pipeline id is required")
		}
		if pipelineComp.Image == "" {
			return fmt.Errorf("pipeline image is required for %s", pipelineComp.ID)
		}
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

// getImageRef gets the full image reference with registry prefix if needed
func getImageRef(image string, registryPrefix string) string {
	if registryPrefix == "" || strings.Contains(image, "/") {
		return image
	}
	return registryPrefix + "/" + image
}

// getContainerName builds the container name with optional prefix
func getContainerName(serviceName string, prefix string) string {
	if prefix == "" {
		return serviceName
	}
	return fmt.Sprintf("%s-%s", prefix, serviceName)
}

// mergeEnvs merges component-specific and global environment variables
func mergeEnvs(componentEnv, globalEnv map[string]string) map[string]string {
	if len(componentEnv) == 0 && len(globalEnv) == 0 {
		return nil
	}

	merged := make(map[string]string)

	// Copy global environment variables
	for k, v := range globalEnv {
		merged[k] = v
	}

	// Add component-specific variables (overriding global ones)
	for k, v := range componentEnv {
		merged[k] = v
	}

	return merged
}

// getRestartPolicy returns the appropriate restart policy for a component
func getRestartPolicy(policy string, componentType string) string {
	if policy != "" {
		return policy
	}
	// Default restart policies
	if componentType == "pipeline" {
		return "no"  // Pipelines typically run to completion: after the container finishes, it exits and remains stopped; no automatic cleanup or restart occurs
	}
	return "unless-stopped"  // Other components should restart
}
package model

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
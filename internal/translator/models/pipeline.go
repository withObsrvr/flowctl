package models

// Pipeline represents a data processing pipeline with all components
type Pipeline struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

// Metadata contains pipeline metadata
type Metadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// Spec contains the pipeline specification
type Spec struct {
	Description string      `yaml:"description,omitempty"`
	Sources     []Component `yaml:"sources"`
	Processors  []Component `yaml:"processors"`
	Sinks       []Component `yaml:"sinks"`
	Config      map[string]interface{} `yaml:"config,omitempty"`
}

// Component represents a pipeline component (source, processor, or sink)
type Component struct {
	ID      string                 `yaml:"id"`
	Type    string                 `yaml:"type,omitempty"`
	Image   string                 `yaml:"image"`
	Inputs  []string               `yaml:"inputs,omitempty"`
	Config  map[string]interface{} `yaml:"config,omitempty"`
	Env     map[string]string      `yaml:"env,omitempty"`
	Volumes []Volume               `yaml:"volumes,omitempty"`
	Ports   []Port                 `yaml:"ports,omitempty"`
}

// Volume represents a volume mount in a component
type Volume struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
	HostPath  string `yaml:"hostPath,omitempty"`
}

// Port represents a port exposed by a component
type Port struct {
	Name          string `yaml:"name,omitempty"`
	ContainerPort int    `yaml:"containerPort"`
	HostPort      int    `yaml:"hostPort,omitempty"`
	Protocol      string `yaml:"protocol,omitempty"`
}
package model


// Pipeline represents a data processing pipeline with all components
type Pipeline struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
}

// Metadata contains pipeline metadata
type Metadata struct {
	Name        string            `yaml:"name" json:"name"`
	Namespace   string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// Spec contains the pipeline specification
type Spec struct {
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Driver      string                 `yaml:"driver,omitempty" json:"driver,omitempty"`
	Sources     []Component            `yaml:"sources" json:"sources"`
	Processors  []Component            `yaml:"processors" json:"processors"`
	Sinks       []Component            `yaml:"sinks" json:"sinks"`
	Config      map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

// Component represents a pipeline component (source, processor, or sink)
type Component struct {
	ID              string                 `yaml:"id" json:"id"`
	Type            string                 `yaml:"type,omitempty" json:"type,omitempty"`
	Image           string                 `yaml:"image" json:"image"`
	Command         []string               `yaml:"command,omitempty" json:"command,omitempty"`
	Inputs          []string               `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	InputEventTypes []string               `yaml:"input_event_types,omitempty" json:"input_event_types,omitempty"`
	OutputEventTypes []string              `yaml:"output_event_types,omitempty" json:"output_event_types,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
	Env             map[string]string      `yaml:"env,omitempty" json:"env,omitempty"`
	Volumes         []Volume               `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Ports           []Port                 `yaml:"ports,omitempty" json:"ports,omitempty"`
	HealthCheck     string                 `yaml:"health_check,omitempty" json:"health_check,omitempty"`
	HealthPort      int                    `yaml:"health_port,omitempty" json:"health_port,omitempty"`
	Replicas        int                    `yaml:"replicas,omitempty" json:"replicas,omitempty"`
	Resources       Resources              `yaml:"resources,omitempty" json:"resources,omitempty"`
}

// Volume represents a volume mount in a component
type Volume struct {
	Name      string `yaml:"name" json:"name"`
	MountPath string `yaml:"mountPath" json:"mountPath"`
	HostPath  string `yaml:"hostPath,omitempty" json:"hostPath,omitempty"`
}

// Port represents a port exposed by a component
type Port struct {
	Name          string `yaml:"name,omitempty" json:"name,omitempty"`
	ContainerPort int    `yaml:"containerPort" json:"containerPort"`
	HostPort      int    `yaml:"hostPort,omitempty" json:"hostPort,omitempty"`
	Protocol      string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

// Resources represents compute resources requests and limits
type Resources struct {
	Requests ResourceList `yaml:"requests,omitempty" json:"requests,omitempty"`
	Limits   ResourceList `yaml:"limits,omitempty" json:"limits,omitempty"`
}

// ResourceList is a set of resource requests or limits
type ResourceList struct {
	CPU    string `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty" json:"memory,omitempty"`
}
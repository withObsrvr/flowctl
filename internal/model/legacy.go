package model

// Config represents the legacy configuration format
type Config struct {
	Version    string            `yaml:"version"`
	LogLevel   string            `yaml:"log_level"`
	Source     SourceConfig      `yaml:"source"`
	Processors []ProcessorConfig `yaml:"processors"`
	Sink       SinkConfig        `yaml:"sink"`
}

// SourceConfig represents source configuration
type SourceConfig struct {
	Type   string         `yaml:"type"`
	Params map[string]any `yaml:"params"`
}

// ProcessorConfig represents processor configuration
type ProcessorConfig struct {
	Name   string         `yaml:"name"`
	Plugin string         `yaml:"plugin"`
	Params map[string]any `yaml:"params"`
}

// SinkConfig represents sink configuration
type SinkConfig struct {
	Type   string         `yaml:"type"`
	Params map[string]any `yaml:"params"`
}

// ToConfig converts a Pipeline to the legacy Config format
func (p *Pipeline) ToConfig() *Config {
	cfg := &Config{
		Version:  "0.1",
		LogLevel: "info",
	}

	// Convert source
	if len(p.Spec.Sources) > 0 {
		src := p.Spec.Sources[0]
		cfg.Source = SourceConfig{
			Type:   src.Type,
			Params: make(map[string]any),
		}

		// Copy fields from Component to Params
		cfg.Source.Params["image"] = src.Image
		cfg.Source.Params["command"] = src.Command
		cfg.Source.Params["env"] = src.Env
		if len(src.Ports) > 0 {
			ports := make([]int, len(src.Ports))
			for i, p := range src.Ports {
				ports[i] = p.ContainerPort
			}
			cfg.Source.Params["ports"] = ports
		}
		if src.HealthCheck != "" {
			cfg.Source.Params["health_check"] = src.HealthCheck
		}
		if src.HealthPort != 0 {
			cfg.Source.Params["health_port"] = src.HealthPort
		}
	}

	// Convert processors
	cfg.Processors = make([]ProcessorConfig, len(p.Spec.Processors))
	for i, proc := range p.Spec.Processors {
		cfg.Processors[i] = ProcessorConfig{
			Name:   proc.ID,
			Plugin: proc.Type,
			Params: make(map[string]any),
		}

		// Copy fields to Params
		cfg.Processors[i].Params["image"] = proc.Image
		cfg.Processors[i].Params["command"] = proc.Command
		cfg.Processors[i].Params["env"] = proc.Env
		if len(proc.Ports) > 0 {
			ports := make([]int, len(proc.Ports))
			for j, p := range proc.Ports {
				ports[j] = p.ContainerPort
			}
			cfg.Processors[i].Params["ports"] = ports
		}
		if proc.HealthCheck != "" {
			cfg.Processors[i].Params["health_check"] = proc.HealthCheck
		}
		if proc.HealthPort != 0 {
			cfg.Processors[i].Params["health_port"] = proc.HealthPort
		}
	}

	// Convert sink
	if len(p.Spec.Sinks) > 0 {
		sink := p.Spec.Sinks[0]
		cfg.Sink = SinkConfig{
			Type:   sink.Type,
			Params: make(map[string]any),
		}

		// Copy fields to Params
		cfg.Sink.Params["image"] = sink.Image
		cfg.Sink.Params["command"] = sink.Command
		cfg.Sink.Params["env"] = sink.Env
		if len(sink.Ports) > 0 {
			ports := make([]int, len(sink.Ports))
			for i, p := range sink.Ports {
				ports[i] = p.ContainerPort
			}
			cfg.Sink.Params["ports"] = ports
		}
		if sink.HealthCheck != "" {
			cfg.Sink.Params["health_check"] = sink.HealthCheck
		}
		if sink.HealthPort != 0 {
			cfg.Sink.Params["health_port"] = sink.HealthPort
		}
	}

	return cfg
}
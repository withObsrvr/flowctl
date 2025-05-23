package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// LegacyLocalGenerator implements Generator for local execution with bash scripts
// Deprecated: Use DockerComposeLocalGenerator instead
type LegacyLocalGenerator struct{}

// NewLegacyLocalGenerator creates a new local execution generator using bash scripts
// Deprecated: Use NewLocalGenerator instead
func NewLegacyLocalGenerator() *LegacyLocalGenerator {
	return &LegacyLocalGenerator{}
}

// LocalRunConfig represents the local run configuration
type LocalRunConfig struct {
	Name          string
	Components    []LocalComponent
	EnvFile       string
	LogDir        string
	RestartPolicy string
}

// LocalComponent represents a component in the local run configuration
type LocalComponent struct {
	ID              string
	Name            string
	Type            string
	Command         string
	Args            []string
	EnvVars         map[string]string
	Ports           []int
	HealthCheck     string
	HealthCheckPort int
	DependsOn       []string
	WaitFor         string
	LogFile         string
}

const localRunnerTemplate = `#!/bin/bash

# {{ .Name }} - Local Pipeline Runner
# Generated by flowctl

set -e

# Configuration
LOG_DIR="{{ .LogDir }}"
ENV_FILE="{{ .EnvFile }}"
RESTART_POLICY="{{ .RestartPolicy }}"
PIPELINE_NAME="{{ .Name }}"

# Create log directory
mkdir -p "$LOG_DIR"

# Process tracking
declare -A PIDS

# Cleanup function
cleanup() {
  echo "Shutting down pipeline..."
  for pid in "${PIDS[@]}"; do
    if kill -0 "$pid" 2>/dev/null; then
      echo "Stopping process $pid"
      kill "$pid" || true
    fi
  done
  echo "Pipeline shutdown complete"
  exit 0
}

# Set up signal handling
trap cleanup SIGINT SIGTERM

# Load environment variables
if [ -f "$ENV_FILE" ]; then
  echo "Loading environment from $ENV_FILE"
  set -a
  source "$ENV_FILE"
  set +a
fi

# Health check function
check_health() {
  local url="$1"
  local max_retries="$2"
  local retry_delay="$3"
  local retries=0

  echo "Checking health at $url"
  until $(curl --output /dev/null --silent --fail "$url"); do
    retries=$((retries + 1))
    if [ "$retries" -ge "$max_retries" ]; then
      echo "Health check failed after $retries attempts"
      return 1
    fi
    echo "Health check attempt $retries failed, retrying in ${retry_delay}s..."
    sleep "$retry_delay"
  done
  echo "Health check passed"
  return 0
}

# Start all components
{{ range .Components }}
start_{{ .ID }}() {
  echo "Starting {{ .Name }} ({{ .Type }})..."
  
  {{ if .DependsOn }}
  echo "Waiting for dependencies: {{ .WaitFor }}"
  {{ .WaitFor }}
  {{ end }}
  
  # Create component log file
  local log_file="${LOG_DIR}/{{ .LogFile }}"
  touch "$log_file"
  
  # Start component with environment variables
  {{ .Command }} {{ range .Args }}"{{ . }}" {{ end }} > "$log_file" 2>&1 &
  local pid=$!
  PIDS[{{ .ID }}]=$pid
  echo "Started {{ .Name }} with PID: $pid"
  
  {{ if .HealthCheck }}
  # Wait for component to be healthy
  if ! check_health "{{ .HealthCheck }}" 30 2; then
    echo "{{ .Name }} failed health check, exiting"
    cleanup
    exit 1
  fi
  {{ end }}
}
{{ end }}

# Start the pipeline components in dependency order
echo "Starting pipeline: $PIPELINE_NAME"
{{ range .Components }}
start_{{ .ID }}
{{ end }}

echo "Pipeline started successfully"
echo "Logs available in: $LOG_DIR"
echo "Press Ctrl+C to stop the pipeline"

# Keep the script running
while true; do
  sleep 1
  
  # Check if any process has exited
  for component in "${!PIDS[@]}"; do
    pid=${PIDS[$component]}
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "Component $component (PID: $pid) has exited"
      if [ "$RESTART_POLICY" = "always" ]; then
        echo "Restarting component $component..."
        start_$component
      elif [ "$RESTART_POLICY" = "on-failure" ]; then
        wait "$pid" || start_$component
      else
        echo "Component $component exited, shutting down pipeline"
        cleanup
        exit 1
      fi
    fi
  done
done
`

const envFileTemplate = `# Environment variables for {{ .Name }}
# Generated by flowctl

# Global environment variables
FLOWCTL_PIPELINE_NAME={{ .Name }}
FLOWCTL_LOG_DIR={{ .LogDir }}

# Component-specific environment variables
{{ range .Components }}
# {{ .Name }} ({{ .Type }}) environment variables
{{ range $key, $value := .EnvVars }}{{ $key }}={{ $value }}
{{ end }}
{{ end }}
`

// Generate produces local execution configuration
func (g *LegacyLocalGenerator) Generate(pipeline *model.Pipeline, opts model.TranslationOptions) ([]byte, error) {
	logger.Debug("Generating local execution configuration")

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

	// Create local run configuration
	runConfig := LocalRunConfig{
		Name:          pipelineName,
		Components:    make([]LocalComponent, 0),
		EnvFile:       ".env." + pipelineName,
		LogDir:        "logs/" + pipelineName,
		RestartPolicy: "on-failure", // Default restart policy
	}

	// Process sources
	sourceIDs := make([]string, len(pipeline.Spec.Sources))
	for i, src := range pipeline.Spec.Sources {
		sourceID := sanitizeID(src.ID)
		sourceIDs[i] = sourceID
		sourceComponent := buildLocalComponent(sourceID, src.ID, "source", src, []string{})
		runConfig.Components = append(runConfig.Components, sourceComponent)
	}

	// Process processors
	processorIDs := make([]string, len(pipeline.Spec.Processors))
	for i, proc := range pipeline.Spec.Processors {
		procID := sanitizeID(proc.ID)
		processorIDs[i] = procID
		
		// Determine dependencies
		var dependencies []string
		if len(proc.Inputs) > 0 {
			// Use explicit inputs
			dependencies = make([]string, len(proc.Inputs))
			for j, input := range proc.Inputs {
				dependencies[j] = sanitizeID(input)
			}
		} else if len(sourceIDs) > 0 {
			// Default to depending on all sources
			dependencies = sourceIDs
		}
		
		procComponent := buildLocalComponent(procID, proc.ID, "processor", proc, dependencies)
		runConfig.Components = append(runConfig.Components, procComponent)
	}

	// Process sinks
	for _, sink := range pipeline.Spec.Sinks {
		sinkID := sanitizeID(sink.ID)
		
		// Determine dependencies
		var dependencies []string
		if len(sink.Inputs) > 0 {
			// Use explicit inputs
			dependencies = make([]string, len(sink.Inputs))
			for j, input := range sink.Inputs {
				dependencies[j] = sanitizeID(input)
			}
		} else if len(processorIDs) > 0 {
			// Default to depending on all processors
			dependencies = processorIDs
		} else if len(sourceIDs) > 0 {
			// If no processors, depend on sources
			dependencies = sourceIDs
		}
		
		sinkComponent := buildLocalComponent(sinkID, sink.ID, "sink", sink, dependencies)
		runConfig.Components = append(runConfig.Components, sinkComponent)
	}

	// Generate the runner script
	tmpl, err := template.New("local_runner").Parse(localRunnerTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var runnerScript strings.Builder
	if err := tmpl.Execute(&runnerScript, runConfig); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// Generate the environment file
	envTmpl, err := template.New("env_file").Parse(envFileTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse env template: %w", err)
	}

	var envFile strings.Builder
	if err := envTmpl.Execute(&envFile, runConfig); err != nil {
		return nil, fmt.Errorf("failed to execute env template: %w", err)
	}

	// Write the environment file
	if opts.OutputPath != "" {
		envFilePath := filepath.Join(filepath.Dir(opts.OutputPath), runConfig.EnvFile)
		logger.Info("Writing environment file", zap.String("path", envFilePath))
		if err := writeToFile(envFilePath, []byte(envFile.String())); err != nil {
			logger.Warn("Failed to write environment file", zap.Error(err))
		}
	}

	// Return the runner script
	return []byte(runnerScript.String()), nil
}

// Validate checks if the pipeline can be translated to local execution
func (g *LegacyLocalGenerator) Validate(pipeline *model.Pipeline) error {
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
		if len(src.Command) == 0 {
			return fmt.Errorf("source %s must have a command", src.ID)
		}
	}

	for _, proc := range pipeline.Spec.Processors {
		if proc.ID == "" {
			return fmt.Errorf("processor id is required")
		}
		if len(proc.Command) == 0 {
			return fmt.Errorf("processor %s must have a command", proc.ID)
		}
	}

	for _, sink := range pipeline.Spec.Sinks {
		if sink.ID == "" {
			return fmt.Errorf("sink id is required")
		}
		if len(sink.Command) == 0 {
			return fmt.Errorf("sink %s must have a command", sink.ID)
		}
	}

	return nil
}

// Helper functions

// sanitizeID sanitizes a name to be used as a component ID
func sanitizeID(name string) string {
	// Replace any non-alphanumeric character with an underscore
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)

	// Ensure ID starts with a letter
	if len(sanitized) > 0 && !((sanitized[0] >= 'a' && sanitized[0] <= 'z') || (sanitized[0] >= 'A' && sanitized[0] <= 'Z')) {
		sanitized = "component_" + sanitized
	}

	return sanitized
}

// buildLocalComponent creates a LocalComponent from a pipeline component
func buildLocalComponent(id, name, cType string, component model.Component, dependsOn []string) LocalComponent {
	localComponent := LocalComponent{
		ID:      id,
		Name:    name,
		Type:    cType,
		EnvVars: component.Env,
		LogFile: fmt.Sprintf("%s.log", id),
	}

	// Extract command and args
	if len(component.Command) > 0 {
		localComponent.Command = component.Command[0]
		if len(component.Command) > 1 {
			localComponent.Args = component.Command[1:]
		}
	}

	// Extract ports
	if len(component.Ports) > 0 {
		localComponent.Ports = make([]int, len(component.Ports))
		for i, p := range component.Ports {
			localComponent.Ports[i] = p.ContainerPort
		}
	}

	// Extract health check
	if component.HealthCheck != "" {
		localComponent.HealthCheck = component.HealthCheck
		localComponent.HealthCheckPort = component.HealthPort

		// Default to a reasonable port if not specified
		if localComponent.HealthCheckPort == 0 {
			// Try to use the first port if available
			if len(localComponent.Ports) > 0 {
				localComponent.HealthCheckPort = localComponent.Ports[0]
			} else {
				localComponent.HealthCheckPort = 8080 // Default fallback
			}
		}

		// Format full health check URL
		if !strings.HasPrefix(localComponent.HealthCheck, "http") {
			localComponent.HealthCheck = fmt.Sprintf("http://localhost:%d%s", 
				localComponent.HealthCheckPort, 
				localComponent.HealthCheck)
		}
	}

	// Set dependencies
	localComponent.DependsOn = dependsOn

	// Build wait-for command if dependencies exist
	if len(dependsOn) > 0 {
		var waitCmds []string
		for _, dep := range dependsOn {
			waitCmds = append(waitCmds, fmt.Sprintf("while ! kill -0 \"${PIDS[%s]}\" 2>/dev/null; do sleep 1; done", dep))
		}
		localComponent.WaitFor = strings.Join(waitCmds, " && ")
	}

	return localComponent
}

// writeToFile writes data to a file
func writeToFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
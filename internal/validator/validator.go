package validator

import (
	"fmt"
	"os"
	"strings"

	"github.com/withobsrvr/flowctl/internal/model"
)

// ValidationResult represents the result of pipeline validation
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
	Hints    []string
	pipeline *model.Pipeline
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
	Line    int // Optional: YAML line number
	Fix     string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string
	Message string
	Hint    string
}

// Validator validates pipeline configurations
type Validator struct {
	pipeline *model.Pipeline
}

// NewValidator creates a new validator
func NewValidator(pipeline *model.Pipeline) *Validator {
	return &Validator{
		pipeline: pipeline,
	}
}

// Validate performs all validation checks
func (v *Validator) Validate() *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Hints:    []string{},
		pipeline: v.pipeline,
	}

	// Run all validation checks
	v.validateMetadata(result)
	v.validateComponents(result)
	v.validateDuplicateIDs(result)
	v.validatePorts(result)
	v.validateCommands(result)
	v.validateProcessorChain(result)
	v.validateConsumerFanOut(result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	return result
}

// validateMetadata checks pipeline metadata
func (v *Validator) validateMetadata(result *ValidationResult) {
	if v.pipeline.Metadata.Name == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "metadata.name",
			Message: "Pipeline name is required",
			Fix:     "Add a name to the pipeline metadata",
		})
	}
}

// validateComponents checks that at least one source, processor, and sink exist
func (v *Validator) validateComponents(result *ValidationResult) {
	if len(v.pipeline.Spec.Sources) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.sources",
			Message: "At least one source is required",
			Fix:     "Add a source component to the pipeline",
		})
	}

	if len(v.pipeline.Spec.Sources) > 1 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "spec.sources",
			Message: "Multiple sources detected",
			Hint:    "Currently only the first source is used in processor chains",
		})
	}

	if len(v.pipeline.Spec.Processors) == 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "spec.processors",
			Message: "No processors defined",
			Hint:    "Processors transform data between sources and consumers",
		})
	}

	if len(v.pipeline.Spec.Sinks) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.sinks",
			Message: "At least one sink (consumer) is required",
			Fix:     "Add a sink component to the pipeline",
		})
	}
}

// validateDuplicateIDs checks for duplicate component IDs
func (v *Validator) validateDuplicateIDs(result *ValidationResult) {
	seen := make(map[string]string) // id -> component type

	// Check sources
	for _, source := range v.pipeline.Spec.Sources {
		if existingType, exists := seen[source.ID]; exists {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "component ID",
				Message: fmt.Sprintf("Duplicate component ID '%s' (found in %s and source)", source.ID, existingType),
				Fix:     "Use unique IDs for each component",
			})
		}
		seen[source.ID] = "source"
	}

	// Check processors
	for _, proc := range v.pipeline.Spec.Processors {
		if existingType, exists := seen[proc.ID]; exists {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "component ID",
				Message: fmt.Sprintf("Duplicate component ID '%s' (found in %s and processor)", proc.ID, existingType),
				Fix:     "Use unique IDs for each component",
			})
		}
		seen[proc.ID] = "processor"
	}

	// Check sinks
	for _, sink := range v.pipeline.Spec.Sinks {
		if existingType, exists := seen[sink.ID]; exists {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "component ID",
				Message: fmt.Sprintf("Duplicate component ID '%s' (found in %s and sink)", sink.ID, existingType),
				Fix:     "Use unique IDs for each component",
			})
		}
		seen[sink.ID] = "sink"
	}
}

// validatePorts checks for port conflicts
func (v *Validator) validatePorts(result *ValidationResult) {
	ports := make(map[string][]string) // port -> []component IDs

	// Helper to extract port from env
	extractPort := func(env map[string]string, keys []string) string {
		for _, key := range keys {
			if port, ok := env[key]; ok && port != "" {
				return port
			}
		}
		return ""
	}

	// Check all components
	allComponents := []struct {
		ID   string
		Env  map[string]string
		Type string
	}{}

	for _, source := range v.pipeline.Spec.Sources {
		allComponents = append(allComponents, struct {
			ID   string
			Env  map[string]string
			Type string
		}{source.ID, source.Env, "source"})
	}

	for _, proc := range v.pipeline.Spec.Processors {
		allComponents = append(allComponents, struct {
			ID   string
			Env  map[string]string
			Type string
		}{proc.ID, proc.Env, "processor"})
	}

	for _, sink := range v.pipeline.Spec.Sinks {
		allComponents = append(allComponents, struct {
			ID   string
			Env  map[string]string
			Type string
		}{sink.ID, sink.Env, "sink"})
	}

	// Check main ports
	for _, comp := range allComponents {
		port := extractPort(comp.Env, []string{"PORT"})
		if port != "" {
			ports[port] = append(ports[port], comp.ID)
		}
	}

	// Check health ports
	healthPorts := make(map[string][]string)
	for _, comp := range allComponents {
		port := extractPort(comp.Env, []string{"HEALTH_PORT"})
		if port != "" {
			healthPorts[port] = append(healthPorts[port], comp.ID)
		}
	}

	// Report port conflicts
	for port, components := range ports {
		if len(components) > 1 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "PORT",
				Message: fmt.Sprintf("Port conflict: %s used by multiple components: %s", port, strings.Join(components, ", ")),
				Fix:     "Assign unique ports to each component",
			})
		}
	}

	for port, components := range healthPorts {
		if len(components) > 1 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "HEALTH_PORT",
				Message: fmt.Sprintf("Health port conflict: %s used by multiple components: %s", port, strings.Join(components, ", ")),
				Fix:     "Assign unique health ports to each component",
			})
		}
	}
}

// validateCommands checks that command files exist
func (v *Validator) validateCommands(result *ValidationResult) {
	checkCommand := func(id string, command []string, compType string) {
		if len(command) == 0 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("%s.%s.command", compType, id),
				Message: fmt.Sprintf("Component '%s' has no command specified", id),
				Fix:     "Add a command to start the component",
			})
			return
		}

		// Check if first arg looks like a file path (not bash/sh)
		cmdPath := command[0]
		if cmdPath == "bash" || cmdPath == "sh" || strings.HasPrefix(cmdPath, "/bin/") {
			// Shell command, skip file check
			return
		}

		// Check if file exists
		if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   fmt.Sprintf("%s.%s.command", compType, id),
				Message: fmt.Sprintf("Command file not found: %s", cmdPath),
				Hint:    "Ensure the binary exists before running the pipeline",
			})
		}
	}

	for _, source := range v.pipeline.Spec.Sources {
		checkCommand(source.ID, source.Command, "source")
	}

	for _, proc := range v.pipeline.Spec.Processors {
		checkCommand(proc.ID, proc.Command, "processor")
	}

	for _, sink := range v.pipeline.Spec.Sinks {
		checkCommand(sink.ID, sink.Command, "sink")
	}
}

// validateProcessorChain checks processor chain length and configuration
func (v *Validator) validateProcessorChain(result *ValidationResult) {
	numProcessors := len(v.pipeline.Spec.Processors)

	if numProcessors == 0 {
		return // Already warned about this
	}

	// Warn about long chains
	if numProcessors > 5 {
		avgLatency := numProcessors * 20 // Assume 20ms per processor
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "spec.processors",
			Message: fmt.Sprintf("Long processor chain detected (%d processors)", numProcessors),
			Hint:    fmt.Sprintf("Expected latency: ~%dms. Consider splitting into separate pipelines", avgLatency),
		})
	}

	// Add hints for common patterns
	if numProcessors >= 2 {
		result.Hints = append(result.Hints, "Processor chaining enabled: Events will flow sequentially through all processors")
	}
}

// validateConsumerFanOut checks consumer fan-out configuration
func (v *Validator) validateConsumerFanOut(result *ValidationResult) {
	numConsumers := len(v.pipeline.Spec.Sinks)

	if numConsumers == 0 {
		return // Already error'd about this
	}

	// Add hints for fan-out
	if numConsumers > 1 {
		result.Hints = append(result.Hints, fmt.Sprintf("Consumer fan-out enabled: All %d consumers will receive events in parallel", numConsumers))
	}

	// Warn about many consumers
	if numConsumers > 10 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "spec.sinks",
			Message: fmt.Sprintf("Large fan-out detected (%d consumers)", numConsumers),
			Hint:    "High consumer count may increase network bandwidth and CPU usage",
		})
	}
}

// Format returns a human-readable string representation of the validation result
func (r *ValidationResult) Format() string {
	var sb strings.Builder

	if r.Valid {
		sb.WriteString("✓ Pipeline validation passed\n")
		sb.WriteString(fmt.Sprintf("  %d components total", r.countComponents()))

		if len(r.Warnings) > 0 || len(r.Hints) > 0 {
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("✗ Pipeline validation failed with %d error(s)\n", len(r.Errors)))
	}

	// Print errors
	for _, err := range r.Errors {
		sb.WriteString(fmt.Sprintf("\nERROR: %s\n", err.Message))
		if err.Field != "" {
			sb.WriteString(fmt.Sprintf("  Field: %s\n", err.Field))
		}
		if err.Fix != "" {
			sb.WriteString(fmt.Sprintf("  Fix: %s\n", err.Fix))
		}
	}

	// Print warnings
	for _, warn := range r.Warnings {
		sb.WriteString(fmt.Sprintf("\nWARNING: %s\n", warn.Message))
		if warn.Field != "" {
			sb.WriteString(fmt.Sprintf("  Field: %s\n", warn.Field))
		}
		if warn.Hint != "" {
			sb.WriteString(fmt.Sprintf("  Hint: %s\n", warn.Hint))
		}
	}

	// Print hints
	if len(r.Hints) > 0 {
		sb.WriteString("\n")
		for _, hint := range r.Hints {
			sb.WriteString(fmt.Sprintf("💡 %s\n", hint))
		}
	}

	return sb.String()
}

// countComponents counts total components in the result
func (r *ValidationResult) countComponents() int {
	if r.pipeline == nil {
		return 0
	}
	return len(r.pipeline.Spec.Sources) + len(r.pipeline.Spec.Processors) + len(r.pipeline.Spec.Sinks)
}

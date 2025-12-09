package components

import (
	"fmt"
	"strings"
)

// TranslateConfig translates component config to environment variables
// Uses metadata.json env_mapping if available, otherwise uses direct mapping
func TranslateConfig(config map[string]interface{}, metadata *Metadata) map[string]string {
	env := make(map[string]string)

	for key, value := range config {
		envVar := ""

		// Try to use metadata mapping first
		if metadata != nil && metadata.EnvMapping != nil {
			if mapped, ok := metadata.EnvMapping[key]; ok {
				envVar = mapped
			}
		}

		// Fallback: convert snake_case to UPPER_SNAKE_CASE
		if envVar == "" {
			envVar = strings.ToUpper(key)
		}

		// Convert value to string
		env[envVar] = fmt.Sprintf("%v", value)
	}

	return env
}

// MergeEnv merges config-derived env vars with explicit env vars
// Explicit env vars take precedence
func MergeEnv(configEnv, explicitEnv map[string]string) map[string]string {
	merged := make(map[string]string)

	// Add config-derived env
	for k, v := range configEnv {
		merged[k] = v
	}

	// Override with explicit env
	for k, v := range explicitEnv {
		merged[k] = v
	}

	return merged
}

// AddFlowctlEnv adds standard flowctl environment variables
func AddFlowctlEnv(env map[string]string, componentID string, controlPlaneEndpoint string) map[string]string {
	if env == nil {
		env = make(map[string]string)
	}

	// Add flowctl-specific env vars
	env["ENABLE_FLOWCTL"] = "true"
	env["FLOWCTL_ENDPOINT"] = controlPlaneEndpoint
	env["FLOWCTL_COMPONENT_ID"] = componentID

	return env
}

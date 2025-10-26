package runner

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
)

// EnvHandler handles environment variable processing and expansion
type EnvHandler struct {
	logger *zap.Logger
}

// NewEnvHandler creates a new environment handler
func NewEnvHandler(logger *zap.Logger) *EnvHandler {
	return &EnvHandler{
		logger: logger,
	}
}

// ProcessEnvironment processes and expands environment variables
func (h *EnvHandler) ProcessEnvironment(env map[string]string) ([]string, error) {
	// Start with system environment as base
	envMap := h.getSystemEnv()
	
	// Override with pipeline-specific environment
	for k, v := range env {
		// Expand variables in the value
		expanded := h.expandValue(v, envMap)
		envMap[k] = expanded
		
		h.logger.Debug("Set environment variable",
			zap.String("key", k),
			zap.String("original", v),
			zap.String("expanded", expanded))
	}
	
	// Convert to slice format for Docker
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	
	return result, nil
}

// expandValue expands environment variables in a value
func (h *EnvHandler) expandValue(value string, env map[string]string) string {
	// Handle ${VAR} syntax
	expanded := value
	
	// Find all ${VAR} patterns
	for {
		start := strings.Index(expanded, "${")
		if start == -1 {
			break
		}
		
		end := strings.Index(expanded[start:], "}")
		if end == -1 {
			break
		}
		
		// Extract variable name
		varName := expanded[start+2 : start+end]
		
		// Look up value
		replacement := ""
		if val, ok := env[varName]; ok {
			replacement = val
		} else if val, ok := os.LookupEnv(varName); ok {
			replacement = val
		}
		
		// Replace
		expanded = expanded[:start] + replacement + expanded[start+end+1:]
	}
	
	// Handle $VAR syntax
	parts := strings.Fields(expanded)
	for i, part := range parts {
		if strings.HasPrefix(part, "$") && len(part) > 1 {
			varName := part[1:]
			if val, ok := env[varName]; ok {
				parts[i] = val
			} else if val, ok := os.LookupEnv(varName); ok {
				parts[i] = val
			}
		}
	}
	
	// Rejoin if we split
	if len(parts) > 0 && strings.Contains(value, " ") {
		expanded = strings.Join(parts, " ")
	}
	
	return expanded
}

// getSystemEnv returns system environment as a map
func (h *EnvHandler) getSystemEnv() map[string]string {
	env := make(map[string]string)
	
	// Include key system variables
	essentialVars := []string{
		"PATH",
		"HOME",
		"USER",
		"SHELL",
		"LANG",
		"TZ",
		"TMPDIR",
	}
	
	for _, key := range essentialVars {
		if val, ok := os.LookupEnv(key); ok {
			env[key] = val
		}
	}
	
	return env
}

// ValidateEnvironment validates environment variables
func (h *EnvHandler) ValidateEnvironment(env map[string]string) error {
	// Check for required but missing variables
	for k, v := range env {
		// Check if value references undefined variables
		if strings.Contains(v, "${") || strings.Contains(v, "$") {
			expanded := h.expandValue(v, env)
			if strings.Contains(expanded, "${") || strings.Contains(expanded, "$") {
				h.logger.Warn("Environment variable may contain unresolved reference",
					zap.String("key", k),
					zap.String("value", v),
					zap.String("expanded", expanded))
			}
		}
		
		// Validate key format
		if !isValidEnvKey(k) {
			return fmt.Errorf("invalid environment variable key: %s", k)
		}
	}
	
	return nil
}

// isValidEnvKey checks if an environment variable key is valid
func isValidEnvKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	
	// Must start with letter or underscore
	if !isLetter(rune(key[0])) && key[0] != '_' {
		return false
	}
	
	// Rest must be alphanumeric or underscore
	for _, ch := range key[1:] {
		if !isAlphaNumeric(ch) && ch != '_' {
			return false
		}
	}
	
	return true
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isAlphaNumeric(ch rune) bool {
	return isLetter(ch) || (ch >= '0' && ch <= '9')
}
package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// SecretMount represents a secret mount configuration
type SecretMount struct {
	Name          string            `yaml:"name" json:"name"`
	HostPath      string            `yaml:"host_path" json:"host_path"`
	ContainerPath string            `yaml:"container_path" json:"container_path"`
	EnvVar        string            `yaml:"env" json:"env"`                     // Optional: set as environment variable
	Mode          string            `yaml:"mode" json:"mode"`                   // Optional: file permissions (e.g., "0600")
	Type          string            `yaml:"type" json:"type"`                   // Optional: secret type (file, dir, env)
	Required      bool              `yaml:"required" json:"required"`           // Optional: fail if secret missing
	Labels        map[string]string `yaml:"labels" json:"labels"`               // Optional: metadata labels
}

// SecretHandler handles secure mounting of secrets and credentials
type SecretHandler struct {
	logger        *zap.Logger
	volumeHandler *VolumeHandler
}

// NewSecretHandler creates a new secret handler
func NewSecretHandler(logger *zap.Logger) *SecretHandler {
	return &SecretHandler{
		logger:        logger,
		volumeHandler: NewVolumeHandler(logger),
	}
}

// ProcessSecrets processes secret mounts and returns volumes and environment variables
func (h *SecretHandler) ProcessSecrets(secrets []SecretMount) ([]VolumeMount, map[string]string, error) {
	volumes := make([]VolumeMount, 0)
	envVars := make(map[string]string)
	
	for _, secret := range secrets {
		// Validate secret configuration
		if err := h.validateSecret(secret); err != nil {
			if secret.Required {
				return nil, nil, fmt.Errorf("required secret %s validation failed: %w", secret.Name, err)
			}
			h.logger.Warn("Secret validation warning",
				zap.String("name", secret.Name),
				zap.Error(err))
			continue
		}
		
		// Process based on type
		switch secret.Type {
		case "env", "":
			// Environment variable only
			if secret.EnvVar != "" && secret.HostPath != "" {
				value, err := h.readSecretValue(secret.HostPath)
				if err != nil {
					if secret.Required {
						return nil, nil, fmt.Errorf("failed to read secret %s: %w", secret.Name, err)
					}
					h.logger.Warn("Failed to read secret",
						zap.String("name", secret.Name),
						zap.Error(err))
					continue
				}
				envVars[secret.EnvVar] = value
			}
			
		case "file", "dir":
			// File or directory mount
			volume, env, err := h.processSecretMount(secret)
			if err != nil {
				if secret.Required {
					return nil, nil, fmt.Errorf("failed to process secret mount %s: %w", secret.Name, err)
				}
				h.logger.Warn("Failed to process secret mount",
					zap.String("name", secret.Name),
					zap.Error(err))
				continue
			}
			
			volumes = append(volumes, volume)
			
			// Add environment variable if specified
			if secret.EnvVar != "" {
				envVars[secret.EnvVar] = env
			}
			
		default:
			return nil, nil, fmt.Errorf("unknown secret type: %s", secret.Type)
		}
		
		// Log secret handling (without exposing values)
		h.logSecretHandling(secret)
	}
	
	return volumes, envVars, nil
}

// validateSecret validates a secret configuration
func (h *SecretHandler) validateSecret(secret SecretMount) error {
	if secret.Name == "" {
		return fmt.Errorf("secret name is required")
	}
	
	if secret.HostPath == "" && secret.Type != "env" {
		return fmt.Errorf("host path is required for secret %s", secret.Name)
	}
	
	// Validate secret type
	validTypes := []string{"file", "dir", "env", ""}
	validType := false
	for _, t := range validTypes {
		if secret.Type == t {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("invalid secret type: %s", secret.Type)
	}
	
	// Validate file mode if specified
	if secret.Mode != "" {
		if _, err := parseFileMode(secret.Mode); err != nil {
			return fmt.Errorf("invalid file mode %s: %w", secret.Mode, err)
		}
	}
	
	return nil
}

// processSecretMount processes a secret file/directory mount
func (h *SecretHandler) processSecretMount(secret SecretMount) (VolumeMount, string, error) {
	// Resolve host path
	hostPath, err := h.volumeHandler.resolveHostPath(secret.HostPath)
	if err != nil {
		return VolumeMount{}, "", fmt.Errorf("failed to resolve secret path: %w", err)
	}
	
	// Check if secret exists and is readable
	if err := h.checkSecretAccess(hostPath); err != nil {
		return VolumeMount{}, "", fmt.Errorf("secret access denied: %w", err)
	}
	
	// Determine container path
	containerPath := secret.ContainerPath
	if containerPath == "" {
		// Default to /secrets/<name>
		containerPath = filepath.Join("/secrets", secret.Name)
	}
	
	// Create volume mount
	mount := VolumeMount{
		HostPath:      hostPath,
		ContainerPath: containerPath,
		ReadOnly:      true, // Secrets are always read-only
	}
	
	// Return environment value if needed
	envValue := containerPath
	if secret.Type == "file" && secret.EnvVar != "" {
		envValue = containerPath
	}
	
	return mount, envValue, nil
}

// readSecretValue reads a secret value from a file
func (h *SecretHandler) readSecretValue(path string) (string, error) {
	// Resolve path
	resolvedPath, err := h.volumeHandler.resolveHostPath(path)
	if err != nil {
		return "", err
	}
	
	// Check access
	if err := h.checkSecretAccess(resolvedPath); err != nil {
		return "", err
	}
	
	// Read file
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read secret file: %w", err)
	}
	
	// Trim whitespace
	value := strings.TrimSpace(string(content))
	
	return value, nil
}

// checkSecretAccess checks if a secret file/directory is accessible
func (h *SecretHandler) checkSecretAccess(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("secret not found: %w", err)
	}
	
	// Check permissions
	mode := info.Mode()
	
	// For files, check if it's not world-readable
	if !info.IsDir() && mode.Perm()&0004 != 0 {
		h.logger.Warn("Secret file is world-readable",
			zap.String("path", path),
			zap.String("mode", mode.String()))
	}
	
	// Check if we can read it
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read secret: %w", err)
	}
	file.Close()
	
	return nil
}

// logSecretHandling logs secret handling without exposing values
func (h *SecretHandler) logSecretHandling(secret SecretMount) {
	// Create hash of the host path for tracking
	hash := sha256.Sum256([]byte(secret.HostPath))
	pathHash := hex.EncodeToString(hash[:8])
	
	h.logger.Info("Processing secret",
		zap.String("name", secret.Name),
		zap.String("type", secret.Type),
		zap.String("path_hash", pathHash),
		zap.Bool("required", secret.Required))
}

// parseFileMode parses a file mode string (e.g., "0600")
func parseFileMode(mode string) (os.FileMode, error) {
	if len(mode) != 4 || mode[0] != '0' {
		return 0, fmt.Errorf("mode must be in format '0xxx'")
	}
	
	var perm os.FileMode
	for i := 1; i < 4; i++ {
		digit := mode[i] - '0'
		if digit < 0 || digit > 7 {
			return 0, fmt.Errorf("invalid digit in mode: %c", mode[i])
		}
		perm = perm*8 + os.FileMode(digit)
	}
	
	return perm, nil
}

// GetSecretEnvVars returns environment variables for secrets
func (h *SecretHandler) GetSecretEnvVars(secrets []SecretMount) map[string]string {
	envVars := make(map[string]string)
	
	for _, secret := range secrets {
		if secret.EnvVar != "" && secret.ContainerPath != "" {
			envVars[secret.EnvVar] = secret.ContainerPath
		}
	}
	
	return envVars
}
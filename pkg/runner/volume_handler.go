package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// VolumeHandler handles volume mount processing and validation
type VolumeHandler struct {
	logger *zap.Logger
}

// NewVolumeHandler creates a new volume handler
func NewVolumeHandler(logger *zap.Logger) *VolumeHandler {
	return &VolumeHandler{
		logger: logger,
	}
}

// ProcessVolumes processes and validates volume mounts
func (h *VolumeHandler) ProcessVolumes(volumes []Volume) ([]VolumeMount, error) {
	processed := make([]VolumeMount, 0, len(volumes))
	
	for _, vol := range volumes {
		mount, err := h.processVolume(vol)
		if err != nil {
			return nil, fmt.Errorf("failed to process volume %s: %w", vol.Name, err)
		}
		processed = append(processed, mount)
	}
	
	return processed, nil
}

// processVolume processes a single volume mount
func (h *VolumeHandler) processVolume(vol Volume) (VolumeMount, error) {
	// Resolve host path
	hostPath, err := h.resolveHostPath(vol.HostPath)
	if err != nil {
		return VolumeMount{}, fmt.Errorf("failed to resolve host path: %w", err)
	}
	
	// Determine container path
	containerPath := vol.ContainerPath
	if containerPath == "" {
		containerPath = vol.MountPath
	}
	if containerPath == "" {
		return VolumeMount{}, fmt.Errorf("container path not specified")
	}
	
	// Validate paths
	if err := h.validatePaths(hostPath, containerPath); err != nil {
		return VolumeMount{}, err
	}
	
	// Check if host path exists and is accessible
	if err := h.checkHostPath(hostPath, vol.ReadOnly); err != nil {
		h.logger.Warn("Host path validation warning",
			zap.String("path", hostPath),
			zap.Error(err))
		// Continue anyway - Docker might create it
	}
	
	mount := VolumeMount{
		HostPath:      hostPath,
		ContainerPath: containerPath,
		ReadOnly:      vol.ReadOnly,
	}
	
	h.logger.Debug("Processed volume mount",
		zap.String("name", vol.Name),
		zap.String("host", mount.HostPath),
		zap.String("container", mount.ContainerPath),
		zap.Bool("readonly", mount.ReadOnly))
	
	return mount, nil
}

// resolveHostPath resolves environment variables and relative paths
func (h *VolumeHandler) resolveHostPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("host path is empty")
	}
	
	// Expand environment variables
	expanded := h.expandEnvVars(path)
	
	// Handle home directory expansion
	if strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		expanded = filepath.Join(home, expanded[2:])
	}
	
	// Convert to absolute path if relative
	if !filepath.IsAbs(expanded) {
		abs, err := filepath.Abs(expanded)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		expanded = abs
	}
	
	// Clean the path
	cleaned := filepath.Clean(expanded)
	
	return cleaned, nil
}

// expandEnvVars expands environment variables in a path
func (h *VolumeHandler) expandEnvVars(path string) string {
	// First pass: expand $VAR and ${VAR} syntax
	expanded := os.ExpandEnv(path)
	
	// Second pass: handle $HOME specially if not set
	if strings.Contains(expanded, "$HOME") {
		home, _ := os.UserHomeDir()
		expanded = strings.ReplaceAll(expanded, "$HOME", home)
	}
	
	// Handle common variables that might not be in environment
	replacements := map[string]string{
		"$PWD":  mustGetwd(),
		"$USER": os.Getenv("USER"),
	}
	
	for placeholder, value := range replacements {
		if value != "" {
			expanded = strings.ReplaceAll(expanded, placeholder, value)
		}
	}
	
	return expanded
}

// validatePaths validates host and container paths
func (h *VolumeHandler) validatePaths(hostPath, containerPath string) error {
	// Validate host path
	if hostPath == "" {
		return fmt.Errorf("host path is empty after resolution")
	}
	
	// Validate container path
	if containerPath == "" {
		return fmt.Errorf("container path is empty")
	}
	
	if !filepath.IsAbs(containerPath) {
		return fmt.Errorf("container path must be absolute: %s", containerPath)
	}
	
	// Check for dangerous paths
	dangerousPaths := []string{"/", "/etc", "/bin", "/sbin", "/usr", "/lib", "/var"}
	for _, dangerous := range dangerousPaths {
		if hostPath == dangerous {
			return fmt.Errorf("mounting system directory %s is not allowed", dangerous)
		}
	}
	
	return nil
}

// checkHostPath checks if the host path exists and is accessible
func (h *VolumeHandler) checkHostPath(hostPath string, readOnly bool) error {
	info, err := os.Stat(hostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("host path does not exist: %s", hostPath)
		}
		return fmt.Errorf("failed to stat host path: %w", err)
	}
	
	// Check read permissions
	if err := h.checkReadAccess(hostPath); err != nil {
		return fmt.Errorf("no read access to host path: %w", err)
	}
	
	// Check write permissions if not read-only
	if !readOnly && !info.IsDir() {
		if err := h.checkWriteAccess(hostPath); err != nil {
			return fmt.Errorf("no write access to host path: %w", err)
		}
	}
	
	return nil
}

// checkReadAccess checks if the path is readable
func (h *VolumeHandler) checkReadAccess(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

// checkWriteAccess checks if the path is writable
func (h *VolumeHandler) checkWriteAccess(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	
	if info.IsDir() {
		// For directories, check if we can create a temp file
		testFile := filepath.Join(path, ".flowctl-write-test")
		file, err := os.Create(testFile)
		if err != nil {
			return err
		}
		file.Close()
		os.Remove(testFile)
		return nil
	}
	
	// For files, check if we can open for writing
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

// mustGetwd returns the current working directory or empty string
func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// Volume represents the input volume configuration
type Volume struct {
	Name          string
	MountPath     string
	ContainerPath string
	HostPath      string
	ReadOnly      bool
}
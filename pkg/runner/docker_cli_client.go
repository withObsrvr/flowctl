package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// DockerCLIClient implements DockerClient using Docker CLI commands
type DockerCLIClient struct {
	logger *zap.Logger
}

// NewDockerClient creates a new Docker client using CLI
func NewDockerClient(logger *zap.Logger) (*DockerCLIClient, error) {
	// Check if docker command is available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker command not found: %w", err)
	}
	
	return &DockerCLIClient{
		logger: logger,
	}, nil
}

// CreateContainer creates a new container using docker create
func (d *DockerCLIClient) CreateContainer(ctx context.Context, config *ContainerConfig) (string, error) {
	args := []string{"create"}
	
	// Add name
	if config.Name != "" {
		args = append(args, "--name", config.Name)
	}
	
	// Add network mode
	if config.NetworkMode != "" {
		args = append(args, "--network", config.NetworkMode)
	}
	
	// Add restart policy
	if config.RestartPolicy != "" && config.RestartPolicy != "no" {
		args = append(args, "--restart", config.RestartPolicy)
	}
	
	// Add labels
	for k, v := range config.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	
	// Add environment variables
	for _, env := range config.Environment {
		args = append(args, "-e", env)
	}
	
	// Add volume mounts
	d.logger.Debug("Adding volumes to container",
		zap.String("name", config.Name),
		zap.Int("count", len(config.Volumes)))
		
	for _, vol := range config.Volumes {
		hostPath := vol.HostPath
		// Expand environment variables
		hostPath = os.ExpandEnv(hostPath)
		
		// Handle $HOME specially if not in environment
		if strings.Contains(hostPath, "$HOME") {
			home, _ := os.UserHomeDir()
			hostPath = strings.ReplaceAll(hostPath, "$HOME", home)
		}
		
		// Handle $PWD if not in environment
		if strings.Contains(hostPath, "$PWD") {
			pwd, _ := os.Getwd()
			hostPath = strings.ReplaceAll(hostPath, "$PWD", pwd)
		}
		
		mount := fmt.Sprintf("%s:%s", hostPath, vol.ContainerPath)
		if vol.ReadOnly {
			mount += ":ro"
		}
		args = append(args, "-v", mount)
	}
	
	// Add port mappings
	for _, port := range config.Ports {
		portMap := fmt.Sprintf("%d:%d", port.HostPort, port.ContainerPort)
		if port.Protocol != "" && port.Protocol != "tcp" {
			portMap += "/" + strings.ToLower(port.Protocol)
		}
		args = append(args, "-p", portMap)
	}
	
	// Add image
	args = append(args, config.Image)
	
	// Add command
	if len(config.Command) > 0 {
		args = append(args, config.Command...)
	}
	
	// Run docker create
	cmd := exec.CommandContext(ctx, "docker", args...)
	d.logger.Debug("Docker create command",
		zap.String("cmd", cmd.String()))
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("docker create failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("docker create failed: %w", err)
	}
	
	// Extract container ID from output
	containerID := strings.TrimSpace(string(output))
	
	d.logger.Info("Created container",
		zap.String("id", containerID[:12]),
		zap.String("image", config.Image))
	
	return containerID, nil
}

// StartContainer starts a created container
func (d *DockerCLIClient) StartContainer(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "docker", "start", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start container: %w, output: %s", err, string(output))
	}
	
	d.logger.Info("Started container", zap.String("id", containerID[:12]))
	return nil
}

// StopContainer stops a running container
func (d *DockerCLIClient) StopContainer(ctx context.Context, containerID string) error {
	d.logger.Info("Stopping container", zap.String("id", containerID[:12]))
	
	cmd := exec.CommandContext(ctx, "docker", "stop", "--time", "10", containerID)
	if err := cmd.Run(); err != nil {
		// Check if container is already stopped
		if !strings.Contains(err.Error(), "is not running") {
			return fmt.Errorf("failed to stop container: %w", err)
		}
	}
	
	d.logger.Info("Container stopped", zap.String("id", containerID[:12]))
	
	// Remove container
	removeCmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	if err := removeCmd.Run(); err != nil {
		d.logger.Warn("Failed to remove container", zap.Error(err))
	} else {
		d.logger.Info("Container removed", zap.String("id", containerID[:12]))
	}
	
	return nil
}

// WaitContainer waits for a container to exit
func (d *DockerCLIClient) WaitContainer(ctx context.Context, containerID string) (int, error) {
	cmd := exec.CommandContext(ctx, "docker", "wait", containerID)
	output, err := cmd.Output()
	if err != nil {
		return -1, fmt.Errorf("failed to wait for container: %w", err)
	}
	
	// Parse exit code
	var exitCode int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &exitCode); err != nil {
		return -1, fmt.Errorf("failed to parse exit code: %w", err)
	}
	
	return exitCode, nil
}

// InspectContainer gets container information
func (d *DockerCLIClient) InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}
	
	// For now, return basic info
	// In a real implementation, we would parse the JSON output
	info := &ContainerInfo{
		ID:      containerID,
		Created: time.Now().Format(time.RFC3339),
		State: ContainerState{
			Status:  "running",
			Running: true,
		},
	}
	
	// Check if container is actually running
	statusCmd := exec.CommandContext(ctx, "docker", "ps", "-q", "--filter", fmt.Sprintf("id=%s", containerID))
	statusOutput, _ := statusCmd.Output()
	if len(statusOutput) == 0 {
		info.State.Running = false
		info.State.Status = "stopped"
	}
	
	return info, nil
}

// GetLogs retrieves container logs
func (d *DockerCLIClient) GetLogs(ctx context.Context, containerID string, follow bool) (LogStream, error) {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, "--timestamps", containerID)
	
	cmd := exec.CommandContext(ctx, "docker", args...)
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start docker logs: %w", err)
	}
	
	// Combine stdout and stderr
	reader := io.MultiReader(stdout, stderr)
	
	return &cliLogStream{
		reader: reader,
		cmd:    cmd,
	}, nil
}

// cliLogStream implements LogStream for CLI-based logs
type cliLogStream struct {
	reader io.Reader
	cmd    *exec.Cmd
}

func (s *cliLogStream) Read(p []byte) (n int, err error) {
	return s.reader.Read(p)
}

func (s *cliLogStream) Close() error {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
	return nil
}
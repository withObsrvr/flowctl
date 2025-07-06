package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/withobsrvr/flowctl/internal/sandbox/config"
	"github.com/withobsrvr/flowctl/internal/sandbox/watcher"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Runtime manages the container runtime for the sandbox
type Runtime struct {
	config       *Config
	containerCmd string
	network      string
	containers   map[string]*Container
	watcher      *watcher.Watcher
	mu           sync.RWMutex
}

// Config represents runtime configuration
type Config struct {
	Backend          string
	Network          string
	UseSystemRuntime bool
	LogFormat        string
}

// Container represents a running container
type Container struct {
	ID     string
	Name   string
	Image  string
	Status string
	Ports  []PortMapping
	Uptime string
}

// PortMapping represents a port mapping
type PortMapping struct {
	Host      int
	Container int
	Protocol  string
}

// NetworkInfo represents network information
type NetworkInfo struct {
	Name   string
	Subnet string
}

// LogsOptions represents options for log streaming
type LogsOptions struct {
	Follow bool
	Tail   int
}

// StopOptions represents options for stopping containers
type StopOptions struct {
	Timeout int
	Force   bool
}

// UpgradeOptions represents options for upgrading runtime
type UpgradeOptions struct {
	Version string
	Force   bool
}

var (
	globalRuntime *Runtime
	runtimeMutex  sync.Mutex
)

// NewRuntime creates a new container runtime
func NewRuntime(cfg *Config) (*Runtime, error) {
	runtimeMutex.Lock()
	defer runtimeMutex.Unlock()

	if globalRuntime != nil {
		return globalRuntime, nil
	}

	rt := &Runtime{
		config:     cfg,
		network:    cfg.Network,
		containers: make(map[string]*Container),
	}

	// Determine container command
	var err error
	rt.containerCmd, err = rt.getContainerCommand()
	if err != nil {
		return nil, fmt.Errorf("failed to determine container command: %w", err)
	}

	// Ensure bundled runtime is available if needed
	if !cfg.UseSystemRuntime {
		if err := rt.ensureBundledRuntime(); err != nil {
			return nil, fmt.Errorf("failed to setup bundled runtime: %w", err)
		}
	}

	// Create network
	if err := rt.createNetwork(); err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	globalRuntime = rt
	return rt, nil
}

// GetExistingRuntime returns the existing runtime instance
func GetExistingRuntime() (*Runtime, error) {
	runtimeMutex.Lock()
	defer runtimeMutex.Unlock()

	if globalRuntime == nil {
		return nil, fmt.Errorf("no runtime instance found, please start the sandbox first")
	}

	return globalRuntime, nil
}

// getContainerCommand determines the container command to use
func (r *Runtime) getContainerCommand() (string, error) {
	if r.config.UseSystemRuntime {
		// Try docker first, then nerdctl
		if _, err := exec.LookPath("docker"); err == nil {
			return "docker", nil
		}
		if _, err := exec.LookPath("nerdctl"); err == nil {
			return "nerdctl", nil
		}
		return "", fmt.Errorf("no system container runtime found (docker or nerdctl)")
	}

	// Use bundled nerdctl
	bundledPath := filepath.Join(os.Getenv("HOME"), ".flowctl", "runtime", "bin", "nerdctl")
	return bundledPath, nil
}

// ensureBundledRuntime ensures the bundled runtime is available
func (r *Runtime) ensureBundledRuntime() error {
	runtimeDir := filepath.Join(os.Getenv("HOME"), ".flowctl", "runtime")
	binDir := filepath.Join(runtimeDir, "bin")
	nerdctlPath := filepath.Join(binDir, "nerdctl")

	// Check if already exists
	if _, err := os.Stat(nerdctlPath); err == nil {
		return nil
	}

	logger.Info("Setting up bundled container runtime")

	// Create directories
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Download and extract runtime (simplified - in real implementation, would download actual binaries)
	// For now, create a placeholder
	if err := r.downloadBundledRuntime(binDir); err != nil {
		return fmt.Errorf("failed to download bundled runtime: %w", err)
	}

	return nil
}

// downloadBundledRuntime downloads the bundled runtime
func (r *Runtime) downloadBundledRuntime(binDir string) error {
	// This is a placeholder implementation
	// In a real implementation, this would download nerdctl and containerd binaries
	logger.Info("Downloading bundled runtime binaries")
	
	// For demonstration, create a placeholder script
	nerdctlPath := filepath.Join(binDir, "nerdctl")
	content := `#!/bin/bash
echo "Bundled nerdctl placeholder - would execute actual nerdctl commands"
echo "Args: $@"
`
	
	if err := os.WriteFile(nerdctlPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to create nerdctl placeholder: %w", err)
	}

	return nil
}

// createNetwork creates the sandbox network
func (r *Runtime) createNetwork() error {
	logger.Info("Creating sandbox network", zap.String("network", r.network))

	// Check if network already exists
	cmd := exec.Command(r.containerCmd, "network", "ls", "--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	networks := strings.Split(string(output), "\n")
	for _, network := range networks {
		if strings.TrimSpace(network) == r.network {
			logger.Info("Network already exists", zap.String("network", r.network))
			return nil
		}
	}

	// Create network
	cmd = exec.Command(r.containerCmd, "network", "create", r.network)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	logger.Info("Network created successfully", zap.String("network", r.network))
	return nil
}

// StartServices starts all services defined in the configuration
func (r *Runtime) StartServices(services map[string]config.ServiceConfig) error {
	logger.Info("Starting sandbox services", zap.Int("count", len(services)))

	// Start services in dependency order
	started := make(map[string]bool)
	
	for name := range services {
		if err := r.startServiceWithDeps(name, services, started); err != nil {
			return fmt.Errorf("failed to start service %s: %w", name, err)
		}
	}

	return nil
}

// startServiceWithDeps starts a service and its dependencies
func (r *Runtime) startServiceWithDeps(name string, services map[string]config.ServiceConfig, started map[string]bool) error {
	if started[name] {
		return nil
	}

	service := services[name]

	// Start dependencies first
	for _, dep := range service.DependsOn {
		if err := r.startServiceWithDeps(dep, services, started); err != nil {
			return err
		}
	}

	// Start this service
	if err := r.startService(name, service); err != nil {
		return err
	}

	started[name] = true
	return nil
}

// startService starts a single service
func (r *Runtime) startService(name string, service config.ServiceConfig) error {
	logger.Info("Starting service", zap.String("name", name), zap.String("image", service.Image))

	args := []string{"run", "-d", "--name", name, "--network", r.network}

	// Add port mappings
	for _, port := range service.Ports {
		args = append(args, "-p", fmt.Sprintf("%d:%d", port.Host, port.Container))
	}

	// Add environment variables
	for key, value := range service.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add volume mounts
	for _, volume := range service.Volumes {
		if volume.ReadOnly {
			args = append(args, "-v", fmt.Sprintf("%s:%s:ro", volume.Host, volume.Container))
		} else {
			args = append(args, "-v", fmt.Sprintf("%s:%s", volume.Host, volume.Container))
		}
	}

	// Add image
	args = append(args, service.Image)

	// Add command
	if len(service.Command) > 0 {
		args = append(args, service.Command...)
	}

	cmd := exec.Command(r.containerCmd, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	logger.Info("Service started", zap.String("name", name))
	return nil
}

// StartPipeline starts the pipeline container
func (r *Runtime) StartPipeline(pipelinePath string) error {
	logger.Info("Starting pipeline", zap.String("path", pipelinePath))

	// Mount pipeline file and start flowctl
	args := []string{
		"run", "-d",
		"--name", "flowctl-pipeline",
		"--network", r.network,
		"-v", fmt.Sprintf("%s:/pipeline.yaml", pipelinePath),
		"flowctl:latest", // Assuming a flowctl image exists
		"run", "/pipeline.yaml",
	}

	cmd := exec.Command(r.containerCmd, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start pipeline: %w", err)
	}

	logger.Info("Pipeline started")
	return nil
}

// LoadEnvFile loads environment variables from a file
func (r *Runtime) LoadEnvFile(envFile string) error {
	logger.Info("Loading environment file", zap.String("file", envFile))
	
	// This would load and parse the .env file
	// For now, just log that we would do this
	return nil
}

// StartWatcher starts file watching for hot reload
func (r *Runtime) StartWatcher(pipelinePath string) error {
	logger.Info("Starting file watcher", zap.String("path", pipelinePath))
	
	// Create watcher with reload function
	w, err := watcher.NewWatcher(func(changedFile string) error {
		return r.reloadPipeline(changedFile)
	})
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Watch the pipeline file and its directory
	if err := w.Watch(pipelinePath); err != nil {
		return fmt.Errorf("failed to start watching: %w", err)
	}

	r.watcher = w
	return nil
}

// reloadPipeline reloads the pipeline when files change
func (r *Runtime) reloadPipeline(changedFile string) error {
	logger.Info("Reloading pipeline due to file change", zap.String("file", changedFile))
	
	// Stop current pipeline container
	cmd := exec.Command(r.containerCmd, "stop", "flowctl-pipeline")
	cmd.Run() // Ignore errors

	// Remove the container
	cmd = exec.Command(r.containerCmd, "rm", "flowctl-pipeline")
	cmd.Run() // Ignore errors

	// Restart pipeline with updated file
	return r.StartPipeline(changedFile)
}

// ListContainers returns a list of running containers
func (r *Runtime) ListContainers() ([]*Container, error) {
	cmd := exec.Command(r.containerCmd, "ps", "--format", "table {{.Names}}\t{{.Status}}\t{{.Image}}\t{{.CreatedAt}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Parse output (simplified)
	lines := strings.Split(string(output), "\n")
	var containers []*Container
	
	for _, line := range lines[1:] { // Skip header
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		parts := strings.Split(line, "\t")
		if len(parts) >= 4 {
			containers = append(containers, &Container{
				Name:   parts[0],
				Status: parts[1],
				Image:  parts[2],
				Uptime: parts[3],
			})
		}
	}

	return containers, nil
}

// GetNetworkInfo returns network information
func (r *Runtime) GetNetworkInfo() (*NetworkInfo, error) {
	return &NetworkInfo{
		Name:   r.network,
		Subnet: "172.20.0.0/16", // Placeholder
	}, nil
}

// StreamServiceLogs streams logs from a specific service
func (r *Runtime) StreamServiceLogs(serviceName string, opts *LogsOptions) error {
	args := []string{"logs"}
	
	if opts.Follow {
		args = append(args, "-f")
	}
	
	if opts.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", opts.Tail))
	}
	
	args = append(args, serviceName)

	cmd := exec.Command(r.containerCmd, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// StreamAllLogs streams logs from all services
func (r *Runtime) StreamAllLogs(opts *LogsOptions) error {
	containers, err := r.ListContainers()
	if err != nil {
		return err
	}

	// Stream logs from all containers (simplified)
	for _, container := range containers {
		go func(name string) {
			r.StreamServiceLogs(name, opts)
		}(container.Name)
	}

	// Wait indefinitely if following logs
	if opts.Follow {
		select {}
	}

	return nil
}

// StopAll stops all sandbox containers
func (r *Runtime) StopAll(opts *StopOptions) error {
	logger.Info("Stopping all sandbox containers")

	containers, err := r.ListContainers()
	if err != nil {
		return err
	}

	for _, container := range containers {
		args := []string{"stop"}
		
		if opts.Timeout > 0 {
			args = append(args, "-t", fmt.Sprintf("%d", opts.Timeout))
		}
		
		args = append(args, container.Name)

		cmd := exec.Command(r.containerCmd, args...)
		if err := cmd.Run(); err != nil {
			logger.Warn("Failed to stop container", zap.String("name", container.Name), zap.Error(err))
		}
	}

	// Remove containers
	for _, container := range containers {
		cmd := exec.Command(r.containerCmd, "rm", container.Name)
		cmd.Run() // Ignore errors
	}

	// Remove network
	cmd := exec.Command(r.containerCmd, "network", "rm", r.network)
	cmd.Run() // Ignore errors

	// Clear global runtime
	runtimeMutex.Lock()
	globalRuntime = nil
	runtimeMutex.Unlock()

	return nil
}

// UpgradeBundledRuntime upgrades the bundled runtime
func UpgradeBundledRuntime(opts *UpgradeOptions) error {
	logger.Info("Upgrading bundled runtime", zap.String("version", opts.Version))
	
	// This would download and install new runtime binaries
	// For now, just log that we would do this
	return nil
}
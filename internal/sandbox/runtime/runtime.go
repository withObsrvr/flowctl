package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
			return nil, fmt.Errorf("failed to setup bundled runtime: %w\n\nThe bundled runtime feature is not yet implemented. Please use the --use-system-runtime flag to use your system's Docker or nerdctl instead.\n\nExample: flowctl sandbox start --use-system-runtime --pipeline pipeline.yaml --services sandbox.yaml", err)
		}
	}

	// Perform pre-flight checks
	if err := rt.performPreflightChecks(); err != nil {
		return nil, fmt.Errorf("pre-flight checks failed: %w", err)
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
		return "", fmt.Errorf("no system container runtime found (docker or nerdctl).\n\nPlease install either Docker or nerdctl:\n- Docker: https://docs.docker.com/get-docker/\n- nerdctl: https://github.com/containerd/nerdctl\n\nFor NixOS users, add to your configuration.nix:\n  environment.systemPackages = with pkgs; [ docker ];")
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
	// TODO: This is a placeholder implementation
	// The actual implementation will download nerdctl and containerd binaries
	logger.Info("Downloading bundled runtime binaries")
	
	// For now, return an error indicating this feature is not yet available
	return fmt.Errorf("bundled runtime download is not yet implemented. Please use --use-system-runtime flag with your system's Docker or nerdctl")
}

// performPreflightChecks validates that we can access the container runtime
func (r *Runtime) performPreflightChecks() error {
	logger.Info("Performing pre-flight checks", zap.String("command", r.containerCmd))
	
	// Try a simple version command to test access
	cmd := exec.Command(r.containerCmd, "version", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		
		// Check for permission errors
		if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "docker.sock") {
			return r.handlePermissionError(errMsg)
		}
		
		// Check for daemon not running
		if strings.Contains(errMsg, "Cannot connect to the Docker daemon") || strings.Contains(errMsg, "Is the docker daemon running") {
			return fmt.Errorf("Docker daemon is not running. Please start Docker and try again.\n\nOriginal error: %s", errMsg)
		}
		
		// Check for command not found
		if strings.Contains(err.Error(), "executable file not found") {
			cmdName := filepath.Base(r.containerCmd)
			return fmt.Errorf("%s not found. Please install Docker or nerdctl, or use --use-system-runtime flag with your system's container runtime", cmdName)
		}
		
		return fmt.Errorf("failed to access container runtime: %s", errMsg)
	}
	
	logger.Info("Pre-flight checks passed")
	return nil
}

// handlePermissionError provides platform-specific guidance for permission errors
func (r *Runtime) handlePermissionError(originalError string) error {
	var guidance strings.Builder
	
	guidance.WriteString("Permission denied accessing Docker. ")
	
	// Detect if running on NixOS
	if _, err := os.Stat("/etc/NIXOS"); err == nil {
		guidance.WriteString("NixOS specific solutions:\n\n")
		guidance.WriteString("1. Run with sudo:\n")
		guidance.WriteString("   sudo flowctl sandbox start --use-system-runtime ...\n\n")
		guidance.WriteString("2. Add your user to the docker group in /etc/nixos/configuration.nix:\n")
		guidance.WriteString("   users.users.YOUR_USERNAME = {\n")
		guidance.WriteString("     extraGroups = [ \"docker\" ];\n")
		guidance.WriteString("   };\n")
		guidance.WriteString("   Then run: sudo nixos-rebuild switch\n")
		guidance.WriteString("   And log out/in for group changes to take effect\n\n")
		guidance.WriteString("3. Enable rootless Docker (recommended):\n")
		guidance.WriteString("   See: docs/nixos-docker-setup.md\n")
	} else {
		// Generic Linux/macOS guidance
		switch runtime.GOOS {
		case "linux":
			guidance.WriteString("Linux solutions:\n\n")
			guidance.WriteString("1. Run with sudo:\n")
			guidance.WriteString("   sudo flowctl sandbox start --use-system-runtime ...\n\n")
			guidance.WriteString("2. Add your user to the docker group:\n")
			guidance.WriteString("   sudo usermod -aG docker $USER\n")
			guidance.WriteString("   Then log out and back in\n\n")
			guidance.WriteString("3. Check if Docker service is running:\n")
			guidance.WriteString("   sudo systemctl status docker\n")
		case "darwin":
			guidance.WriteString("macOS solutions:\n\n")
			guidance.WriteString("1. Ensure Docker Desktop is running\n")
			guidance.WriteString("2. Check Docker Desktop permissions in System Preferences\n")
			guidance.WriteString("3. Try restarting Docker Desktop\n")
		default:
			guidance.WriteString("General solutions:\n\n")
			guidance.WriteString("1. Run with elevated permissions (sudo)\n")
			guidance.WriteString("2. Ensure your user has access to Docker\n")
			guidance.WriteString("3. Check if Docker is properly installed and running\n")
		}
	}
	
	guidance.WriteString("\nOriginal error: ")
	guidance.WriteString(originalError)
	
	return fmt.Errorf(guidance.String())
}

// createNetwork creates the sandbox network
func (r *Runtime) createNetwork() error {
	logger.Info("Creating sandbox network", zap.String("network", r.network))

	// Check if network already exists
	cmd := exec.Command(r.containerCmd, "network", "ls", "--format", "{{.Name}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		
		// Check for permission denied errors
		if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "docker.sock") {
			return r.handlePermissionError(errMsg)
		}
		
		// Check for daemon not running
		if strings.Contains(errMsg, "Cannot connect to the Docker daemon") || strings.Contains(errMsg, "Is the docker daemon running") {
			return fmt.Errorf("Docker daemon is not running. Please start Docker and try again.\n\nOriginal error: %s", errMsg)
		}
		
		// Check for command not found
		if !r.config.UseSystemRuntime && strings.Contains(err.Error(), "no such file or directory") {
			return fmt.Errorf("failed to list networks: %w\n\nThis error often occurs when the bundled runtime is not properly set up. Please use the --use-system-runtime flag instead.", err)
		}
		
		return fmt.Errorf("failed to list networks: %s", errMsg)
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
	output, err = cmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		
		// Check for permission errors on create as well
		if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "docker.sock") {
			return r.handlePermissionError(errMsg)
		}
		
		return fmt.Errorf("failed to create network: %s", errMsg)
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		
		// Check for permission errors when starting containers
		if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "docker.sock") {
			return r.handlePermissionError(errMsg)
		}
		
		return fmt.Errorf("failed to start container: %s", errMsg)
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		
		// Check for permission errors when starting pipeline
		if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "docker.sock") {
			return r.handlePermissionError(errMsg)
		}
		
		return fmt.Errorf("failed to start pipeline: %s", errMsg)
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

// DisplayConnectionInfo shows connection information for running services
func (r *Runtime) DisplayConnectionInfo(services map[string]config.ServiceConfig) {
	logger.Info("🚀 Infrastructure services started successfully:")
	logger.Info("")
	
	for name, service := range services {
		var info strings.Builder
		info.WriteString(fmt.Sprintf("  ✅ %s", strings.Title(name)))
		
		// Add connection details based on service type
		switch name {
		case "redis":
			info.WriteString(" - localhost:6379")
			info.WriteString("\n     Connection: redis://localhost:6379")
		case "clickhouse":
			for _, port := range service.Ports {
				if port.Host == 8123 {
					info.WriteString(" - HTTP: localhost:8123")
				} else if port.Host == 9000 {
					info.WriteString(", Native: localhost:9000")
				}
			}
			info.WriteString("\n     HTTP: http://localhost:8123")
			info.WriteString("\n     Native: clickhouse://localhost:9000")
		case "postgres", "postgresql":
			info.WriteString(" - localhost:5432")
			if db, ok := service.Environment["POSTGRES_DB"]; ok {
				info.WriteString(fmt.Sprintf("\n     Database: %s", db))
			}
			if user, ok := service.Environment["POSTGRES_USER"]; ok {
				info.WriteString(fmt.Sprintf("\n     User: %s", user))
			}
			info.WriteString("\n     Connection: postgresql://localhost:5432")
		case "kafka":
			info.WriteString(" - localhost:9092")
			info.WriteString("\n     Bootstrap servers: localhost:9092")
		case "zookeeper":
			info.WriteString(" - localhost:2181")
		case "prometheus":
			info.WriteString(" - localhost:9090")
			info.WriteString("\n     Web UI: http://localhost:9090")
		case "grafana":
			info.WriteString(" - localhost:3000")
			info.WriteString("\n     Web UI: http://localhost:3000 (admin/admin)")
		default:
			// Generic port display
			if len(service.Ports) > 0 {
				var ports []string
				for _, port := range service.Ports {
					ports = append(ports, fmt.Sprintf("%d", port.Host))
				}
				info.WriteString(fmt.Sprintf(" - localhost:%s", strings.Join(ports, ", ")))
			}
		}
		
		logger.Info(info.String())
	}
	
	logger.Info("")
	logger.Info("📋 Next steps:")
	logger.Info("  1. Modify your pipeline YAML to use localhost endpoints")
	logger.Info("  2. Run: ./bin/flowctl run your-pipeline.yaml")
	logger.Info("")
}

// UpgradeBundledRuntime upgrades the bundled runtime
func UpgradeBundledRuntime(opts *UpgradeOptions) error {
	logger.Info("Upgrading bundled runtime", zap.String("version", opts.Version))
	
	// This would download and install new runtime binaries
	// For now, just log that we would do this
	return nil
}
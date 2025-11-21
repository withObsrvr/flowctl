package orchestrator

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// ProcessOrchestrator manages components as local processes
type ProcessOrchestrator struct {
	controlPlaneEndpoint string
	processes            map[string]*ProcessInfo
	mu                   sync.RWMutex
	logDir               string
}

// ProcessInfo holds information about a running process
type ProcessInfo struct {
	Component   *Component
	Cmd         *exec.Cmd
	Status      string
	StartTime   time.Time
	LogFile     *os.File
	Error       error
	PID         int
}

// ProcessLogStream implements LogStream for process logs
type ProcessLogStream struct {
	file   *os.File
	reader *bufio.Reader
	follow bool
}

// NewProcessOrchestrator creates a new process orchestrator
func NewProcessOrchestrator(controlPlaneEndpoint string) *ProcessOrchestrator {
	return &ProcessOrchestrator{
		controlPlaneEndpoint: controlPlaneEndpoint,
		processes:            make(map[string]*ProcessInfo),
		logDir:               "logs",
	}
}

// GetType returns the orchestrator type
func (p *ProcessOrchestrator) GetType() string {
	return TypeProcess
}

// StartComponent starts a component as a local process
func (p *ProcessOrchestrator) StartComponent(ctx context.Context, component *Component) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if component is already running
	if info, exists := p.processes[component.ID]; exists {
		if info.Status == StatusRunning {
			return fmt.Errorf("component %s is already running", component.ID)
		}
	}

	// Build command based on component type
	cmd, err := p.buildCommand(component)
	if err != nil {
		return fmt.Errorf("failed to build command for component %s: %w", component.ID, err)
	}

	// Set up environment variables
	env := p.buildEnvironment(component)
	cmd.Env = env

	// Set working directory if specified
	if component.WorkingDir != "" {
		cmd.Dir = component.WorkingDir
	}

	// Create log file
	logFile, err := p.createLogFile(component.ID)
	if err != nil {
		return fmt.Errorf("failed to create log file for component %s: %w", component.ID, err)
	}

	// Set up stdout/stderr to log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start the process
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start component %s: %w", component.ID, err)
	}

	// Store process information
	processInfo := &ProcessInfo{
		Component: component,
		Cmd:       cmd,
		Status:    StatusStarting,
		StartTime: time.Now(),
		LogFile:   logFile,
		PID:       cmd.Process.Pid,
	}

	p.processes[component.ID] = processInfo

	// Monitor process in background
	go p.monitorProcess(component.ID, processInfo)

	logger.Info("Started component process",
		zap.String("id", component.ID),
		zap.String("type", component.Type),
		zap.Int("pid", cmd.Process.Pid))

	return nil
}

// StopComponent stops a component process
func (p *ProcessOrchestrator) StopComponent(ctx context.Context, componentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, exists := p.processes[componentID]
	if !exists {
		return &ComponentNotFoundError{ComponentID: componentID}
	}

	if info.Status == StatusStopped || info.Status == StatusFailed {
		return nil
	}

	info.Status = StatusStopping

	// Send SIGTERM for graceful shutdown
	if err := info.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		logger.Error("Failed to send SIGTERM", zap.String("id", componentID), zap.Error(err))
		// Force kill if graceful shutdown fails
		if killErr := info.Cmd.Process.Kill(); killErr != nil {
			logger.Error("Failed to kill process", zap.String("id", componentID), zap.Error(killErr))
		}
	}

	// Close log file
	if info.LogFile != nil {
		info.LogFile.Close()
	}

	info.Status = StatusStopped

	logger.Info("Stopped component process", zap.String("id", componentID))
	return nil
}

// StopAll stops all managed components
func (p *ProcessOrchestrator) StopAll(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errors []error

	for componentID, info := range p.processes {
		if info.Status == StatusStopped || info.Status == StatusFailed {
			continue
		}

		info.Status = StatusStopping

		// Send SIGTERM for graceful shutdown
		if err := info.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
			logger.Error("Failed to send SIGTERM", zap.String("id", componentID), zap.Error(err))
			// Force kill if graceful shutdown fails
			if killErr := info.Cmd.Process.Kill(); killErr != nil {
				errors = append(errors, fmt.Errorf("failed to kill process %s: %w", componentID, killErr))
			}
		}

		// Close log file
		if info.LogFile != nil {
			info.LogFile.Close()
		}

		info.Status = StatusStopped
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping components: %v", errors)
	}

	logger.Info("Stopped all component processes")
	return nil
}

// GetStatus returns the status of a specific component
func (p *ProcessOrchestrator) GetStatus(ctx context.Context, componentID string) (*ComponentStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info, exists := p.processes[componentID]
	if !exists {
		return nil, &ComponentNotFoundError{ComponentID: componentID}
	}

	status := &ComponentStatus{
		ID:        componentID,
		Status:    info.Status,
		PID:       info.PID,
		StartTime: info.StartTime,
		Metadata:  make(map[string]string),
	}

	if info.Error != nil {
		status.Error = info.Error.Error()
	}

	// Add component type to metadata
	status.Metadata["type"] = info.Component.Type
	status.Metadata["name"] = info.Component.Name

	return status, nil
}

// GetAllStatus returns the status of all managed components
func (p *ProcessOrchestrator) GetAllStatus(ctx context.Context) (map[string]*ComponentStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	statuses := make(map[string]*ComponentStatus)

	for componentID, info := range p.processes {
		status := &ComponentStatus{
			ID:        componentID,
			Status:    info.Status,
			PID:       info.PID,
			StartTime: info.StartTime,
			Metadata:  make(map[string]string),
		}

		if info.Error != nil {
			status.Error = info.Error.Error()
		}

		// Add component type to metadata
		status.Metadata["type"] = info.Component.Type
		status.Metadata["name"] = info.Component.Name

		statuses[componentID] = status
	}

	return statuses, nil
}

// IsHealthy checks if a component is healthy
func (p *ProcessOrchestrator) IsHealthy(ctx context.Context, componentID string) (bool, error) {
	status, err := p.GetStatus(ctx, componentID)
	if err != nil {
		return false, err
	}

	return status.Status == StatusRunning, nil
}

// WaitForHealthy waits for a component to become healthy
func (p *ProcessOrchestrator) WaitForHealthy(ctx context.Context, componentID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		healthy, err := p.IsHealthy(ctx, componentID)
		if err != nil {
			return err
		}

		if healthy {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			// Continue checking
		}
	}

	return fmt.Errorf("timeout waiting for component %s to become healthy", componentID)
}

// GetLogs retrieves logs for a component
func (p *ProcessOrchestrator) GetLogs(ctx context.Context, componentID string, follow bool) (LogStream, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, exists := p.processes[componentID]
	if !exists {
		return nil, &ComponentNotFoundError{ComponentID: componentID}
	}

	// Open log file for reading
	logPath := fmt.Sprintf("%s/%s.log", p.logDir, componentID)
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &ProcessLogStream{
		file:   file,
		reader: bufio.NewReader(file),
		follow: follow,
	}, nil
}

// buildCommand builds the exec.Cmd for a component
func (p *ProcessOrchestrator) buildCommand(component *Component) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	// If command is explicitly specified, use it
	if len(component.Command) > 0 {
		if len(component.Args) > 0 {
			cmd = exec.Command(component.Command[0], append(component.Command[1:], component.Args...)...)
		} else {
			cmd = exec.Command(component.Command[0], component.Command[1:]...)
		}
		return cmd, nil
	}

	// Build command based on component type
	switch component.Type {
	case "stellar-live-source-datalake":
		cmd = exec.Command("./bin/stellar-live-source-datalake")
	case "ttp-processor":
		cmd = exec.Command("npm", "start", "--", "1234", "2345")
	case "consumer-app":
		cmd = exec.Command("./bin/consumer-app")
	default:
		return nil, fmt.Errorf("unknown component type: %s", component.Type)
	}

	// Add any additional args
	if len(component.Args) > 0 {
		cmd.Args = append(cmd.Args, component.Args...)
	}

	return cmd, nil
}

// buildEnvironment builds the environment variables for a component
func (p *ProcessOrchestrator) buildEnvironment(component *Component) []string {
	env := os.Environ()

	// Add flowctl integration environment variables
	env = append(env,
		"ENABLE_FLOWCTL=true",
		fmt.Sprintf("FLOWCTL_ENDPOINT=%s", p.controlPlaneEndpoint),
		"FLOWCTL_HEARTBEAT_INTERVAL=10s",
		fmt.Sprintf("FLOWCTL_SERVICE_ID=%s", component.ID),
		fmt.Sprintf("FLOWCTL_COMPONENT_ID=%s", component.ID), // Component ID for registration matching
	)

	// Add component-specific environment variables
	if component.Environment != nil {
		for key, value := range component.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return env
}

// createLogFile creates a log file for a component
func (p *ProcessOrchestrator) createLogFile(componentID string) (*os.File, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(p.logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file
	logPath := fmt.Sprintf("%s/%s.log", p.logDir, componentID)
	file, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	return file, nil
}

// monitorProcess monitors a process and updates its status
func (p *ProcessOrchestrator) monitorProcess(componentID string, info *ProcessInfo) {
	// Wait for process to complete
	err := info.Cmd.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	if err != nil {
		info.Status = StatusFailed
		info.Error = err
		logger.Error("Process failed", zap.String("id", componentID), zap.Error(err))
	} else {
		info.Status = StatusStopped
		logger.Info("Process completed", zap.String("id", componentID))
	}

	// Close log file
	if info.LogFile != nil {
		info.LogFile.Close()
	}
}

// Read implements io.Reader for ProcessLogStream
func (p *ProcessLogStream) Read(buf []byte) (int, error) {
	return p.reader.Read(buf)
}

// Close implements io.Closer for ProcessLogStream
func (p *ProcessLogStream) Close() error {
	return p.file.Close()
}
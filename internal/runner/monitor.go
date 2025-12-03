package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Monitor provides real-time monitoring and status display for the pipeline
type Monitor struct {
	runner     *PipelineRunner
	displayCh  chan bool
	logCh      chan LogEntry
	quit       chan struct{}
	clearScreen bool
}

// LogEntry represents a log entry from a component
type LogEntry struct {
	ComponentID string
	Timestamp   time.Time
	Level       string
	Message     string
}

// NewMonitor creates a new monitor for the pipeline runner
func NewMonitor(runner *PipelineRunner) *Monitor {
	return &Monitor{
		runner:      runner,
		displayCh:   make(chan bool, 1),
		logCh:       make(chan LogEntry, 100),
		quit:        make(chan struct{}),
		clearScreen: true,
	}
}

// Start begins monitoring the pipeline
func (m *Monitor) Start(ctx context.Context) {
	go m.monitorLoop(ctx)
	go m.displayLoop(ctx)
}

// Stop stops the monitoring
func (m *Monitor) Stop() {
	close(m.quit)
}

// monitorLoop monitors the pipeline status
func (m *Monitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.quit:
			return
		case <-ticker.C:
			m.displayCh <- true
		}
	}
}

// displayLoop handles the display updates
func (m *Monitor) displayLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.quit:
			return
		case <-m.displayCh:
			m.displayStatus()
		case logEntry := <-m.logCh:
			m.displayLogEntry(logEntry)
		}
	}
}

// displayStatus displays the current pipeline status
func (m *Monitor) displayStatus() {
	services, err := m.runner.controlPlane.GetServiceList()
	if err != nil {
		logger.Error("Failed to get service list", zap.Error(err))
		return
	}

	if m.clearScreen {
		// Clear screen and move cursor to top
		fmt.Print("\033[2J\033[H")
	}

	// Header
	fmt.Printf("🚀 Pipeline: %s\n", m.runner.pipeline.Metadata.Name)
	fmt.Printf("⏰ Status as of: %s\n", time.Now().Format("15:04:05"))
	fmt.Printf("📡 Control Plane: %s\n", m.runner.controlPlane.GetEndpoint())
	fmt.Println()

	// Component status
	healthyCount := 0
	for _, service := range services {
		var status string
		var statusColor *color.Color

		if service.Status == 1 { // HEALTH_STATUS_HEALTHY
			status = "🟢 HEALTHY"
			statusColor = color.New(color.FgGreen)
			healthyCount++
		} else {
			status = "🔴 UNHEALTHY"
			statusColor = color.New(color.FgRed)
		}

		// Component name with color
		componentColor := color.New(color.FgCyan, color.Bold)
		componentId := "unknown"
		if service.Component != nil {
			componentId = service.Component.Id
		}
		componentColor.Printf("%-30s ", componentId)
		statusColor.Printf("%s", status)
		fmt.Println()

		// Show metrics if available
		if len(service.Metrics) > 0 {
			for k, v := range service.Metrics {
				fmt.Printf("  %s: %.2f\n", k, v)
			}
		}

		// Show last heartbeat
		if service.LastHeartbeat != nil {
			fmt.Printf("  Last seen: %s\n", service.LastHeartbeat.AsTime().Format("15:04:05"))
		}
		fmt.Println()
	}

	// Summary
	summaryColor := color.New(color.FgYellow, color.Bold)
	summaryColor.Printf("📊 Summary: %d/%d components healthy\n", healthyCount, len(services))
	fmt.Println()

	// Instructions
	fmt.Println("Press Ctrl+C to stop pipeline")
	fmt.Println("=====================================")
}

// displayLogEntry displays a log entry
func (m *Monitor) displayLogEntry(entry LogEntry) {
	timestamp := entry.Timestamp.Format("15:04:05")
	
	// Color code by component
	componentColor := m.getComponentColor(entry.ComponentID)
	
	// Color code by log level
	var levelColor *color.Color
	switch entry.Level {
	case "ERROR":
		levelColor = color.New(color.FgRed)
	case "WARN":
		levelColor = color.New(color.FgYellow)
	case "INFO":
		levelColor = color.New(color.FgGreen)
	default:
		levelColor = color.New(color.FgWhite)
	}

	prefix := fmt.Sprintf("[%s] %s:", timestamp, entry.ComponentID)
	componentColor.Printf("%-30s ", prefix)
	levelColor.Printf("%-5s ", entry.Level)
	fmt.Printf("%s\n", entry.Message)
}

// getComponentColor returns a color for a component
func (m *Monitor) getComponentColor(componentID string) *color.Color {
	colors := []*color.Color{
		color.New(color.FgCyan),
		color.New(color.FgMagenta),
		color.New(color.FgBlue),
		color.New(color.FgYellow),
		color.New(color.FgGreen),
	}

	// Simple hash to assign consistent colors
	hash := 0
	for _, c := range componentID {
		hash += int(c)
	}

	return colors[hash%len(colors)]
}

// RunWithIntegratedMonitoring runs the pipeline with integrated monitoring
func (r *PipelineRunner) RunWithIntegratedMonitoring() error {
	logger.Info("Starting pipeline with integrated monitoring",
		zap.String("pipeline", r.pipeline.Metadata.Name))

	// Create monitor
	monitor := NewMonitor(r)

	// Start embedded control plane first
	if err := r.controlPlane.Start(r.ctx); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	// Wait for control plane to be ready
	if err := r.waitForControlPlaneReady(); err != nil {
		r.controlPlane.Stop()
		return fmt.Errorf("control plane not ready: %w", err)
	}

	// Start components
	if err := r.startComponents(); err != nil {
		r.controlPlane.Stop()
		return err
	}

	// Wait for components to register
	if err := r.waitForComponentRegistration(); err != nil {
		r.controlPlane.Stop()
		return err
	}

	// Start monitoring
	monitor.Start(r.ctx)
	defer monitor.Stop()

	logger.Info("Pipeline started with integrated monitoring")

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown
	select {
	case <-sigChan:
		logger.Info("Received shutdown signal")
	case <-r.ctx.Done():
		logger.Info("Context cancelled")
	}

	return r.shutdown()
}

// StartTUIMonitoring starts a text-based user interface for monitoring
func (r *PipelineRunner) StartTUIMonitoring() error {
	if !r.config.ShowStatus {
		return r.Run()
	}

	return r.RunWithIntegratedMonitoring()
}
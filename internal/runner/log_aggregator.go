package runner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// LogAggregator aggregates logs from multiple components
type LogAggregator struct {
	streams map[string]io.ReadCloser
	colors  map[string]*color.Color
	mu      sync.RWMutex
	quit    chan struct{}
	logCh   chan LogEntry
}

// NewLogAggregator creates a new log aggregator
func NewLogAggregator() *LogAggregator {
	return &LogAggregator{
		streams: make(map[string]io.ReadCloser),
		colors:  make(map[string]*color.Color),
		quit:    make(chan struct{}),
		logCh:   make(chan LogEntry, 100),
	}
}

// AddStream adds a log stream for a component
func (l *LogAggregator) AddStream(componentID string, stream io.ReadCloser) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.streams[componentID] = stream
	l.assignColor(componentID)

	go l.streamLogs(componentID, stream)
}

// GetLogChannel returns the channel for log entries
func (l *LogAggregator) GetLogChannel() <-chan LogEntry {
	return l.logCh
}

// Stop stops the log aggregator
func (l *LogAggregator) Stop() {
	close(l.quit)

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, stream := range l.streams {
		stream.Close()
	}
}

// assignColor assigns a color to a component
func (l *LogAggregator) assignColor(componentID string) {
	colors := []*color.Color{
		color.New(color.FgCyan),
		color.New(color.FgGreen),
		color.New(color.FgYellow),
		color.New(color.FgMagenta),
		color.New(color.FgBlue),
		color.New(color.FgRed),
	}

	l.colors[componentID] = colors[len(l.colors)%len(colors)]
}

// streamLogs streams logs from a component
func (l *LogAggregator) streamLogs(componentID string, stream io.ReadCloser) {
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	// Use a larger buffer to handle long log lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-l.quit:
			return
		default:
			line := scanner.Text()
			if line != "" {
				l.logCh <- LogEntry{
					ComponentID: componentID,
					Timestamp:   time.Now(),
					Level:       "INFO", // Default level, could be parsed from log line
					Message:     line,
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error("Error reading log stream",
			zap.String("component", componentID),
			zap.Error(err))
	}
}

// PrintLog prints a log entry with color formatting
func (l *LogAggregator) PrintLog(entry LogEntry) {
	l.mu.RLock()
	componentColor := l.colors[entry.ComponentID]
	l.mu.RUnlock()

	timestamp := entry.Timestamp.Format("15:04:05")
	prefix := fmt.Sprintf("[%s] %s:", timestamp, entry.ComponentID)

	if componentColor != nil {
		componentColor.Printf("%-30s %s\n", prefix, entry.Message)
	} else {
		fmt.Printf("%-30s %s\n", prefix, entry.Message)
	}
	// Force flush stdout to ensure logs appear immediately
	os.Stdout.Sync()
}

// StartAggregatedLogging starts aggregated logging for all components
func (r *PipelineRunner) StartAggregatedLogging() *LogAggregator {
	aggregator := NewLogAggregator()

	// Get logs from orchestrator for each component
	go func() {
		time.Sleep(3 * time.Second) // Wait for components to start

		// Get all component IDs
		var componentIDs []string
		for _, source := range r.pipeline.Spec.Sources {
			componentIDs = append(componentIDs, source.ID)
		}
		for _, processor := range r.pipeline.Spec.Processors {
			componentIDs = append(componentIDs, processor.ID)
		}
		for _, sink := range r.pipeline.Spec.Sinks {
			componentIDs = append(componentIDs, sink.ID)
		}

		// Start streaming logs for each component
		for _, componentID := range componentIDs {
			stream, err := r.orchestrator.GetLogs(r.ctx, componentID, true)
			if err != nil {
				logger.Error("Failed to get logs for component", 
					zap.String("component", componentID), 
					zap.Error(err))
				continue
			}

			aggregator.AddStream(componentID, stream)
		}
	}()

	return aggregator
}
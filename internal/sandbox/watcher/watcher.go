package watcher

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Watcher manages file watching for hot reload
type Watcher struct {
	watcher    *fsnotify.Watcher
	reloadFunc func(string) error
	debouncer  *Debouncer
}

// Debouncer prevents rapid-fire reloads
type Debouncer struct {
	timer    *time.Timer
	duration time.Duration
}

// NewWatcher creates a new file watcher
func NewWatcher(reloadFunc func(string) error) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &Watcher{
		watcher:    fsWatcher,
		reloadFunc: reloadFunc,
		debouncer: &Debouncer{
			duration: 500 * time.Millisecond, // 500ms debounce
		},
	}, nil
}

// Watch starts watching files and directories
func (w *Watcher) Watch(paths ...string) error {
	logger.Info("Starting file watcher", zap.Strings("paths", paths))

	for _, path := range paths {
		// Add the file/directory to watch
		if err := w.watcher.Add(path); err != nil {
			return fmt.Errorf("failed to watch %s: %w", path, err)
		}

		// Also watch the directory containing the file
		dir := filepath.Dir(path)
		if err := w.watcher.Add(dir); err != nil {
			logger.Warn("Failed to watch directory", zap.String("dir", dir), zap.Error(err))
		}
	}

	// Start the event processing goroutine
	go w.processEvents()

	return nil
}

// processEvents processes file system events
func (w *Watcher) processEvents() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			logger.Error("File watcher error", zap.Error(err))
		}
	}
}

// handleEvent handles a single file system event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Only handle write and create events
	if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
		logger.Debug("File changed", zap.String("file", event.Name), zap.String("op", event.Op.String()))

		// Debounce the reload to prevent rapid-fire triggers
		w.debouncer.Debounce(func() {
			if err := w.reloadFunc(event.Name); err != nil {
				logger.Error("Failed to reload after file change", 
					zap.String("file", event.Name), 
					zap.Error(err))
			} else {
				logger.Info("Reloaded after file change", zap.String("file", event.Name))
			}
		})
	}
}

// Debounce debounces function calls
func (d *Debouncer) Debounce(fn func()) {
	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.duration, fn)
}

// Close closes the watcher
func (w *Watcher) Close() error {
	if w.debouncer.timer != nil {
		w.debouncer.timer.Stop()
	}
	return w.watcher.Close()
}
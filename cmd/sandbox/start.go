package sandbox

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/sandbox/config"
	"github.com/withobsrvr/flowctl/internal/sandbox/runtime"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

type startOptions struct {
	pipeline         string
	services         string
	backend          string
	watch            bool
	envFile          string
	logFormat        string
	network          string
	useSystemRuntime bool
}

func newStartCommand() *cobra.Command {
	opts := &startOptions{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the sandbox environment",
		Long: `Start a local development environment for Flowctl pipelines with supporting
infrastructure like Redis, Kafka, ClickHouse, and other dependencies.

The sandbox uses containerized services defined in sandbox.yaml and runs your
pipeline with hot reload capabilities.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(opts)
		},
	}

	// Flags
	cmd.Flags().StringVar(&opts.pipeline, "pipeline", "pipeline.yaml", "Path to the Flowctl pipeline YAML file")
	cmd.Flags().StringVar(&opts.services, "services", "sandbox.yaml", "Path to the sandbox service definition file")
	cmd.Flags().StringVar(&opts.backend, "backend", "containerd", "Container runtime backend (containerd|docker)")
	cmd.Flags().BoolVar(&opts.watch, "watch", false, "Enable file watching and hot reload")
	cmd.Flags().StringVar(&opts.envFile, "env-file", "", "Path to .env file for environment variables")
	cmd.Flags().StringVar(&opts.logFormat, "log-format", "plain", "Log output format (plain|json)")
	cmd.Flags().StringVar(&opts.network, "network", "sandbox_net", "Container network to use for sandbox")
	cmd.Flags().BoolVar(&opts.useSystemRuntime, "use-system-runtime", false, "Use system-installed container runtime instead of bundled")

	return cmd
}

func runStart(opts *startOptions) error {
	logger.Info("Starting Flowctl sandbox", 
		zap.String("pipeline", opts.pipeline),
		zap.String("services", opts.services),
		zap.String("backend", opts.backend))

	// Validate pipeline file exists
	if _, err := os.Stat(opts.pipeline); os.IsNotExist(err) {
		return fmt.Errorf("pipeline file not found: %s", opts.pipeline)
	}

	// Load sandbox configuration
	cfg, err := config.LoadSandboxConfig(opts.services)
	if err != nil {
		return fmt.Errorf("failed to load sandbox config: %w", err)
	}

	// Initialize runtime
	rt, err := runtime.NewRuntime(&runtime.Config{
		Backend:          opts.backend,
		Network:          opts.network,
		UseSystemRuntime: opts.useSystemRuntime,
		LogFormat:        opts.logFormat,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	// Setup environment
	if opts.envFile != "" {
		if err := rt.LoadEnvFile(opts.envFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}
	}

	// Start services
	if err := rt.StartServices(cfg.Services); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	// Start pipeline
	pipelineAbsPath, err := filepath.Abs(opts.pipeline)
	if err != nil {
		return fmt.Errorf("failed to resolve pipeline path: %w", err)
	}

	if err := rt.StartPipeline(pipelineAbsPath); err != nil {
		return fmt.Errorf("failed to start pipeline: %w", err)
	}

	// Setup file watching if requested
	if opts.watch {
		if err := rt.StartWatcher(opts.pipeline); err != nil {
			logger.Warn("Failed to start file watcher", zap.Error(err))
		}
	}

	logger.Info("Sandbox started successfully")
	logger.Info("Use 'flowctl sandbox status' to check service status")
	logger.Info("Use 'flowctl sandbox logs' to view logs")
	logger.Info("Use 'flowctl sandbox stop' to shutdown")

	return nil
}
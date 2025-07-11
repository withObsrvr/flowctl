package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/runner"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

var (
	orchestratorType     string
	controlPlanePort     int
	controlPlaneAddress  string
	showStatus           bool
	logDir               string
	heartbeatTTL         time.Duration
	janitorInterval      time.Duration
)

var runCmd = &cobra.Command{
	Use:   "run [pipeline-file]",
	Short: "Run a data pipeline with integrated control plane",
	Long: `Run a data pipeline by starting components and an embedded control plane.
The control plane automatically starts and components register themselves for monitoring.

Examples:
  # Run a pipeline with default settings
  flowctl run my-pipeline.yaml

  # Run with custom control plane port
  flowctl run --control-plane-port 9090 my-pipeline.yaml

  # Run with container orchestrator
  flowctl run --orchestrator container my-pipeline.yaml`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default pipeline file
		pipelineFile := "flow.yml"
		if len(args) > 0 {
			pipelineFile = args[0]
		}

		// Check if file exists
		if _, err := os.Stat(pipelineFile); err != nil {
			return fmt.Errorf("pipeline file not found: %s", pipelineFile)
		}

		// Load pipeline configuration
		pipeline, err := runner.LoadPipelineFromFile(pipelineFile)
		if err != nil {
			return fmt.Errorf("failed to load pipeline: %w", err)
		}

		logger.Info("Starting pipeline with embedded control plane",
			zap.String("name", pipeline.Metadata.Name),
			zap.String("file", pipelineFile),
			zap.String("orchestrator", orchestratorType),
			zap.Int("control_plane_port", controlPlanePort))

		// Create logs directory
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		}

		// Create runner config
		runnerConfig := runner.Config{
			OrchestratorType:    orchestratorType,
			ControlPlanePort:    controlPlanePort,
			ControlPlaneAddress: controlPlaneAddress,
			ShowStatus:          showStatus,
			LogDir:              logDir,
			HeartbeatTTL:        heartbeatTTL,
			JanitorInterval:     janitorInterval,
		}

		// Create pipeline runner with embedded control plane
		pipelineRunner, err := runner.NewPipelineRunner(pipeline, runnerConfig)
		if err != nil {
			return fmt.Errorf("failed to create pipeline runner: %w", err)
		}

		// Run pipeline with integrated control plane
		return pipelineRunner.Run()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	
	// Add flags
	runCmd.Flags().StringVar(&orchestratorType, "orchestrator", "process", "orchestrator type (process|container)")
	runCmd.Flags().IntVar(&controlPlanePort, "control-plane-port", 8080, "embedded control plane port")
	runCmd.Flags().StringVar(&controlPlaneAddress, "control-plane-address", "127.0.0.1", "embedded control plane address")
	runCmd.Flags().BoolVar(&showStatus, "show-status", true, "show periodic status updates")
	runCmd.Flags().StringVar(&logDir, "log-dir", "logs", "directory for component logs")
	runCmd.Flags().DurationVar(&heartbeatTTL, "heartbeat-ttl", 30*time.Second, "heartbeat TTL for component health")
	runCmd.Flags().DurationVar(&janitorInterval, "janitor-interval", 10*time.Second, "interval for health checks")
}

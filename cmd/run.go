package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/runner"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

var (
	orchestratorType    string
	controlPlanePort    int
	controlPlaneAddress string
	useExternalCP       bool
	showStatus          bool
	logDir              string
	heartbeatTTL        time.Duration
	janitorInterval     time.Duration
	runDBPath           string
	runNoPersistence    bool
)

var runCmd = &cobra.Command{
	Use:   "run [pipeline-file]",
	Short: "Run a data pipeline with integrated control plane",
	Long: `Run a data pipeline by starting components and an embedded control plane.
The control plane automatically starts and components register themselves for monitoring.

Examples:
  # Run a pipeline with the default process orchestrator
  flowctl run my-pipeline.yaml

  # Run with custom control plane port
  flowctl run --control-plane-port 9090 my-pipeline.yaml

  # Let flowctl choose a free control plane port automatically
  flowctl run --control-plane-port 0 my-pipeline.yaml

  # Run against an existing external control plane
  flowctl run --use-external-control-plane --control-plane-port 9090 my-pipeline.yaml`,
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

		// Run v1 pipeline
		return runV1Pipeline(cmd, pipelineFile)
	},
}

// runV1Pipeline runs a v1 pipeline (existing behavior)
func runV1Pipeline(cmd *cobra.Command, pipelineFile string) error {
	// Load pipeline configuration
	pipeline, err := runner.LoadPipelineFromFile(pipelineFile)
	if err != nil {
		return fmt.Errorf("failed to load pipeline: %w", err)
	}

	printRunPreflight(pipelineFile, pipeline.Metadata.Name)

	logMsg := "Starting pipeline with embedded control plane"
	if useExternalCP {
		logMsg = "Starting pipeline with external control plane"
	}
	logger.Info(logMsg,
		zap.String("name", pipeline.Metadata.Name),
		zap.String("file", pipelineFile),
		zap.String("orchestrator", orchestratorType),
		zap.String("control_plane_address", controlPlaneAddress),
		zap.Int("control_plane_port", controlPlanePort))

	// Create logs directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Build control plane endpoint
	controlPlaneEndpoint := fmt.Sprintf("%s:%d", controlPlaneAddress, controlPlanePort)

	// Resolve component registry references
	ctx := context.Background()
	if err := runner.ResolveComponents(ctx, pipeline, controlPlaneEndpoint); err != nil {
		return fmt.Errorf("failed to resolve components: %w", err)
	}

	// Create runner config
	runnerConfig := runner.Config{
		OrchestratorType:    orchestratorType,
		ControlPlanePort:    controlPlanePort,
		ControlPlaneAddress: controlPlaneAddress,
		UseExternalCP:       useExternalCP,
		ShowStatus:          showStatus,
		LogDir:              logDir,
		HeartbeatTTL:        heartbeatTTL,
		JanitorInterval:     janitorInterval,
		DBPath:              runDBPath,
		NoPersistence:       runNoPersistence,
	}

	// Create pipeline runner with embedded control plane
	pipelineRunner, err := runner.NewPipelineRunner(pipeline, runnerConfig)
	if err != nil {
		return fmt.Errorf("failed to create pipeline runner: %w", err)
	}

	// Run pipeline with integrated control plane
	return pipelineRunner.Run()
}

func printRunPreflight(pipelineFile, pipelineName string) {
	fmt.Println("Starting pipeline")
	fmt.Println("-----------------")
	fmt.Printf("Pipeline:      %s\n", pipelineName)
	fmt.Printf("Config file:   %s\n", pipelineFile)
	fmt.Printf("Orchestrator:  %s\n", orchestratorType)
	if useExternalCP {
		fmt.Printf("Control plane: external (%s:%d)\n", controlPlaneAddress, controlPlanePort)
	} else if controlPlanePort == 0 {
		fmt.Printf("Control plane: embedded (%s:auto)\n", controlPlaneAddress)
	} else {
		fmt.Printf("Control plane: embedded (%s:%d)\n", controlPlaneAddress, controlPlanePort)
	}
	fmt.Printf("Logs:          %s\n", logDir)
	fmt.Println()
	fmt.Println("What happens next:")
	fmt.Println("  1. flowctl resolves and downloads any referenced components")
	fmt.Println("  2. the control plane starts")
	fmt.Println("  3. components launch and register")
	fmt.Println("  4. the pipeline is wired and starts running")
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Add flags
	runCmd.Flags().StringVar(&orchestratorType, "orchestrator", "process", "orchestrator type (process|container; process is the primary supported mode)")
	runCmd.Flags().IntVar(&controlPlanePort, "control-plane-port", 8080, "control plane port (embedded or external; use 0 to auto-select a free port)")
	runCmd.Flags().StringVar(&controlPlaneAddress, "control-plane-address", "127.0.0.1", "control plane address (embedded or external)")
	runCmd.Flags().BoolVar(&useExternalCP, "use-external-control-plane", false, "use existing external control plane instead of starting embedded one")
	runCmd.Flags().BoolVar(&showStatus, "show-status", true, "show periodic status updates")
	runCmd.Flags().StringVar(&logDir, "log-dir", "logs", "directory for component logs")
	runCmd.Flags().DurationVar(&heartbeatTTL, "heartbeat-ttl", 30*time.Second, "heartbeat TTL for component health")
	runCmd.Flags().DurationVar(&janitorInterval, "janitor-interval", 10*time.Second, "interval for health checks")
	runCmd.Flags().StringVar(&runDBPath, "db-path", "", "path to BoltDB file for embedded control plane persistence")
	runCmd.Flags().BoolVar(&runNoPersistence, "no-persistence", false, "disable embedded control plane persistence and use in-memory storage")
}

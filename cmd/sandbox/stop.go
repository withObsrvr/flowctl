package sandbox

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/sandbox/runtime"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

type stopOptions struct {
	timeout int
	force   bool
}

func newStopCommand() *cobra.Command {
	opts := &stopOptions{}

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the sandbox environment",
		Long: `Gracefully shutdown all sandbox services and containers.
		
This will stop all running containers, remove the sandbox network,
and clean up any temporary resources.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(opts)
		},
	}

	cmd.Flags().IntVar(&opts.timeout, "timeout", 30, "Timeout in seconds for graceful shutdown")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Force stop containers without graceful shutdown")

	return cmd
}

func runStop(opts *stopOptions) error {
	logger.Info("Stopping sandbox environment", 
		zap.Int("timeout", opts.timeout),
		zap.Bool("force", opts.force))

	// Get runtime instance
	rt, err := runtime.GetExistingRuntime()
	if err != nil {
		return fmt.Errorf("failed to connect to runtime: %w", err)
	}

	// Stop services
	stopOptions := &runtime.StopOptions{
		Timeout: opts.timeout,
		Force:   opts.force,
	}

	if err := rt.StopAll(stopOptions); err != nil {
		return fmt.Errorf("failed to stop services: %w", err)
	}

	logger.Info("Sandbox stopped successfully")
	return nil
}
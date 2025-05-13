package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/config"
	"github.com/withobsrvr/flowctl/internal/core"
)

var runCmd = &cobra.Command{
	Use:   "run [config-file]",
	Short: "Run a data pipeline",
	Long:  `Run a data pipeline using the specified configuration file.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.LoadFromFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Create pipeline
		pipeline, err := core.NewPipeline(cfg)
		if err != nil {
			return fmt.Errorf("failed to create pipeline: %w", err)
		}

		// Set up context with cancellation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nShutting down...")
			cancel()
		}()

		// Start pipeline
		if err := pipeline.Start(ctx); err != nil {
			return fmt.Errorf("failed to start pipeline: %w", err)
		}

		// Wait for context cancellation
		<-ctx.Done()

		// Stop pipeline
		if err := pipeline.Stop(); err != nil {
			return fmt.Errorf("failed to stop pipeline: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

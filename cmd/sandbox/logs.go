package sandbox

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/sandbox/runtime"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

type logsOptions struct {
	follow bool
	tail   int
}

func newLogsCommand() *cobra.Command {
	opts := &logsOptions{}

	cmd := &cobra.Command{
		Use:   "logs [service-name]",
		Short: "Show logs from sandbox services",
		Long: `Stream logs from sandbox services. If no service name is provided,
logs from all services will be displayed.

Examples:
  flowctl sandbox logs              # Show logs from all services
  flowctl sandbox logs redis       # Show logs from redis service only
  flowctl sandbox logs -f pipeline # Follow pipeline logs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var serviceName string
			if len(args) > 0 {
				serviceName = args[0]
			}
			return runLogs(serviceName, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVar(&opts.tail, "tail", 100, "Number of lines to show from the end of the logs")

	return cmd
}

func runLogs(serviceName string, opts *logsOptions) error {
	if serviceName != "" {
		logger.Info("Streaming logs", zap.String("service", serviceName))
	} else {
		logger.Info("Streaming logs from all services")
	}

	// Get runtime instance
	rt, err := runtime.GetExistingRuntime()
	if err != nil {
		return fmt.Errorf("failed to connect to runtime: %w", err)
	}

	// Stream logs
	logOptions := &runtime.LogsOptions{
		Follow: opts.follow,
		Tail:   opts.tail,
	}

	if serviceName != "" {
		return rt.StreamServiceLogs(serviceName, logOptions)
	}

	return rt.StreamAllLogs(logOptions)
}
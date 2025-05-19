package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print version information for flowctl and its components.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Debug("Version command executed", 
			zap.String("version", version),
			zap.String("commit", commit),
			zap.String("build_time", buildTime),
			zap.String("go_version", runtime.Version()),
			zap.String("os", runtime.GOOS),
			zap.String("arch", runtime.GOARCH))

		fmt.Printf("flowctl version %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built: %s\n", buildTime)
		fmt.Printf("  go: %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

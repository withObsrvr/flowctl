package sandbox

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/sandbox/runtime"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

type upgradeOptions struct {
	version string
	force   bool
}

func newUpgradeCommand() *cobra.Command {
	opts := &upgradeOptions{}

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade bundled container runtime",
		Long: `Download and install the latest vetted versions of nerdctl and containerd
for the bundled runtime system.

This only affects the bundled runtime. If you're using --use-system-runtime,
this command has no effect.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(opts)
		},
	}

	cmd.Flags().StringVar(&opts.version, "version", "", "Specific version to upgrade to (default: latest)")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Force upgrade even if already at target version")

	return cmd
}

func runUpgrade(opts *upgradeOptions) error {
	logger.Info("Upgrading bundled container runtime",
		zap.String("version", opts.version),
		zap.Bool("force", opts.force))

	upgradeOptions := &runtime.UpgradeOptions{
		Version: opts.version,
		Force:   opts.force,
	}

	if err := runtime.UpgradeBundledRuntime(upgradeOptions); err != nil {
		return fmt.Errorf("failed to upgrade runtime: %w", err)
	}

	logger.Info("Runtime upgrade completed successfully")
	return nil
}
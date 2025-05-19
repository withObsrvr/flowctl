package apply

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

var (
	filePath       string
	dryRun         bool
	outputPath     string
	outputFormat   string
	translateOnly  bool
	resourcePrefix string
	registryPrefix string
)

// NewCommand creates the apply command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply -f <file>",
		Short: "Create or update resources",
		Long: `Create or update resources from a file or directory.
This command follows the Kubernetes-style declarative approach to resource management.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				logger.Error("Missing file path", zap.String("command", "apply"))
				return fmt.Errorf("file path is required. Use -f to specify a file or directory")
			}

			logger.Debug("Starting apply command", 
				zap.String("file", filePath),
				zap.Bool("dry_run", dryRun),
				zap.String("output_format", outputFormat),
				zap.String("output_path", outputPath),
				zap.Bool("translate_only", translateOnly))

			// Check if path exists
			info, err := os.Stat(filePath)
			if err != nil {
				logger.Error("Failed to access path", 
					zap.String("file", filePath),
					zap.Error(err))
				return fmt.Errorf("failed to access path: %w", err)
			}

			if info.IsDir() {
				// Process directory
				logger.Info("Processing directory", zap.String("dir", filePath))
				return processDirectory(filePath, dryRun)
			}

			// Process single file
			logger.Info("Processing file", zap.String("file", filePath))
			return processFile(filePath, dryRun)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "file or directory to apply")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be applied without making changes")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file path for translation")
	cmd.Flags().StringVar(&outputFormat, "target", "", "target format for translation (docker-compose, kubernetes, nomad, local)")
	cmd.Flags().BoolVar(&translateOnly, "translate-only", false, "only translate the file without applying")
	cmd.Flags().StringVar(&resourcePrefix, "prefix", "", "prefix for resource names")
	cmd.Flags().StringVar(&registryPrefix, "registry", "", "prefix for container registry")

	return cmd
}
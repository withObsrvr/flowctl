package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/translator"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

var (
	translateOpts struct {
		inputFile     string
		outputFormat  string
		outputFile    string
		resourcePrefix string
		registryPrefix string
	}
)

// translateCmd represents the translate command
var translateCmd = &cobra.Command{
	Use:   "translate -f <file> -o <format>",
	Short: "Translate pipeline to different deployment formats",
	Long: `Translate a pipeline specification into different deployment formats
such as Docker Compose, Kubernetes, Nomad, or for local execution.

Examples:
  # Translate to Docker Compose and print to stdout
  flowctl translate -f pipeline.yaml -o docker-compose

  # Translate to Kubernetes and save to a file
  flowctl translate -f pipeline.yaml -o kubernetes --to-file k8s-manifests.yaml

  # Translate with resource prefix for naming consistency
  flowctl translate -f pipeline.yaml -o docker-compose --prefix myapp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if translateOpts.inputFile == "" {
			logger.Error("Missing input file", zap.String("command", "translate"))
			return fmt.Errorf("input file is required. Use -f to specify a file")
		}

		if translateOpts.outputFormat == "" {
			logger.Error("Missing output format", zap.String("command", "translate"))
			return fmt.Errorf("output format is required. Use -o to specify a format")
		}

		// Create translator
		options := translator.TranslationOptions{
			ResourcePrefix: translateOpts.resourcePrefix,
			RegistryPrefix: translateOpts.registryPrefix,
			OutputPath:     translateOpts.outputFile,
		}
		t := translator.NewTranslator(options)

		// Validate output format
		format := translator.Format(translateOpts.outputFormat)
		validFormats := t.ValidFormats()
		isValid := false
		for _, f := range validFormats {
			if f == format {
				isValid = true
				break
			}
		}
		if !isValid {
			logger.Error("Invalid output format", 
				zap.String("format", string(format)),
				zap.Any("valid_formats", validFormats))
			return fmt.Errorf("invalid output format: %s. Valid formats: %v", format, validFormats)
		}

		// Translate file
		logger.Info("Translating pipeline", 
			zap.String("input", translateOpts.inputFile), 
			zap.String("format", string(format)))

		ctx := context.Background()
		result, err := t.TranslateFromFile(ctx, translateOpts.inputFile, format)
		if err != nil {
			logger.Error("Translation failed", zap.Error(err))
			return fmt.Errorf("translation failed: %w", err)
		}

		// If no output file was specified, print to stdout
		if translateOpts.outputFile == "" {
			fmt.Println(string(result))
		} else {
			logger.Info("Output written to file", zap.String("path", translateOpts.outputFile))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(translateCmd)
	translateCmd.Flags().StringVarP(&translateOpts.inputFile, "file", "f", "", "input pipeline specification file")
	translateCmd.Flags().StringVarP(&translateOpts.outputFormat, "output", "o", "", "output format (docker-compose, kubernetes, nomad, local)")
	translateCmd.Flags().StringVar(&translateOpts.outputFile, "to-file", "", "write output to file instead of stdout")
	translateCmd.Flags().StringVar(&translateOpts.resourcePrefix, "prefix", "", "prefix for resource names")
	translateCmd.Flags().StringVar(&translateOpts.registryPrefix, "registry", "", "container registry prefix")
}
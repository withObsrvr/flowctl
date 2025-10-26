package translate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/interfaces"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/translator"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// Options contains the translate command options
type Options struct {
	InputFile      string
	OutputFormat   string
	OutputFile     string
	ResourcePrefix string
	RegistryPrefix string
}

var opts Options

// NewCommand creates the translate command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "translate -f <file> -o <format>",
		Short: "Translate pipeline to different deployment formats",
		Long: `Translate a pipeline specification into different deployment formats
such as Docker Compose, Kubernetes, or for local execution.

Examples:
  # Translate to Docker Compose and print to stdout
  flowctl translate -f pipeline.yaml -o docker-compose

  # Translate to Kubernetes and save to a file
  flowctl translate -f pipeline.yaml -o kubernetes --to-file k8s-manifests.yaml

  # Translate with resource prefix for naming consistency
  flowctl translate -f pipeline.yaml -o docker-compose --prefix myapp`,
		RunE: runTranslate,
	}

	// Add flags
	cmd.Flags().StringVarP(&opts.InputFile, "file", "f", "", "input pipeline specification file")
	cmd.Flags().StringVarP(&opts.OutputFormat, "output", "o", "", "output format (docker-compose, kubernetes, local)")
	cmd.Flags().StringVar(&opts.OutputFile, "to-file", "", "write output to file instead of stdout")
	cmd.Flags().StringVar(&opts.ResourcePrefix, "prefix", "", "prefix for resource names")
	cmd.Flags().StringVar(&opts.RegistryPrefix, "registry", "", "container registry prefix")

	return cmd
}

// runTranslate handles the execution of the translate command
func runTranslate(cmd *cobra.Command, args []string) error {
	if opts.InputFile == "" {
		logger.Error("Missing input file", zap.String("command", "translate"))
		return fmt.Errorf("input file is required. Use -f to specify a file")
	}

	if opts.OutputFormat == "" {
		logger.Error("Missing output format", zap.String("command", "translate"))
		return fmt.Errorf("output format is required. Use -o to specify a format")
	}

	// Create translator
	options := model.TranslationOptions{
		ResourcePrefix: opts.ResourcePrefix,
		RegistryPrefix: opts.RegistryPrefix,
		OutputPath:     opts.OutputFile,
	}
	var t interfaces.Translator = translator.NewTranslator(options)

	// Validate output format
	format := model.Format(opts.OutputFormat)
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
		zap.String("input", opts.InputFile), 
		zap.String("format", string(format)))

	ctx := context.Background()
	result, err := t.TranslateFromFile(ctx, opts.InputFile, format)
	if err != nil {
		logger.Error("Translation failed", zap.Error(err))
		return fmt.Errorf("translation failed: %w", err)
	}

	// If no output file was specified, print to stdout
	if opts.OutputFile == "" {
		fmt.Println(string(result))
	} else {
		logger.Info("Output written to file", zap.String("path", opts.OutputFile))
	}

	return nil
}
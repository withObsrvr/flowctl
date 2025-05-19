package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/translator"
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

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
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

func processDirectory(dir string, dryRun bool) error {
	// Walk through directory
	logger.Debug("Walking directory", zap.String("dir", dir))
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Error("Error walking directory", 
				zap.String("path", path), 
				zap.Error(err))
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Process only YAML files
		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			logger.Debug("Skipping non-YAML file", zap.String("file", path))
			return nil
		}

		logger.Debug("Found YAML file", zap.String("file", path))
		return processFile(path, dryRun)
	})
}

func processFile(file string, dryRun bool) error {
	// Determine output path if not specified
	outPath := outputPath
	if outPath == "" {
		// Default output path is the same directory as the input file
		dir := filepath.Dir(file)
		baseName := filepath.Base(file)
		nameWithoutExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]
		
		if outputFormat == "docker-compose" {
			outPath = filepath.Join(dir, "docker-compose.yml")
		} else {
			outPath = filepath.Join(dir, nameWithoutExt+".translated.yaml")
		}
	}

	// If we're translating
	if outputFormat != "" {
		// Create translator options
		opts := translator.TranslationOptions{
			ResourcePrefix: resourcePrefix,
			RegistryPrefix: registryPrefix,
			OutputPath:     outPath,
		}

		// Create translator
		t := translator.NewTranslator(opts)

		// Parse format
		format := translator.Format(outputFormat)

		// Validate format
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

		// Translate the file
		ctx := context.Background()
		result, err := t.TranslateFromFile(ctx, file, format)
		if err != nil {
			logger.Error("Failed to translate file", 
				zap.String("input", file),
				zap.String("output", outPath),
				zap.Error(err))
			return fmt.Errorf("failed to translate file: %w", err)
		}

		// Write the result to file if output path is specified
		if outPath != "" {
			if err := os.WriteFile(outPath, result, 0644); err != nil {
				logger.Error("Failed to write output file", 
					zap.String("path", outPath),
					zap.Error(err))
				return fmt.Errorf("failed to write output: %w", err)
			}
		}

		logger.Info("Successfully translated file",
			zap.String("input", file),
			zap.String("output", outPath),
			zap.String("format", outputFormat))

		// If we're only translating, we're done
		if translateOnly {
			return nil
		}
	}

	// Apply logic (TODO: Implement actual application logic)
	if dryRun {
		logger.Info("Dry run", 
			zap.String("file", file), 
			zap.String("action", "would apply"))
	} else {
		logger.Info("Applied resource", zap.String("file", file))
	}

	return nil
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringVarP(&filePath, "file", "f", "", "file or directory to apply")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be applied without making changes")
	applyCmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file path for translation")
	applyCmd.Flags().StringVar(&outputFormat, "target", "", "target format for translation (docker-compose, kubernetes, nomad, local)")
	applyCmd.Flags().BoolVar(&translateOnly, "translate-only", false, "only translate the file without applying")
	applyCmd.Flags().StringVar(&resourcePrefix, "prefix", "", "prefix for resource names")
	applyCmd.Flags().StringVar(&registryPrefix, "registry", "", "prefix for container registry")
}
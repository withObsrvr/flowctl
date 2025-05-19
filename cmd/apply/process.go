package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/withobsrvr/flowctl/internal/interfaces"
	"github.com/withobsrvr/flowctl/internal/model"
	"github.com/withobsrvr/flowctl/internal/translator"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// processDirectory processes all applicable files in a directory
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

// processFile processes a single file for the apply command
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
		opts := model.TranslationOptions{
			ResourcePrefix: resourcePrefix,
			RegistryPrefix: registryPrefix,
			OutputPath:     outPath,
		}

		// Create translator
		var t interfaces.Translator = translator.NewTranslator(opts)

		// Parse format
		format := model.Format(outputFormat)

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
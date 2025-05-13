package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	filePath string
	dryRun   bool
)

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply -f <file>",
	Short: "Create or update resources",
	Long: `Create or update resources from a file or directory.
This command follows the Kubernetes-style declarative approach to resource management.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if filePath == "" {
			return fmt.Errorf("file path is required. Use -f to specify a file or directory")
		}

		// Check if path exists
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to access path: %w", err)
		}

		if info.IsDir() {
			// Process directory
			return processDirectory(filePath, dryRun)
		}

		// Process single file
		return processFile(filePath, dryRun)
	},
}

func processDirectory(dir string, dryRun bool) error {
	// Walk through directory
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Process only YAML files
		if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
			return nil
		}

		return processFile(path, dryRun)
	})
}

func processFile(file string, dryRun bool) error {
	// TODO: Read and parse YAML
	// TODO: Validate resource
	// TODO: Apply resource to runtime

	if dryRun {
		fmt.Printf("Would apply %s (dry run)\n", file)
	} else {
		fmt.Printf("Applied %s\n", file)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringVarP(&filePath, "file", "f", "", "file or directory to apply")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be applied without making changes")
}

package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ProcessorResolver resolves processor references to executable binary paths.
// It handles:
// - Registry references (e.g., "stellar/ttp-processor:v2.0")
// - Local cache lookups
// - OCI image download and extraction
// - Binary extraction and permission setting
type ProcessorResolver struct {
	client      *Client
	cacheDir    string
	defaultOrg  string // Default organization for short refs (e.g., "stellar")
}

// NewProcessorResolver creates a new processor resolver.
// If cacheDir is empty, defaults to ~/.flowctl/processors
func NewProcessorResolver(cacheDir string) (*ProcessorResolver, error) {
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".flowctl", "processors")
	}

	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create processor cache directory: %w", err)
	}

	// Create OCI client (uses separate image cache)
	client, err := NewClient("")
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI client: %w", err)
	}

	return &ProcessorResolver{
		client:     client,
		cacheDir:   cacheDir,
		defaultOrg: "stellar", // Default organization for short refs
	}, nil
}

// Resolve resolves a processor reference to an executable binary path.
// Supports formats:
// - "stellar/ttp-processor:v2.0" → ghcr.io/stellar/ttp-processor:v2.0
// - "ttp-processor:v2.0" → ghcr.io/stellar/ttp-processor:v2.0 (uses default org)
// - "ghcr.io/stellar/ttp-processor:v2.0" → ghcr.io/stellar/ttp-processor:v2.0
//
// Returns the path to the executable binary.
func (r *ProcessorResolver) Resolve(ctx context.Context, ref string) (string, error) {
	// Parse as registry reference
	imgRef, err := r.parseProcessorRef(ref)
	if err != nil {
		return "", fmt.Errorf("invalid processor reference %s: %w", ref, err)
	}

	// Check if processor is already in cache
	binaryPath := r.processorBinaryPath(imgRef)
	if _, err := os.Stat(binaryPath); err == nil {
		// Already cached
		return binaryPath, nil
	}

	// Download and extract processor
	fmt.Printf("⬇ Downloading %s...\n", ref)
	if err := r.download(ctx, imgRef); err != nil {
		return "", fmt.Errorf("failed to download processor %s: %w", ref, err)
	}

	// Verify binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		return "", fmt.Errorf("processor binary not found after download: %s", binaryPath)
	}

	fmt.Printf("✓ Downloaded %s\n", ref)
	return binaryPath, nil
}

// IsCached checks if a processor is already cached locally
func (r *ProcessorResolver) IsCached(ref string) bool {
	imgRef, err := r.parseProcessorRef(ref)
	if err != nil {
		return false
	}

	binaryPath := r.processorBinaryPath(imgRef)
	_, err = os.Stat(binaryPath)
	return err == nil
}

// download downloads a processor image from the registry and extracts the binary
func (r *ProcessorResolver) download(ctx context.Context, imgRef *ImageReference) error {
	// Extract image filesystem using OCI client
	extractedPath, err := r.client.Extract(ctx, imgRef)
	if err != nil {
		return fmt.Errorf("failed to extract image: %w", err)
	}

	// Find processor binary in extracted filesystem
	// Convention: processor binary is at /processor or /bin/processor
	possiblePaths := []string{
		filepath.Join(extractedPath, "processor"),
		filepath.Join(extractedPath, "bin", "processor"),
		filepath.Join(extractedPath, "app", "processor"),
	}

	var sourcePath string
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			sourcePath = p
			break
		}
	}

	if sourcePath == "" {
		return fmt.Errorf("processor binary not found in image (expected /processor or /bin/processor)")
	}

	// Copy binary to processor cache with proper structure
	destPath := r.processorBinaryPath(imgRef)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create processor cache directory: %w", err)
	}

	// Copy file
	if err := copyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy processor binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("failed to make processor executable: %w", err)
	}

	return nil
}

// parseProcessorRef parses a processor reference into an OCI image reference
// Examples:
// - "ttp-processor:v2.0" → ghcr.io/stellar/ttp-processor:v2.0
// - "stellar/ttp-processor:v2.0" → ghcr.io/stellar/ttp-processor:v2.0
// - "ghcr.io/stellar/ttp-processor:v2.0" → ghcr.io/stellar/ttp-processor:v2.0
func (r *ProcessorResolver) parseProcessorRef(ref string) (*ImageReference, error) {
	// Check if it's already a full reference (contains registry)
	if contains(ref, ".") || contains(ref, "localhost") {
		// Full reference like "ghcr.io/stellar/ttp-processor:v2.0"
		return ParseImageReference(ref)
	}

	// Short reference - add default registry and org if needed
	// "ttp-processor:v2.0" → "ghcr.io/stellar/ttp-processor:v2.0"
	// "stellar/ttp-processor:v2.0" → "ghcr.io/stellar/ttp-processor:v2.0"
	fullRef := "ghcr.io/"
	if !contains(ref, "/") {
		// No org specified, add default
		fullRef += r.defaultOrg + "/"
	}
	fullRef += ref

	return ParseImageReference(fullRef)
}

// processorBinaryPath returns the filesystem path for a processor binary
// Structure: ~/.flowctl/processors/<repository>/<version>/processor
// Example: ~/.flowctl/processors/stellar/ttp-processor/v2.0/processor
func (r *ProcessorResolver) processorBinaryPath(imgRef *ImageReference) string {
	// Use repository path directly (handles slashes)
	// e.g., "stellar/ttp-processor" → "stellar/ttp-processor"
	repoPath := filepath.FromSlash(imgRef.Repository)

	version := imgRef.Tag
	if version == "" {
		version = "latest"
	}

	return filepath.Join(r.cacheDir, repoPath, version, "processor")
}

// ClearCache removes all cached processors
func (r *ProcessorResolver) ClearCache() error {
	return os.RemoveAll(r.cacheDir)
}

// ListCached returns a list of all cached processor references
func (r *ProcessorResolver) ListCached() ([]string, error) {
	var cached []string

	// Walk processor cache directory
	err := filepath.Walk(r.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for processor binaries
		if info.Name() == "processor" && !info.IsDir() {
			// Extract reference from path
			// ~/.flowctl/processors/stellar/ttp-processor/v2.0/processor
			// → stellar/ttp-processor:v2.0
			relPath, err := filepath.Rel(r.cacheDir, filepath.Dir(path))
			if err != nil {
				return nil // Skip if can't compute relative path
			}

			parts := filepath.SplitList(relPath)
			if len(parts) >= 2 {
				// Last part is version, rest is org/name
				version := parts[len(parts)-1]
				repoPath := filepath.Join(parts[:len(parts)-1]...)
				ref := repoPath + ":" + version
				cached = append(cached, ref)
			}
		}

		return nil
	})

	return cached, err
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := destFile.ReadFrom(sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return filepath.Base(s) != s || s == substr
}

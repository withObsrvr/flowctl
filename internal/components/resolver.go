package components

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/withobsrvr/flowctl/internal/registry"
)

// Resolver resolves component references to executable binary paths.
// It handles:
// - Registry references (e.g., "stellar-live-source@v1.0.0")
// - Local cache lookups
// - OCI image download and extraction
// - Binary extraction and permission setting
type Resolver struct {
	client      *registry.Client
	cacheDir    string
	defaultOrg  string // Default organization for short refs (e.g., "withobsrvr")
}

// NewResolver creates a new component resolver.
// If cacheDir is empty, defaults to ~/.flowctl/components
func NewResolver(cacheDir string) (*Resolver, error) {
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".flowctl", "components")
	}

	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create component cache directory: %w", err)
	}

	// Create OCI client (uses separate image cache)
	client, err := registry.NewClient("")
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI client: %w", err)
	}

	return &Resolver{
		client:     client,
		cacheDir:   cacheDir,
		defaultOrg: "withobsrvr", // Default organization
	}, nil
}

// Component represents a resolved component with its binary and metadata
type Component struct {
	Name       string
	Version    string
	BinaryPath string
	Metadata   *Metadata
}

// Resolve resolves a component reference to a Component.
// Supports formats:
// - "stellar-live-source@v1.0.0"
// - "withobsrvr/stellar-live-source@v1.0.0"
// - "ghcr.io/withobsrvr/stellar-live-source:v1.0.0"
//
// Returns the Component with binary path and metadata.
func (r *Resolver) Resolve(ctx context.Context, ref string) (*Component, error) {
	// Parse component reference
	name, version, err := r.parseComponentRef(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid component reference %s: %w", ref, err)
	}

	// Check if component is already in cache
	binaryPath := r.componentBinaryPath(name, version)
	metadataPath := r.componentMetadataPath(name, version)

	if _, err := os.Stat(binaryPath); err == nil {
		// Already cached - load metadata
		metadata, err := LoadMetadata(metadataPath)
		if err != nil {
			// If metadata doesn't exist, create minimal metadata
			metadata = &Metadata{
				Name:    name,
				Version: version,
			}
		}

		return &Component{
			Name:       name,
			Version:    version,
			BinaryPath: binaryPath,
			Metadata:   metadata,
		}, nil
	}

	// Download and extract component
	fmt.Printf("→ Downloading %s@%s...\n", name, version)
	if err := r.download(ctx, name, version); err != nil {
		return nil, fmt.Errorf("failed to download component %s@%s: %w", name, version, err)
	}

	// Verify binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		return nil, fmt.Errorf("component binary not found after download: %s", binaryPath)
	}

	// Load metadata
	metadata, err := LoadMetadata(metadataPath)
	if err != nil {
		// If metadata doesn't exist, create minimal metadata
		metadata = &Metadata{
			Name:    name,
			Version: version,
		}
	}

	fmt.Printf("✓ Downloaded %s@%s\n", name, version)

	return &Component{
		Name:       name,
		Version:    version,
		BinaryPath: binaryPath,
		Metadata:   metadata,
	}, nil
}

// IsCached checks if a component is already cached locally
func (r *Resolver) IsCached(ref string) bool {
	name, version, err := r.parseComponentRef(ref)
	if err != nil {
		return false
	}

	binaryPath := r.componentBinaryPath(name, version)
	_, err = os.Stat(binaryPath)
	return err == nil
}

// download downloads a component image from the registry and extracts the binary
func (r *Resolver) download(ctx context.Context, name, version string) error {
	// Build OCI image reference
	imgRef, err := r.buildImageReference(name, version)
	if err != nil {
		return err
	}

	// Extract image filesystem using OCI client
	extractedPath, err := r.client.Extract(ctx, imgRef)
	if err != nil {
		return fmt.Errorf("failed to extract image: %w", err)
	}

	// Find component binary in extracted filesystem
	// Convention: component binary is at /component or /bin/component
	possiblePaths := []string{
		filepath.Join(extractedPath, "component"),
		filepath.Join(extractedPath, "bin", "component"),
		filepath.Join(extractedPath, "app", "component"),
	}

	var sourceBinaryPath string
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			sourceBinaryPath = p
			break
		}
	}

	if sourceBinaryPath == "" {
		return fmt.Errorf("component binary not found in image (expected /component or /bin/component)")
	}

	// Find metadata.json in extracted filesystem
	possibleMetadataPaths := []string{
		filepath.Join(extractedPath, "metadata.json"),
		filepath.Join(extractedPath, "bin", "metadata.json"),
		filepath.Join(extractedPath, "app", "metadata.json"),
	}

	var sourceMetadataPath string
	for _, p := range possibleMetadataPaths {
		if _, err := os.Stat(p); err == nil {
			sourceMetadataPath = p
			break
		}
	}

	// Copy binary to component cache with proper structure
	destBinaryPath := r.componentBinaryPath(name, version)
	if err := os.MkdirAll(filepath.Dir(destBinaryPath), 0755); err != nil {
		return fmt.Errorf("failed to create component cache directory: %w", err)
	}

	// Copy binary file
	if err := copyFile(sourceBinaryPath, destBinaryPath); err != nil {
		return fmt.Errorf("failed to copy component binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(destBinaryPath, 0755); err != nil {
		return fmt.Errorf("failed to make component executable: %w", err)
	}

	// Copy metadata file if it exists
	if sourceMetadataPath != "" {
		destMetadataPath := r.componentMetadataPath(name, version)
		if err := copyFile(sourceMetadataPath, destMetadataPath); err != nil {
			// Non-fatal - metadata is optional
			fmt.Printf("  Warning: failed to copy metadata.json: %v\n", err)
		}
	}

	return nil
}

// parseComponentRef parses a component reference into name and version
// Examples:
// - "stellar-live-source@v1.0.0" → "stellar-live-source", "v1.0.0"
// - "withobsrvr/stellar-live-source@v1.0.0" → "withobsrvr/stellar-live-source", "v1.0.0"
func (r *Resolver) parseComponentRef(ref string) (string, string, error) {
	parts := strings.Split(ref, "@")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format, expected name@version (e.g., stellar-live-source@v1.0.0)")
	}

	name := parts[0]
	version := parts[1]

	if name == "" || version == "" {
		return "", "", fmt.Errorf("name and version cannot be empty")
	}

	return name, version, nil
}

// buildImageReference builds an OCI image reference from component name and version
func (r *Resolver) buildImageReference(name, version string) (*registry.ImageReference, error) {
	// Build full OCI reference
	// "stellar-live-source" → "ghcr.io/withobsrvr/stellar-live-source:v1.0.0"
	var fullRef string

	if strings.Contains(name, "/") {
		// Already has org: "withobsrvr/stellar-live-source"
		fullRef = fmt.Sprintf("ghcr.io/%s:%s", name, version)
	} else {
		// Add default org: "stellar-live-source" → "ghcr.io/withobsrvr/stellar-live-source"
		fullRef = fmt.Sprintf("ghcr.io/%s/%s:%s", r.defaultOrg, name, version)
	}

	return registry.ParseImageReference(fullRef)
}

// componentBinaryPath returns the filesystem path for a component binary
// Structure: ~/.flowctl/components/<name>/<version>/component
// Example: ~/.flowctl/components/stellar-live-source/v1.0.0/component
func (r *Resolver) componentBinaryPath(name, version string) string {
	// Use name directly (handles slashes)
	// e.g., "stellar-live-source" or "withobsrvr/stellar-live-source"
	namePath := filepath.FromSlash(name)
	return filepath.Join(r.cacheDir, namePath, version, "component")
}

// componentMetadataPath returns the filesystem path for component metadata
// Structure: ~/.flowctl/components/<name>/<version>/metadata.json
func (r *Resolver) componentMetadataPath(name, version string) string {
	namePath := filepath.FromSlash(name)
	return filepath.Join(r.cacheDir, namePath, version, "metadata.json")
}

// ClearCache removes all cached components
func (r *Resolver) ClearCache() error {
	return os.RemoveAll(r.cacheDir)
}

// ListCached returns a list of all cached component references
func (r *Resolver) ListCached() ([]string, error) {
	var cached []string

	// Walk component cache directory
	err := filepath.Walk(r.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for component binaries
		if info.Name() == "component" && !info.IsDir() {
			// Extract reference from path
			// ~/.flowctl/components/stellar-live-source/v1.0.0/component
			// → stellar-live-source@v1.0.0
			relPath, err := filepath.Rel(r.cacheDir, filepath.Dir(path))
			if err != nil {
				return nil // Skip if can't compute relative path
			}

			parts := strings.Split(filepath.ToSlash(relPath), "/")
			if len(parts) >= 2 {
				// Last part is version, rest is name
				version := parts[len(parts)-1]
				nameParts := parts[:len(parts)-1]
				name := strings.Join(nameParts, "/")
				ref := name + "@" + version
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

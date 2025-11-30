package registry

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Client handles OCI image operations (pull, cache, extract)
type Client struct {
	cacheDir string // Base cache directory (e.g., ~/.flowctl/cache)
}

// NewClient creates a new registry client with the specified cache directory
func NewClient(cacheDir string) (*Client, error) {
	if cacheDir == "" {
		// Default to ~/.flowctl/cache
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".flowctl", "cache")
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Client{
		cacheDir: cacheDir,
	}, nil
}

// Pull downloads an image to the local cache if not already present.
// Returns the path to the cached image tarball.
func (c *Client) Pull(ctx context.Context, ref *ImageReference) (string, error) {
	// Check if image is already cached
	cachedPath := c.ImagePath(ref)
	if _, err := os.Stat(cachedPath); err == nil {
		// Image already exists in cache
		return cachedPath, nil
	}

	// Parse image reference for go-containerregistry
	imgRef, err := name.ParseReference(ref.String())
	if err != nil {
		return "", fmt.Errorf("invalid image reference: %w", err)
	}

	// Pull image from registry with authentication
	img, err := remote.Image(imgRef, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(cachedPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create cache subdirectory: %w", err)
	}

	// Save image to cache as tarball
	if err := c.saveImageToCache(img, cachedPath); err != nil {
		// Clean up partial download
		os.Remove(cachedPath)
		return "", fmt.Errorf("failed to save image to cache: %w", err)
	}

	return cachedPath, nil
}

// Exists checks if an image is present in the local cache
func (c *Client) Exists(ref *ImageReference) bool {
	cachedPath := c.ImagePath(ref)
	_, err := os.Stat(cachedPath)
	return err == nil
}

// ImagePath returns the filesystem path where an image is (or would be) cached
func (c *Client) ImagePath(ref *ImageReference) string {
	// Create a hierarchical path: registry/repository/tag.tar
	// Example: ~/.flowctl/cache/images/ghcr.io/withobsrvr/stellar-source/v1.2.3.tar
	imagePath := filepath.Join(
		c.cacheDir,
		"images",
		ref.Registry,
		ref.Repository,
	)

	// Use tag or digest for filename
	filename := ref.Tag + ".tar"
	if ref.Digest != "" {
		// Use digest as filename (sanitized)
		filename = ref.CacheKey() + ".tar"
	}

	return filepath.Join(imagePath, filename)
}

// ExtractedPath returns the path where an image's filesystem is extracted
func (c *Client) ExtractedPath(ref *ImageReference) string {
	return c.ImagePath(ref) + ".extracted"
}

// Extract extracts the image filesystem to a directory.
// Returns the path to the extracted directory.
func (c *Client) Extract(ctx context.Context, ref *ImageReference) (string, error) {
	extractedPath := c.ExtractedPath(ref)

	// Check if already extracted
	if _, err := os.Stat(extractedPath); err == nil {
		return extractedPath, nil
	}

	// Ensure image is cached
	cachedPath, err := c.Pull(ctx, ref)
	if err != nil {
		return "", err
	}

	// Load image from tarball
	img, err := tarball.ImageFromPath(cachedPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to load image from cache: %w", err)
	}

	// Create extraction directory
	if err := os.MkdirAll(extractedPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create extraction directory: %w", err)
	}

	// Export image filesystem
	if err := c.extractImageLayers(img, extractedPath); err != nil {
		// Clean up partial extraction
		os.RemoveAll(extractedPath)
		return "", fmt.Errorf("failed to extract image: %w", err)
	}

	return extractedPath, nil
}

// saveImageToCache saves an OCI image to the cache as a tarball
func (c *Client) saveImageToCache(img v1.Image, path string) error {
	// Create output file
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write image as tarball
	return tarball.Write(nil, img, f)
}

// extractImageLayers extracts all layers of an image to a directory
func (c *Client) extractImageLayers(img v1.Image, destDir string) error {
	// Get image layers
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	// Extract each layer in order
	for _, layer := range layers {
		// Get layer contents as tar stream
		rc, err := layer.Uncompressed()
		if err != nil {
			return fmt.Errorf("failed to uncompress layer: %w", err)
		}

		// Extract tar to destination
		if err := extractTar(rc, destDir); err != nil {
			rc.Close()
			return fmt.Errorf("failed to extract layer: %w", err)
		}
		rc.Close()
	}

	return nil
}

// extractTar extracts a tar stream to a destination directory
func extractTar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	cleanDest := filepath.Clean(dest)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Skip empty names or root directory entries like "./" or "."
		cleanName := filepath.Clean(header.Name)
		if cleanName == "." || header.Name == "" {
			continue
		}

		// Construct target path
		target := filepath.Join(dest, header.Name)

		// Prevent path traversal attacks
		// Allow exact match to dest (for root-level files) or paths under dest
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in tar: %s", header.Name)
		}

		// Handle OCI whiteout files (deletions in upper layers)
		// Reference: https://github.com/opencontainers/image-spec/blob/main/layer.md#whiteouts
		baseName := filepath.Base(header.Name)
		dirName := filepath.Dir(header.Name)

		// Check for whiteout prefix (.wh.)
		if strings.HasPrefix(baseName, ".wh.") {
			// Handle opaque whiteout (.wh..wh..opq) - delete all files in directory
			if baseName == ".wh..wh..opq" {
				opaqueDir := filepath.Join(dest, dirName)
				// Remove all contents of the directory
				if err := os.RemoveAll(opaqueDir); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to apply opaque whiteout to %s: %w", opaqueDir, err)
				}
				// Recreate the directory
				if err := os.MkdirAll(opaqueDir, 0755); err != nil {
					return fmt.Errorf("failed to recreate directory after opaque whiteout %s: %w", opaqueDir, err)
				}
				continue
			}

			// Regular whiteout - delete the specific file
			targetName := strings.TrimPrefix(baseName, ".wh.")
			targetPath := filepath.Join(dest, dirName, targetName)

			// Remove the file/directory indicated by the whiteout
			if err := os.RemoveAll(targetPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to apply whiteout to %s: %w", targetPath, err)
			}
			continue
		}

		// Handle different file types
		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}

		case tar.TypeReg:
			// Create regular file
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", target, err)
			}

			// Create file
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}

			// Copy file contents
			if _, err := io.Copy(outFile, tr); err != nil {
				closeErr := outFile.Close()
				if closeErr != nil {
					return fmt.Errorf("failed to write file %s: %v; additionally, failed to close file: %w", target, err, closeErr)
				}
				return fmt.Errorf("failed to write file %s: %w", target, err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close file %s: %w", target, err)
			}

		case tar.TypeSymlink:
			// Create symlink
			// Validate symlink target doesn't escape extraction directory
			linkTarget := filepath.Join(filepath.Dir(target), header.Linkname)
			cleanLinkTarget := filepath.Clean(linkTarget)
			if !strings.HasPrefix(cleanLinkTarget, cleanDest+string(os.PathSeparator)) && cleanLinkTarget != cleanDest {
				return fmt.Errorf("symlink target escapes extraction directory: %s -> %s", header.Name, header.Linkname)
			}

			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for symlink %s: %w", target, err)
			}

			// Remove any existing file/symlink
			os.Remove(target)

			// Create symlink
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s: %w", target, header.Linkname, err)
			}

		case tar.TypeLink:
			// Create hard link
			linkTarget := filepath.Join(dest, header.Linkname)

			// Validate hard link target doesn't escape extraction directory
			cleanLinkTarget := filepath.Clean(linkTarget)
			if !strings.HasPrefix(cleanLinkTarget, cleanDest+string(os.PathSeparator)) && cleanLinkTarget != cleanDest {
				return fmt.Errorf("hard link target escapes extraction directory: %s -> %s", header.Name, header.Linkname)
			}

			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for hardlink %s: %w", target, err)
			}

			// Remove any existing file
			os.Remove(target)

			// Create hard link
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("failed to create hardlink %s -> %s: %w", target, linkTarget, err)
			}

		default:
			// Skip unsupported types (char devices, block devices, fifos, etc.)
			continue
		}
	}

	return nil
}

// CacheSize returns the total size of the cache in bytes
func (c *Client) CacheSize() (int64, error) {
	var size int64
	err := filepath.Walk(c.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// Prune removes cached images to free up space
// Implements simple FIFO eviction when cache exceeds maxSize
func (c *Client) Prune(maxSize int64) error {
	currentSize, err := c.CacheSize()
	if err != nil {
		return err
	}

	if currentSize <= maxSize {
		return nil // Cache is within limits
	}

	// TODO: Implement FIFO eviction
	// For Phase 1, we'll keep this simple
	return fmt.Errorf("cache pruning not yet implemented")
}

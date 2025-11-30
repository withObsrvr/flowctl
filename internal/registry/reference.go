package registry

import (
	"fmt"
	"regexp"
	"strings"
)

// ImageReference represents a parsed OCI image reference
type ImageReference struct {
	Registry   string // Registry hostname (e.g., ghcr.io, docker.io)
	Repository string // Repository path (e.g., withobsrvr/stellar-source)
	Tag        string // Image tag (e.g., v1.2.3, latest)
	Digest     string // Optional digest (e.g., sha256:abc...)
	Raw        string // Original unparsed reference
}

// ParseImageReference parses an OCI image reference string into components.
//
// Supported formats:
// - nginx                                  → docker.io/library/nginx:latest
// - nginx:v1.21                            → docker.io/library/nginx:v1.21
// - myorg/myapp                            → docker.io/myorg/myapp:latest
// - myorg/myapp:v1.0.0                     → docker.io/myorg/myapp:v1.0.0
// - ghcr.io/myorg/myapp:v1.0.0             → ghcr.io/myorg/myapp:v1.0.0
// - ghcr.io/myorg/myapp@sha256:abc...      → ghcr.io/myorg/myapp@sha256:abc...
// - localhost:5000/myapp:v1.0.0            → localhost:5000/myapp:v1.0.0
//
// Returns an error if the reference format is invalid.
func ParseImageReference(ref string) (*ImageReference, error) {
	if ref == "" {
		return nil, fmt.Errorf("image reference cannot be empty")
	}

	result := &ImageReference{
		Raw: ref,
	}

	// Handle digest references (image@sha256:...)
	if strings.Contains(ref, "@") {
		parts := strings.SplitN(ref, "@", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid digest reference: %s", ref)
		}
		ref = parts[0]
		result.Digest = parts[1]

		// Validate digest format
		if !isValidDigest(result.Digest) {
			return nil, fmt.Errorf("invalid digest format: %s", result.Digest)
		}
	}

	// Split on first ':' to separate reference from tag
	var repoWithRegistry string
	if strings.Contains(ref, ":") {
		// Check if it's a port (registry with port) or a tag
		colonIdx := strings.LastIndex(ref, ":")
		possibleTag := ref[colonIdx+1:]

		// If it contains a slash after the colon, it's a port, not a tag
		if strings.Contains(possibleTag, "/") {
			repoWithRegistry = ref
		} else {
			repoWithRegistry = ref[:colonIdx]
			result.Tag = possibleTag
		}
	} else {
		repoWithRegistry = ref
	}

	// Extract registry and repository
	parts := strings.SplitN(repoWithRegistry, "/", 2)

	if len(parts) == 1 {
		// No registry specified: nginx → docker.io/library/nginx
		result.Registry = "docker.io"
		result.Repository = "library/" + parts[0]
	} else if isRegistryHost(parts[0]) {
		// Registry specified: ghcr.io/myorg/myapp
		result.Registry = parts[0]
		result.Repository = parts[1]
	} else {
		// No registry but has org: myorg/myapp → docker.io/myorg/myapp
		result.Registry = "docker.io"
		result.Repository = repoWithRegistry
	}

	// Set default tag to "latest" if no tag or digest was specified
	if result.Tag == "" && result.Digest == "" {
		result.Tag = "latest"
	}

	// Validate components
	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("invalid image reference: %w", err)
	}

	return result, nil
}

// Validate checks if the image reference components are valid
func (r *ImageReference) Validate() error {
	if r.Registry == "" {
		return fmt.Errorf("registry cannot be empty")
	}
	if r.Repository == "" {
		return fmt.Errorf("repository cannot be empty")
	}
	if r.Tag == "" && r.Digest == "" {
		return fmt.Errorf("must specify either tag or digest")
	}
	if !isValidTag(r.Tag) && r.Tag != "" {
		return fmt.Errorf("invalid tag format: %s", r.Tag)
	}
	return nil
}

// String returns the canonical string representation of the image reference
func (r *ImageReference) String() string {
	ref := r.Registry + "/" + r.Repository
	if r.Digest != "" {
		ref += "@" + r.Digest
	} else if r.Tag != "" {
		ref += ":" + r.Tag
	}
	return ref
}

// WithoutTag returns the reference without tag or digest (for cache key)
func (r *ImageReference) WithoutTag() string {
	return r.Registry + "/" + r.Repository
}

// CacheKey returns a filesystem-safe cache key for this image
func (r *ImageReference) CacheKey() string {
	// Replace invalid filesystem characters
	key := strings.ReplaceAll(r.String(), ":", "_")
	key = strings.ReplaceAll(key, "@", "_")
	key = strings.ReplaceAll(key, "/", "_")
	return key
}

// isRegistryHost checks if a string looks like a registry hostname
func isRegistryHost(s string) bool {
	// Registry hosts typically:
	// - Contain a dot (ghcr.io, docker.io)
	// - OR contain a colon for port (localhost:5000)
	// - OR are "localhost"
	return strings.Contains(s, ".") || strings.Contains(s, ":") || s == "localhost"
}

// isValidTag checks if a tag follows Docker tag naming rules
func isValidTag(tag string) bool {
	if tag == "" {
		return true // Empty is valid (means use digest)
	}
	// Tags can contain lowercase/uppercase letters, digits, underscores, periods, and hyphens
	// Must not start with a period or hyphen
	// Max length 128 characters
	if len(tag) > 128 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]*$`, tag)
	return matched
}

// isValidDigest checks if a digest follows the format algorithm:hex
func isValidDigest(digest string) bool {
	// Format: algorithm:hexstring
	// Example: sha256:abcdef1234567890...
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) != 2 {
		return false
	}
	algorithm := parts[0]
	hexString := parts[1]

	// Common algorithms: sha256, sha512
	validAlgorithms := map[string]bool{
		"sha256": true,
		"sha512": true,
	}
	if !validAlgorithms[algorithm] {
		return false
	}

	// Hex string should be lowercase hex characters
	matched, _ := regexp.MatchString(`^[a-f0-9]{32,}$`, hexString)
	return matched
}

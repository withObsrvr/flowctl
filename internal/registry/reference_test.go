package registry

import (
	"testing"
)

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantRegistry   string
		wantRepository string
		wantTag        string
		wantDigest     string
		wantErr        bool
	}{
		{
			name:           "simple image name",
			input:          "nginx",
			wantRegistry:   "docker.io",
			wantRepository: "library/nginx",
			wantTag:        "latest",
		},
		{
			name:           "image with tag",
			input:          "nginx:v1.21",
			wantRegistry:   "docker.io",
			wantRepository: "library/nginx",
			wantTag:        "v1.21",
		},
		{
			name:           "org/repo format",
			input:          "myorg/myapp",
			wantRegistry:   "docker.io",
			wantRepository: "myorg/myapp",
			wantTag:        "latest",
		},
		{
			name:           "org/repo with tag",
			input:          "myorg/myapp:v1.0.0",
			wantRegistry:   "docker.io",
			wantRepository: "myorg/myapp",
			wantTag:        "v1.0.0",
		},
		{
			name:           "ghcr.io registry",
			input:          "ghcr.io/withobsrvr/stellar-source:v1.2.3",
			wantRegistry:   "ghcr.io",
			wantRepository: "withobsrvr/stellar-source",
			wantTag:        "v1.2.3",
		},
		{
			name:           "gcr.io registry",
			input:          "gcr.io/myproject/myapp:latest",
			wantRegistry:   "gcr.io",
			wantRepository: "myproject/myapp",
			wantTag:        "latest",
		},
		{
			name:           "localhost with port",
			input:          "localhost:5000/myapp:v1.0.0",
			wantRegistry:   "localhost:5000",
			wantRepository: "myapp",
			wantTag:        "v1.0.0",
		},
		{
			name:           "digest reference",
			input:          "ghcr.io/myorg/myapp@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantRegistry:   "ghcr.io",
			wantRepository: "myorg/myapp",
			wantTag:        "",
			wantDigest:     "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:    "empty reference",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid digest",
			input:   "nginx@sha256:invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImageReference(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Registry != tt.wantRegistry {
				t.Errorf("Registry = %v, want %v", got.Registry, tt.wantRegistry)
			}
			if got.Repository != tt.wantRepository {
				t.Errorf("Repository = %v, want %v", got.Repository, tt.wantRepository)
			}
			if got.Tag != tt.wantTag {
				t.Errorf("Tag = %v, want %v", got.Tag, tt.wantTag)
			}
			if got.Digest != tt.wantDigest {
				t.Errorf("Digest = %v, want %v", got.Digest, tt.wantDigest)
			}
		})
	}
}

func TestImageReference_String(t *testing.T) {
	tests := []struct {
		name string
		ref  *ImageReference
		want string
	}{
		{
			name: "with tag",
			ref: &ImageReference{
				Registry:   "ghcr.io",
				Repository: "myorg/myapp",
				Tag:        "v1.0.0",
			},
			want: "ghcr.io/myorg/myapp:v1.0.0",
		},
		{
			name: "with digest",
			ref: &ImageReference{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Digest:     "sha256:abc123",
			},
			want: "docker.io/library/nginx@sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.String(); got != tt.want {
				t.Errorf("ImageReference.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageReference_CacheKey(t *testing.T) {
	ref := &ImageReference{
		Registry:   "ghcr.io",
		Repository: "withobsrvr/stellar-source",
		Tag:        "v1.2.3",
	}

	got := ref.CacheKey()
	want := "ghcr.io_withobsrvr_stellar-source_v1.2.3"

	if got != want {
		t.Errorf("CacheKey() = %v, want %v", got, want)
	}
}

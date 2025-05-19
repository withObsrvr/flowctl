package generators

import (
	"io"

	"github.com/withobsrvr/flowctl/internal/translator/models"
)

// Generator defines the interface for deployment generators
type Generator interface {
	// Generate generates a deployment configuration for a pipeline
	Generate(pipeline *models.Pipeline, writer io.Writer) error
}
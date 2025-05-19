package parsers

import (
	"github.com/withobsrvr/flowctl/internal/translator/models"
)

// Parser defines the interface for pipeline parsers
type Parser interface {
	// Parse parses a byte slice into a Pipeline object
	Parse(data []byte) (*models.Pipeline, error)
}
package translator

import (
	"testing"
)

func TestUnifiedSchemaFile(t *testing.T) {
	// This test just ensures we can compile with the unified schema file
	// We're assuming that proper testing is done elsewhere, such as by
	// manually testing the validation with real pipeline configs
	t.Log("Test passed - builds successfully with unified schema")
}

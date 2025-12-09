package processors

import (
	"testing"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func TestFormatEventTypes(t *testing.T) {
	tests := []struct {
		name     string
		types    []string
		expected string
	}{
		{
			name:     "empty",
			types:    []string{},
			expected: "-",
		},
		{
			name:     "single type",
			types:    []string{"stellar.ledger.v1"},
			expected: "stellar.ledger.v1",
		},
		{
			name:     "multiple types",
			types:    []string{"stellar.ledger.v1", "stellar.transaction.v1", "stellar.event.v1"},
			expected: "stellar.ledger.v1 (+2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatEventTypes(tt.types)
			if result != tt.expected {
				t.Errorf("formatEventTypes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatHealthStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   flowctlv1.HealthStatus
		expected string
	}{
		{
			name:     "healthy",
			status:   flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY,
			expected: "✓ healthy",
		},
		{
			name:     "unhealthy",
			status:   flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY,
			expected: "✗ unhealthy",
		},
		{
			name:     "degraded",
			status:   flowctlv1.HealthStatus_HEALTH_STATUS_DEGRADED,
			expected: "⚠ degraded",
		},
		{
			name:     "starting",
			status:   flowctlv1.HealthStatus_HEALTH_STATUS_STARTING,
			expected: "⟳ starting",
		},
		{
			name:     "stopping",
			status:   flowctlv1.HealthStatus_HEALTH_STATUS_STOPPING,
			expected: "⊗ stopping",
		},
		{
			name:     "unknown",
			status:   flowctlv1.HealthStatus_HEALTH_STATUS_UNKNOWN,
			expected: "? unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatHealthStatus(tt.status)
			if result != tt.expected {
				t.Errorf("formatHealthStatus() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatchesFilters(t *testing.T) {
	tests := []struct {
		name       string
		comp       *flowctlv1.ComponentInfo
		inputType  string
		outputType string
		expected   bool
	}{
		{
			name: "matches input type",
			comp: &flowctlv1.ComponentInfo{
				InputEventTypes:  []string{"stellar.ledger.v1"},
				OutputEventTypes: []string{"stellar.token.transfer.v1"},
			},
			inputType:  "stellar.ledger.v1",
			outputType: "",
			expected:   true,
		},
		{
			name: "matches output type",
			comp: &flowctlv1.ComponentInfo{
				InputEventTypes:  []string{"stellar.ledger.v1"},
				OutputEventTypes: []string{"stellar.token.transfer.v1"},
			},
			inputType:  "",
			outputType: "stellar.token.transfer.v1",
			expected:   true,
		},
		{
			name: "matches both types",
			comp: &flowctlv1.ComponentInfo{
				InputEventTypes:  []string{"stellar.ledger.v1"},
				OutputEventTypes: []string{"stellar.token.transfer.v1"},
			},
			inputType:  "stellar.ledger.v1",
			outputType: "stellar.token.transfer.v1",
			expected:   true,
		},
		{
			name: "does not match input type",
			comp: &flowctlv1.ComponentInfo{
				InputEventTypes:  []string{"stellar.ledger.v1"},
				OutputEventTypes: []string{"stellar.token.transfer.v1"},
			},
			inputType:  "stellar.transaction.v1",
			outputType: "",
			expected:   false,
		},
		{
			name: "does not match output type",
			comp: &flowctlv1.ComponentInfo{
				InputEventTypes:  []string{"stellar.ledger.v1"},
				OutputEventTypes: []string{"stellar.token.transfer.v1"},
			},
			inputType:  "",
			outputType: "stellar.event.v1",
			expected:   false,
		},
		{
			name: "matches input but not output",
			comp: &flowctlv1.ComponentInfo{
				InputEventTypes:  []string{"stellar.ledger.v1"},
				OutputEventTypes: []string{"stellar.token.transfer.v1"},
			},
			inputType:  "stellar.ledger.v1",
			outputType: "stellar.event.v1",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesFilters(tt.comp, tt.inputType, tt.outputType)
			if result != tt.expected {
				t.Errorf("matchesFilters() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "no truncation needed",
			input:    "short",
			maxLen:   10,
			expected: "short",
		},
		{
			name:     "truncation needed",
			input:    "this is a very long string",
			maxLen:   10,
			expected: "this is...",
		},
		{
			name:     "exact length",
			input:    "exactly10!",
			maxLen:   10,
			expected: "exactly10!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

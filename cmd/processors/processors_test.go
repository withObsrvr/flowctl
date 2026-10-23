package processors

import (
	"testing"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
)

func TestSummariseTypes(t *testing.T) {
	tests := []struct {
		name     string
		types    any
		expected string
	}{
		{"nil", nil, "-"},
		{"empty", []string{}, "-"},
		{"single type", []string{"stellar.ledger.v1"}, "stellar.ledger.v1"},
		{
			name:     "multiple types are condensed for the table",
			types:    []string{"stellar.ledger.v1", "stellar.transaction.v1", "stellar.event.v1"},
			expected: "stellar.ledger.v1 (+2)",
		},
		{
			// CLI-22: a type string comes from a registered component.
			name:     "control characters are stripped",
			types:    []string{"evil\x1b[2K.v1"},
			expected: "evil[2K.v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summariseTypes(tt.types); got != tt.expected {
				t.Errorf("summariseTypes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHealthName(t *testing.T) {
	// These strings appear in --output json and are therefore a contract:
	// changing one is a breaking change under VER-3.
	tests := []struct {
		name     string
		status   flowctlv1.HealthStatus
		expected string
	}{
		{"healthy", flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY, "healthy"},
		{"unhealthy", flowctlv1.HealthStatus_HEALTH_STATUS_UNHEALTHY, "unhealthy"},
		{"degraded", flowctlv1.HealthStatus_HEALTH_STATUS_DEGRADED, "degraded"},
		{"starting", flowctlv1.HealthStatus_HEALTH_STATUS_STARTING, "starting"},
		{"stopping", flowctlv1.HealthStatus_HEALTH_STATUS_STOPPING, "stopping"},
		{"unknown", flowctlv1.HealthStatus_HEALTH_STATUS_UNKNOWN, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthName(tt.status); got != tt.expected {
				t.Errorf("healthName() = %v, want %v", got, tt.expected)
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

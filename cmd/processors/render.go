package processors

import (
	"fmt"
	"strings"

	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"github.com/withobsrvr/flowctl/internal/cliout"
)

// Schema identifiers for this command group's machine-readable output. These
// are contracts: changing a shape is a breaking change under VER-3.
const (
	processorListSchema = "flowctl.processor_list.v1"
	processorSchema     = "flowctl.processor.v1"
)

// componentRow converts a component into a record.
//
// Machine output carries the full values — every event type, the status as a
// stable lowercase string — while the table columns below condense them. The
// table is not a contract and may be shortened freely; the record may not.
func componentRow(comp *flowctlv1.ComponentStatusResponse) cliout.Row {
	info := comp.GetComponent()
	row := cliout.Row{
		"id":                 info.GetId(),
		"type":               componentTypeName(info.GetType()),
		"status":             healthName(comp.GetStatus()),
		"healthy":            comp.GetStatus() == flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY,
		"input_event_types":  info.GetInputEventTypes(),
		"output_event_types": info.GetOutputEventTypes(),
	}
	if v := info.GetName(); v != "" {
		row["name"] = v
	}
	if v := info.GetVersion(); v != "" {
		row["version"] = v
	}
	if v := info.GetDescription(); v != "" {
		row["description"] = v
	}
	if v := info.GetEndpoint(); v != "" {
		row["endpoint"] = v
	}
	if len(info.GetMetadata()) > 0 {
		row["metadata"] = info.GetMetadata()
	}
	if ts := comp.GetLastHeartbeat(); ts != nil {
		row["last_heartbeat"] = ts.AsTime()
	}
	if ts := comp.GetRegisteredAt(); ts != nil {
		row["registered_at"] = ts.AsTime()
	}
	return row
}

// processorColumns is the table layout shared by `list` and `find`.
func processorColumns() []cliout.Column {
	return []cliout.Column{
		{Header: "ID", Key: "id"},
		{Header: "Input Types", Key: "input_event_types", Format: summariseTypes},
		{Header: "Output Types", Key: "output_event_types", Format: summariseTypes},
		{Header: "Status", Key: "status"},
		{Header: "Endpoint", Key: "endpoint"},
	}
}

// summariseTypes condenses an event-type list for a table cell. The record
// keeps the full list.
func summariseTypes(v any) string {
	types, ok := v.([]string)
	if !ok || len(types) == 0 {
		return "-"
	}
	if len(types) == 1 {
		return cliout.Value(types[0])
	}
	return fmt.Sprintf("%s (+%d)", cliout.Value(types[0]), len(types)-1)
}

// healthName renders a health status as a stable lowercase string. Machine
// consumers match on this, so the values are part of the schema.
func healthName(status flowctlv1.HealthStatus) string {
	name := strings.TrimPrefix(status.String(), "HEALTH_STATUS_")
	if name == "" || name == "UNSPECIFIED" {
		return "unknown"
	}
	return strings.ToLower(name)
}

// componentTypeName renders a component type as a stable lowercase string.
func componentTypeName(t flowctlv1.ComponentType) string {
	name := strings.TrimPrefix(t.String(), "COMPONENT_TYPE_")
	if name == "" || name == "UNSPECIFIED" {
		return "unknown"
	}
	return strings.ToLower(name)
}

// componentInfoRow is componentRow for a bare ComponentInfo, which is what
// DiscoverComponents returns — it carries no health status.
func componentInfoRow(info *flowctlv1.ComponentInfo) cliout.Row {
	row := cliout.Row{
		"id":                 info.GetId(),
		"type":               componentTypeName(info.GetType()),
		"input_event_types":  info.GetInputEventTypes(),
		"output_event_types": info.GetOutputEventTypes(),
	}
	if v := info.GetName(); v != "" {
		row["name"] = v
	}
	if v := info.GetVersion(); v != "" {
		row["version"] = v
	}
	if v := info.GetDescription(); v != "" {
		row["description"] = v
	}
	if v := info.GetEndpoint(); v != "" {
		row["endpoint"] = v
	}
	if len(info.GetMetadata()) > 0 {
		row["metadata"] = info.GetMetadata()
	}
	return row
}

// discoveryColumns is the table layout for `find`, which has no health column.
func discoveryColumns() []cliout.Column {
	return []cliout.Column{
		{Header: "ID", Key: "id"},
		{Header: "Input Types", Key: "input_event_types", Format: summariseTypes},
		{Header: "Output Types", Key: "output_event_types", Format: summariseTypes},
		{Header: "Endpoint", Key: "endpoint"},
	}
}

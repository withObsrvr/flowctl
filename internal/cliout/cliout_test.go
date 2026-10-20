package cliout

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/withobsrvr/flowctl/internal/exitcode"
)

func newTestPrinter(format Format) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	var out, errw bytes.Buffer
	return New(format, &out, &errw), &out, &errw
}

func sampleList() List {
	return List{
		Schema:     "flowctl.service_list.v1",
		ItemSchema: "flowctl.service.v1",
		Columns: []Column{
			{Header: "ID", Key: "id"},
			{Header: "Type", Key: "type"},
		},
		Rows: []Row{
			{"id": "src-1", "type": "SOURCE"},
			{"id": "sink-1", "type": "SINK"},
		},
		Empty: "Nothing here.",
	}
}

// --- CLI-6: machine-readable shapes are contracts ------------------------------

func TestJSONListIsOneDocumentWithSchema(t *testing.T) {
	p, out, errw := newTestPrinter(FormatJSON)
	if err := p.List(sampleList()); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Schema string           `json:"schema"`
		Items  []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("output is not one JSON document: %v\ngot: %s", err, out.String())
	}
	if doc.Schema != "flowctl.service_list.v1" {
		t.Errorf("schema = %q", doc.Schema)
	}
	if len(doc.Items) != 2 || doc.Items[0]["id"] != "src-1" {
		t.Errorf("items = %#v", doc.Items)
	}
	if errw.Len() != 0 {
		t.Errorf("stderr should be empty, got %q", errw.String())
	}
}

func TestNDJSONIsOneObjectPerLineEachWithSchema(t *testing.T) {
	p, out, _ := newTestPrinter(FormatNDJSON)
	if err := p.List(sampleList()); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), out.String())
	}
	for i, line := range lines {
		var rec struct {
			Schema string         `json:"schema"`
			Data   map[string]any `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d is not JSON: %v", i, err)
		}
		if rec.Schema != "flowctl.service.v1" {
			t.Errorf("line %d schema = %q, want the item schema", i, rec.Schema)
		}
		if rec.Data["id"] == nil {
			t.Errorf("line %d lost its payload: %s", i, line)
		}
	}
}

func TestEmptyListStillEmitsWellFormedJSON(t *testing.T) {
	// A consumer must get [] rather than null, and the "nothing here" message
	// must not land on stdout where it would break parsing (CLI-5).
	p, out, errw := newTestPrinter(FormatJSON)
	l := sampleList()
	l.Rows = nil
	if err := p.List(l); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != `{"items":[],"schema":"flowctl.service_list.v1"}` {
		t.Errorf("got %s", got)
	}
	if errw.Len() != 0 {
		t.Errorf("stderr should be empty in machine mode, got %q", errw.String())
	}
}

func TestEmptyListInTableModeWritesMessageToStderr(t *testing.T) {
	p, out, errw := newTestPrinter(FormatTable)
	l := sampleList()
	l.Rows = nil
	if err := p.List(l); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout should carry results only, got %q", out.String())
	}
	if !strings.Contains(errw.String(), "Nothing here.") {
		t.Errorf("stderr = %q", errw.String())
	}
}

func TestObjectCarriesSchemaInBothMachineFormats(t *testing.T) {
	for _, format := range []Format{FormatJSON, FormatNDJSON} {
		p, out, _ := newTestPrinter(format)
		err := p.Object(Object{
			Schema: "flowctl.version.v1",
			Fields: []Field{{Label: "version", Key: "version"}},
			Data:   Row{"version": "v0.0.12"},
		})
		if err != nil {
			t.Fatal(err)
		}
		var rec struct {
			Schema string         `json:"schema"`
			Data   map[string]any `json:"data"`
		}
		if err := json.Unmarshal(out.Bytes(), &rec); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if rec.Schema != "flowctl.version.v1" || rec.Data["version"] != "v0.0.12" {
			t.Errorf("%s: got %#v", format, rec)
		}
	}
}

// --- CLI-5: stdout carries results only ----------------------------------------

func TestInfoNeverReachesStdout(t *testing.T) {
	for _, format := range []Format{FormatTable, FormatJSON, FormatNDJSON} {
		p, out, errw := newTestPrinter(format)
		p.Info("connecting to %s", "localhost:8080")
		if out.Len() != 0 {
			t.Errorf("%s: Info wrote %q to stdout", format, out.String())
		}
		if !strings.Contains(errw.String(), "connecting to localhost:8080") {
			t.Errorf("%s: stderr = %q", format, errw.String())
		}
	}
}

// --- CLI-22: untrusted values cannot reach the terminal as control codes --------

func TestTableCellsAreSanitised(t *testing.T) {
	p, out, _ := newTestPrinter(FormatTable)
	err := p.List(List{
		Schema:     "flowctl.service_list.v1",
		ItemSchema: "flowctl.service.v1",
		Columns:    []Column{{Header: "ID", Key: "id"}},
		// A hostile component registering under this name.
		Rows: []Row{{"id": "svc\x1b]0;pwned\x07\nFAKE\tROW"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(out.String(), 0x1b) {
		t.Errorf("ESC reached the terminal: %q", out.String())
	}
	// One header line plus one data line; the forged row must not appear.
	if n := strings.Count(strings.TrimRight(out.String(), "\n"), "\n"); n != 1 {
		t.Errorf("expected 2 lines, got %d: %q", n+1, out.String())
	}
}

func TestJSONIsNotSanitisedButIsEscaped(t *testing.T) {
	// encoding/json escapes control characters, so they cannot reach the terminal
	// as control codes. Stripping them here would corrupt data the consumer is
	// entitled to receive intact.
	p, out, _ := newTestPrinter(FormatJSON)
	err := p.List(List{
		Schema:     "flowctl.service_list.v1",
		ItemSchema: "flowctl.service.v1",
		Rows:       []Row{{"id": "svc\x1b[2K"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(out.String(), 0x1b) {
		t.Errorf("raw ESC byte in JSON output: %q", out.String())
	}
	if !strings.Contains(out.String(), `\u001b`) {
		t.Errorf("expected the ESC to be u-escaped, got %q", out.String())
	}
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Items[0]["id"] != "svc\x1b[2K" {
		t.Errorf("JSON round-trip lost data: %q", doc.Items[0]["id"])
	}
}

// --- CLI-6: format parsing ------------------------------------------------------

func TestParseFormat(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Format
	}{
		{"", FormatTable},
		{"table", FormatTable},
		{"json", FormatJSON},
		{"ndjson", FormatNDJSON},
		{"  JSON  ", FormatJSON},
	} {
		got, err := ParseFormat(tc.in)
		if err != nil {
			t.Errorf("ParseFormat(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseFormatRejectsUnimplementedFormatsAsUsageErrors(t *testing.T) {
	// Falling back to a table when a script asked for YAML would feed it garbage.
	for _, in := range []string{"yaml", "wide", "xml", "nonsense"} {
		_, err := ParseFormat(in)
		if err == nil {
			t.Errorf("ParseFormat(%q) should fail", in)
			continue
		}
		if got := exitcode.Of(err); got != exitcode.Usage {
			t.Errorf("ParseFormat(%q) exit code = %d, want %d", in, got, exitcode.Usage)
		}
	}
}

// --- value rendering ------------------------------------------------------------

func TestValue(t *testing.T) {
	ts := time.Date(2026, 10, 20, 14, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	for _, tc := range []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "-"},
		{"empty string", "", "-"},
		{"string", "src-1", "src-1"},
		{"true", true, "yes"},
		{"false", false, "no"},
		// DATA-3: RFC 3339, UTC, Z suffix.
		{"time", ts, "2026-10-20T12:30:00Z"},
		{"zero time", time.Time{}, "-"},
		{"duration", 90 * time.Second, "1m30s"},
		{"float trims zeros", 12.50, "12.5"},
		{"float whole", 12.00, "12"},
		{"int", 42, "42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Value(tc.in); got != tc.want {
				t.Errorf("Value(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// --- CLI-7: colour --------------------------------------------------------------

func TestColorOffWhenNotATerminal(t *testing.T) {
	p, _, _ := newTestPrinter(FormatTable)
	if p.Color() {
		t.Error("colour should be off when stdout is a buffer")
	}
}

func TestColorOffInMachineFormats(t *testing.T) {
	for _, format := range []Format{FormatJSON, FormatNDJSON} {
		p, _, _ := newTestPrinter(format)
		if p.Color() {
			t.Errorf("%s: colour should never be on in a machine format", format)
		}
	}
}

func TestShouldColorHonoursNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if ShouldColor(nil) {
		t.Error("NO_COLOR set to empty string must still disable colour (no-color.org)")
	}
	t.Setenv("NO_COLOR", "1")
	if ShouldColor(nil) {
		t.Error("NO_COLOR must disable colour")
	}
}

func TestMachineReportsFormatClass(t *testing.T) {
	table, _, _ := newTestPrinter(FormatTable)
	if table.Machine() {
		t.Error("table is not a machine format")
	}
	for _, format := range []Format{FormatJSON, FormatNDJSON} {
		p, _, _ := newTestPrinter(format)
		if !p.Machine() {
			t.Errorf("%s should be a machine format", format)
		}
	}
}

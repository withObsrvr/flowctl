// Package cliout renders command results.
//
// It exists so that the rules in §3 of the OBSRVR Engineering Standard v2.0 are
// implemented once rather than in every command:
//
//   - CLI-5: stdout carries results only. Progress, hints and warnings go to stderr,
//     so `flowctl … | jq` always works.
//   - CLI-6: --output json emits one JSON document, --output ndjson one object per
//     line flushed as it is written, and both carry a schema identifier.
//   - CLI-7: colour and animation are off when stdout is not a terminal, when
//     NO_COLOR is set, or when TERM=dumb.
//   - CLI-22: values that came from a registered component are stripped of terminal
//     control characters before they reach a terminal.
//
// Human table output is deliberately not a contract and may change at any time. The
// JSON and NDJSON shapes are contracts: changing one is a breaking change under
// VER-3. See docs/adr/0001-cli-output-and-exit-code-contract.md.
package cliout

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/withobsrvr/flowctl/internal/exitcode"
	"github.com/withobsrvr/flowctl/internal/sanitize"
)

// Format is an output format selected by --output.
type Format string

const (
	// FormatTable is the human-readable default. Not a contract.
	FormatTable Format = "table"
	// FormatJSON emits a single JSON document.
	FormatJSON Format = "json"
	// FormatNDJSON emits one JSON object per line, flushed per line.
	FormatNDJSON Format = "ndjson"
)

// ParseFormat resolves the value of --output.
//
// Unknown values are a usage error rather than a silent fallback: a script that asked
// for JSON and got a table would otherwise parse garbage. "yaml" and "wide" were
// advertised by earlier versions and never implemented; they are named explicitly so
// the error can say so.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "table":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "ndjson":
		return FormatNDJSON, nil
	case "yaml", "wide":
		return "", exitcode.Errorf(exitcode.Usage,
			"output format %q is not implemented; use table, json or ndjson", s)
	default:
		return "", exitcode.Errorf(exitcode.Usage,
			"unknown output format %q; use table, json or ndjson", s)
	}
}

// Printer renders results to stdout and everything else to stderr.
type Printer struct {
	out    io.Writer
	errw   io.Writer
	format Format
	color  bool
}

// New returns a Printer writing results to out and messages to errw.
func New(format Format, out, errw io.Writer) *Printer {
	return &Printer{
		out:    out,
		errw:   errw,
		format: format,
		color:  format == FormatTable && ShouldColor(out),
	}
}

// Format reports the selected output format.
func (p *Printer) Format() Format { return p.format }

// Machine reports whether output is being produced for a program rather than a person.
// Use it to skip decorative work, never to change what data is emitted.
func (p *Printer) Machine() bool { return p.format != FormatTable }

// Color reports whether ANSI colour may be written.
func (p *Printer) Color() bool { return p.color }

// Out returns the result stream. Prefer List and Object; reach for this only when
// emitting something that is genuinely not a record, such as generated YAML.
func (p *Printer) Out() io.Writer { return p.out }

// Err returns the message stream.
func (p *Printer) Err() io.Writer { return p.errw }

// Info writes a human-facing message to stderr. It never appears on stdout, so it
// cannot corrupt piped output, and it is written in every format: a person watching a
// terminal still wants progress even when the result is JSON.
func (p *Printer) Info(format string, args ...any) {
	fmt.Fprintln(p.errw, sanitize.String(fmt.Sprintf(format, args...)))
}

// Row is one record. Keys are the field names used in JSON output and referenced by
// Column.Key and Field.Key.
type Row map[string]any

// Column describes one table column and the field it reads.
type Column struct {
	// Header is the column heading in table output.
	Header string
	// Key is the Row key to read.
	Key string
	// Format converts the value to a cell. Defaults to Value.
	Format func(any) string
}

// List is a collection result.
type List struct {
	// Schema identifies the collection, e.g. "flowctl.service_list.v1".
	Schema string
	// ItemSchema identifies one element, e.g. "flowctl.service.v1".
	ItemSchema string
	// Columns define table rendering, in order.
	Columns []Column
	// Rows are the records, already converted to plain values.
	Rows []Row
	// Empty is the message shown on a terminal when there are no rows. It goes to
	// stderr, because "nothing matched" is a message, not a result: machine output
	// still emits a well-formed empty collection.
	Empty string
}

// List renders a collection.
func (p *Printer) List(l List) error {
	switch p.format {
	case FormatJSON:
		return p.encode(map[string]any{
			"schema": l.Schema,
			"items":  itemsOf(l.Rows),
		})
	case FormatNDJSON:
		for _, row := range l.Rows {
			if err := p.encode(map[string]any{
				"schema": l.ItemSchema,
				"data":   map[string]any(row),
			}); err != nil {
				return err
			}
		}
		return nil
	default:
		if len(l.Rows) == 0 {
			if l.Empty != "" {
				p.Info("%s", l.Empty)
			}
			return nil
		}
		return p.table(l)
	}
}

// Field describes one labelled field of an object in table output.
type Field struct {
	Label  string
	Key    string
	Format func(any) string
}

// Object is a single-record result.
type Object struct {
	// Schema identifies the record, e.g. "flowctl.version.v1".
	Schema string
	// Fields define table rendering, in order.
	Fields []Field
	// Data is the record.
	Data Row
}

// Object renders a single record. In NDJSON it is a single line, which keeps
// `flowctl … --output ndjson` uniform regardless of arity.
func (p *Printer) Object(o Object) error {
	switch p.format {
	case FormatJSON, FormatNDJSON:
		return p.encode(map[string]any{
			"schema": o.Schema,
			"data":   map[string]any(o.Data),
		})
	default:
		// Pad to the longest label including its colon, so values line up.
		width := 0
		for _, f := range o.Fields {
			if n := len(f.Label) + 1; n > width {
				width = n
			}
		}
		for _, f := range o.Fields {
			format := f.Format
			if format == nil {
				format = Value
			}
			if _, err := fmt.Fprintf(p.out, "%-*s  %s\n",
				width, f.Label+":", format(o.Data[f.Key])); err != nil {
				return err
			}
		}
		return nil
	}
}

// table writes tab-aligned columns to stdout.
func (p *Printer) table(l List) error {
	w := tabwriter.NewWriter(p.out, 0, 0, 2, ' ', 0)

	headers := make([]string, len(l.Columns))
	for i, c := range l.Columns {
		headers[i] = strings.ToUpper(c.Header)
	}
	if _, err := fmt.Fprintln(w, strings.Join(headers, "\t")); err != nil {
		return err
	}

	for _, row := range l.Rows {
		cells := make([]string, len(l.Columns))
		for i, c := range l.Columns {
			format := c.Format
			if format == nil {
				format = Value
			}
			cells[i] = format(row[c.Key])
		}
		if _, err := fmt.Fprintln(w, strings.Join(cells, "\t")); err != nil {
			return err
		}
	}
	return w.Flush()
}

// encode writes one JSON document followed by a newline, flushing as it goes so that
// NDJSON consumers see each line as it is produced rather than at exit (CLI-6).
func (p *Printer) encode(v any) error {
	enc := json.NewEncoder(p.out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding output: %w", err)
	}
	if f, ok := p.out.(interface{ Sync() error }); ok {
		// Best effort: a closed or non-syncable stream is the caller's problem,
		// and failing the command over it would be worse than the missed flush.
		_ = f.Sync()
	}
	return nil
}

func itemsOf(rows []Row) []map[string]any {
	// Always a non-nil slice, so an empty collection marshals as [] and not null.
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any(r))
	}
	return items
}

// Value renders a field for table output.
//
// Every string passes through sanitize.Line, because most values reaching here were
// supplied by a registered component and are untrusted (CLI-22). Times render as
// RFC 3339 UTC to match DATA-3.
func Value(v any) string {
	switch t := v.(type) {
	case nil:
		return "-"
	case string:
		if t == "" {
			return "-"
		}
		return sanitize.Line(t)
	case time.Time:
		if t.IsZero() {
			return "-"
		}
		return t.UTC().Format(time.RFC3339)
	case *time.Time:
		if t == nil {
			return "-"
		}
		return Value(*t)
	case time.Duration:
		return t.String()
	case bool:
		if t {
			return "yes"
		}
		return "no"
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", t), "0"), ".")
	default:
		return sanitize.Line(fmt.Sprint(v))
	}
}

// ShouldColor reports whether ANSI colour may be written to w.
//
// Colour is off unless w is a terminal, and off regardless when NO_COLOR is set to
// any value (per no-color.org) or TERM is "dumb".
func ShouldColor(w io.Writer) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// IsTerminal reports whether w is an interactive terminal. Commands use it to decide
// whether prompting is allowed: CLI-8 requires a tool to run without prompts when
// stdin is not a terminal.
func IsTerminal(w any) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}

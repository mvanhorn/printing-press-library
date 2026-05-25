package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Format is the user-visible output mode.
type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
)

// Parse returns the validated Format or an error for unknown values.
func Parse(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "json", "":
		return FormatJSON, nil
	case "table":
		return FormatTable, nil
	default:
		return "", fmt.Errorf("unknown format %q (expected: json, table)", s)
	}
}

// Write prints v to stdout in the requested format.
//
// For table mode we expect either a typed "Tabler" or fall back to JSON
// — agents always get parseable output, never a "no table view" error.
func Write(v any, f Format) error {
	return WriteTo(os.Stdout, v, f)
}

// WriteTo is the io.Writer-aware variant used by tests.
func WriteTo(w io.Writer, v any, f Format) error {
	if f == FormatTable {
		if t, ok := v.(Tabler); ok {
			return writeTable(w, t)
		}
		// Soft fallback to JSON for un-tabled payloads.
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// WriteError emits a structured error in the same format as a success payload,
// so agents parsing stdout don't have to special-case stderr.
func WriteError(code, message string, f Format) {
	payload := map[string]any{
		"ok":      false,
		"error":   map[string]any{"code": code, "message": message},
		"message": message,
	}
	if f == FormatTable {
		fmt.Fprintf(os.Stderr, "Error: %s — %s\n", code, message)
		return
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

// Tabler is implemented by typed response wrappers that know how to render
// themselves as a 2D table.
type Tabler interface {
	TableHeaders() []string
	TableRows() [][]string
}

func writeTable(w io.Writer, t Tabler) error {
	headers := t.TableHeaders()
	rows := t.TableRows()
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	fmt.Fprintln(tw, strings.Repeat("-", 8)+strings.Repeat("\t"+strings.Repeat("-", 8), len(headers)-1))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	return tw.Flush()
}

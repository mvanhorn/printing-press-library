package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

// As is a thin alias so command files can unwrap *cliError without importing errors.
var As = errors.As

// noColor is set by --no-color; humanFriendly by --human-friendly. Colors are
// OFF by default so output is agent-safe.
var (
	noColor       bool
	humanFriendly bool
)

// colorEnabled reports whether ANSI color should be emitted. It honors the
// NO_COLOR convention and only colors a real terminal.
func colorEnabled() bool {
	if noColor || !humanFriendly {
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(os.Stdout)
}

// isTerminal is our isatty check: it reports whether w is a character device.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

func bold(s string) string   { return wrap(s, "1") }
func green(s string) string  { return wrap(s, "32") }
func red(s string) string    { return wrap(s, "31") }
func yellow(s string) string { return wrap(s, "33") }
func dim(s string) string    { return wrap(s, "2") }

func wrap(s, code string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

// cliError carries a process exit code alongside an error so different failures
// map to distinct, scriptable exit codes.
type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func usageErr(err error) error    { return &cliError{code: 2, err: err} }
func notFoundErr(err error) error { return &cliError{code: 3, err: err} }
func dataErr(err error) error     { return &cliError{code: 5, err: err} }
func configErr(err error) error   { return &cliError{code: 10, err: err} }

// ExitCode extracts the intended process exit code from an error.
func ExitCode(err error) int {
	var ce *cliError
	if As(err, &ce) {
		return ce.code
	}
	if err != nil {
		return 1
	}
	return 0
}

// --- Output formatting -------------------------------------------------------

// emitJSON marshals v to indented JSON on w, optionally narrowing to selected
// fields. It is the JSON path shared by every command.
func emitJSON(w io.Writer, v any, selectFields string, compact bool) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if selectFields != "" {
		raw = filterFields(raw, selectFields)
	} else if compact {
		raw = compactFields(raw)
	}
	var pretty json.RawMessage = raw
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(pretty)
}

// emitCSV renders an array-of-objects value as CSV (header + rows). Non-array
// values fall back to JSON.
func emitCSV(w io.Writer, v any) error {
	raw, _ := json.Marshal(v)
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		return emitJSON(w, v, "", false)
	}
	keys := unionKeys(items)
	fmt.Fprintln(w, strings.Join(keys, ","))
	for _, it := range items {
		cells := make([]string, len(keys))
		for i, k := range keys {
			cells[i] = csvCell(it[k])
		}
		fmt.Fprintln(w, strings.Join(cells, ","))
	}
	return nil
}

// emitTSV renders array data as a tab-separated table; single objects as
// key<TAB>value lines (used by --plain).
func emitTSV(w io.Writer, v any) error {
	raw, _ := json.Marshal(v)
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
		keys := unionKeys(items)
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, strings.Join(keys, "\t"))
		for _, it := range items {
			cells := make([]string, len(keys))
			for i, k := range keys {
				cells[i] = plainCell(it[k])
			}
			fmt.Fprintln(tw, strings.Join(cells, "\t"))
		}
		return tw.Flush()
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		keys := sortedKeys(obj)
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		for _, k := range keys {
			fmt.Fprintf(tw, "%s\t%s\n", k, plainCell(obj[k]))
		}
		return tw.Flush()
	}
	fmt.Fprintln(w, string(raw))
	return nil
}

func unionKeys(items []map[string]any) []string {
	set := map[string]bool{}
	for _, it := range items {
		for k := range it {
			set[k] = true
		}
	}
	return sortedSet(set)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedSet(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func csvCell(v any) string {
	s := scalar(v)
	if strings.ContainsAny(s, ",\"\n") {
		s = `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func plainCell(v any) string {
	return strings.ReplaceAll(strings.ReplaceAll(scalar(v), "\t", " "), "\n", " ")
}

func scalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case bool:
		return fmt.Sprintf("%t", x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// filterFields keeps only the named top-level fields (comma-separated) from JSON
// objects/arrays. Case-insensitive.
func filterFields(data json.RawMessage, fields string) json.RawMessage {
	want := map[string]bool{}
	for _, f := range strings.Split(fields, ",") {
		if f = strings.TrimSpace(strings.ToLower(f)); f != "" {
			want[f] = true
		}
	}
	if len(want) == 0 {
		return data
	}
	var arr []json.RawMessage
	if json.Unmarshal(data, &arr) == nil {
		out := make([]json.RawMessage, len(arr))
		for i, el := range arr {
			out[i] = filterFields(el, fields)
		}
		b, _ := json.Marshal(out)
		return b
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) == nil {
		keep := map[string]json.RawMessage{}
		for k, v := range obj {
			if want[strings.ToLower(k)] {
				keep[k] = v
			}
		}
		b, _ := json.Marshal(keep)
		return b
	}
	return data
}

// compactStrip is the set of verbose fields dropped by --compact.
var compactStrip = map[string]bool{"notes": true, "detail": true, "reasons": true, "cautions": true, "field_hints": true}

// compactFields drops known-verbose fields for minimal agent token usage. It
// recurses into nested objects and arrays so verbose fields are stripped at
// every level (e.g. a recommendation's nested provider.notes), not just the top.
func compactFields(data json.RawMessage) json.RawMessage {
	var arr []json.RawMessage
	if json.Unmarshal(data, &arr) == nil {
		out := make([]json.RawMessage, len(arr))
		for i, el := range arr {
			out[i] = compactFields(el)
		}
		b, _ := json.Marshal(out)
		return b
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) == nil {
		kept := make(map[string]json.RawMessage, len(obj))
		for k, v := range obj {
			if compactStrip[strings.ToLower(k)] {
				continue
			}
			kept[k] = compactFields(v)
		}
		b, _ := json.Marshal(kept)
		return b
	}
	return data
}

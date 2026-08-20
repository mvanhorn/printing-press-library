// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

func printJSON(v any) error {
	return emitOutput(os.Stdout, v, &rootFlags{asJSON: true})
}

func emitOutput(w io.Writer, v any, flags *rootFlags) error {
	if flags == nil {
		flags = &rootFlags{}
	}
	if flags.asJSON {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		}
		if flags.compact {
			data = compactObjectFields(data)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		var decoded any
		if err := json.Unmarshal(data, &decoded); err == nil {
			return enc.Encode(decoded)
		}
		return enc.Encode(v)
	}
	if flags.asCSV {
		return writeCSV(w, v)
	}
	if flags.plain {
		return writePlain(w, v)
	}
	if flags.quiet {
		return nil
	}
	if rows, ok := v.([]map[string]string); ok && len(rows) > 0 {
		tw := newTabWriter(w)
		first := true
		for _, row := range rows {
			if first {
				for k := range row {
					fmt.Fprintf(tw, "%s\t", k)
				}
				fmt.Fprintln(tw)
				first = false
			}
			for _, k := range sortedKeys(row) {
				fmt.Fprintf(tw, "%s\t", row[k])
			}
			fmt.Fprintln(tw)
		}
		return tw.Flush()
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writePlain(w io.Writer, v any) error {
	switch t := v.(type) {
	case string:
		_, err := fmt.Fprintln(w, t)
		return err
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	}
}

func writeCSV(w io.Writer, v any) error {
	rows, ok := v.([]map[string]any)
	if !ok {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var arr []map[string]any
		if err := json.Unmarshal(b, &arr); err != nil {
			return fmt.Errorf("csv output requires an array of objects")
		}
		rows = arr
	}
	cw := csv.NewWriter(w)
	if len(rows) == 0 {
		cw.Flush()
		return cw.Error()
	}
	headers := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		headers = append(headers, k)
	}
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		rec := make([]string, len(headers))
		for i, h := range headers {
			rec[i] = fmt.Sprint(row[h])
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// filterFields keeps only comma-separated top-level JSON fields using json.Unmarshal.
func filterFields(data []byte, fields string) []byte {
	fields = strings.TrimSpace(fields)
	if fields == "" {
		return data
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return data
	}
	want := map[string]bool{}
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			want[f] = true
		}
	}
	filtered := filterValue(decoded, want)
	out, err := json.Marshal(filtered)
	if err != nil {
		return data
	}
	return out
}

func filterValue(v any, want map[string]bool) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(want))
		for k, val := range t {
			if want[k] {
				out[k] = val
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, el := range t {
			out[i] = filterValue(el, want)
		}
		return out
	default:
		return v
	}
}

// compactObjectFields removes verbose metadata keys from JSON objects.
func compactObjectFields(data []byte) []byte {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return data
	}
	compact := stripVerboseFields(decoded)
	out, err := json.Marshal(compact)
	if err != nil {
		return data
	}
	return out
}

func stripVerboseFields(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isVerboseField(k) {
				continue
			}
			out[k] = stripVerboseFields(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, el := range t {
			out[i] = stripVerboseFields(el)
		}
		return out
	default:
		return v
	}
}

func isVerboseField(name string) bool {
	switch strings.ToLower(name) {
	case "payload", "raw", "metadata", "debug", "internal":
		return true
	default:
		return false
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return true
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

var errDryRun = errors.New("dry-run")

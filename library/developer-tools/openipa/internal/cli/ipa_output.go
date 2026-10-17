// Hand-written addition: output flags on IPA responses — preserve on regeneration.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// splitIPAEnvelope recognizes the wrapper every IPA web service returns:
// {"result":{"cod_err":0,"desc_err":"","num_items":N},"data":<payload>}.
// Output flags (--csv, --plain, --select, --compact) must act on <payload>.
func splitIPAEnvelope(data json.RawMessage) (map[string]json.RawMessage, json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil || len(obj) != 2 {
		return nil, nil, false
	}
	result, okResult := obj["result"]
	inner, okData := obj["data"]
	if !okResult || !okData {
		return nil, nil, false
	}
	var r struct {
		CodErr *int `json:"cod_err"`
	}
	if json.Unmarshal(result, &r) != nil || r.CodErr == nil {
		return nil, nil, false
	}
	return obj, inner, true
}

// ipaPayload returns the payload of an IPA envelope (an empty list when it is
// null), or data unchanged when it is not an envelope.
func ipaPayload(data json.RawMessage) json.RawMessage {
	_, inner, ok := splitIPAEnvelope(data)
	if !ok {
		return data
	}
	if strings.TrimSpace(string(inner)) == "null" {
		return json.RawMessage("[]")
	}
	return inner
}

// withIPAPayload applies fn to the payload of an IPA envelope and keeps the
// wrapper, so the JSON shape that documented jq recipes rely on is unchanged.
func withIPAPayload(data json.RawMessage, fn func(json.RawMessage) json.RawMessage) json.RawMessage {
	obj, inner, ok := splitIPAEnvelope(data)
	if !ok {
		return fn(data)
	}
	if strings.TrimSpace(string(inner)) == "null" {
		return data
	}
	obj["data"] = fn(inner)
	out, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return out
}

// tabularRows turns a list of objects, or a single object, into table rows.
func tabularRows(data json.RawMessage) ([]map[string]any, bool) {
	var items []map[string]any
	if json.Unmarshal(data, &items) == nil {
		return items, true
	}
	var obj map[string]any
	if json.Unmarshal(data, &obj) == nil && obj != nil {
		return []map[string]any{obj}, true
	}
	return nil, false
}

func rowKeys(items []map[string]any) []string {
	keySet := map[string]bool{}
	for _, item := range items {
		for k := range item {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// cellValue renders one cell: nested objects and arrays as JSON, not Go literals.
func cellValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case map[string]any, []any:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	default:
		return fmt.Sprint(t)
	}
}

// printCSV renders rows as CSV with a header row.
func printCSV(w io.Writer, data json.RawMessage) error {
	items, ok := tabularRows(data)
	if !ok {
		fmt.Fprintln(w, string(data))
		return nil
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stderr, "nessun risultato")
		return nil
	}
	keys := rowKeys(items)
	cw := csv.NewWriter(w)
	if err := cw.Write(keys); err != nil {
		return err
	}
	for _, item := range items {
		vals := make([]string, len(keys))
		for i, k := range keys {
			vals[i] = cellValue(item[k])
		}
		if err := cw.Write(vals); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// printPlain renders rows as tab-separated text with a header row.
func printPlain(w io.Writer, data json.RawMessage) error {
	items, ok := tabularRows(data)
	if !ok {
		fmt.Fprintln(w, string(data))
		return nil
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stderr, "nessun risultato")
		return nil
	}
	keys := rowKeys(items)
	clean := strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")
	if _, err := fmt.Fprintln(w, strings.Join(keys, "\t")); err != nil {
		return err
	}
	for _, item := range items {
		vals := make([]string, len(keys))
		for i, k := range keys {
			vals[i] = clean.Replace(cellValue(item[k]))
		}
		if _, err := fmt.Fprintln(w, strings.Join(vals, "\t")); err != nil {
			return err
		}
	}
	return nil
}

// wantsFlatOutput reports whether a format flag must bypass a command's own
// JSON envelope or human text and go through printOutputWithFlags. Callers
// that keep a dry-run envelope still route --quiet, which suppresses it.
func wantsFlatOutput(flags *rootFlags) bool {
	return flags.csv || flags.plain || flags.quiet
}

// printStructured renders a hand-written command's result for every
// non-human output mode: --csv/--plain/--quiet through printOutputWithFlags,
// --json (or a pipe) as indented JSON with --select/--compact applied.
// It returns false when the caller should print its human view instead.
func printStructured(w io.Writer, flags *rootFlags, v any) (bool, error) {
	flat := wantsFlatOutput(flags)
	if !flat && !flags.asJSON && isTerminal(w) {
		return false, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return true, err
	}
	if flat {
		return true, printOutputWithFlags(w, data, flags)
	}
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	return true, printOutput(w, data, true)
}

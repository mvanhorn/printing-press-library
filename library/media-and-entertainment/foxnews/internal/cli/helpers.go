package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"unicode"
)

func dryRunOK(flags *rootFlags) bool {
	return flags != nil && flags.dryRun
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

// wantsHumanTable follows the catalog smart default: table in an interactive
// terminal, JSON when stdout is piped (agent/bash tool) or a machine flag is set.
func wantsHumanTable(w io.Writer, flags *rootFlags) bool {
	if flags.asJSON || flags.compact || flags.quiet {
		return false
	}
	if flags.selectFields != "" {
		return false
	}
	return isTerminal(w)
}

func printJSON(w io.Writer, v any, indent bool) error {
	enc := json.NewEncoder(w)
	if indent {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

func filterSelect(data json.RawMessage, selectFields string) json.RawMessage {
	selectFields = strings.TrimSpace(selectFields)
	if selectFields == "" {
		return data
	}
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		var obj map[string]any
		if err2 := json.Unmarshal(data, &obj); err2 != nil {
			return data
		}
		filtered := pickFields(obj, selectFields)
		out, _ := json.Marshal(filtered)
		return out
	}
	keep := parseSelectFields(selectFields)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{}
		for k, v := range item {
			if keep[strings.ToLower(k)] || keep[camelToKebab(k)] {
				row[k] = v
			}
		}
		out = append(out, row)
	}
	raw, _ := json.Marshal(out)
	return raw
}

func parseSelectFields(raw string) map[string]bool {
	keep := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			keep[part] = true
		}
	}
	return keep
}

func pickFields(obj map[string]any, selectFields string) map[string]any {
	keep := parseSelectFields(selectFields)
	out := map[string]any{}
	for k, v := range obj {
		if keep[strings.ToLower(k)] || keep[camelToKebab(k)] {
			out[k] = v
		}
	}
	return out
}

func camelToKebab(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			if unicode.IsLower(runes[i-1]) {
				b.WriteByte('-')
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				end := i
				for end+1 < len(runes) && unicode.IsUpper(runes[end+1]) {
					end++
				}
				if i == end {
					b.WriteByte('-')
				}
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func wrapAPIErr(err error) error {
	if err == nil {
		return nil
	}
	var codeErr *cliError
	if errors.As(err, &codeErr) {
		return err
	}
	return apiErr(err)
}

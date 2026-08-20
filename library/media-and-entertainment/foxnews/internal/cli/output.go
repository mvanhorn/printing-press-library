package cli

import (
	"encoding/json"
	"io"
	"strings"
)

// responseEnvelope matches the Printing Press agent contract documented in SKILL.md.
type responseEnvelope struct {
	Meta    map[string]any `json:"meta"`
	Results any            `json:"results"`
}

// headlineCompactKeep is the high-gravity field set for --compact / --agent.
var headlineCompactKeep = map[string]bool{
	"title":     true,
	"link":      true,
	"published": true,
	"section":   true,
}

var sectionCompactKeep = map[string]bool{
	"id":    true,
	"label": true,
	"path":  true,
}

func compactResultItems(raw json.RawMessage) (json.RawMessage, error) {
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return raw, nil
	}
	if len(items) == 0 {
		return raw, nil
	}
	if _, ok := items[0]["title"]; ok {
		return compactItemList(items, headlineCompactKeep)
	}
	if _, ok := items[0]["id"]; ok {
		return compactItemList(items, sectionCompactKeep)
	}
	return raw, nil
}

func compactItemList(items []map[string]any, keep map[string]bool) (json.RawMessage, error) {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{}
		for k, v := range item {
			if keep[strings.ToLower(k)] {
				row[k] = v
			}
		}
		out = append(out, row)
	}
	return json.Marshal(out)
}

// printMachineOutput emits enveloped JSON for agents (meta + results).
func printMachineOutput(w io.Writer, flags *rootFlags, meta map[string]any, results any) error {
	raw, err := json.Marshal(results)
	if err != nil {
		return err
	}
	processed, err := processResultJSON(raw, flags)
	if err != nil {
		return err
	}
	var parsed any
	if err := json.Unmarshal(processed, &parsed); err != nil {
		return err
	}
	payload := responseEnvelope{Meta: meta, Results: parsed}
	return printJSON(w, payload, !flags.compact)
}

func processResultJSON(raw json.RawMessage, flags *rootFlags) (json.RawMessage, error) {
	if flags.selectFields != "" {
		return filterSelect(raw, flags.selectFields), nil
	}
	if flags.compact {
		return compactResultItems(raw)
	}
	return raw, nil
}

package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Flags struct {
	JSON    bool
	Select  string
	Compact bool
	CSV     bool
	Quiet   bool
	Limit   int
	Command string
}

func DefaultToJSONIfNotTTY(flags *Flags) {
	if flags.JSON || flags.CSV {
		return
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return
	}
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		flags.JSON = true
	}
}

func Render(flags Flags, data any) error {
	if flags.Quiet {
		return nil
	}
	data = applyCompact(flags, data)
	filtered, err := applySelect(flags.Select, data)
	if err != nil {
		return err
	}
	if flags.CSV {
		return renderCSV(filtered)
	}
	if flags.JSON {
		return renderJSON(filtered)
	}
	return renderTable(filtered)
}

func applyCompact(flags Flags, data any) any {
	if !flags.Compact || strings.TrimSpace(flags.Select) != "" {
		return data
	}
	proj, ok := compactByCommand[strings.TrimSpace(flags.Command)]
	if !ok {
		return data
	}
	return projectCompact(data, proj)
}

var compactByCommand = map[string]compactProjection{
	"search":    {arrayFields: []string{"url", "title", "last_visit_time", "origin"}},
	"list":      {arrayFields: []string{"url", "title", "last_visit", "origin"}},
	"topic":     {arrayFields: []string{"url", "title", "when"}},
	"domains":   {arrayFields: []string{"domain", "visit_sum", "category"}},
	"journeys":  {arrayFields: []string{"label", "page_count"}},
	"downloads": {arrayFields: []string{"filename", "target_path", "when"}},
	"visited":   {arrayFields: []string{"target", "total_visits", "last_seen"}},
	"dwell":     {objectArrayField: "rows", arrayFields: []string{"domain", "estimated_dwell_seconds"}},
	// report emits per_day + hour_of_day (its primary breakdowns); peak_hours/
	// busiest_weekday/totals are profile-only and must not stand in for them.
	"report": {
		objectFields: []string{"productivity_split", "per_day", "hour_of_day", "top_domains"},
	},
	"profile": {
		objectFields: []string{"productivity_split", "peak_hours", "busiest_weekday", "totals", "top_search_terms", "top_domains"},
	},
}

type compactProjection struct {
	arrayFields      []string
	objectFields     []string
	objectArrayField string
}

func projectCompact(data any, p compactProjection) any {
	if arr, ok := data.([]map[string]any); ok {
		return compactArray(arr, p.arrayFields)
	}
	obj, ok := data.(map[string]any)
	if !ok {
		return data
	}
	if len(p.objectFields) > 0 {
		return compactObject(obj, p.objectFields)
	}
	if p.objectArrayField != "" {
		out := map[string]any{}
		if arrAny, ok := obj[p.objectArrayField]; ok {
			if arr, ok := arrAny.([]map[string]any); ok {
				out[p.objectArrayField] = compactArray(arr, p.arrayFields)
				return out
			}
			if arrAny2, ok := arrAny.([]any); ok {
				rows := make([]map[string]any, 0, len(arrAny2))
				for _, item := range arrAny2 {
					if row, ok := item.(map[string]any); ok {
						rows = append(rows, row)
					}
				}
				out[p.objectArrayField] = compactArray(rows, p.arrayFields)
				return out
			}
		}
		return out
	}
	return data
}

func compactArray(arr []map[string]any, fields []string) []map[string]any {
	if len(fields) == 0 {
		return arr
	}
	out := make([]map[string]any, 0, len(arr))
	for _, row := range arr {
		n := map[string]any{}
		for _, f := range fields {
			if v, ok := row[f]; ok {
				n[f] = v
			}
		}
		out = append(out, n)
	}
	return out
}

func compactObject(obj map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		if v, ok := obj[f]; ok {
			out[f] = v
		}
	}
	return out
}

func renderJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func renderCSV(v any) error {
	rows, ok := v.([]map[string]any)
	if !ok {
		return fmt.Errorf("csv output requires []map[string]any")
	}
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()
	keys := sortedKeys(rows)
	if err := w.Write(keys); err != nil {
		return err
	}
	for _, r := range rows {
		line := make([]string, 0, len(keys))
		for _, k := range keys {
			line = append(line, fmt.Sprint(r[k]))
		}
		if err := w.Write(line); err != nil {
			return err
		}
	}
	return nil
}

func renderTable(v any) error {
	switch t := v.(type) {
	case []map[string]any:
		keys := sortedKeys(t)
		fmt.Println(strings.Join(keys, "\t"))
		for _, r := range t {
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				parts = append(parts, fmt.Sprint(r[k]))
			}
			fmt.Println(strings.Join(parts, "\t"))
		}
		return nil
	default:
		b, _ := json.Marshal(t)
		fmt.Println(string(b))
		return nil
	}
}

func sortedKeys(rows []map[string]any) []string {
	if len(rows) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func applySelect(sel string, data any) (any, error) {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return data, nil
	}
	paths := strings.Split(sel, ",")
	trimmed := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			trimmed = append(trimmed, p)
		}
	}
	if len(trimmed) == 0 {
		return data, nil
	}
	arr, ok := data.([]map[string]any)
	if ok {
		out := make([]map[string]any, 0, len(arr))
		for _, row := range arr {
			n := map[string]any{}
			for _, p := range trimmed {
				if v, ok := getDotted(row, p); ok {
					n[p] = v
				}
			}
			out = append(out, n)
		}
		return out, nil
	}
	obj, ok := data.(map[string]any)
	if ok {
		n := map[string]any{}
		for _, p := range trimmed {
			if v, ok := getDotted(obj, p); ok {
				n[p] = v
			}
		}
		return n, nil
	}
	return data, nil
}

func getDotted(v any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	curr := v
	for _, p := range parts {
		m, ok := curr.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[p]
		if !ok {
			return nil, false
		}
		curr = next
	}
	return curr, true
}

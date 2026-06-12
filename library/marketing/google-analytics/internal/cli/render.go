package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
)

func renderRows(v map[string]any) string { rows, _ := v["rows"].([]map[string]any); return table(rows) }
func table(rows []map[string]any) string {
	if len(rows) == 0 {
		return "No rows.\n"
	}
	keys := []string{}
	for k := range rows[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(keys, "\t"))
	for _, r := range rows {
		vals := []string{}
		for _, k := range keys {
			vals = append(vals, fmt.Sprint(r[k]))
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	tw.Flush()
	return b.String()
}
func renderHealth(v map[string]any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b) + "\n"
}
func renderCompare(v map[string]any) string {
	rows, _ := v["rows"].([]map[string]any)
	return table(rows)
}
func renderMovers(v map[string]any) string {
	rows, _ := v["movers"].([]map[string]any)
	return table(rows)
}

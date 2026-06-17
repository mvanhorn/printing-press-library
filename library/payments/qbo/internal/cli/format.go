// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// printJSONFiltered marshals a Go-typed value through the same output
// pipeline endpoint-mirror commands use. Hand-written novel commands that
// build a typed slice/struct call this so --select, --compact, --csv, and
// --quiet all behave the same way as on generator-emitted commands.
func printJSONFiltered(w io.Writer, v any, flags *rootFlags) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlags(w, json.RawMessage(raw), flags)
}

// filterFields keeps only the specified fields (comma-separated) from JSON objects/arrays.
// Supports dotted paths like "events.shortName" to descend into nested structures.
// Arrays are traversed element-wise: "events.shortName" keeps shortName on each event.
func filterFields(data json.RawMessage, fields string) json.RawMessage {
	var paths [][]string
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		parts := strings.Split(f, ".")
		for i := range parts {
			parts[i] = strings.ToLower(parts[i])
		}
		paths = append(paths, parts)
	}
	if len(paths) == 0 {
		return data
	}
	return filterFieldsRec(data, paths)
}

// filterFieldsRec applies path filters to a JSON value. Each path is a list of
// lowercase segments; arrays descend element-wise.
func filterFieldsRec(data json.RawMessage, paths [][]string) json.RawMessage {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		out := make([]json.RawMessage, len(arr))
		for i, el := range arr {
			out[i] = filterFieldsRec(el, paths)
		}
		result, _ := json.Marshal(out)
		return result
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		keepWhole := map[string]bool{}
		subPaths := map[string][][]string{}
		for _, p := range paths {
			if len(p) == 0 {
				continue
			}
			head := p[0]
			if len(p) == 1 {
				keepWhole[head] = true
			} else {
				subPaths[head] = append(subPaths[head], p[1:])
			}
		}
		filtered := map[string]json.RawMessage{}
		matchedAny := false
		for k, v := range obj {
			matched := matchSelectSegment(k, keepWhole, subPaths)
			if matched == "" {
				continue
			}
			matchedAny = true
			if keepWhole[matched] {
				filtered[k] = v
				continue
			}
			if subs := subPaths[matched]; subs != nil {
				filtered[k] = filterFieldsRec(v, subs)
			}
		}
		// Envelope fallback: when no top-level keys matched but at least one
		// sibling is a non-null array, treat the object as a list envelope
		// (`{"items":[...]}`, `{"data":[...]}`, `{"total_count":N,"items":[...]}`)
		// and apply the selector inside the array(s). Non-array siblings pass
		// through verbatim so envelope metadata (counts, null pagination
		// cursors) stays visible. The foundArray guard preserves the prior
		// empty-object result for flat objects where no key matches and no
		// array exists. The `arr != nil` check rejects JSON null, which
		// json.Unmarshal otherwise accepts into a []json.RawMessage as a
		// nil slice and would coerce to `[]`.
		if !matchedAny {
			pending := map[string]json.RawMessage{}
			foundArray := false
			for k, v := range obj {
				var arr []json.RawMessage
				if json.Unmarshal(v, &arr) == nil && arr != nil {
					foundArray = true
					pending[k] = filterFieldsRec(v, paths)
				} else {
					pending[k] = v
				}
			}
			if foundArray {
				for k, v := range pending {
					filtered[k] = v
				}
			}
		}
		result, _ := json.Marshal(filtered)
		return result
	}

	return data
}

// matchSelectSegment returns the matching lowercase segment, or "" if no match.
// Supports direct case-insensitive match and camelCase→kebab-case conversion.
func matchSelectSegment(fieldName string, keepWhole map[string]bool, subPaths map[string][][]string) string {
	lower := strings.ToLower(fieldName)
	if keepWhole[lower] || subPaths[lower] != nil {
		return lower
	}
	kebab := camelToKebab(fieldName)
	if kebab != lower && (keepWhole[kebab] || subPaths[kebab] != nil) {
		return kebab
	}
	return ""
}

// camelToKebab converts "orderDate" or "orderdate" to "order-date" by splitting on
// uppercase boundaries. For already-lowercase input, splits on known word boundaries.
func camelToKebab(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// printOutputWithFlags routes output through the right format based on flags.
func printOutputWithFlags(w io.Writer, data json.RawMessage, flags *rootFlags) error {
	// --select wins over --compact when both are set: an explicit field list
	// is the user's authoritative request, so the high-gravity allow-list
	// must not strip those fields out before --select can pick them. When
	// only --compact is set (e.g., --agent without --select), the allow-list
	// still runs.
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	// --quiet: suppress all output, exit code communicates result
	if flags.quiet {
		return nil
	}
	// --csv: render as CSV
	if flags.csv {
		return printCSV(w, data)
	}
	return printOutput(w, data, flags.asJSON)
}

// compactVerboseListFields are prose-shaped fields stripped from list-item
// projections. On lists, "body"/"content"/"html"/"markdown" are verbose
// noise and the row's identity is carried by id/name/title/etc.
var compactVerboseListFields = map[string]bool{
	"description": true, "body": true, "content": true,
	"comments": true, "attachments": true, "html": true, "markdown": true,
}

// compactVerboseObjectFields are metadata fields stripped from single-object
// responses. "body"/"content"/"html"/"markdown" are intentionally absent:
// for a `get` command those fields are the primary payload, and stripping
// them under `--agent`/`--compact` silently emits a useless envelope.
// Use `--select` to drop them explicitly.
var compactVerboseObjectFields = map[string]bool{
	"description": true,
	"comments":    true,
	"attachments": true,
}

func compactFields(data json.RawMessage) json.RawMessage {
	var arr []map[string]any
	if json.Unmarshal(data, &arr) == nil {
		return compactListFields(arr)
	}
	var obj map[string]any
	if json.Unmarshal(data, &obj) == nil {
		return compactObjectFields(obj)
	}
	return data
}

// When an item still carries none of the keep keys, the original is
// preserved so `--agent` does not silently emit {} for shapes whose key
// names are entirely off-canonical.
func compactListFields(items []map[string]any) json.RawMessage {
	keepFields := map[string]bool{
		// Identity
		"id": true, "name": true, "title": true, "identifier": true,
		"code": true, "slug": true, "key": true,
		// Categorization
		"status": true, "state": true, "type": true, "kind": true, "priority": true,
		// Communication
		"url": true, "email": true,
		// Monetary
		"price": true, "amount": true, "cost": true, "fare": true,
		"rate": true, "currency": true,
		// Metrics
		"rating": true, "score": true, "count": true,
		// Locale / geo
		"language": true, "locale": true, "country": true, "region": true,
		"city": true, "domain": true,
		// Temporal
		"created_at": true, "updated_at": true, "createdAt": true, "updatedAt": true,
		"date": true,
		// Versioning
		"version": true,
	}
	if len(items) > 0 {
		keyCounts := map[string]int{}
		for _, item := range items {
			for k, v := range item {
				if compactVerboseListFields[k] || !isCompactScalar(v) {
					continue
				}
				keyCounts[k]++
			}
		}
		// ceil(len(items) * 0.8) without importing math. Capped at len-1 for
		// len >= 2 so a single missing row cannot veto a key on small lists
		threshold := (len(items)*4 + 4) / 5
		if len(items) >= 2 && threshold > len(items)-1 {
			threshold = len(items) - 1
		}
		for k, count := range keyCounts {
			if count >= threshold {
				keepFields[k] = true
			}
		}
	}

	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		compact := map[string]any{}
		for k, v := range item {
			if keepFields[k] {
				compact[k] = v
			}
		}
		if len(compact) == 0 {
			compact = item
		}
		filtered = append(filtered, compact)
	}
	result, _ := json.Marshal(filtered)
	return result
}

// isCompactScalar reports whether v is a small primitive (string, number,
// bool, null) suitable for --compact projection. Objects and arrays are
// rejected.
func isCompactScalar(v any) bool {
	switch v.(type) {
	case nil, bool, float64, string:
		return true
	default:
		return false
	}
}

// compactObjectFields strips known-verbose metadata fields from single-object
// responses. The blocklist deliberately excludes "body"/"content"/"html"/
// "markdown" — those fields are payload on `get` commands.
func compactObjectFields(obj map[string]any) json.RawMessage {
	compact := map[string]any{}
	for k, v := range obj {
		if !compactVerboseObjectFields[k] {
			compact[k] = v
		}
	}
	result, _ := json.Marshal(compact)
	return result
}

// printCSV renders JSON arrays as CSV with header row.
func printCSV(w io.Writer, data json.RawMessage) error {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil || len(items) == 0 {
		// Single object or empty - just print as JSON
		fmt.Fprintln(w, string(data))
		return nil
	}
	// Collect all keys for header
	keySet := map[string]bool{}
	for _, item := range items {
		for k := range item {
			keySet[k] = true
		}
	}
	var keys []string
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Header
	fmt.Fprintln(w, strings.Join(keys, ","))
	// Rows
	for _, item := range items {
		var vals []string
		for _, k := range keys {
			v := item[k]
			if v == nil {
				vals = append(vals, "")
			} else {
				var s string
				if f, ok := v.(float64); ok {
					s = strconv.FormatFloat(f, 'f', -1, 64)
				} else {
					s = fmt.Sprintf("%v", v)
				}
				if strings.ContainsAny(s, ",\"\n") {
					s = `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
				}
				vals = append(vals, s)
			}
		}
		fmt.Fprintln(w, strings.Join(vals, ","))
	}
	return nil
}

// printOutput auto-detects arrays and renders as tables, or prints raw JSON for objects.
func printOutput(w io.Writer, data json.RawMessage, asJSON bool) error {
	if !asJSON && !isTerminal(w) {
		asJSON = true
	}

	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// Try to detect if response is an array
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
		if err := printAutoTable(w, items); err != nil {
			return err
		}
		// Agent-friendly: show count and suggest narrowing when results are large
		if len(items) >= 25 {
			fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
		}
		return nil
	}

	// Single object - pretty print
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(obj)
	}

	// Fallback: print raw
	fmt.Fprintln(w, string(data))
	return nil
}

// levenshteinDistance computes the edit distance between two strings using a two-row DP approach.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			min := ins
			if del < min {
				min = del
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// suggestFlag returns the closest known flag name to the unknown string, or "" if none is close enough.
func suggestFlag(unknown string, cmd *cobra.Command) string {
	unknown = strings.TrimLeft(unknown, "-")
	best := ""
	bestDist := 4 // only consider distance <= 3
	check := func(name string) {
		d := levenshteinDistance(unknown, name)
		if d < bestDist && d*5 <= len(unknown)*2 {
			bestDist = d
			best = name
		}
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		check(f.Name)
	})
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		check(f.Name)
	})
	return best
}

// wantsHumanTable returns true when output should be a human-friendly table.
func wantsHumanTable(w io.Writer, flags *rootFlags) bool {
	if flags.asJSON || flags.csv || flags.compact || flags.quiet || flags.plain {
		return false
	}
	if flags.selectFields != "" {
		return false
	}
	return isTerminal(w)
}

func printAutoTable(w io.Writer, items []map[string]any) error {
	if len(items) == 0 {
		return nil
	}

	// Count scalar vs complex fields to decide format
	scalarCount := 0
	for _, v := range items[0] {
		if isCompactScalar(v) {
			scalarCount++
		}
	}

	// Use sectional/card layout for complex items (many fields or nested data)
	if len(items[0]) > 8 || scalarCount < len(items[0])-2 {
		return printAutoCards(w, items)
	}

	headers := prioritizeHeaders(items[0])

	// Limit to 6 columns max for readability
	if len(headers) > 6 {
		headers = headers[:6]
	}

	// Build rows
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := make([]string, len(headers))
		for i, h := range headers {
			row[i] = formatCellValue(item[h])
		}
		rows = append(rows, row)
	}

	// Print with tab alignment using tabwriter
	tw := newTabWriter(w)
	// Print headers
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, strings.ToUpper(h))
	}
	fmt.Fprintln(tw)

	// Print rows
	for _, row := range rows {
		for i, val := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, val)
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

// prioritizeHeaders orders scalar fields by importance for table display.
func prioritizeHeaders(item map[string]any) []string {
	return prioritizeFields(item, false)
}

// prioritizeAllHeaders orders all fields (including arrays) by importance for card display.
func prioritizeAllHeaders(item map[string]any) []string {
	return prioritizeFields(item, true)
}

// prioritizeFields orders fields by importance: identity → temporal → status → other.
func prioritizeFields(item map[string]any, includeComplex bool) []string {
	exactMatches := map[string]int{
		"id": 0, "name": 0, "title": 0, "slug": 0, "key": 0,
		"date": 1, "created": 1, "updated": 1, "createdat": 1, "updatedat": 1,
		"status": 2, "state": 2, "statuscode": 2,
		"summary": 3, "description": 3, "price": 3, "amount": 3, "total": 3,
		"cost": 3, "points": 3, "score": 3,
		"type": 4, "kind": 4, "category": 4, "email": 4, "phone": 4, "url": 4,
	}
	suffixMatches := map[string]int{
		"id": 0, "name": 0, "title": 0,
		"date": 1, "time": 1,
		"status": 2, "state": 2, "code": 2,
		"price": 3, "amount": 3, "total": 3, "cost": 3,
		"summary": 3, "description": 3, "points": 3, "score": 3,
		"type": 4, "kind": 4, "category": 4, "method": 4,
	}

	numTiers := 5

	type scored struct {
		name  string
		tier  int
		index int
	}

	var all []scored
	idx := 0
	for k, v := range item {
		if !includeComplex {
			switch v.(type) {
			case []any, map[string]any:
				continue
			}
		}
		if includeComplex {
			formatted := formatCellValue(v)
			if formatted == "" {
				continue
			}
		}

		tier := numTiers
		lower := strings.ToLower(k)

		if t, ok := exactMatches[lower]; ok {
			tier = t
		} else {
			segments := splitCamelCase(lower)
			if len(segments) > 0 {
				lastSeg := segments[len(segments)-1]
				if t, ok := suffixMatches[lastSeg]; ok {
					if len(segments) > 1 {
						tier = t + 1
					} else {
						tier = t
					}
				}
			}
		}

		if _, ok := v.(bool); ok && tier >= numTiers {
			tier = numTiers + 1
		}
		all = append(all, scored{name: k, tier: tier, index: idx})
		idx++
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].tier != all[j].tier {
			return all[i].tier < all[j].tier
		}
		return all[i].index < all[j].index
	})

	headers := make([]string, len(all))
	for i, s := range all {
		headers[i] = s.name
	}
	return headers
}

// splitCamelCase splits "OrderDate" → ["order", "date"], "statusCode" → ["status", "code"],
// "page_size" → ["page", "size"].
func splitCamelCase(s string) []string {
	var segments []string
	var current strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if r == '_' || r == '-' {
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
			continue
		}
		if i > 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		}
		current.WriteRune(unicode.ToLower(r))
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}
	return segments
}

// printAutoCards renders items as labeled cards — one block per item.
func printAutoCards(w io.Writer, items []map[string]any) error {
	headers := prioritizeAllHeaders(items[0])

	maxLen := 0
	for _, h := range headers {
		if len(h) > maxLen {
			maxLen = len(h)
		}
	}

	for i, item := range items {
		if i > 0 {
			fmt.Fprintln(w)
		}

		titleVal := formatCellValue(item[headers[0]])
		if len(headers) > 1 {
			secondVal := formatCellValue(item[headers[1]])
			if secondVal != "" {
				fmt.Fprintf(w, "%s %s — %s\n", bold(strings.ToUpper(headers[0])), titleVal, secondVal)
			} else {
				fmt.Fprintf(w, "%s %s\n", bold(strings.ToUpper(headers[0])), titleVal)
			}
		} else {
			fmt.Fprintf(w, "%s %s\n", bold(strings.ToUpper(headers[0])), titleVal)
		}

		for _, h := range headers[2:] {
			v := formatCellValue(item[h])
			if v == "" || v == "false" || v == "0" || v == "[]" || v == "null" {
				continue
			}
			if strings.HasPrefix(v, "\n") {
				fmt.Fprintf(w, "  %s:%s\n", h, v)
			} else {
				fmt.Fprintf(w, "  %-*s  %s\n", maxLen, h+":", v)
			}
		}
	}
	return nil
}

func formatCellValue(v any) string {
	switch val := v.(type) {
	case string:
		if len(val) >= 19 && val[4] == '-' && val[7] == '-' && val[10] == 'T' {
			return val[:10]
		}
		return truncate(val, 60)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.2f", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return ""
	case []any:
		if len(val) == 0 {
			return ""
		}
		if _, isObj := val[0].(map[string]any); isObj {
			return formatObjectArray(val)
		}
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			} else {
				b, _ := json.Marshal(item)
				parts = append(parts, string(b))
			}
		}
		return truncate(strings.Join(parts, ", "), 60)
	case map[string]any:
		return formatSingleObject(val)
	default:
		b, _ := json.Marshal(val)
		return truncate(string(b), 60)
	}
}

func formatObjectArray(items []any) string {
	var lines []string
	for _, raw := range items {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		lines = append(lines, formatObjectSummary(obj))
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n" + strings.Join(lines, "\n")
}

func formatObjectSummary(obj map[string]any) string {
	var parts []string

	qty := findField(obj, "qty", "count", "quantity")
	if qty != "" && qty != "1" && qty != "0" {
		parts = append(parts, qty+"x")
	} else if qty == "1" {
		parts = append(parts, "1x")
	}

	name := findField(obj, "name", "title", "label", "description")
	if name == "" {
		for _, key := range []string{"Side1", "side1", "Item", "item", "Product", "product"} {
			if nested, ok := obj[key].(map[string]any); ok {
				name = findField(nested, "name", "title", "label")
				if name != "" {
					break
				}
			}
		}
	}
	if name != "" {
		parts = append(parts, name)
	}

	size := findField(obj, "sizename", "size_name")
	if size == "" {
		size = findField(obj, "catname", "cat_name", "category")
	}
	if size != "" {
		parts = append(parts, "—")
		parts = append(parts, size)
	}

	price := findField(obj, "extprice", "price", "amount", "total")
	if price != "" && price != "0" {
		parts = append(parts, fmt.Sprintf("($%s)", price))
	}

	if len(parts) == 0 {
		b, _ := json.Marshal(obj)
		return truncate(string(b), 80)
	}
	return "    " + strings.Join(parts, " ")
}

func formatSingleObject(obj map[string]any) string {
	name := findField(obj, "name", "title", "label", "description")
	if name != "" {
		return name
	}
	id := findField(obj, "id", "key", "code")
	if id != "" {
		return id
	}
	return ""
}

func findField(obj map[string]any, names ...string) string {
	for _, name := range names {
		for k, v := range obj {
			if strings.EqualFold(k, name) {
				return formatCellValue(v)
			}
		}
	}
	return ""
}

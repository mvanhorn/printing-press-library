// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared helpers for the nisifilters-pp-cli novel commands
// (shop, filters, read, digest, since, image). These read the local SQLite
// mirror produced by `sync` and tame the verbose WordPress / WooCommerce
// response envelope into agent-clean output. Not generated; safe across regen.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/internal/store"
)

// fgStoreName is the binary name used to resolve the default DB path.
const fgStoreName = "nisifilters-pp-cli"

// fgOpenStore opens the local SQLite mirror at the default path.
func fgOpenStore(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath(fgStoreName))
}

// fgDecode unmarshals a raw JSON object into a string-keyed map. A nil map is
// returned for non-object input so callers can range over it safely.
func fgDecode(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	return m
}

// fgString returns the string value at key, or "" when absent / not a string.
func fgString(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

// fgInt returns the integer value at key. WordPress numeric IDs and counts are
// JSON numbers; some fields arrive as numeric strings, so both are accepted.
func fgInt(m map[string]json.RawMessage, key string) (int, bool) {
	raw, ok := m[key]
	if !ok {
		return 0, false
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return v, true
		}
	}
	return 0, false
}

// fgBool returns the bool value at key, or false when absent / not a bool.
func fgBool(m map[string]json.RawMessage, key string) bool {
	raw, ok := m[key]
	if !ok {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	return false
}

// fgIntSlice returns the integer slice at key (e.g. a post's categories/tags).
func fgIntSlice(m map[string]json.RawMessage, key string) []int {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	var out []int
	if err := json.Unmarshal(raw, &out); err == nil {
		return out
	}
	return nil
}

// fgRendered returns the .rendered string of a WordPress {rendered:...} field,
// falling back to a bare string value. Returned text is NOT HTML-stripped.
func fgRendered(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var obj struct {
		Rendered string `json:"rendered"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Rendered != "" {
		return obj.Rendered
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

var (
	fgScriptStyleRE = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	fgBlockCloseRE  = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|blockquote)>`)
	fgBrRE          = regexp.MustCompile(`(?i)<br\s*/?>`)
	fgTagRE         = regexp.MustCompile(`<[^>]*>`)
	fgInlineWSRE    = regexp.MustCompile(`[ \t]+`)
	fgManyNLRE      = regexp.MustCompile(`\n{3,}`)
)

// fgStripHTML converts rendered WordPress HTML to readable plain text: script
// and style blocks are dropped, block-level closers become newlines, remaining
// tags are removed, and HTML entities are decoded.
func fgStripHTML(s string) string {
	if s == "" {
		return ""
	}
	s = fgScriptStyleRE.ReplaceAllString(s, "")
	s = fgBrRE.ReplaceAllString(s, "\n")
	s = fgBlockCloseRE.ReplaceAllString(s, "\n")
	s = fgTagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = fgInlineWSRE.ReplaceAllString(s, " ")
	// Trim trailing spaces left on each line, then collapse blank-line runs.
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	s = strings.Join(lines, "\n")
	s = fgManyNLRE.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// fgPlainTitle returns the HTML-stripped, entity-decoded title of a WordPress
// (title.rendered) or WooCommerce (name) entity.
func fgPlainTitle(m map[string]json.RawMessage) string {
	if t := fgRendered(m, "title"); t != "" {
		return fgStripHTML(t)
	}
	// WooCommerce store products use a flat "name".
	if n := fgString(m, "name"); n != "" {
		return html.UnescapeString(n)
	}
	return ""
}

// fgMediaSourceURL returns the full-resolution source_url of a media object.
func fgMediaSourceURL(media map[string]json.RawMessage) string {
	return fgString(media, "source_url")
}

// fgMediaSizes returns the available size variants of a media object as a map
// of size name -> source URL (from media_details.sizes.<name>.source_url).
func fgMediaSizes(media map[string]json.RawMessage) map[string]string {
	out := map[string]string{}
	details := fgDecode(media["media_details"])
	sizesRaw, ok := details["sizes"]
	if !ok {
		return out
	}
	sizes := fgDecode(sizesRaw)
	for name, raw := range sizes {
		size := fgDecode(raw)
		if u := fgString(size, "source_url"); u != "" {
			out[name] = u
		}
	}
	return out
}

// fgWooPrice formats a WooCommerce Store API prices object into a display
// string and a comparable numeric value. The Store API encodes amounts as
// minor-unit integer strings (e.g. "12000" with currency_minor_unit 2 == 120.00).
func fgWooPrice(pricesRaw json.RawMessage) (display string, value float64, ok bool) {
	m := fgDecode(pricesRaw)
	priceStr := fgString(m, "price")
	if priceStr == "" {
		return "", 0, false
	}
	cents, err := strconv.ParseFloat(strings.TrimSpace(priceStr), 64)
	if err != nil {
		return "", 0, false
	}
	minor, _ := fgInt(m, "currency_minor_unit")
	value = cents / math.Pow(10, float64(minor))
	code := fgString(m, "currency_code")
	if code == "" {
		display = fmt.Sprintf("%.2f", value)
	} else {
		display = fmt.Sprintf("%.2f %s", value, code)
	}
	return display, value, true
}

// fgStockStatus classifies a WooCommerce Store API product's availability:
// "in" (in stock now), "backorder" (orderable but ships later —
// is_on_backorder), or "out". Backorder wins over in-stock because the Store
// API reports backorder items as is_in_stock=true.
func fgStockStatus(m map[string]json.RawMessage) string {
	if fgBool(m, "is_on_backorder") {
		return "backorder"
	}
	if fgBool(m, "is_in_stock") {
		return "in"
	}
	return "out"
}

// fgCategoryNames returns the names of a WooCommerce product's inline
// categories array ([{id,name,slug}, ...]).
func fgCategoryNames(m map[string]json.RawMessage) []string {
	raw, ok := m["categories"]
	if !ok {
		return nil
	}
	var cats []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &cats); err != nil {
		return nil
	}
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		if c.Name != "" {
			out = append(out, html.UnescapeString(c.Name))
		}
	}
	return out
}

// fgEmptyStoreNote returns a short, honest message telling the user to sync
// before an offline command can return anything.
func fgEmptyStoreNote(what string) string {
	return fmt.Sprintf("no %s in the local mirror yet — run `%s sync` first", what, fgStoreName)
}

// fgSplitArray splits a raw JSON array into its element messages. Non-arrays
// return nil.
func fgSplitArray(raw json.RawMessage) []json.RawMessage {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	return arr
}

// containsInt reports whether needle is present in haystack.
func containsInt(haystack []int, needle int) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

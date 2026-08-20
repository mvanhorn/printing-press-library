// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/store"
)

// fetchHTML hits a Food52 path with the existing client's transport and
// returns the response body. Replaces the generated extractHTMLResponse path
// with one that also surfaces Vercel-challenge HTML as an actionable error.
func fetchHTML(c *client.Client, path string, params map[string]string) ([]byte, error) {
	if c.DryRun {
		// Mirror the generated dryRun preview behavior: print the would-be URL.
		full := c.BaseURL + path
		if len(params) > 0 {
			vals := url.Values{}
			for k, v := range params {
				vals.Set(k, v)
			}
			full = full + "?" + vals.Encode()
		}
		fmt.Fprintf(os.Stderr, "[dry-run] GET %s\n", full)
		return []byte("{}"), nil
	}
	body, err := c.Get(path, params)
	if err != nil {
		return nil, err
	}
	if food52.LooksLikeChallenge(body) {
		return nil, fmt.Errorf("food52: response is a Vercel Security Checkpoint page — the CLI's HTTP transport is not clearing it. Re-run `food52-pp-cli doctor` and rebuild")
	}
	return body, nil
}

// canonicalRecipeURL builds the public Food52 URL for a recipe slug.
func canonicalRecipeURL(slug string) string {
	return "https://food52.com/recipes/" + slug
}

// canonicalArticleURL builds the public Food52 URL for an article slug.
func canonicalArticleURL(slug string) string {
	return "https://food52.com/story/" + slug
}

// canonicalTagURL builds the URL for a recipe tag listing.
func canonicalTagURL(tag string) string {
	return "https://food52.com/recipes/" + tag
}

// recipeSlugFromArg accepts either a bare slug or a full Food52 recipe URL
// and returns the slug. Used by `recipes get`, `scale`, `print`, and
// `articles for-recipe`.
func recipeSlugFromArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}
	if strings.Contains(arg, "/recipes/") {
		i := strings.Index(arg, "/recipes/")
		rest := arg[i+len("/recipes/"):]
		if end := strings.IndexAny(rest, "/?#"); end >= 0 {
			rest = rest[:end]
		}
		return rest
	}
	return arg
}

// articleSlugFromArg accepts either a bare slug or a full Food52 story URL
// and returns the slug.
func articleSlugFromArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}
	if strings.Contains(arg, "/story/") {
		i := strings.Index(arg, "/story/")
		rest := arg[i+len("/story/"):]
		if end := strings.IndexAny(rest, "/?#"); end >= 0 {
			rest = rest[:end]
		}
		return rest
	}
	if strings.Contains(arg, "/blog/") {
		i := strings.Index(arg, "/blog/")
		rest := arg[i+len("/blog/"):]
		if end := strings.IndexAny(rest, "/?#"); end >= 0 {
			rest = rest[:end]
		}
		return rest
	}
	return arg
}

// emitJSON marshals v indented and writes it to stdout. Used by every
// food52-specific command that doesn't need the full provenance-wrap +
// table-render pipeline of the generated handlers.
func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// emitFromFlags switches output style based on rootFlags. Most food52
// commands return small structured responses; we default to JSON when the
// user asked for it (--json, --agent) or when stdout is not a TTY (so pipes
// see clean JSON), and pretty text otherwise.
//
// Recognised output-shape flags:
//
//   - --quiet       suppress all output; rely on the exit code
//   - --select      project a comma-separated list of dotted JSON paths
//     (arrays traverse element-wise) before printing
//   - --csv         render the first JSON array found as CSV with a header row
//   - --plain       render the first JSON array found as tab-separated text
//     (TSV-style; works with cut, awk, column)
//   - --json/--agent or piped stdout
//     pretty-printed JSON (after --select if set)
//   - default       call the textRenderer for human-readable terminal output
func emitFromFlags(flags *rootFlags, jsonObj any, textRenderer func()) error {
	if flags.quiet {
		return nil
	}

	wantsMachine := flags.asJSON || flags.csv || flags.plain || !isStdoutTTY()
	if !wantsMachine {
		textRenderer()
		return nil
	}

	raw, err := json.Marshal(jsonObj)
	if err != nil {
		return err
	}
	if flags.selectFields != "" {
		raw = filterFields(raw, flags.selectFields)
	}

	switch {
	case flags.csv:
		return emitDelimited(raw, ",")
	case flags.plain:
		return emitDelimited(raw, "\t")
	default:
		// --json or default machine output: pretty JSON.
		// raw is already-marshalled (compact); pretty-print to match emitJSON.
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, raw, "", "  "); err != nil {
			os.Stdout.Write(raw)
			os.Stdout.Write([]byte("\n"))
			return nil
		}
		os.Stdout.Write(pretty.Bytes())
		os.Stdout.Write([]byte("\n"))
		return nil
	}
}

// emitDelimited renders the first JSON array found inside raw as
// delimiter-separated rows with a header row of sorted keys. Used by --csv
// (delim=",") and --plain (delim="\t"). Falls back to printing raw JSON
// when no array shape is detected.
func emitDelimited(raw json.RawMessage, delim string) error {
	rows, headers := extractRowsForDelimited(raw)
	if len(rows) == 0 {
		os.Stdout.Write(raw)
		os.Stdout.Write([]byte("\n"))
		return nil
	}
	if delim == "," {
		os.Stdout.Write([]byte(joinCSV(headers) + "\n"))
		for _, r := range rows {
			vals := make([]string, len(headers))
			for i, h := range headers {
				vals[i] = stringifyCell(r[h])
			}
			os.Stdout.Write([]byte(joinCSV(vals) + "\n"))
		}
		return nil
	}
	// Plain tab-separated.
	os.Stdout.Write([]byte(strings.Join(headers, delim) + "\n"))
	for _, r := range rows {
		vals := make([]string, len(headers))
		for i, h := range headers {
			vals[i] = stringifyCell(r[h])
		}
		os.Stdout.Write([]byte(strings.Join(vals, delim) + "\n"))
	}
	return nil
}

// extractRowsForDelimited finds the first JSON array of objects to render.
// Looks at top-level array shape first; otherwise picks the first
// array-of-objects field of an object payload. Returns the rows plus a
// stable, sorted header set.
func extractRowsForDelimited(raw json.RawMessage) ([]map[string]any, []string) {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr, sortedKeys(arr)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, nil
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var inner []map[string]any
		if err := json.Unmarshal(obj[k], &inner); err == nil && len(inner) > 0 {
			return inner, sortedKeys(inner)
		}
	}
	return nil, nil
}

func sortedKeys(rows []map[string]any) []string {
	set := map[string]struct{}{}
	for _, r := range rows {
		for k := range r {
			set[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func stringifyCell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func joinCSV(parts []string) string {
	out := make([]string, len(parts))
	for i, p := range parts {
		if strings.ContainsAny(p, ",\"\n\r") {
			out[i] = `"` + strings.ReplaceAll(p, `"`, `""`) + `"`
		} else {
			out[i] = p
		}
	}
	return strings.Join(out, ",")
}

// isStdoutTTY is the same predicate the generated helpers use to decide
// JSON-by-default for piped output. We re-export it as a small wrapper so
// our food52 commands don't have to import the cli internals twice.
func isStdoutTTY() bool {
	return isTerminal(os.Stdout)
}

// openStoreOrErr opens the SQLite store for read/write, returning a friendly
// error when the DB does not exist yet. Used by `sync`, `search`, and the
// pantry commands.
func openStoreOrErr() (*store.Store, error) {
	dbPath := defaultDBPath("food52-pp-cli")
	return store.Open(dbPath)
}

// httpClientForFood52 returns the underlying *http.Client from the printed
// CLI's client wrapper. food52.LoadDiscovery and food52.SearchRecipes use it
// directly so absolute Typesense URLs go through the same Surf-built
// transport as everything else.
func httpClientForFood52(c *client.Client) *http.Client {
	if c == nil || c.HTTPClient == nil {
		return http.DefaultClient
	}
	return c.HTTPClient
}

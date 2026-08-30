// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared output helper for the hand-written novel commands.

package cli

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/logitech-docs/internal/cliutil"
)

// printNovelJSON marshals a novel command's typed result through the same
// output pipeline the endpoint-mirror commands use, but stamps the provenance
// the command actually used. printJSONFiltered defaults meta.source to
// "local", which is wrong for live-only commands like compare and download and
// misleading for find when it falls back to the live search.
func printNovelJSON(w io.Writer, v any, flags *rootFlags, source string) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(w, json.RawMessage(raw), flags, map[string]any{"source": source})
}

// highlightTagRe matches the <em> highlight markup Zendesk wraps around matched
// terms in search snippets.
var highlightTagRe = regexp.MustCompile(`</?em[^>]*>`)

// cleanSnippet strips Zendesk's highlight markup and decodes HTML entities so
// snippet text reads as plain prose in both table and JSON output.
func cleanSnippet(s string) string {
	return strings.TrimSpace(cliutil.CleanText(highlightTagRe.ReplaceAllString(s, "")))
}

// cleanSnippets rewrites the "snippet" field of each row in place.
func cleanSnippets(rows []map[string]any) {
	for _, row := range rows {
		if snippet, ok := row["snippet"].(string); ok {
			row["snippet"] = cleanSnippet(snippet)
		}
	}
}

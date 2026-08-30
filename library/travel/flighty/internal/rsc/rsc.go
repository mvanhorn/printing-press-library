// Package rsc extracts structured JSON from Next.js App Router RSC flight
// payloads embedded in server-rendered HTML (self.__next_f.push chunks).
//
// flighty.com/airports is a Next.js SSR site with no JSON API: every page
// embeds its data as React Server Component flight chunks. Each chunk is an
// inline script of the form:
//
//	self.__next_f.push([1,"<escaped payload>"])
//
// The escaped payload is a JSON string literal containing lines such as
// `2:["$","div",...]` and named data islands like `1f:[[...]]`. The data
// objects this CLI needs (the airport catalog, per-airport detail, and flight
// boards) appear as ordinary JSON object/array literals inside the
// concatenated chunk text, so marker-anchored balanced scanning recovers them
// without depending on RSC stream internals.
package rsc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// chunkRe matches the string-literal argument of a Next.js RSC flight push.
var chunkRe = regexp.MustCompile(`self\.__next_f\.push\(\[1,"((?:[^"\\]|\\.)*)"\]\)`)

// ExtractChunks finds every self.__next_f.push([1,"..."]) script payload in
// raw HTML, JSON-unescapes each string literal, and returns the concatenated
// chunk text. Extraction is order-preserving, matching the server emission
// order the RSC stream uses.
func ExtractChunks(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("rsc: empty document")
	}
	matches := chunkRe.FindAllSubmatch(raw, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("rsc: no Next.js flight chunks found (no self.__next_f.push scripts)")
	}
	var buf bytes.Buffer
	for _, m := range matches {
		// The capture is a JSON string literal body; unescape via json.
		var unescaped string
		literal := make([]byte, 0, len(m[1])+2)
		literal = append(literal, '"')
		literal = append(literal, m[1]...)
		literal = append(literal, '"')
		if err := json.Unmarshal(literal, &unescaped); err != nil {
			continue // skip malformed chunk rather than failing the page
		}
		buf.WriteString(unescaped)
	}
	if buf.Len() == 0 {
		return "", fmt.Errorf("rsc: flight chunks present but all unescapes failed")
	}
	return buf.String(), nil
}

// FindObject locates marker in text and returns the balanced JSON object
// anchored at the opening token the marker includes. The marker must begin
// with '{' (e.g. `{"iata":"DEN"`) or end with '{' (e.g. `"regions":{`);
// scanning starts at that token so string-literal state is well-defined.
func FindObject(text, marker string) (json.RawMessage, error) {
	return findAnchored(text, marker, '{', '}')
}

// FindArray locates marker in text and returns the balanced JSON array
// anchored at the opening token the marker includes. The marker must begin
// with '[' or end with '[' (e.g. `"initialFlights":[`).
func FindArray(text, marker string) (json.RawMessage, error) {
	return findAnchored(text, marker, '[', ']')
}

// findAnchored scans from the opening token inside the marker to its matching
// close token, honoring string-literal state and escape sequences.
func findAnchored(text, marker string, open, close byte) (json.RawMessage, error) {
	if text == "" {
		return nil, fmt.Errorf("rsc: empty payload")
	}
	start := strings.Index(text, marker)
	if start < 0 {
		return nil, fmt.Errorf("rsc: marker %q not found in payload", marker)
	}
	var anchor int
	switch {
	case marker[0] == open:
		anchor = start
	case marker[len(marker)-1] == open:
		anchor = start + len(marker) - 1
	default:
		return nil, fmt.Errorf("rsc: marker %q must include the %q opening token at its start or end", marker, string(open))
	}
	fragment, ok := scanBalanced(text, anchor, open, close)
	if !ok {
		return nil, fmt.Errorf("rsc: unbalanced fragment at marker %q", marker)
	}
	if !json.Valid([]byte(fragment)) {
		return nil, fmt.Errorf("rsc: fragment at marker %q is not valid JSON", marker)
	}
	return json.RawMessage(fragment), nil
}

// scanBalanced returns the balanced fragment opening at start, honoring
// string-literal state. ok is false when the fragment never closes.
func scanBalanced(text string, start int, open, close byte) (string, bool) {
	depth := 0
	inString := false
	escaped := false
	for j := start; j < len(text); j++ {
		c := text[j]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return text[start : j+1], true
			}
		}
	}
	return "", false
}

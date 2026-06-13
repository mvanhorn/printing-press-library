// SEMrush CSV response auto-conversion. The SEMrush public API returns
// semicolon-delimited CSV by default; this converts to a JSON array of objects
// so downstream output (--json, --select, --csv re-format) sees structured data
// instead of an opaque CSV string. Conversion is a no-op for already-JSON
// responses (detected by leading { [ or "), HTML error pages, or empty bodies.
package cliutil

import (
	"encoding/json"
	"strings"
)

// MaybeConvertSemrushCSV detects semicolon-delimited CSV bodies typical of the
// SEMrush public API and converts them to a JSON array. Returns the input
// unchanged for any non-CSV shape.
func MaybeConvertSemrushCSV(body []byte) []byte {
	if !looksLikeSemrushCSV(body) {
		return body
	}
	text := string(body)
	// Normalize line endings — SEMrush emits CRLF
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
	if len(lines) < 1 {
		return body
	}
	headers := strings.Split(lines[0], ";")
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}
	rows := make([]map[string]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		cells := strings.Split(line, ";")
		row := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(cells) {
				row[h] = cells[i]
			}
		}
		rows = append(rows, row)
	}
	out, err := json.Marshal(rows)
	if err != nil {
		return body
	}
	return out
}

func looksLikeSemrushCSV(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	// Skip leading whitespace
	i := 0
	for i < len(body) && (body[i] == ' ' || body[i] == '\t' || body[i] == '\n' || body[i] == '\r') {
		i++
	}
	if i >= len(body) {
		return false
	}
	first := body[i]
	// JSON or HTML or quoted string — not CSV
	if first == '{' || first == '[' || first == '"' || first == '<' {
		return false
	}
	// Examine the first line for semicolons and absence of JSON markers
	end := i
	for end < len(body) && body[end] != '\n' {
		end++
	}
	firstLine := string(body[i:end])
	if !strings.Contains(firstLine, ";") {
		return false
	}
	if strings.Contains(firstLine, "{") || strings.Contains(firstLine, "}") {
		return false
	}
	// Header heuristic: SEMrush CSVs always have a header row with words
	// containing letters and spaces. If the line is all numeric, it's not CSV.
	hasLetter := false
	for _, c := range firstLine {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			hasLetter = true
			break
		}
	}
	return hasLetter
}

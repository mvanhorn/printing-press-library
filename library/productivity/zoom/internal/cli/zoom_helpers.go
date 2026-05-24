package cli

import (
	"strconv"
	"strings"
)

// jsonStringField does a tiny brace-respecting search for `"key": "value"`
// in a JSON document — enough for the small set of well-known cloud meeting
// fields (`topic`, `start_time`, `join_url`) that `today` and `schedule`
// extract from the generic resources blob. Returns "" when not found.
//
// We deliberately avoid json.Unmarshal here because (a) the generated
// types are not imported by the cli package, and (b) a full unmarshal of a
// 2 KB meeting blob just to read three keys is wasteful when called per row.
func jsonStringField(doc, key string) string {
	needle := `"` + key + `"`
	i := strings.Index(doc, needle)
	if i < 0 {
		return ""
	}
	// Walk past whitespace + colon.
	j := i + len(needle)
	for j < len(doc) && (doc[j] == ' ' || doc[j] == '\t' || doc[j] == ':') {
		j++
	}
	if j >= len(doc) || doc[j] != '"' {
		return ""
	}
	j++ // skip opening quote
	var b strings.Builder
	for j < len(doc) {
		switch doc[j] {
		case '\\':
			if j+1 < len(doc) {
				b.WriteByte(doc[j+1])
				j += 2
				continue
			}
		case '"':
			return b.String()
		}
		b.WriteByte(doc[j])
		j++
	}
	return ""
}

// jsonNumberField returns the numeric value for `"key": <number>` from a JSON
// document. Returns 0 when missing or unparseable.
func jsonNumberField(doc, key string) float64 {
	needle := `"` + key + `"`
	i := strings.Index(doc, needle)
	if i < 0 {
		return 0
	}
	j := i + len(needle)
	for j < len(doc) && (doc[j] == ' ' || doc[j] == '\t' || doc[j] == ':') {
		j++
	}
	end := j
	for end < len(doc) && (doc[end] == '-' || doc[end] == '+' || doc[end] == '.' || (doc[end] >= '0' && doc[end] <= '9') || doc[end] == 'e' || doc[end] == 'E') {
		end++
	}
	if end == j {
		return 0
	}
	v, err := strconv.ParseFloat(doc[j:end], 64)
	if err != nil {
		return 0
	}
	return v
}

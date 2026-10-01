// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package tsadmin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/tailscale/hujson"
)

// ArraySections are the top-level policy-file sections that hold a list of
// entries, so a new entry can be appended without touching anything else.
var ArraySections = []string{"nodeAttrs", "grants", "acls", "ssh"}

// CanonicalSection maps a user-typed section name (any case) to its policy
// file key, or "" when the section is not an appendable list.
func CanonicalSection(s string) string {
	for _, sec := range ArraySections {
		if strings.EqualFold(sec, strings.TrimSpace(s)) {
			return sec
		}
	}
	return ""
}

// cloneBytes returns a private copy. hujson.Parse aliases its input and
// Standardize/Format rewrite those bytes in place, so every parse here works
// on a copy to keep callers' buffers (and their comments) intact.
func cloneBytes(b []byte) []byte { return append([]byte(nil), b...) }

// StandardizeJSON converts HuJSON (comments, trailing commas) to standard
// JSON without mutating the input.
func StandardizeJSON(b []byte) ([]byte, error) {
	v, err := hujson.Parse(cloneBytes(b))
	if err != nil {
		return nil, err
	}
	v.Standardize()
	return v.Pack(), nil
}

// AddEntryResult describes the outcome of AddEntry.
type AddEntryResult struct {
	Text           []byte `json:"-"`
	AlreadyPresent bool   `json:"already_present"`
	Formatted      bool   `json:"formatted"`
	CreatedSection bool   `json:"created_section"`
	FormatNote     string `json:"format_note,omitempty"`
}

// AddEntry appends entry to the named list section of a HuJSON policy file,
// preserving every comment and byte outside the insertion. When an equal
// entry already exists the policy is returned unchanged. The result is
// re-formatted only when the original file was already in canonical hujson
// format, so a hand-formatted file is not rewritten wholesale.
func AddEntry(policy []byte, section string, entry []byte) (AddEntryResult, error) {
	sec := CanonicalSection(section)
	if sec == "" {
		return AddEntryResult{}, fmt.Errorf("section %q is not supported; use one of %s", section, strings.Join(ArraySections, ", "))
	}
	// Parse a private copy once; Clone deep-copies, so formatting and
	// standardizing the clones never touches root or the caller's buffer.
	root, err := hujson.Parse(cloneBytes(policy))
	if err != nil {
		return AddEntryResult{}, fmt.Errorf("parsing policy file: %w", err)
	}
	entryStd, err := StandardizeJSON(entry)
	if err != nil {
		return AddEntryResult{}, fmt.Errorf("parsing entry: %w", err)
	}
	var entryVal any
	if err := json.Unmarshal(entryStd, &entryVal); err != nil {
		return AddEntryResult{}, fmt.Errorf("parsing entry: %w", err)
	}
	if _, ok := entryVal.(map[string]any); !ok {
		return AddEntryResult{}, fmt.Errorf("entry must be a JSON object")
	}

	canonical := root.Clone()
	canonical.Format()
	wasFormatted := bytes.Equal(canonical.Pack(), policy)

	std := root.Clone()
	std.Standardize()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(std.Pack(), &top); err != nil {
		return AddEntryResult{}, fmt.Errorf("policy file is not a JSON object: %w", err)
	}

	var patch string
	res := AddEntryResult{}
	if rawSection, ok := top[sec]; ok {
		var items []any
		if err := json.Unmarshal(rawSection, &items); err != nil {
			return AddEntryResult{}, fmt.Errorf("policy section %q is not a list", sec)
		}
		for _, it := range items {
			if reflect.DeepEqual(it, entryVal) {
				res.AlreadyPresent = true
				res.Text = append([]byte{}, policy...)
				res.Formatted = wasFormatted
				return res, nil
			}
		}
		patch = fmt.Sprintf(`[{"op":"add","path":"/%s/-","value":%s}]`, sec, entry)
	} else {
		res.CreatedSection = true
		patch = fmt.Sprintf(`[{"op":"add","path":"/%s","value":[%s]}]`, sec, entry)
	}
	if err := root.Patch([]byte(patch)); err != nil {
		return AddEntryResult{}, fmt.Errorf("applying entry: %w", err)
	}
	if wasFormatted {
		root.Format()
		res.Formatted = true
	} else {
		res.FormatNote = "policy file is not in canonical hujson format; the new entry was inserted without reformatting other lines"
	}
	res.Text = root.Pack()
	return res, nil
}

// ValidateResult interprets a POST /acl/validate response.
type ValidateResult struct {
	OK       bool            `json:"ok"`
	Warnings bool            `json:"warnings,omitempty"`
	Message  string          `json:"message,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Status   int             `json:"status"`
}

// InterpretValidate maps a validate response to pass/fail. An empty 200 body
// means the policy is valid and tests pass; a message containing "warning"
// is reported but does not fail; any other message or non-2xx status fails.
func InterpretValidate(status int, body []byte) ValidateResult {
	res := ValidateResult{Status: status}
	trimmed := bytes.TrimSpace(body)
	var parsed struct {
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if len(trimmed) > 0 {
		if err := json.Unmarshal(trimmed, &parsed); err != nil {
			parsed.Message = strings.TrimSpace(string(trimmed))
		}
	}
	res.Message = parsed.Message
	if len(parsed.Data) > 0 && string(parsed.Data) != "null" {
		res.Data = parsed.Data
	}
	if status < 200 || status >= 300 {
		if res.Message == "" {
			res.Message = fmt.Sprintf("validate returned HTTP %d", status)
		}
		return res
	}
	if res.Message == "" {
		res.OK = true
		return res
	}
	if strings.Contains(strings.ToLower(res.Message), "warning") {
		res.OK = true
		res.Warnings = true
	}
	return res
}

// RuleStartLines counts, per 1-based line number, how many acls and grants
// rules start on that line of a HuJSON policy. The preview API identifies a
// matched rule only by line number, so a line holding more than one rule
// cannot be attributed to a single rule.
func RuleStartLines(policy []byte) (map[int]int, error) {
	v, err := hujson.Parse(cloneBytes(policy))
	if err != nil {
		return nil, err
	}
	var offsets []int
	for _, sec := range []string{"acls", "grants"} {
		if arr := v.Find("/" + sec); arr != nil {
			if a, ok := arr.Value.(*hujson.Array); ok {
				for _, el := range a.Elements {
					offsets = append(offsets, min(el.StartOffset, len(policy)))
				}
			}
		}
	}
	sort.Ints(offsets)
	counts := map[int]int{}
	line, pos := 1, 0
	for _, off := range offsets {
		line += bytes.Count(policy[pos:off], []byte("\n"))
		pos = off
		counts[line]++
	}
	return counts, nil
}

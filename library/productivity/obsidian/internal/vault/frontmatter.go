package vault

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Frontmatter is the parsed YAML block at the top of a vault note.
//
// Only Type/Date/Description/Status are first-class — every other field
// flows through Extra so type-specific schemas (person.email, meeting.people,
// etc.) can be validated without churning this struct on every protocol bump.
type Frontmatter struct {
	Type         string                 `yaml:"type"`
	Date         string                 `yaml:"date"`
	Description  string                 `yaml:"description"`
	Status       string                 `yaml:"status"`
	SupersededBy string                 `yaml:"superseded_by,omitempty"`
	Tags         []string               `yaml:"tags,omitempty"`
	FactsFile    string                 `yaml:"facts_file,omitempty"`
	Facts        []Fact                 `yaml:"facts,omitempty"`
	Extra        map[string]interface{} `yaml:"-"`
	Raw          string                 `yaml:"-"`
}

// Fact mirrors the inline facts: list shape of the UCE three-layer protocol.
type Fact struct {
	ID              string `yaml:"id" toml:"id"`
	Fact            string `yaml:"fact" toml:"fact"`
	Category        string `yaml:"category" toml:"category"`
	Timestamp       string `yaml:"timestamp,omitempty" toml:"timestamp,omitempty"`
	Status          string `yaml:"status,omitempty" toml:"status,omitempty"`
	Source          string `yaml:"source,omitempty" toml:"source,omitempty"`
	DecisionTraceID string `yaml:"decision_trace_id,omitempty" toml:"decision_trace_id,omitempty"`
}

// Note is a parsed `.md` note from a vault: frontmatter + raw body.
type Note struct {
	Path        string // path relative to vault root
	AbsPath     string // absolute filesystem path
	Frontmatter Frontmatter
	HasFM       bool
	Body        string
	BodyOffset  int    // byte offset where body starts (after closing ---)
	FMError     string // non-empty when frontmatter was present but malformed (degraded read)
}

// SplitFrontmatter pulls the leading `---\n...\n---\n` YAML block out of a
// note body. Returns (frontmatter-yaml, body, bodyOffset, found).
//
// Tolerates: BOM, leading whitespace before opening ---, CRLF line endings.
// Refuses: opening --- without closing --- (returns found=false).
func SplitFrontmatter(content []byte) (fmYAML []byte, body []byte, bodyOffset int, found bool) {
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF}) // strip UTF-8 BOM
	lines := bytes.SplitN(content, []byte("\n"), 2)
	if len(lines) == 0 {
		return nil, content, 0, false
	}
	if !isFenceLine(lines[0]) {
		return nil, content, 0, false
	}
	rest := lines[1]
	// Find the closing fence: a "---" on its own line.
	idx := indexOfFenceLine(rest)
	if idx < 0 {
		return nil, content, 0, false
	}
	fmYAML = rest[:idx]
	// Skip the closing fence + newline.
	after := rest[idx:]
	closeLine := bytes.SplitN(after, []byte("\n"), 2)
	if len(closeLine) == 2 {
		body = closeLine[1]
	} else {
		body = nil
	}
	bodyOffset = len(content) - len(body)
	return fmYAML, body, bodyOffset, true
}

func isFenceLine(line []byte) bool {
	s := strings.TrimRight(string(line), "\r")
	return strings.TrimSpace(s) == "---"
}

func indexOfFenceLine(b []byte) int {
	for i := 0; i < len(b); {
		end := bytes.IndexByte(b[i:], '\n')
		var line []byte
		if end < 0 {
			line = b[i:]
		} else {
			line = b[i : i+end]
		}
		if isFenceLine(line) {
			return i
		}
		if end < 0 {
			break
		}
		i += end + 1
	}
	return -1
}

// ParseFrontmatter decodes the YAML block into a Frontmatter struct,
// stashing unrecognized fields in Extra.
func ParseFrontmatter(fmYAML []byte) (Frontmatter, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(fmYAML, &raw); err != nil {
		return Frontmatter{}, fmt.Errorf("yaml decode: %w", err)
	}
	fm := Frontmatter{
		Raw:   string(fmYAML),
		Extra: map[string]interface{}{},
	}
	if raw == nil {
		return fm, nil
	}
	if v, ok := raw["type"].(string); ok {
		fm.Type = v
	}
	switch v := raw["date"].(type) {
	case string:
		fm.Date = v
	case nil:
	case time.Time:
		// YAML auto-parses unquoted `date: 2026-01-15` into time.Time. Preserve
		// the ISO date portion only — the protocol stores dates as YYYY-MM-DD.
		fm.Date = v.Format("2006-01-02")
	default:
		fm.Date = fmt.Sprintf("%v", v)
	}
	if v, ok := raw["description"].(string); ok {
		fm.Description = v
	}
	if v, ok := raw["status"].(string); ok {
		fm.Status = v
	}
	if v, ok := raw["superseded_by"].(string); ok {
		fm.SupersededBy = v
	}
	if v, ok := raw["facts_file"].(string); ok {
		fm.FactsFile = v
	}
	if tags, ok := raw["tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				fm.Tags = append(fm.Tags, s)
			}
		}
	}
	if facts, ok := raw["facts"].([]interface{}); ok {
		for _, f := range facts {
			m, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			fact := Fact{}
			if v, ok := m["id"].(string); ok {
				fact.ID = v
			}
			if v, ok := m["fact"].(string); ok {
				fact.Fact = v
			}
			if v, ok := m["category"].(string); ok {
				fact.Category = v
			}
			if v, ok := m["timestamp"]; ok {
				switch t := v.(type) {
				case string:
					fact.Timestamp = t
				case time.Time:
					fact.Timestamp = t.Format("2006-01-02")
				default:
					fact.Timestamp = fmt.Sprintf("%v", v)
				}
			}
			if v, ok := m["status"].(string); ok {
				fact.Status = v
			}
			if v, ok := m["source"].(string); ok {
				fact.Source = v
			}
			if v, ok := m["decision_trace_id"].(string); ok {
				fact.DecisionTraceID = v
			}
			fm.Facts = append(fm.Facts, fact)
		}
	}
	// Stash unrecognized top-level keys.
	known := map[string]bool{
		"type": true, "date": true, "description": true, "status": true,
		"superseded_by": true, "tags": true, "facts_file": true, "facts": true,
	}
	for k, v := range raw {
		if !known[k] {
			fm.Extra[k] = v
		}
	}
	return fm, nil
}

// Encode serializes the Frontmatter back to a YAML block (no enclosing --- fences).
// Field order is stable: type, date, description, status, superseded_by, tags,
// facts_file, then sorted Extra keys, then facts.
func (fm *Frontmatter) Encode() ([]byte, error) {
	out := map[string]interface{}{}
	if fm.Type != "" {
		out["type"] = fm.Type
	}
	if fm.Date != "" {
		out["date"] = fm.Date
	}
	if fm.Description != "" {
		out["description"] = fm.Description
	}
	if fm.Status != "" {
		out["status"] = fm.Status
	}
	if fm.SupersededBy != "" {
		out["superseded_by"] = fm.SupersededBy
	}
	if len(fm.Tags) > 0 {
		out["tags"] = fm.Tags
	}
	if fm.FactsFile != "" {
		out["facts_file"] = fm.FactsFile
	}
	for k, v := range fm.Extra {
		out[k] = v
	}
	if len(fm.Facts) > 0 {
		out["facts"] = fm.Facts
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	enc.Close()
	return buf.Bytes(), nil
}

// AssembleNote serializes frontmatter + body back to file bytes ready for write.
func AssembleNote(fm Frontmatter, body []byte) ([]byte, error) {
	fmBytes, err := fm.Encode()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n")
	if len(body) > 0 {
		if !bytes.HasPrefix(body, []byte("\n")) {
			buf.WriteByte('\n')
		}
		buf.Write(body)
	}
	return buf.Bytes(), nil
}

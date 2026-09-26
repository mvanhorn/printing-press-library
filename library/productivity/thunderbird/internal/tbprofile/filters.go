package tbprofile

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FilterAction is one action="..." / actionValue="..." pair.
type FilterAction struct {
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
}

// FilterTerm is one (field,op,value) condition term.
type FilterTerm struct {
	Field     string `json:"field"`
	Op        string `json:"op"`
	Value     string `json:"value"`
	Supported bool   `json:"supported"`
}

// Filter is one message filter rule of msgFilterRules.dat.
type Filter struct {
	Index     int            `json:"index"`
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	Type      string         `json:"type"`
	Actions   []FilterAction `json:"actions"`
	MatchType string         `json:"match_type"`
	Terms     []FilterTerm   `json:"terms"`
	Condition string         `json:"condition"`
	Raw       string         `json:"raw"`
}

// ParseFilterRules reads a msgFilterRules.dat file.
func ParseFilterRules(path string) ([]Filter, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return []Filter{}, nil
		}
		return nil, err
	}
	defer f.Close()
	return ParseFilterRulesReader(f)
}

// ParseFilterRulesReader parses msgFilterRules.dat content.
func ParseFilterRulesReader(r io.Reader) ([]Filter, error) {
	out := make([]Filter, 0)
	var cur *Filter
	var raw strings.Builder
	finish := func() {
		if cur != nil {
			cur.Raw = strings.TrimRight(raw.String(), "\n")
			out = append(out, *cur)
		}
		raw.Reset()
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = unquoteFilterValue(val)
		switch key {
		case "name":
			finish()
			cur = &Filter{Index: len(out), Name: val, Enabled: true, Actions: []FilterAction{}, Terms: []FilterTerm{}}
		case "enabled":
			if cur != nil {
				cur.Enabled = val == "yes"
			}
		case "type":
			if cur != nil {
				cur.Type = val
			}
		case "action":
			if cur != nil {
				cur.Actions = append(cur.Actions, FilterAction{Type: val})
			}
		case "actionValue":
			if cur != nil && len(cur.Actions) > 0 {
				cur.Actions[len(cur.Actions)-1].Value = val
			}
		case "condition":
			if cur != nil {
				cur.Condition = val
				cur.MatchType, cur.Terms = ParseCondition(val)
			}
		}
		if cur != nil {
			raw.WriteString(line)
			raw.WriteByte('\n')
		}
	}
	finish()
	return out, sc.Err()
}

func unquoteFilterValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(v)
}

// ParseCondition splits a condition string into its match type (AND, OR or
// ALL) and terms. Terms with fields or operators this CLI cannot evaluate
// are kept with Supported=false.
func ParseCondition(s string) (string, []FilterTerm) {
	s = strings.TrimSpace(s)
	terms := make([]FilterTerm, 0)
	if s == "" || strings.EqualFold(s, "ALL") {
		return "ALL", terms
	}
	match := ""
	i := 0
	for i < len(s) {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		if i >= len(s) {
			break
		}
		if strings.HasPrefix(s[i:], "AND") || strings.HasPrefix(s[i:], "OR") {
			word := "AND"
			if s[i] == 'O' {
				word = "OR"
			}
			if match == "" {
				match = word
			}
			i += len(word)
			continue
		}
		if s[i] != '(' {
			i++
			continue
		}
		fields, next := scanTerm(s, i+1)
		i = next
		if len(fields) >= 5 && fields[0] == "from" && fields[1] == "to" && fields[2] == "cc" && fields[3] == "or bcc" {
			fields = append([]string{"from,to,cc,or bcc"}, fields[4:]...)
		}
		t := FilterTerm{}
		if len(fields) >= 3 {
			t.Field, t.Op, t.Value = fields[0], fields[1], strings.Join(fields[2:], ",")
		} else if len(fields) == 2 {
			t.Field, t.Op = fields[0], fields[1]
		} else if len(fields) == 1 {
			t.Field = fields[0]
		}
		t.Supported = TermSupported(t)
		terms = append(terms, t)
	}
	if match == "" {
		match = "AND"
	}
	return match, terms
}

// scanTerm reads comma-separated fields until the closing ')' of a term,
// honouring "quoted" fields that may contain commas or parentheses.
func scanTerm(s string, i int) ([]string, int) {
	var fields []string
	var cur strings.Builder
	inQuote := false
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			cur.WriteByte(s[i+1])
			i += 2
			continue
		case c == '"':
			inQuote = !inQuote
		case c == ',' && !inQuote:
			fields = append(fields, cur.String())
			cur.Reset()
		case c == ')' && !inQuote:
			fields = append(fields, cur.String())
			return fields, i + 1
		default:
			cur.WriteByte(c)
		}
		i++
	}
	fields = append(fields, cur.String())
	return fields, i
}

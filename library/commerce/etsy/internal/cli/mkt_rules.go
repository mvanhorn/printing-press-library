// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written rule-file parser for `market gaps` (Slice 4). The module has no
// YAML dependency, so this parses exactly the documented subset:
//
//	targets:
//	  - name: "linen apron"
//	    include: ["apron", "pinafore"]   # flow list, or a block list of "- item"
//	    exclude:
//	      - "toddler"

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type mktTarget struct {
	Name      string
	Include   []*regexp.Regexp
	Exclude   []*regexp.Regexp
	All       []*regexp.Regexp
	TitleOnly bool
	Fold      bool
}

// mktMatches requires any include, every all pattern, and no exclude.
func (t *mktTarget) mktMatches(text string) bool {
	if t.Fold {
		text = mktFold(text)
	}
	hit := len(t.Include) == 0
	for _, re := range t.Include {
		if re.MatchString(text) {
			hit = true
			break
		}
	}
	if !hit {
		return false
	}
	for _, re := range t.All {
		if !re.MatchString(text) {
			return false
		}
	}
	for _, re := range t.Exclude {
		if re.MatchString(text) {
			return false
		}
	}
	return true
}

// mktLoadRules reads a rules file; path "-" reads the rules from stdin.
func mktLoadRules(path string, stdin io.Reader) ([]*mktTarget, error) {
	if path == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		return mktParseRules(string(data))
	}
	data, err := os.ReadFile(path) // #nosec G304 -- operator-supplied --rules file is this flag's documented input.
	if err != nil {
		return nil, err
	}
	return mktParseRules(string(data))
}

func mktUnquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		q := s[0]
		s = s[1 : len(s)-1]
		if q == '"' {
			s = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(s)
		}
	}
	return s
}

// mktStripComment removes a trailing # comment that is outside quotes.
func mktStripComment(ln string) string {
	var q byte
	for i := 0; i < len(ln); i++ {
		c := ln[i]
		switch {
		case q != 0:
			if c == '\\' && q == '"' {
				i++
			} else if c == q {
				q = 0
			}
		case c == '"' || c == '\'':
			q = c
		case c == '#' && (i == 0 || ln[i-1] == ' ' || ln[i-1] == '\t'):
			return ln[:i]
		}
	}
	return ln
}

func mktSplitFlow(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	out := make([]string, 0)
	var cur strings.Builder
	var q byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			cur.WriteByte(c)
			if c == '\\' && q == '"' && i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
			} else if c == q {
				q = 0
			}
		case c == '"' || c == '\'':
			q = c
			cur.WriteByte(c)
		case c == ',':
			if v := strings.TrimSpace(cur.String()); v != "" {
				out = append(out, mktUnquote(v))
			}
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if v := strings.TrimSpace(cur.String()); v != "" {
		out = append(out, mktUnquote(v))
	}
	return out
}

type mktRawTarget struct {
	name                  string
	include, exclude, all []string
	fields                []string
	fieldsSet, fold       bool
	fieldsLine            int
	foldErrorLine         int
}

func (r *mktRawTarget) fieldsError() error {
	prefix := ""
	if r.fieldsLine > 0 {
		prefix = fmt.Sprintf("line %d: ", r.fieldsLine)
	}
	return fmt.Errorf("%starget %q: fields must be [title] or [title, tags]", prefix, r.name)
}

// mktParseRulesJSON parses the JSON spelling of the rules schema
// ({"targets":[{"name":..,"include":[..],"exclude":[..]}]}). JSON is the
// stdin-friendly form: the live-dogfood stdin fixture must be valid JSON.
func mktParseRulesJSON(src string) ([]*mktTarget, error) {
	var doc struct {
		Targets []struct {
			Name    string          `json:"name"`
			Include []string        `json:"include"`
			Exclude []string        `json:"exclude"`
			All     []string        `json:"all"`
			Fields  json.RawMessage `json:"fields"`
			Fold    json.RawMessage `json:"fold"`
		} `json:"targets"`
	}
	dec := json.NewDecoder(strings.NewReader(src))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return nil, fmt.Errorf("JSON: %v (want name, include, exclude, all, fields, fold)", err)
		}
		return nil, fmt.Errorf("JSON: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("JSON: expected a single document")
	}
	// Exact keys are part of the rules interface; encoding/json otherwise ignores case.
	var keys map[string]json.RawMessage
	_ = json.Unmarshal([]byte(src), &keys)
	for key := range keys {
		if key != "targets" {
			return nil, fmt.Errorf("JSON: unknown field %q (want name, include, exclude, all, fields, fold)", key)
		}
	}
	var entries []map[string]json.RawMessage
	_ = json.Unmarshal(keys["targets"], &entries)
	for _, entry := range entries {
		for key := range entry {
			switch key {
			case "name", "include", "exclude", "all", "fields", "fold":
			default:
				return nil, fmt.Errorf("JSON: unknown field %q (want name, include, exclude, all, fields, fold)", key)
			}
		}
	}
	targets := make([]*mktRawTarget, 0, len(doc.Targets))
	for _, t := range doc.Targets {
		r := &mktRawTarget{name: t.Name, include: t.Include, exclude: t.Exclude, all: t.All}
		if t.Fields != nil {
			r.fieldsSet = true
			if string(t.Fields) == "null" {
				return nil, r.fieldsError()
			}
			if err := json.Unmarshal(t.Fields, &r.fields); err != nil {
				return nil, fmt.Errorf("target %q: fields must be [title] or [title, tags]: %w", r.name, err)
			}
		}
		if t.Fold != nil {
			if string(t.Fold) != "true" && string(t.Fold) != "false" {
				return nil, fmt.Errorf("target %q: fold must be true or false", r.name)
			}
			r.fold = string(t.Fold) == "true"
		}
		targets = append(targets, r)
	}
	return mktCompileTargets(targets)
}

func mktParseRules(src string) ([]*mktTarget, error) {
	if strings.HasPrefix(strings.TrimSpace(src), "{") {
		return mktParseRulesJSON(src)
	}
	type raw = mktRawTarget
	var targets []*raw
	var cur *raw
	var listKey string // key collecting a block list
	inTargets := false
	for n, line := range strings.Split(src, "\n") {
		ln := strings.TrimRight(mktStripComment(line), " \t\r")
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "targets:") && !strings.HasPrefix(ln, " ") {
			inTargets = true
			continue
		}
		if !inTargets {
			return nil, fmt.Errorf("line %d: expected top-level 'targets:'", n+1)
		}
		if strings.HasPrefix(trim, "- ") || trim == "-" {
			item := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
			if listKey != "" && cur != nil && !strings.Contains(item, ": ") && !strings.HasSuffix(item, ":") {
				v := mktUnquote(item)
				cur.appendList(listKey, []string{v})
				continue
			}
			cur = &raw{}
			targets = append(targets, cur)
			listKey = ""
			if item == "" {
				continue
			}
			trim = item
		}
		if cur == nil {
			return nil, fmt.Errorf("line %d: expected a '- name:' target entry", n+1)
		}
		i := strings.Index(trim, ":")
		if i < 0 {
			return nil, fmt.Errorf("line %d: expected key: value", n+1)
		}
		key, val := strings.TrimSpace(trim[:i]), strings.TrimSpace(trim[i+1:])
		listKey = ""
		switch key {
		case "name":
			cur.name = mktUnquote(val)
		case "fold":
			if val != "true" && val != "false" {
				cur.foldErrorLine = n + 1
			}
			cur.fold = val == "true"
		case "include", "exclude", "all", "fields":
			if key == "fields" {
				cur.fieldsSet = true
				cur.fieldsLine = n + 1
			}
			var items []string
			switch {
			case strings.HasPrefix(val, "["):
				items = mktSplitFlow(val)
			case val == "":
				listKey = key
			default:
				items = []string{mktUnquote(val)}
			}
			cur.appendList(key, items)
		default:
			return nil, fmt.Errorf("line %d: unknown key %q (want name, include, exclude, all, fields, fold)", n+1, key)
		}
	}
	return mktCompileTargets(targets)
}

func mktCompileTargets(targets []*mktRawTarget) ([]*mktTarget, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("rules file defines no targets")
	}
	// Bound compilation work across include, exclude and all in every rules source.
	count := 0
	for _, r := range targets {
		count += len(r.include) + len(r.exclude) + len(r.all)
		if count > 256 {
			return nil, fmt.Errorf("maximum 256 patterns across include, exclude and all")
		}
		for _, patterns := range [][]string{r.include, r.exclude, r.all} {
			for _, p := range patterns {
				if len(p) > 512 {
					return nil, fmt.Errorf("%s", mktEcho("pattern exceeds 512 bytes: "+p))
				}
			}
		}
	}
	out := make([]*mktTarget, 0, len(targets))
	for _, r := range targets {
		if r.name == "" {
			return nil, fmt.Errorf("a target is missing 'name'")
		}
		if r.foldErrorLine > 0 {
			return nil, fmt.Errorf("line %d: target %q: fold must be true or false", r.foldErrorLine, r.name)
		}
		t := &mktTarget{Name: r.name, Fold: r.fold}
		if r.fieldsSet {
			seen := map[string]bool{}
			for _, f := range r.fields {
				if (f != "title" && f != "tags") || seen[f] {
					return nil, r.fieldsError()
				}
				seen[f] = true
			}
			if !seen["title"] {
				return nil, r.fieldsError()
			}
			t.TitleOnly = !seen["tags"]
		}
		for i, patterns := range [][]string{r.include, r.exclude, r.all} {
			for _, p := range patterns {
				re, err := regexp.Compile("(?i)" + p)
				if err != nil {
					return nil, fmt.Errorf("%s", mktEcho(fmt.Sprintf("target %q %s %q: %v", r.name, []string{"include", "exclude", "all"}[i], p, err)))
				}
				if r.fold {
					tree, _ := syntax.Parse(p, syntax.Perl)
					diagnostic := func(message string) error {
						return fmt.Errorf("%s", mktEcho(fmt.Sprintf("target %q: pattern %q: %s", r.name, p, message)))
					}
					var check func(*syntax.Regexp) error
					check = func(node *syntax.Regexp) error {
						if node.Op == syntax.OpCharClass {
							kept := false
							for j := 0; j < len(node.Rune); j += 2 {
								lo, hi := node.Rune[j], node.Rune[j+1]
								if lo == hi && lo != ' ' && !mktFoldRune(lo) && !mktBuiltinClass(node) {
									return diagnostic(fmt.Sprintf("fold removes %q in class; it and adjacent spaces become one space", lo))
								}
								for c := lo; c <= hi; c++ {
									if c == ' ' || mktFoldRune(c) {
										kept = true
										break
									}
								}
							}
							if !kept {
								return diagnostic(fmt.Sprintf("fold removes every character in class (first removed rune %q)", node.Rune[0]))
							}
						}
						if node.Op == syntax.OpConcat {
							for j := 1; j < len(node.Sub); j++ {
								if mktForcedSpace(node.Sub[j-1], false) && mktForcedSpace(node.Sub[j], true) {
									return diagnostic("fold collapses whitespace runs to one space")
								}
							}
						}
						if node.Op == syntax.OpRepeat && node.Min >= 2 && mktWhitespaceOnly(node.Sub[0]) {
							return diagnostic("fold collapses whitespace runs to one space")
						}
						if node.Op == syntax.OpLiteral {
							if strings.Contains(string(node.Rune), "  ") {
								return diagnostic("fold collapses whitespace runs to one space")
							}
							for _, c := range node.Rune {
								if !mktFoldRune(c) && c != ' ' {
									message := fmt.Sprintf("fold removes %q; it and adjacent spaces become one space", c)
									// A complete literal tree is safe to render without rewriting operators.
									if tree.Op == syntax.OpLiteral && !strings.ContainsAny(p, "[]|") {
										message += fmt.Sprintf(", so write %q", mktLiteralSpaceRewrite(node.Rune))
									}
									return diagnostic(message)
								}
							}
						}
						for _, sub := range node.Sub {
							if err := check(sub); err != nil {
								return err
							}
						}
						return nil
					}
					if err := check(tree); err != nil {
						return nil, err
					}

				}
				switch i {
				case 0:
					t.Include = append(t.Include, re)
				case 1:
					t.Exclude = append(t.Exclude, re)
				case 2:
					t.All = append(t.All, re)
				}
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// mktRowText is the text rules match against: title plus tags.
func mktRowText(r mktRow) string {
	return r.Title + " " + strings.Join(r.Tags, " ")
}

// etsy-ext: inline content is bounded independently of platform argv limits.
const mktInlineMaxBytes = 98304

// mktEcho caps diagnostic content including its ellipsis, at a UTF-8 boundary.
func mktEcho(s string) string {
	if len(s) <= 200 {
		return s
	}
	n := 197
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

func mktInlineRules(src string) ([]*mktTarget, error) {
	if len(src) > mktInlineMaxBytes {
		return nil, fmt.Errorf("--rules-json exceeds %d bytes", mktInlineMaxBytes)
	}
	if !json.Valid([]byte(src)) {
		return nil, fmt.Errorf("--rules-json must be JSON: %s", mktEcho(src))
	}
	targets, err := mktParseRules(src)
	if err != nil {
		return nil, fmt.Errorf("--rules-json: %s", mktEcho(err.Error()))
	}
	return targets, nil
}

func mktInlineControl(src string) ([]string, error) {
	if len(src) > mktInlineMaxBytes {
		return nil, fmt.Errorf("--control-ids exceeds %d bytes", mktInlineMaxBytes)
	}
	ids := []string{}
	for _, id := range strings.Split(src, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil || n <= 0 || strings.IndexFunc(id, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return nil, fmt.Errorf("--control-ids requires positive integer ids: %s", mktEcho(id))
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// etsy-ext: matching options are Etsy-owned policy over local market rows.
func (r *mktRawTarget) appendList(key string, items []string) {
	switch key {
	case "include":
		r.include = append(r.include, items...)
	case "exclude":
		r.exclude = append(r.exclude, items...)
	case "all":
		r.all = append(r.all, items...)
	case "fields":
		r.fields = append(r.fields, items...)
	}
}
func mktFoldRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '%' }
func mktFold(text string) string {
	var out strings.Builder
	space := false
	for _, r := range strings.ToLower(text) {
		if mktFoldRune(r) {
			out.WriteRune(r)
			space = false
		} else if !space {
			out.WriteByte(' ')
			space = true
		}
	}
	return out.String()
}
func (t *mktTarget) matchesRow(r mktRow) bool {
	if t.TitleOnly {
		return t.mktMatches(r.Title)
	}
	return t.mktMatches(mktRowText(r))
}
func mktMatchesAllNotes(targets []*mktTarget) []string {
	notes := []string{}
	for _, t := range targets {
		if len(t.Include) == 0 && len(t.All) == 0 {
			notes = append(notes, fmt.Sprintf("target %q has no include or all patterns; it matches every listing not excluded", t.Name))
		}
	}
	return notes
}

// mktWhitespaceOnly describes nodes that can only consume spaces in folded text.
func mktWhitespaceOnly(n *syntax.Regexp) bool {
	switch n.Op {
	case syntax.OpLiteral:
		return len(n.Rune) > 0 && strings.Trim(string(n.Rune), " ") == ""
	case syntax.OpCharClass:
		space := false
		for j := 0; j < len(n.Rune); j += 2 {
			for c := n.Rune[j]; c <= n.Rune[j+1]; c++ {
				if c == ' ' {
					space = true
				} else if mktFoldRune(c) {
					return false
				}
			}
		}
		return space
	case syntax.OpCapture:
		return mktWhitespaceOnly(n.Sub[0])
	}
	return false
}

// mktForcedSpace reports a mandatory space at the selected boundary.
func mktForcedSpace(n *syntax.Regexp, start bool) bool {
	if mktWhitespaceOnly(n) {
		return true
	}
	switch n.Op {
	case syntax.OpLiteral:
		if len(n.Rune) == 0 {
			return false
		}
		if start {
			return n.Rune[0] == ' '
		}
		return n.Rune[len(n.Rune)-1] == ' '
	case syntax.OpCapture, syntax.OpPlus:
		return mktForcedSpace(n.Sub[0], start)
	case syntax.OpRepeat:
		return n.Min > 0 && mktForcedSpace(n.Sub[0], start)
	case syntax.OpConcat:
		if start {
			return mktForcedSpace(n.Sub[0], start)
		}
		return mktForcedSpace(n.Sub[len(n.Sub)-1], start)
	}
	return false
}

// mktLiteralSpaceRewrite replaces removed runes and flanking literal spaces only.
func mktLiteralSpaceRewrite(runes []rune) string {
	var out strings.Builder
	for i := 0; i < len(runes); {
		if runes[i] == ' ' || !mktFoldRune(runes[i]) {
			j, removed := i, false
			for j < len(runes) && (runes[j] == ' ' || !mktFoldRune(runes[j])) {
				removed = removed || runes[j] != ' '
				j++
			}
			if removed {
				out.WriteByte(' ')
			} else {
				out.WriteString(string(runes[i:j]))
			}
			i = j
		} else {
			out.WriteRune(runes[i])
			i++
		}
	}
	return out.String()
}

// Built-in broad classes may include isolated punctuation in their expansion.
func mktBuiltinClass(n *syntax.Regexp) bool {
	for _, pattern := range []string{`\w`, `\W`, `\S`} {
		class, _ := syntax.Parse(pattern, syntax.Perl)
		if n.String() == class.String() {
			return true
		}
	}
	return false
}

// Rewrite only removed literal runes and their flanking literal spaces.
func mktLiteralRewrite(runes []rune) string {
	var out strings.Builder
	for i := 0; i < len(runes); {
		if runes[i] != ' ' && mktFoldRune(runes[i]) {
			out.WriteRune(runes[i])
			i++
			continue
		}
		end, removed := i, false
		for end < len(runes) && (runes[end] == ' ' || !mktFoldRune(runes[end])) {
			removed = removed || runes[end] != ' '
			end++
		}
		if removed {
			out.WriteByte(' ')
		} else {
			out.WriteString(string(runes[i:end]))
		}
		i = end
	}
	return out.String()
}

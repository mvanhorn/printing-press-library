package gplay

import (
	"encoding/json"
	neturl "net/url"
	"regexp"
	"strings"
)

// node wraps a decoded protojson value (anonymous nested arrays) for safe
// positional access. Out-of-range or wrong-type access yields a nil node
// rather than panicking, which is essential against Play's fragile index
// paths: a shifted layout degrades to empty fields, not a crash.
type node struct {
	v any
}

func decode(raw json.RawMessage) node {
	if len(raw) == 0 {
		return node{}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return node{}
	}
	return node{v: v}
}

func wrap(v any) node { return node{v: v} }

// at indexes into an array node. Negative indices count from the end.
func (n node) at(i int) node {
	arr, ok := n.v.([]any)
	if !ok {
		return node{}
	}
	if i < 0 {
		i = len(arr) + i
	}
	if i < 0 || i >= len(arr) {
		return node{}
	}
	return node{v: arr[i]}
}

// path walks a sequence of array indices.
func (n node) path(idx ...int) node {
	cur := n
	for _, i := range idx {
		cur = cur.at(i)
	}
	return cur
}

// key indexes into an object node by string key. Play occasionally encodes a
// level as a sparse object (e.g. datasafety's "138") instead of an array.
func (n node) key(k string) node {
	m, ok := n.v.(map[string]any)
	if !ok {
		return node{}
	}
	v, ok := m[k]
	if !ok {
		return node{}
	}
	return node{v: v}
}

// walkEntries recursively collects arrays whose element [1] is a short string
// label and element [2] is an array — the shape of a datasafety data entry —
// returning (label, description) pairs. maxLabelLen filters out section blurbs.
func (n node) walkEntries(maxLabelLen int) []DataSafetyEntry {
	var out []DataSafetyEntry
	seen := map[string]bool{}
	var walk func(x node)
	walk = func(x node) {
		arr, ok := x.v.([]any)
		if !ok {
			return
		}
		if len(arr) >= 3 {
			if label, ok := arr[1].(string); ok && label != "" && len([]rune(label)) <= maxLabelLen {
				if _, isArr := arr[2].([]any); isArr && !seen[label] {
					seen[label] = true
					out = append(out, DataSafetyEntry{
						Data: cleanText(label),
						Type: node{v: arr[2]}.path(1).cleanStr(),
					})
				}
			}
		}
		for _, e := range arr {
			walk(node{v: e})
		}
	}
	walk(n)
	return out
}

func (n node) str() string {
	if s, ok := n.v.(string); ok {
		return s
	}
	return ""
}

func (n node) cleanStr() string { return cleanText(n.str()) }

func (n node) float() float64 {
	switch t := n.v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	}
	return 0
}

func (n node) int() int { return int(n.float()) }

func (n node) int64() int64 { return int64(n.float()) }

func (n node) bool() bool {
	switch t := n.v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	}
	return false
}

func (n node) len() int {
	if arr, ok := n.v.([]any); ok {
		return len(arr)
	}
	return 0
}

func (n node) isArray() bool {
	_, ok := n.v.([]any)
	return ok
}

func (n node) arr() []node {
	arr, ok := n.v.([]any)
	if !ok {
		return nil
	}
	out := make([]node, len(arr))
	for i, e := range arr {
		out[i] = node{v: e}
	}
	return out
}

var (
	brTagRe   = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
)

// cleanText converts HTML markup carried in Play description/summary fields to
// clean text: <br> becomes a newline, other tags are stripped, then common
// entities are unescaped. Play returns descriptions with literal <br> markup;
// without this a user piping `summary` sees raw tags.
func cleanText(s string) string {
	if s == "" {
		return s
	}
	s = brTagRe.ReplaceAllString(s, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	r := strings.NewReplacer(
		"&amp;", "&",
		"&#39;", "'",
		"&#039;", "'",
		"&quot;", "\"",
		"&lt;", "<",
		"&gt;", ">",
		"&nbsp;", " ",
		"\\u003d", "=",
		"\\u0026", "&",
	)
	return strings.TrimSpace(r.Replace(s))
}

// pkgFromURL pulls the appId out of a /store/apps/details?id=<pkg> URL.
func pkgFromURL(u string) string {
	i := strings.Index(u, "id=")
	if i < 0 {
		return ""
	}
	rest := u[i+3:]
	if amp := strings.IndexAny(rest, "&"); amp >= 0 {
		rest = rest[:amp]
	}
	return rest
}

// devIDFromURL pulls the developer id out of a /store/apps/dev?id= or
// /developer?id= URL and URL-decodes it (display names arrive as
// "Dream+Games%2C+Ltd." and must become "Dream Games, Ltd." so the value can be
// re-encoded correctly when fetched).
func devIDFromURL(u string) string {
	raw := pkgFromURL(u)
	if raw == "" {
		return ""
	}
	if dec, err := neturl.QueryUnescape(raw); err == nil {
		return dec
	}
	return raw
}

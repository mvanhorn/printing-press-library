package cli

import (
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/internal/pocket"
	"strings"
)

// project applies a tree of requested dotted paths while preserving meta.
func project(v any, selectFields string) (any, error) {
	if selectFields == "" {
		return v, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err = json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	tree := map[string]any{}
	for _, p := range strings.Split(selectFields, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, pocket.Fail("usage", "--select contains an empty field path")
		}
		parts := strings.Split(p, ".")
		node := tree
		for i, k := range parts {
			if k == "" {
				return nil, pocket.Fail("usage", "--select contains an empty path segment")
			}
			if i == len(parts)-1 {
				node[k] = true
				break
			}
			if node[k] == true {
				break
			}
			if node[k] == nil {
				node[k] = map[string]any{}
			}
			node = node[k].(map[string]any)
		}
	}
	out, ok := projectNode(data, tree)
	if !ok {
		return nil, pocket.Fail("usage", "--select matched no fields; inspect command output or schema")
	}
	m := out.(map[string]any)
	if meta, exists := data["meta"]; exists {
		m["meta"] = meta
	}
	return m, nil
}
func projectNode(v any, tree map[string]any) (any, bool) {
	if a, ok := v.([]any); ok {
		r := []any{}
		matched := len(a) == 0
		for _, x := range a {
			y, hit := projectNode(x, tree)
			r = append(r, y)
			matched = matched || hit
		}
		return r, matched
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	r := map[string]any{}
	matched := false
	for key, t := range tree {
		x, exists := m[key]
		if !exists {
			continue
		}
		if t == true {
			r[key] = x
			matched = true
			continue
		}
		if x == nil {
			r[key] = nil
			matched = true
			continue
		}
		y, hit := projectNode(x, t.(map[string]any))
		if hit {
			r[key] = y
			matched = true
		}
	}
	return r, matched
}

package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
)

// Codex parity r2 P2: the GET /deals 200 is a oneOf, so a real snapshot envelope must validate against exactly ONE branch and
// a real paginated envelope against exactly ONE (the other must refuse it). Checked in both spec files.
const (
	specSnapshotAnswer  = `{"data":{"columns":[{"stageId":"44444444-4444-4444-8444-444444444441","limit":25,"deals":[{"id":"d1"}]}],"meta":{"board_snapshot":true}}}`
	specPaginatedAnswer = `{"data":{"data":[{"id":"d1"}],"meta":{"page":1,"per_page":25,"total":1}}}`
)

// dealsListBranches returns the resolved 200 oneOf branches of GET /deals (each a root schema that can reach the components).
func dealsListBranches(t *testing.T, file string) []*jsonschema.Resolved {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	dig := func(v any, keys ...string) any {
		for _, k := range keys {
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("%s: no %q on the way to the GET /deals 200 schema", file, k)
			}
			v = m[k]
		}
		return v
	}
	schema := dig(doc, "paths", "/deals", "get", "responses", "200", "content", "application/json", "schema")
	branches, ok := dig(schema, "oneOf").([]any)
	if !ok || len(branches) < 2 {
		t.Fatalf("%s: GET /deals 200 has no oneOf with at least two branches", file)
	}
	all, _ := dig(doc, "components", "schemas").(map[string]any)
	var out []*jsonschema.Resolved
	for i, b := range branches {
		// only the components this branch reaches: the full public document carries patterns (lookaheads) that Go's
		// regexp, and so the resolver, cannot compile, in schemas no GET /deals branch uses (reprint637)
		defs := map[string]any{}
		var walk func(v any)
		walk = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				if ref, ok := x["$ref"].(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
					name := strings.TrimPrefix(ref, "#/components/schemas/")
					if _, seen := defs[name]; !seen && all[name] != nil {
						defs[name] = all[name]
						walk(all[name])
					}
				}
				for _, e := range x {
					walk(e)
				}
			case []any:
				for _, e := range x {
					walk(e)
				}
			}
		}
		walk(b)
		j, err := json.Marshal(map[string]any{"$defs": defs, "allOf": []any{b}})
		if err != nil {
			t.Fatal(err)
		}
		var s jsonschema.Schema
		if err := json.Unmarshal([]byte(strings.ReplaceAll(string(j), "#/components/schemas/", "#/$defs/")), &s); err != nil {
			t.Fatalf("%s branch %d: %v", file, i, err)
		}
		r, err := s.Resolve(nil)
		if err != nil {
			t.Fatalf("%s branch %d: %v", file, i, err)
		}
		out = append(out, r)
	}
	return out
}

func matchingBranches(t *testing.T, branches []*jsonschema.Resolved, answer string) int {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(answer), &v); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range branches {
		if r.Validate(v) == nil {
			n++
		}
	}
	return n
}

func TestDealsListOKBranchesAreMutuallyExclusive(t *testing.T) {
	for _, f := range []string{"../../spec.yaml", "../../upstream/openapi.yaml"} {
		branches := dealsListBranches(t, f)
		if n := matchingBranches(t, branches, specSnapshotAnswer); n != 1 {
			t.Errorf("%s: a snapshot answer matches %d branches, want exactly 1", f, n)
		}
		if n := matchingBranches(t, branches, specPaginatedAnswer); n != 1 {
			t.Errorf("%s: a paginated answer matches %d branches, want exactly 1", f, n)
		}
		// a half-and-half envelope never matches two branches (reprint637: the public document's list branch does not forbid
		// extra keys, so it may match that one; it must never also match the snapshot branch)
		if n := matchingBranches(t, branches, `{"data":{"data":[],"meta":{"page":1},"columns":[]}}`); n > 1 {
			t.Errorf("%s: an answer with both data.data and data.columns matches %d branches, want at most 1", f, n)
		}
	}
}

// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestScalarItemID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"integer", `39152`, "39152"},
		{"large integer keeps exact form", `100000000000001`, "100000000000001"},
		{"string scalar", `"abc"`, "abc"},
		{"object is not a scalar id", `{"id":1}`, ""},
		{"array is not a scalar id", `[1,2]`, ""},
		{"null", `null`, ""},
		{"empty", ``, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scalarItemID(json.RawMessage(tc.in)); got != tc.want {
				t.Fatalf("scalarItemID(%s) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// fakeHydrateClient serves canned item responses keyed by request path and
// records which paths were requested.
type fakeHydrateClient struct {
	responses map[string]json.RawMessage
	errs      map[string]error
	calls     []string
}

func (f *fakeHydrateClient) Get(path string, _ map[string]string) (json.RawMessage, error) {
	f.calls = append(f.calls, path)
	if err, ok := f.errs[path]; ok {
		return nil, err
	}
	if r, ok := f.responses[path]; ok {
		return r, nil
	}
	return json.RawMessage(`null`), nil
}

func TestHydrateScalarItems_HydratesIDsIntoObjects(t *testing.T) {
	c := &fakeHydrateClient{
		responses: map[string]json.RawMessage{
			"/item/1.json": json.RawMessage(`{"id":1,"title":"one"}`),
			"/item/2.json": json.RawMessage(`{"id":2,"title":"two"}`),
		},
	}
	in := []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)}

	out, failures := hydrateScalarItems(c, "stories", in)

	if failures != 0 {
		t.Fatalf("failures = %d, want 0", failures)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	for i, raw := range out {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("item %d not a JSON object after hydration: %v", i, err)
		}
		if _, ok := obj["id"]; !ok {
			t.Fatalf("item %d missing extractable id after hydration: %s", i, raw)
		}
	}
}

func TestHydrateScalarItems_CountsUnfetchableIDsAsFailures(t *testing.T) {
	c := &fakeHydrateClient{
		responses: map[string]json.RawMessage{
			"/item/1.json": json.RawMessage(`{"id":1}`),
			"/item/3.json": json.RawMessage(`null`), // deleted/unknown item
		},
		errs: map[string]error{
			"/item/2.json": fmt.Errorf("network boom"),
		},
	}
	in := []json.RawMessage{
		json.RawMessage(`1`),
		json.RawMessage(`2`),
		json.RawMessage(`3`),
	}

	out, failures := hydrateScalarItems(c, "stories", in)

	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1 (only id 1 hydrated)", len(out))
	}
	if failures != 2 {
		t.Fatalf("failures = %d, want 2 (network error + null body)", failures)
	}
}

func TestHydrateScalarItems_PassesThroughObjectsAndUnknownResources(t *testing.T) {
	c := &fakeHydrateClient{}

	// Already-object items must not trigger a hydration fetch.
	objItem := json.RawMessage(`{"id":7,"title":"already an object"}`)
	out, failures := hydrateScalarItems(c, "stories", []json.RawMessage{objItem})
	if failures != 0 || len(out) != 1 {
		t.Fatalf("object pass-through: out=%d failures=%d, want 1/0", len(out), failures)
	}
	if len(c.calls) != 0 {
		t.Fatalf("object item triggered %d hydration calls, want 0", len(c.calls))
	}

	// A resource not registered for hydration is returned unchanged.
	in := []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)}
	out, failures = hydrateScalarItems(c, "not-a-hydration-resource", in)
	if failures != 0 || len(out) != 2 {
		t.Fatalf("unknown resource: out=%d failures=%d, want 2/0", len(out), failures)
	}
	if len(c.calls) != 0 {
		t.Fatalf("unknown resource triggered %d hydration calls, want 0", len(c.calls))
	}
}

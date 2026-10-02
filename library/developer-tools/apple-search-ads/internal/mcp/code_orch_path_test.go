// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"net/http"
	"strings"
	"testing"
)

func TestCodeOrchResolveNestedPathIDs(t *testing.T) {
	var endpoint *codeOrchEndpoint
	for i := range codeOrchEndpoints {
		if codeOrchEndpoints[i].ID == "apps.get_product_pages" {
			endpoint = &codeOrchEndpoints[i]
			break
		}
	}
	if endpoint == nil {
		t.Fatal("nested product-page endpoint missing")
	}
	params := map[string]any{"id": []any{"app/one", "page?two"}}
	got, err := codeOrchResolvePath(*endpoint, params)
	if err != nil {
		t.Fatal(err)
	}
	if want := "/apps/app%2Fone/product-pages/page%3Ftwo"; got != want {
		t.Fatalf("path=%q, want %q", got, want)
	}
	if _, ok := params["id"]; ok {
		t.Fatal("path IDs leaked into query params")
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.test"+got, nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.EscapedPath() != got {
		t.Fatalf("HTTP path=%v error=%v, want %q", req.URL, err, got)
	}
}

func TestCodeOrchResolvePathRejectsAmbiguousOrMissingIDs(t *testing.T) {
	ep := codeOrchEndpoint{ID: "nested", Path: "/apps/{id}/product-pages/{id}", Positional: []string{"id"}}
	for _, params := range []map[string]any{
		{},
		{"id": "one"},
		{"id": []any{"one"}},
		{"id": []any{"one", ""}},
		{"id": []any{"one", "two", "three"}},
	} {
		if path, err := codeOrchResolvePath(ep, params); err == nil {
			t.Fatalf("ambiguous path accepted: %q", path)
		}
	}
	if path, err := codeOrchResolvePath(codeOrchEndpoint{ID: "bad", Path: "/apps/{id}"}, map[string]any{}); err == nil {
		t.Fatalf("unresolved metadata placeholder accepted: %q", path)
	}
}

func TestCodeOrchResolvePathRejectsDotSegmentsAndEncodesReservedCharacters(t *testing.T) {
	ep := codeOrchEndpoint{ID: "single", Path: "/apps/{id}", Positional: []string{"id"}}
	for _, id := range []string{".", ".."} {
		if path, err := codeOrchResolvePath(ep, map[string]any{"id": id}); err == nil {
			t.Fatalf("dot-only ID %q accepted as %q", id, path)
		}
	}
	for _, tc := range []struct{ id, want string }{
		{"a/b", "/apps/a%2Fb"},
		{"a?b", "/apps/a%3Fb"},
	} {
		path, err := codeOrchResolvePath(ep, map[string]any{"id": tc.id})
		if err != nil || path != tc.want {
			t.Fatalf("ID=%q path=%q err=%v, want %q", tc.id, path, err, tc.want)
		}
	}
	if _, err := codeOrchResolvePath(ep, map[string]any{"id": []any{"unexpected"}}); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("array for a single ID was accepted: %v", err)
	}
}

// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package substack

import (
	"encoding/json"
	"testing"
)

func TestParsePostRef(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantKind RefKind
		wantHost string
		wantSlug string
		wantID   string
		wantErr  bool
	}{
		{name: "bare handle/slug", input: "astralcodexten/open-thread-441", wantKind: RefSlugHost, wantHost: "astralcodexten.substack.com", wantSlug: "open-thread-441"},
		{name: "subdomain /p/ url", input: "https://uxmentor.substack.com/p/my-slug", wantKind: RefSlugHost, wantHost: "uxmentor.substack.com", wantSlug: "my-slug"},
		{name: "custom domain /p/ url", input: "https://creatoreconomy.so/p/behind-the-craft", wantKind: RefSlugHost, wantHost: "creatoreconomy.so", wantSlug: "behind-the-craft"},
		{name: "custom domain www /p/ url", input: "www.userresearchstrategist.com/p/the-post", wantKind: RefSlugHost, wantHost: "www.userresearchstrategist.com", wantSlug: "the-post"},
		{name: "trailing slash and query", input: "https://uxmentor.substack.com/p/my-slug/?utm=x", wantKind: RefSlugHost, wantHost: "uxmentor.substack.com", wantSlug: "my-slug"},
		{name: "reader-app home post p-id", input: "https://substack.com/home/post/p-198935601", wantKind: RefByID, wantID: "198935601"},
		{name: "reader-app inbox post id", input: "https://substack.com/inbox/post/205636469", wantKind: RefByID, wantID: "205636469"},
		{name: "reader-app profile rejected form parses", input: "https://substack.com/@uxmentor", wantKind: RefProfile},
		{name: "empty", input: "", wantErr: true},
		{name: "substack.com no post", input: "https://substack.com/foo/bar", wantErr: true},
		{name: "host without slug", input: "uxmentor.substack.com", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := ParsePostRef(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got ref %+v", ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.Kind != tc.wantKind {
				t.Errorf("kind = %v, want %v", ref.Kind, tc.wantKind)
			}
			if ref.Host != tc.wantHost {
				t.Errorf("host = %q, want %q", ref.Host, tc.wantHost)
			}
			if ref.Slug != tc.wantSlug {
				t.Errorf("slug = %q, want %q", ref.Slug, tc.wantSlug)
			}
			if ref.ID != tc.wantID {
				t.Errorf("id = %q, want %q", ref.ID, tc.wantID)
			}
		})
	}
}

func TestExtractByIDPost(t *testing.T) {
	env := json.RawMessage(`{"post":{"id":123,"slug":"s","body_html":"<p>full</p>","is_viewed":true},"publication":{"subdomain":"x"}}`)
	post, err := ExtractByIDPost(env)
	if err != nil {
		t.Fatalf("ExtractByIDPost: %v", err)
	}
	meta, err := ParsePostMeta(post)
	if err != nil {
		t.Fatalf("ParsePostMeta: %v", err)
	}
	if meta.ID != "123" {
		t.Errorf("ID = %q, want 123", meta.ID)
	}
	if !meta.HasIsViewed {
		t.Error("expected HasIsViewed true from the authed by-id post")
	}
	if _, err := ExtractByIDPost(json.RawMessage(`{"publication":{}}`)); err == nil {
		t.Error("expected error when envelope has no post object")
	}
}

func TestCanonicalHost(t *testing.T) {
	cases := []struct {
		sub, custom, want string
	}{
		{"uxmentor", "", "uxmentor.substack.com"},
		{"peteryang", "creatoreconomy.so", "creatoreconomy.so"},
		{"peteryang", "https://creatoreconomy.so", "creatoreconomy.so"},
		{"", "", ""},
	}
	for _, tc := range cases {
		if got := CanonicalHost(tc.sub, tc.custom); got != tc.want {
			t.Errorf("CanonicalHost(%q,%q) = %q, want %q", tc.sub, tc.custom, got, tc.want)
		}
	}
}

// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestRootCmdRegistersSubdomainFlag(t *testing.T) {
	t.Parallel()

	root := RootCmd()
	if root.PersistentFlags().Lookup("subdomain") == nil {
		t.Fatal("root command must expose --subdomain for publication-scoped endpoints")
	}
}

func TestPublicationAPIPath(t *testing.T) {
	t.Parallel()

	got := publicationAPIPath("/drafts")
	want := "https://{publication}.substack.com/api/v1/drafts"
	if got != want {
		t.Fatalf("publicationAPIPath = %q, want %q", got, want)
	}
}

func TestSyncResourcePathPublicationScopedResourcesUsePublicationHost(t *testing.T) {
	t.Parallel()

	for _, resource := range []string{"drafts", "posts", "posts-published", "posts-ranked", "sections", "subs", "tags"} {
		resource := resource
		t.Run(resource, func(t *testing.T) {
			t.Parallel()
			got, err := syncResourcePath(resource)
			if err != nil {
				t.Fatalf("syncResourcePath returned error: %v", err)
			}
			if got == "" || got[0] == '/' {
				t.Fatalf("syncResourcePath(%q) = %q, want publication host URL", resource, got)
			}
			if !strings.HasPrefix(got, substackPublicationAPIBase) {
				t.Fatalf("syncResourcePath(%q) = %q, want %q prefix", resource, got, substackPublicationAPIBase)
			}
		})
	}
}

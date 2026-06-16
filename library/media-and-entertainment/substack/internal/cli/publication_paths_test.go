// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
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

func TestDraftCreateDryRunJSONReportsResolvedPublicationURL(t *testing.T) {
	t.Setenv("SUBSTACK_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--subdomain", "trevinsays",
		"drafts", "create",
		"--title", "CLI verification dry-run",
		"--body", "Verification only.",
		"--dry-run",
		"--agent",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute dry-run: %v; stderr=%s", err, stderr.String())
	}
	var envelope struct {
		Path   string `json:"path"`
		DryRun bool   `json:"dry_run"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &envelope); err != nil {
		t.Fatalf("parse stdout JSON %q: %v", stdout.String(), err)
	}
	if !envelope.DryRun {
		t.Fatalf("dry_run = false, want true; stdout=%s", stdout.String())
	}
	if strings.Contains(envelope.Path, "{publication}") {
		t.Fatalf("path = %q, still contains unresolved publication placeholder", envelope.Path)
	}
	if got, want := envelope.Path, "https://trevinsays.substack.com/api/v1/drafts"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
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

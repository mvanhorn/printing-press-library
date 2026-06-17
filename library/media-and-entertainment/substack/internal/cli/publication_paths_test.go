// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var processStderrMu sync.Mutex

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

func TestDraftCreateDryRunJSONReportsGlobalWriterURL(t *testing.T) {
	t.Setenv("SUBSTACK_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--subdomain", "trevinsays",
		"--publication-id", "7019888",
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
	if strings.Contains(envelope.Path, "trevinsays.substack.com") || strings.Contains(envelope.Path, "{publication}") {
		t.Fatalf("path = %q, want global writer endpoint without publication host", envelope.Path)
	}
	if got, want := envelope.Path, "https://substack.com/api/v1/drafts?publication_id=7019888"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestImagesDryRunUsesGlobalUploadEndpoint(t *testing.T) {
	t.Setenv("SUBSTACK_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	restoreStderr := captureProcessStderr(t)

	root := RootCmd()
	var stdout, cmdStderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&cmdStderr)
	root.SetArgs([]string{
		"--subdomain", "trevinsays",
		"images",
		"--image", "data:image/png;base64,AAAA",
		"--dry-run",
		"--agent",
	})

	if err := root.Execute(); err != nil {
		got := restoreStderr()
		t.Fatalf("execute dry-run: %v; cmd stderr=%s; process stderr=%s", err, cmdStderr.String(), got)
	}
	got := restoreStderr()
	if !strings.Contains(got, "POST https://substack.com/api/v1/image") {
		t.Fatalf("stderr = %q, want global image upload endpoint", got)
	}
	if strings.Contains(got, "trevinsays.substack.com") || strings.Contains(got, "{publication}") {
		t.Fatalf("stderr = %q, should not use publication-host image endpoint", got)
	}
}

func TestDraftCreateDryRunWithoutPublicationIDDoesNotLookupLiveProfile(t *testing.T) {
	t.Setenv("SUBSTACK_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("SUBSTACK_BASE_URL", "http://127.0.0.1:1")

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
		t.Fatalf("execute dry-run without publication id should not perform live lookup: %v; stderr=%s", err, stderr.String())
	}
	var envelope struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &envelope); err != nil {
		t.Fatalf("parse stdout JSON %q: %v", stdout.String(), err)
	}
	if got, want := envelope.Path, "https://substack.com/api/v1/drafts?publication_id=dry-run-placeholder"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if strings.Contains(envelope.Path, "trevinsays.substack.com") {
		t.Fatalf("path = %q, should not use publication host", envelope.Path)
	}
}

func TestPublicationIDFromProfileMatchesPrimaryPublication(t *testing.T) {
	raw := []byte(`{"primaryPublication":{"id":7019888,"subdomain":"trevinsays","custom_domain":"trevinsays.com"}}`)
	got, err := publicationIDFromProfile(raw, "trevinsays")
	if err != nil {
		t.Fatalf("publicationIDFromProfile returned error: %v", err)
	}
	if got != "7019888" {
		t.Fatalf("publication id = %q, want 7019888", got)
	}
}

func TestPublicationIDFromProfilePrefersPublicationID(t *testing.T) {
	raw := []byte(`{"publicationUsers":[{"id":111,"publication_id":7019888,"subdomain":"trevinsays"}]}`)
	got, err := publicationIDFromProfile(raw, "trevinsays")
	if err != nil {
		t.Fatalf("publicationIDFromProfile returned error: %v", err)
	}
	if got != "7019888" {
		t.Fatalf("publication id = %q, want publication_id 7019888 instead of row id 111", got)
	}
}

func captureProcessStderr(t *testing.T) func() string {
	t.Helper()
	processStderrMu.Lock()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		processStderrMu.Unlock()
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stderr = w
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()
	return func() string {
		_ = w.Close()
		os.Stderr = old
		<-done
		_ = r.Close()
		processStderrMu.Unlock()
		return buf.String()
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

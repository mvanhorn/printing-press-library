// Copyright 2026 Anton Sidorov aka anticodeguy and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSentinelConfig writes a templated config with NO default_workspace, so
// config.Load falls the {slug} var to the "my-workspace" sentinel.
func writeSentinelConfig(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	body := "base_url = 'https://api.plane.so/api/v1/workspaces/{slug}'\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestClassifyAPIError_403NoWorkspace(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	flags := &rootFlags{configPath: writeSentinelConfig(t)}

	err := classifyAPIError(errors.New("GET /members/ returned HTTP 403: forbidden"), flags)
	if got := ExitCode(err); got != 2 {
		t.Fatalf("no-workspace 403 exit code = %d, want 2 (usage)", got)
	}
	if !strings.Contains(err.Error(), "no workspace selected") {
		t.Fatalf("hint should mention workspace selection, got: %s", err.Error())
	}
	if strings.Contains(err.Error(), "API key has the required permissions") {
		t.Fatalf("must not show the generic API-key hint when no workspace is selected")
	}
}

func TestClassifyAPIError_403WithWorkspaceKeepsAuthHint(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	// An explicit --workspace means the slug is real, so a 403 is a genuine
	// access/permission problem — keep the API-key hint (exit 4).
	flags := &rootFlags{workspace: "bbm", configPath: writeSentinelConfig(t)}

	err := classifyAPIError(errors.New("GET /members/ returned HTTP 403: forbidden"), flags)
	if got := ExitCode(err); got != 4 {
		t.Fatalf("workspace-set 403 exit code = %d, want 4 (auth)", got)
	}
	if strings.Contains(err.Error(), "no workspace selected") {
		t.Fatalf("must not show the no-workspace hint when --workspace is set")
	}
}

func TestClassifyAPIError_404NoWorkspace(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	flags := &rootFlags{configPath: writeSentinelConfig(t)}

	err := classifyAPIError(errors.New("GET /projects/ returned HTTP 404: not found"), flags)
	if got := ExitCode(err); got != 2 {
		t.Fatalf("no-workspace 404 exit code = %d, want 2 (usage)", got)
	}
	if !strings.Contains(err.Error(), "no workspace selected") {
		t.Fatalf("404 with no workspace should point at workspace selection, got: %s", err.Error())
	}
}

func TestNoWorkspaceSelected_NilFlags(t *testing.T) {
	if noWorkspaceSelected(nil) {
		t.Fatalf("nil flags must not be treated as no-workspace (avoid hijacking unrelated errors)")
	}
}

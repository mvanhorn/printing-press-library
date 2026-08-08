// Copyright 2026 Cole Grolmus and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakePreflightRunner struct {
	missingTool string
	failVault   bool
	prs         string
}

func (f fakePreflightRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if name == "op" && f.failVault {
		return nil, errors.New("vault unavailable")
	}
	if name == "gh" && len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
		return []byte(f.prs), nil
	}
	return []byte(`{}`), nil
}

func (f fakePreflightRunner) LookPath(name string) error {
	if name == f.missingTool {
		return errors.New("missing")
	}
	return nil
}

func preflightServer(t *testing.T, workspaces string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/me":
			_, _ = w.Write([]byte(`{"id":"user_1"}`))
		case r.URL.Path == "/v0/projects/project_1":
			_, _ = w.Write([]byte(`{"id":"project_1"}`))
		case r.URL.Path == "/v0/projects/project_1/workspaces":
			_, _ = w.Write([]byte(workspaces))
		default:
			http.NotFound(w, r)
		}
	}))
}

func basePreflightOptions() preflightOptions {
	return preflightOptions{
		Issue: "ENG-549", ProjectID: "project_1", Repository: "owner/repo",
		Branch: "main", Agent: "codex", Model: "gpt-5.4", Effort: "high",
		RequireTool: []string{"git"}, RequireVault: []string{"Coding Agents"},
	}
}

func TestPreflightPassesWithoutCreatingWorkspace(t *testing.T) {
	server := preflightServer(t, `{"data":[]}`)
	defer server.Close()
	receipt := runConductorPreflight(context.Background(), workflowTestClient(server), basePreflightOptions(), fakePreflightRunner{prs: `[]`})
	if receipt.Outcome != "PASS" {
		t.Fatalf("outcome = %q, gates = %+v", receipt.Outcome, receipt.Gates)
	}
}

func TestPreflightBlocksUnsupportedModelBeforeNetwork(t *testing.T) {
	opts := basePreflightOptions()
	opts.Model = "made-up-model"
	if err := validatePreflightOptions(opts); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreflightBlocksMissingVault(t *testing.T) {
	server := preflightServer(t, `{"data":[]}`)
	defer server.Close()
	receipt := runConductorPreflight(context.Background(), workflowTestClient(server), basePreflightOptions(), fakePreflightRunner{failVault: true, prs: `[]`})
	if receipt.Outcome != "BLOCKED" {
		t.Fatalf("outcome = %q", receipt.Outcome)
	}
	found := false
	for _, gate := range receipt.Gates {
		if gate.Gate == "vaults" && gate.Status == "BLOCKED" && gate.FailureClass == "auth" {
			found = true
		}
	}
	if !found {
		t.Fatalf("vault gate = %+v", receipt.Gates)
	}
}

func TestPreflightReturnsResumeForExistingWorkspace(t *testing.T) {
	server := preflightServer(t, `{"data":[{"name":"ENG-549-preflight"}]}`)
	defer server.Close()
	receipt := runConductorPreflight(context.Background(), workflowTestClient(server), basePreflightOptions(), fakePreflightRunner{prs: `[]`})
	if receipt.Outcome != "RESUME" {
		t.Fatalf("outcome = %q", receipt.Outcome)
	}
}

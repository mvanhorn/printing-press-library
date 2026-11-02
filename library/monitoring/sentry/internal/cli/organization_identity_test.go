// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestAgentOrganizationListKeepsSlug(t *testing.T) {
	recordRequests(t, `[{"id":"123","name":"Test organization","slug":"test-org"}]`)
	cmd := RootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"organizations", "list-your", "--agent", "--no-cache"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Results []struct{ Slug string }
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].Slug != "test-org" {
		t.Fatalf("agent output lost the organization slug: %s", output.String())
	}
}

func TestOrganizationIssueListPreservesNumericIDAndSlug(t *testing.T) {
	requests := recordRequests(t, `[{"id":"7","title":"positive control"}]`)
	for _, target := range []string{"1234567890123456", "test-org"} {
		runCLI(t, "organizations", "issues", "list-an-organization-s", target,
			"--query", "positive control", "--data-source", "live", "--no-cache", "--json")
	}
	got := requests()
	if len(got) != 2 || got[0].URL.Path != "/api/0/organizations/1234567890123456/issues/" ||
		got[1].URL.Path != "/api/0/organizations/test-org/issues/" {
		t.Fatalf("organization ID or slug changed on the request: %v", got)
	}
}

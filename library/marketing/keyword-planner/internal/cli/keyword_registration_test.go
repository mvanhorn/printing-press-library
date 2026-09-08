// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestKeywordPlannerRegistrationRemovesGenericBypassCommands(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"customers", "auth"} {
		if findKeywordRootCommand(root, name) != nil {
			t.Fatalf("generic %s command remains reachable", name)
		}
	}
	doctor := findKeywordRootCommand(root, "doctor")
	if doctor == nil || doctor.Annotations["pp:data-source"] != "local" {
		t.Fatalf("curated local doctor missing or incorrectly annotated: %#v", doctor)
	}
	portfolio := findKeywordRootCommand(root, "portfolio")
	if portfolio == nil {
		t.Fatal("portfolio group is not reachable")
	}
	for _, name := range []string{"list", "show", "search", "export"} {
		if findKeywordRootCommand(portfolio, name) == nil {
			t.Fatalf("portfolio %s command is not reachable", name)
		}
	}
}

func TestPlannerPortfolioMissingDatabaseIsOfflineEmptyResult(t *testing.T) {
	root := RootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{
		"portfolio", "list", "--db", "/path/that/does/not/exist/snapshots.db",
		"--agent", "--no-learn",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Meta    map[string]any `json:"meta"`
		Results []any          `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("missing portfolio must emit JSON: %v\n%s", err, out.String())
	}
	if got, _ := envelope.Meta["source"].(string); got != "local" {
		t.Fatalf("meta.source = %q, want local", got)
	}
	if envelope.Results == nil {
		t.Fatalf("missing portfolio result must be [] rather than null: %s", out.String())
	}
	if strings.Contains(strings.ToLower(out.String()), "access_token") {
		t.Fatalf("offline output mentions credentials: %s", out.String())
	}
}

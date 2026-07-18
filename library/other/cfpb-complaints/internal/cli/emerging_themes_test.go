// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelEmergingThemesHelpWires smoke-tests that the emerging themes command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelEmergingThemesHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"emerging", "themes", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("emerging themes --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "themes"} {
		if !strings.Contains(help, want) {
			t.Fatalf("emerging themes --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestDeltaBucketsKeepsUnknownOneSidedBucketsExplicit(t *testing.T) {
	current := cfpbResponse{}
	current.Aggregations = map[string]json.RawMessage{"product": json.RawMessage(`{"buckets":[{"key":"new","doc_count":3}]}`)}
	rows := deltaBuckets(current, cfpbResponse{}, "product")
	if len(rows) != 1 || rows[0]["count_delta"] != nil {
		t.Fatalf("rows=%#v", rows)
	}
}

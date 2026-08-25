// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPublicImportSurfaceExcludesGenericGraphQLJSONL(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"import", "elements", "--input", "records.jsonl", "--dry-run"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("generic GraphQL import remained public: %s", output.String())
	}
	if !strings.Contains(output.String(), "unknown command") && !strings.Contains(output.String(), "unknown flag") {
		t.Fatalf("unexpected generic import error: %s", output.String())
	}

	root := RootCmd()
	status, _, err := root.Find([]string{"import", "status"})
	if err != nil || status.CommandPath() != "cosmos-pp-cli import status" {
		t.Fatalf("safe import status command missing: command=%v err=%v", status, err)
	}
	importer, _, err := root.Find([]string{"import"})
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"input", "batch-size"} {
		if importer.Flags().Lookup(flag) != nil {
			t.Fatalf("unsafe generic import flag --%s remained public", flag)
		}
	}
}

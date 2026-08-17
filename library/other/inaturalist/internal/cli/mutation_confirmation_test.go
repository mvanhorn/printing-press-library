// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRequireMutationConfirmation(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		cmd := &cobra.Command{Annotations: map[string]string{"pp:method": method}}
		err := requireMutationConfirmation(cmd, &rootFlags{})
		if err == nil || !strings.Contains(err.Error(), "pass --yes") {
			t.Fatalf("%s without confirmation error = %v, want --yes guidance", method, err)
		}
		if err := requireMutationConfirmation(cmd, &rootFlags{yes: true}); err != nil {
			t.Fatalf("%s with --yes rejected: %v", method, err)
		}
		if err := requireMutationConfirmation(cmd, &rootFlags{dryRun: true}); err != nil {
			t.Fatalf("%s dry-run rejected: %v", method, err)
		}
	}

	read := &cobra.Command{Annotations: map[string]string{"pp:method": "GET"}}
	if err := requireMutationConfirmation(read, &rootFlags{}); err != nil {
		t.Fatalf("GET unexpectedly requires confirmation: %v", err)
	}
}

func TestGeneratedMutationStopsBeforeClientWithoutYes(t *testing.T) {
	t.Setenv("INATURALIST_API_TOKEN", "")
	cmd := RootCmd()
	cmd.SetArgs([]string{"observations", "delete", "123"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "confirmation required for DELETE request") {
		t.Fatalf("delete error = %v, want local confirmation error", err)
	}
}

func TestBulkImportStopsBeforeReadingInputWithoutYes(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"import", "observations", "--input", "does-not-exist.jsonl"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "confirmation required for POST request") {
		t.Fatalf("import error = %v, want local confirmation error", err)
	}
}

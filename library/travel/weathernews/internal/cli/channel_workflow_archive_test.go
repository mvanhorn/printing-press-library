// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestWorkflowArchiveFailsClosedWithoutResources(t *testing.T) {
	cases := [][]string{
		{"workflow", "archive"},
		{"--json", "workflow", "archive"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := RootCmd()
			var stdout, stderr strings.Builder
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected non-zero exit\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
			}
			if !strings.Contains(err.Error(), "no archiveable resources") {
				t.Fatalf("error = %v", err)
			}
			combined := stdout.String() + stderr.String()
			if strings.Contains(combined, "Archived 0 items") {
				t.Fatalf("claimed empty success\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
			}
		})
	}
}

// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/internal/cliutil"
	"github.com/spf13/cobra"
)

func TestRequireRemoteMutationConfirmation(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		readOnly    bool
		flags       rootFlags
		wantBlocked bool
	}{
		{name: "post blocked", method: "POST", wantBlocked: true},
		{name: "delete blocked case insensitive", method: "delete", wantBlocked: true},
		{name: "yes confirms", method: "PATCH", flags: rootFlags{yes: true}},
		{name: "dry run previews", method: "PUT", flags: rootFlags{dryRun: true}},
		{name: "read only post exempt", method: "POST", readOnly: true},
		{name: "get unaffected", method: "GET"},
		{name: "unannotated unaffected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{"pp:method": tt.method}
			if tt.readOnly {
				annotations["mcp:read-only"] = "true"
			}
			root := &cobra.Command{Use: "roblox-pp-cli"}
			cmd := &cobra.Command{Use: "example", Annotations: annotations}
			root.AddCommand(cmd)

			err := requireRemoteMutationConfirmation(cmd, &tt.flags)
			if !tt.wantBlocked {
				if err != nil {
					t.Fatalf("unexpected confirmation error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("mutation without confirmation was allowed")
			}
			var cliErr *cliError
			if !errors.As(err, &cliErr) || cliErr.code != 2 {
				t.Fatalf("confirmation error = %T %v, want usage error with exit code 2", err, err)
			}
			for _, want := range []string{"--yes", "--dry-run", strings.ToUpper(tt.method)} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("confirmation error %q missing %q", err, want)
				}
			}
		})
	}
}

func TestRootPreRunRejectsUnconfirmedRemoteMutation(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(func() {
		if _, err := cliutil.SetHomeOverride(""); err != nil {
			t.Errorf("clearing home override: %v", err)
		}
	})
	flags := &rootFlags{}
	root := newRootCmd(flags)
	root.SetArgs([]string{"avatar", "create", "--no-learn", "--home", home})
	root.SilenceErrors = true

	_, err := root.ExecuteC()
	if err == nil {
		t.Fatal("avatar mutation without --yes was allowed")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error %q does not explain how to confirm", err)
	}
}

func TestEveryAnnotatedRemoteMutationIsCoveredByConfirmationGuard(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	mutationCount := 0
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, cmd := range parent.Commands() {
			method := strings.ToUpper(cmd.Annotations["pp:method"])
			switch method {
			case "DELETE", "POST", "PUT", "PATCH":
				if strings.EqualFold(cmd.Annotations["mcp:read-only"], "true") {
					break
				}
				mutationCount++
				if err := requireRemoteMutationConfirmation(cmd, &rootFlags{}); err == nil {
					t.Errorf("%s (%s) is not covered by the confirmation guard", cmd.CommandPath(), method)
				}
			}
			walk(cmd)
		}
	}
	walk(root)
	if mutationCount == 0 {
		t.Fatal("command tree contains no annotated mutations; confirmation coverage test is not exercising the generated surface")
	}
}

// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func findCmd(t *testing.T, root *cobra.Command, path string) *cobra.Command {
	t.Helper()
	cmd, _, err := root.Find(strings.Fields(path))
	if err != nil || cmd == root {
		t.Fatalf("command %q not found: %v", path, err)
	}
	return cmd
}

func TestIsRemoteWriteCommandClassification(t *testing.T) {
	root := RootCmd()
	writes := []string{
		"sp create-campaigns",
		"sponsored-products-sp create-sponsored-products-campaigns",
		"sponsored-products-sp delete-sponsored-products-keywords",
		"sb submit-draft-campaign",
		"dsp create-orders",
		"dsp set-order-status",
		"manager-accounts create",
	}
	reads := []string{
		"sponsored-products-sp list-sponsored-products-campaigns",
		"sponsored-products-sp get-budget-recommendations",
		"sp report request",
		"hsa report create",
		"sp create-keyword-bid-recommendations",
		"sd create-sdforecast",
		"assets search",
		"history get",
		"reports get",
	}
	for _, p := range writes {
		if cmd := findCmd(t, root, p); !isRemoteWriteCommand(cmd) {
			t.Errorf("%s (%s %s) should require --yes", p, cmd.Annotations["pp:method"], cmd.Annotations["pp:path"])
		}
	}
	for _, p := range reads {
		if cmd := findCmd(t, root, p); isRemoteWriteCommand(cmd) {
			t.Errorf("%s (%s %s) is read-style and should not require --yes", p, cmd.Annotations["pp:method"], cmd.Annotations["pp:path"])
		}
	}
}

func TestRemoteWriteGateAndAgentMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	oldNoColor := noColor
	t.Cleanup(func() { noColor = oldNoColor })

	run := func(args ...string) (rootFlags, error) {
		var flags rootFlags
		var captured rootFlags
		root := newRootCmd(&flags)
		write := &cobra.Command{
			Use:         "write-probe",
			Annotations: map[string]string{"pp:endpoint": "probe.create", "pp:method": "POST", "pp:path": "/sp/campaigns"},
			RunE:        func(cmd *cobra.Command, args []string) error { captured = flags; return nil },
		}
		root.AddCommand(write)
		root.SetArgs(append([]string{"write-probe"}, args...))
		err := root.Execute()
		return captured, err
	}

	if _, err := run("--agent"); err == nil || !strings.Contains(err.Error(), "pass --yes") {
		t.Fatalf("--agent alone must not confirm a write, got err=%v", err)
	}
	if _, err := run(); err == nil {
		t.Fatalf("write without --yes must fail")
	}
	got, err := run("--agent", "--yes")
	if err != nil {
		t.Fatalf("--agent --yes should run: %v", err)
	}
	if !got.asJSON || !got.compact || !got.noInput || !got.yes {
		t.Fatalf("--agent should set output defaults and keep explicit --yes, got %+v", got)
	}
	if _, err := run("--dry-run"); err != nil {
		t.Fatalf("--dry-run should bypass the gate: %v", err)
	}
}

func TestAccountFlagSelectsPerAccountConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	run := func(args ...string) (rootFlags, error) {
		var flags rootFlags
		var captured rootFlags
		root := newRootCmd(&flags)
		root.AddCommand(&cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, args []string) error { captured = flags; return nil }})
		root.SetArgs(append([]string{"probe"}, args...))
		err := root.Execute()
		return captured, err
	}

	got, err := run("--account", "brand-b")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "amazon-ads-pp-cli", "brand-b", "config.toml")
	if got.configPath != want {
		t.Fatalf("configPath = %q, want %q", got.configPath, want)
	}
	for _, bad := range []string{"../x", "a/b", ".hidden"} {
		if _, err := run("--account", bad); err == nil {
			t.Errorf("--account %q should be rejected", bad)
		}
	}
	if _, err := run("--account", "a", "--config", "x.toml"); err == nil {
		t.Errorf("--account with --config should be rejected")
	}
}

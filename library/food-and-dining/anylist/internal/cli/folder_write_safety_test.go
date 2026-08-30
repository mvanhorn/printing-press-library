package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFolderCreateRequiresApply(t *testing.T) {
	t.Parallel()

	cmd := newFoldersCreateCmd(&rootFlags{})
	cmd.SetArgs([]string{"--name", "disposable-folder"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

func TestFolderDeleteRequiresApply(t *testing.T) {
	t.Parallel()

	cmd := newFoldersDeleteCmd(&rootFlags{})
	cmd.SetArgs([]string{"--name", "disposable-folder"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

func TestFolderWriteDryRunRemainsOffline(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cmd  func(*rootFlags) *cobra.Command
	}{
		{name: "create", cmd: newFoldersCreateCmd},
		{name: "delete", cmd: newFoldersDeleteCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{dryRun: true}
			cmd := tc.cmd(flags)
			if tc.name == "create" {
				cmd.SetArgs([]string{"--name", "disposable-folder"})
			} else {
				cmd.SetArgs([]string{"--name", "disposable-folder"})
			}
			if err := cmd.Execute(); err != nil {
				t.Fatalf("dry-run returned error: %v", err)
			}
		})
	}
}

func TestFolderCreateDeleteApplyRequiresAuthentication(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cmd  func(*rootFlags) *cobra.Command
	}{
		{name: "create", cmd: newFoldersCreateCmd},
		{name: "delete", cmd: newFoldersDeleteCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd(&rootFlags{configPath: t.TempDir() + "/config.toml"})
			cmd.SetArgs([]string{"--name", "disposable-folder", "--apply"})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not authenticated") {
				t.Fatalf("%s apply error = %v, want authentication preflight", tc.name, err)
			}
		})
	}
}

func TestFolderUpdateLiveMutationsFailClosed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cmd  func(*rootFlags) *cobra.Command
		args []string
	}{
		{name: "update", cmd: newFoldersUpdateCmd, args: []string{"--name", "disposable-folder", "--order", "missing-child", "--apply"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd(&rootFlags{})
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("%s unexpectedly attempted a live mutation", tc.name)
			}
		})
	}
}

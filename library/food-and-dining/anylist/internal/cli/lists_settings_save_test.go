package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestListsSettingsSaveIsRegistered(t *testing.T) {
	root := RootCmd()
	command, _, err := root.Find([]string{"lists", "settings", "save"})
	if err != nil || command == nil {
		t.Fatalf("Find(lists settings save) = %#v, %v", command, err)
	}
}

func TestListsSettingsSaveDefaultsToPreview(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newListsSettingsSaveCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--name", "Groceries"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("preview output is not JSON: %v\n%s", err, out.String())
	}
	if got["dry_run"] != true || got["handler_id"] != saveInitialListSettingsHandler {
		t.Fatalf("preview = %#v", got)
	}
}

func TestListsSettingsSaveApplyRequiresAuth(t *testing.T) {
	flags := &rootFlags{configPath: t.TempDir() + "/config.toml"}
	cmd := newListsSettingsSaveCmd(flags)
	cmd.SetArgs([]string{"--name", "Groceries", "--apply"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("error = %v, want authentication error", err)
	}
}

func TestListsSettingsSaveRejectsUnknownSortOrder(t *testing.T) {
	cmd := newListsSettingsSaveCmd(&rootFlags{})
	cmd.SetArgs([]string{"--name", "Groceries", "--sort-order", "unknown"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "sort order") {
		t.Fatalf("error = %v, want sort-order validation", err)
	}
}

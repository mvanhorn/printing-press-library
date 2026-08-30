package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestFindLiveListSettingsByListID(t *testing.T) {
	data := &pb.PBUserDataResponse{ListSettingsResponse: &pb.PBListSettingsList{Settings: []*pb.PBListSettings{
		{ListId: "first", Identifier: "settings-1"},
		{ListId: "target", Identifier: "settings-2"},
	}}}
	got, found := findLiveListSettingsByListID(data, " target ")
	if !found || got.GetIdentifier() != "settings-2" {
		t.Fatalf("findLiveListSettingsByListID = %#v, %v", got, found)
	}
	if _, found := findLiveListSettingsByListID(data, "missing"); found {
		t.Fatal("found settings for missing list")
	}
}

func TestListsSettingsClearDefaultsToPreview(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newListsSettingsClearCmd(flags)
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
	if got["dry_run"] != true || got["handler_id"] != removeListSettingsHandler {
		t.Fatalf("preview = %#v", got)
	}
}

func TestListsSettingsClearApplyRequiresAuth(t *testing.T) {
	flags := &rootFlags{configPath: t.TempDir() + "/config.toml"}
	cmd := newListsSettingsClearCmd(flags)
	cmd.SetArgs([]string{"--name", "Groceries", "--apply"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("error = %v, want authentication error", err)
	}
}

package cli

import (
	"encoding/json"
	"testing"
)

func TestRecentIsFreshByDefaultAndSupportsCachedOverride(t *testing.T) {
	cmd := cmdRecent()
	flag := cmd.Flags().Lookup("cached")
	if flag == nil {
		t.Fatal("recent must expose --cached")
	}
	if flag.DefValue != "false" {
		t.Fatalf("recent --cached default=%s want=false", flag.DefValue)
	}
}

func TestListSupportsExplicitFreshMode(t *testing.T) {
	cmd := cmdList()
	flag := cmd.Flags().Lookup("fresh")
	if flag == nil {
		t.Fatal("list must expose --fresh")
	}
	if flag.DefValue != "false" {
		t.Fatalf("list --fresh default=%s want=false", flag.DefValue)
	}
}

func TestRootIncludesSyncCommand(t *testing.T) {
	root := buildRootCommand()
	cmd, _, err := root.Find([]string{"sync"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd == root || cmd.Name() != "sync" {
		t.Fatalf("sync command missing; found %q", cmd.Name())
	}
}

func TestRootExposesConfigurableSyncWaits(t *testing.T) {
	root := buildRootCommand()
	for _, name := range []string{"daemon-wait", "app-wait", "poll-interval", "settle-wait"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("missing persistent --%s flag", name)
		}
	}
}

func TestMemoListJSONCarriesFreshnessMetadata(t *testing.T) {
	result := syncResult{SchemaVersion: 1, Refreshed: true, FreshnessConfirmed: false}
	out := newMemoListOutput([]memo{{ID: 7}}, "fresh", &result)
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["schema_version"] != float64(1) {
		t.Fatalf("schema_version=%v", decoded["schema_version"])
	}
	freshness := decoded["freshness"].(map[string]any)
	if freshness["mode"] != "fresh" || freshness["result"] == nil {
		t.Fatalf("freshness=%v", freshness)
	}
	if len(decoded["memos"].([]any)) != 1 {
		t.Fatalf("memos=%v", decoded["memos"])
	}
}

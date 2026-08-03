package config

import "testing"

func TestLoadCommunityHeaderFromEnvironment(t *testing.T) {
	t.Setenv("LAYERS_COMMUNITY_ID", "school-alias")
	t.Setenv("LAYERS_TOKEN", "")

	cfg, err := Load(t.TempDir() + "/missing.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Headers["community-id"]; got != "school-alias" {
		t.Fatalf("community-id header = %q, want school-alias", got)
	}
}

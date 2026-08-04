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

func TestAuthHeaderNormalizesLayersTokenEnvironment(t *testing.T) {
	t.Setenv("LAYERS_COMMUNITY_ID", "")

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "raw token", value: "header.payload.signature", want: "Bearer header.payload.signature"},
		{name: "bearer token", value: "Bearer header.payload.signature", want: "Bearer header.payload.signature"},
		{name: "bearer token extra spaces", value: "  bearer   header.payload.signature  ", want: "Bearer header.payload.signature"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LAYERS_TOKEN", tc.value)
			cfg, err := Load(t.TempDir() + "/missing.toml")
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.AuthHeader(); got != tc.want {
				t.Fatalf("AuthHeader() = %q, want %q", got, tc.want)
			}
		})
	}
}

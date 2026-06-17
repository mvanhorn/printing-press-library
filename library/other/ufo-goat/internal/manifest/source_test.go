package manifest

import "testing"

func TestResolveSource(t *testing.T) {
	t.Setenv("UFO_MANIFEST_URL", "")
	t.Setenv("UFO_SOURCE", "")

	tests := []struct {
		name       string
		flagURL    string
		flagSource string
		envURL     string
		envSource  string
		cfgURL     string
		wantURL    string
		wantName   string
		wantErr    bool
	}{
		{
			name:     "default when nothing set",
			wantURL:  DefaultManifestURL,
			wantName: DefaultSourceName,
		},
		{
			name:     "explicit flag url wins over everything",
			flagURL:  "https://example.com/a.csv",
			envURL:   "https://example.com/b.csv",
			cfgURL:   "https://example.com/c.csv",
			wantURL:  "https://example.com/a.csv",
			wantName: "custom",
		},
		{
			name:       "named source flag resolves to registry url",
			flagSource: "legacy",
			wantURL:    Sources["legacy"].URL,
			wantName:   "legacy",
		},
		{
			name:      "env manifest url beats env source and cfg",
			envURL:    "https://example.com/env.csv",
			envSource: "legacy",
			cfgURL:    "https://example.com/cfg.csv",
			wantURL:   "https://example.com/env.csv",
			wantName:  "custom",
		},
		{
			name:      "env source used when no urls",
			envSource: "legacy",
			wantURL:   Sources["legacy"].URL,
			wantName:  "legacy",
		},
		{
			name:     "cfg url used as fallback before default",
			cfgURL:   "https://example.com/cfg.csv",
			wantURL:  "https://example.com/cfg.csv",
			wantName: "config",
		},
		{
			name:       "unknown named source errors",
			flagSource: "nope",
			wantErr:    true,
		},
		{
			name:       "placeholder source without url errors",
			flagSource: "wargov",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("UFO_MANIFEST_URL", tt.envURL)
			t.Setenv("UFO_SOURCE", tt.envSource)

			url, name, err := ResolveSource(tt.flagURL, tt.flagSource, tt.cfgURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got url=%q name=%q", url, name)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if url != tt.wantURL {
				t.Errorf("url = %q, want %q", url, tt.wantURL)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestSortedSourcesDefaultFirst(t *testing.T) {
	sorted := SortedSources()
	if len(sorted) == 0 {
		t.Fatal("no sources registered")
	}
	if sorted[0].Name != DefaultSourceName {
		t.Errorf("first source = %q, want default %q", sorted[0].Name, DefaultSourceName)
	}
}

func TestResolveSourceUnavailableSourceChecksAvailabilityFlag(t *testing.T) {
	const name = "staging"
	old, hadOld := Sources[name]
	Sources[name] = Source{
		Name:        name,
		URL:         "https://example.com/staging.csv",
		Description: "provisioned but not released",
		Available:   false,
	}
	t.Cleanup(func() {
		if hadOld {
			Sources[name] = old
			return
		}
		delete(Sources, name)
	})

	url, resolved, err := ResolveSource("", name, "")
	if err == nil {
		t.Fatalf("expected unavailable-source error, got url=%q source=%q", url, resolved)
	}
}

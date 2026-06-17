package manifest

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// DefaultManifestURL points at the community mirror that tracks EVERY PURSUE
// release tranche (release_1, release_02, release_03, ...). The original
// default (DenisSergeevitch/UFO-USA) froze at Release 1 on 2026-05-08 and never
// tracked later tranches; this source stays current and uses the same CSV
// schema.
const DefaultManifestURL = "https://raw.githubusercontent.com/abigailhaddad/ufo-releases/main/data/uap-csv.csv"

// DefaultSourceName is the named source used when nothing else is specified.
const DefaultSourceName = "community"

// Source describes a named origin for the UAP manifest.
//
// Today every source is a CSV fetched over HTTPS. The struct is deliberately
// broader than a bare URL so a future sanctioned, direct-from-war.gov feed can
// be added as a new named Source — potentially authenticated — without changing
// any command surface. Selecting it would be `--source wargov`; nothing else in
// the CLI needs to know how the bytes are obtained.
type Source struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Description  string `json:"description"`
	RequiresAuth bool   `json:"requires_auth"` // groundwork: a future war.gov feed may need credentials
	Official     bool   `json:"official"`      // true once the source is a sanctioned/direct origin
	Available    bool   `json:"available"`     // false for placeholder sources not yet wired
}

// Sources is the registry of known manifest origins. Add an entry here to make
// a new origin selectable via `--source <name>`; no other code changes needed.
var Sources = map[string]Source{
	"community": {
		Name:        "community",
		URL:         DefaultManifestURL,
		Description: "Community mirror tracking every PURSUE release tranche (abigailhaddad/ufo-releases). Default.",
		Available:   true,
	},
	"legacy": {
		Name:        "legacy",
		URL:         "https://raw.githubusercontent.com/DenisSergeevitch/UFO-USA/main/metadata/uap-csv.csv",
		Description: "Original Release 1 mirror (DenisSergeevitch/UFO-USA). Frozen 2026-05-08; Release 1 only.",
		Available:   true,
	},
	// Placeholder for the future: direct, sanctioned access from war.gov.
	// war.gov currently blocks programmatic access (Akamai). When a
	// collaboration or official feed exists, fill in URL (and wire any auth)
	// and flip Available — it then becomes selectable via `--source wargov`
	// with zero other changes.
	"wargov": {
		Name:         "wargov",
		URL:          "",
		Description:  "Direct from war.gov PURSUE (placeholder — not yet available; reserved for future direct access)",
		RequiresAuth: true,
		Official:     true,
		Available:    false,
	},
}

// SortedSources returns the registry as a stable, ordered slice for display.
func SortedSources() []Source {
	out := make([]Source, 0, len(Sources))
	for _, s := range Sources {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		// Default first, then available before placeholders, then by name.
		if (out[i].Name == DefaultSourceName) != (out[j].Name == DefaultSourceName) {
			return out[i].Name == DefaultSourceName
		}
		if out[i].Available != out[j].Available {
			return out[i].Available
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ResolveSource determines which manifest URL to sync from and the name of the
// resolved source, in precedence order (highest first):
//
//  1. flagURL          — an explicit --manifest-url value
//  2. flagSource       — a --source <name> value
//  3. UFO_MANIFEST_URL — environment override (explicit URL)
//  4. UFO_SOURCE       — environment override (named source)
//  5. cfgURL           — a persisted default (may be empty; seam for config-file support)
//  6. the built-in default source (community)
//
// A named source that has no URL yet (e.g. the wargov placeholder) returns a
// descriptive error so the user learns it is reserved for future use.
func ResolveSource(flagURL, flagSource, cfgURL string) (url string, name string, err error) {
	if flagURL != "" {
		return flagURL, "custom", nil
	}
	if flagSource != "" {
		return sourceByName(flagSource)
	}
	if v := strings.TrimSpace(os.Getenv("UFO_MANIFEST_URL")); v != "" {
		return v, "custom", nil
	}
	if v := strings.TrimSpace(os.Getenv("UFO_SOURCE")); v != "" {
		return sourceByName(v)
	}
	if cfgURL != "" {
		return cfgURL, "config", nil
	}
	return DefaultManifestURL, DefaultSourceName, nil
}

func sourceByName(name string) (string, string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	s, ok := Sources[key]
	if !ok {
		return "", "", fmt.Errorf("unknown source %q (available: %s)", name, strings.Join(sourceNames(), ", "))
	}
	if !s.Available {
		return "", "", fmt.Errorf("source %q is not available yet: %s", s.Name, s.Description)
	}
	return s.URL, s.Name, nil
}

func sourceNames() []string {
	var names []string
	for _, s := range SortedSources() {
		names = append(names, s.Name)
	}
	return names
}

// Package channels resolves user-friendly channel-group names to the
// canonical AppsFlyer media_source IDs (the `_int` suffix names) the V2 API
// expects. AppsFlyer does not classify media sources into channel groups
// natively, so the mapping ships as a default and is overridable by the user
// via ~/.config/appsflyer-pp-cli/channels.yaml.
package channels

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Group is the registry of channel groups → canonical media_source IDs.
type Group map[string][]string

// Default returns the built-in mapping. The values are the canonical `_int`
// media_source IDs the API accepts as `media_source` query/body parameters.
// Coverage is intentionally a starter set the user can extend, not exhaustive.
func Default() Group {
	return Group{
		"social": {
			"facebook_int",
			"tiktokglobal_int",
			"snapchat_int",
			"reddit_int",
			"pinterest_int",
			"twitter_int",
		},
		"programmatic": {
			"googleadwords_int",
			"iossearchads_int",
			"applovin_int",
			"vungle_int",
			"unityads_int",
			"ironsource_int",
			"liftoff_int",
			"chartboosts2s_int",
			"mintegral_int",
			"moloco_int",
			"criteonew_int",
			"appier_int",
			"jampp_int",
			"smadex_int",
			"remerge_int",
			"bidease_int",
		},
		"oem": {
			"digitalturbine_int",
			"onedigitalturbine_int",
			"xiaomiglobal_int",
			"oppostore_int",
			"samsungdsp_int",
		},
		"rewarded": {
			"tapjoy_int",
			"mistplay_int",
			"appgrowth_int",
			"adgatemedia_int",
			"swagbucks_int",
			"adjoe_int",
			"buzzad_int",
			"scrambly_int",
		},
		"video": {
			"vungle_int",
			"applovin_int",
			"unityads_int",
			"ironsource_int",
			"chartboosts2s_int",
		},
	}
}

// configPath resolves the user's channels.yaml location, honoring
// APPSFLYER_CONFIG_DIR for tests.
func configPath() (string, error) {
	if v := os.Getenv("APPSFLYER_CONFIG_DIR"); v != "" {
		return filepath.Join(v, "channels.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "appsflyer-pp-cli", "channels.yaml"), nil
}

// Load returns the user's channels.yaml merged over the default mapping. A
// missing or unreadable file falls back to the default with no error. Invalid
// YAML returns an error so the user knows their override was rejected.
func Load() (Group, error) {
	merged := Default()
	path, err := configPath()
	if err != nil {
		return merged, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return merged, nil
	}
	var user Group
	if err := yaml.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	for name, sources := range user {
		merged[name] = sources
	}
	return merged, nil
}

// Resolve returns the canonical media_source IDs for the named group, or an
// error listing available group names when the lookup fails.
func (g Group) Resolve(name string) ([]string, error) {
	if v, ok := g[strings.ToLower(strings.TrimSpace(name))]; ok {
		return append([]string(nil), v...), nil
	}
	names := make([]string, 0, len(g))
	for k := range g {
		names = append(names, k)
	}
	sort.Strings(names)
	return nil, fmt.Errorf("unknown channel group %q (available: %s)", name, strings.Join(names, ", "))
}

// Names returns the configured channel-group names in lexical order.
func (g Group) Names() []string {
	names := make([]string, 0, len(g))
	for k := range g {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

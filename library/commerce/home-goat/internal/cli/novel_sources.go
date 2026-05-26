// Novel command: source registry and `sources` top-level command.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// sourceConfig describes one upstream data source for the fan-out search.
type sourceConfig struct {
	Name        string   // canonical key (e.g. "west-elm")
	DisplayName string   // human-facing (e.g. "West Elm")
	BaseURL     string   // API endpoint root
	Transport   string   // "constructor_io", "graphql", "apq_graphql", "shopify_storefront"
	Categories  []string // which category_routing buckets this source serves
	Status      string   // "active" or "stub"
}

// sourceRegistry is the authoritative list of upstream sources. Only "active"
// sources are queried by the fan-out; stubs are shown in `sources` output
// for visibility but skipped during search.
var sourceRegistry = []sourceConfig{
	{
		Name:        "ferguson",
		DisplayName: "Ferguson",
		BaseURL:     "https://www.fergusonhome.com",
		Transport:   "graphql",
		Categories:  []string{"foundational", "appliances"},
		Status:      "active",
	},
	{
		Name:        "west-elm",
		DisplayName: "West Elm",
		BaseURL:     "https://ac.cnstrc.com",
		Transport:   "constructor_io",
		Categories:  []string{"furniture", "decor"},
		Status:      "active",
	},
	{
		Name:        "rejuvenation",
		DisplayName: "Rejuvenation",
		BaseURL:     "https://ac.cnstrc.com",
		Transport:   "constructor_io",
		Categories:  []string{"foundational", "decor"},
		Status:      "active",
	},
	{
		Name:        "article",
		DisplayName: "Article",
		BaseURL:     "https://www.article.com",
		Transport:   "apq_graphql",
		Categories:  []string{"furniture", "decor"},
		Status:      "active",
	},
	{
		Name:        "shopify-dtc",
		DisplayName: "Shopify DTC",
		BaseURL:     "https://{store}.myshopify.com",
		Transport:   "shopify_storefront",
		Categories:  []string{"furniture", "decor"},
		Status:      "active",
	},
	{
		Name:        "wayfair",
		DisplayName: "Wayfair",
		BaseURL:     "https://www.wayfair.com",
		Transport:   "graphql_clearance",
		Categories:  []string{"foundational", "appliances", "furniture"},
		Status:      "stub",
	},
	{
		Name:        "allmodern",
		DisplayName: "AllModern",
		BaseURL:     "https://www.allmodern.com",
		Transport:   "graphql_clearance",
		Categories:  []string{"appliances", "furniture"},
		Status:      "stub",
	},
	{
		Name:        "rh",
		DisplayName: "Restoration Hardware",
		BaseURL:     "https://rh.com",
		Transport:   "unknown",
		Categories:  []string{"foundational", "furniture"},
		Status:      "stub",
	},
	{
		Name:        "ikea",
		DisplayName: "IKEA",
		BaseURL:     "https://www.ikea.com",
		Transport:   "unknown",
		Categories:  []string{"furniture", "decor", "foundational"},
		Status:      "stub",
	},
}

// categoryToSources maps a furnishing category to the source names that
// serve it. Mirrors spec.yaml category_routing.
var categoryToSources = map[string][]string{
	"foundational": {"ferguson", "rejuvenation"},
	"appliances":   {"ferguson"},
	"furniture":    {"west-elm", "article", "shopify-dtc"},
	"decor":        {"west-elm", "rejuvenation", "shopify-dtc"},
}

// roomToCategories maps a room type to the categories typically needed
// for that room. Mirrors spec.yaml room_templates.
var roomToCategories = map[string][]string{
	"bathroom": {"foundational", "furniture", "decor"},
	"kitchen":  {"foundational", "appliances", "decor"},
	"bedroom":  {"furniture", "decor"},
	"living":   {"furniture", "decor"},
	"dining":   {"furniture", "decor"},
	"outdoor":  {"furniture", "decor"},
}

// activeSources returns sourceConfigs filtered to status == "active".
func activeSources() []sourceConfig {
	out := make([]sourceConfig, 0, len(sourceRegistry))
	for _, s := range sourceRegistry {
		if s.Status == "active" {
			out = append(out, s)
		}
	}
	return out
}

// sourceByName returns the sourceConfig for a given name, or nil.
func sourceByName(name string) *sourceConfig {
	for i := range sourceRegistry {
		if sourceRegistry[i].Name == name {
			return &sourceRegistry[i]
		}
	}
	return nil
}

// resolveSourcesForCategories returns the deduplicated set of active source
// names that serve any of the given categories.
func resolveSourcesForCategories(categories []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, cat := range categories {
		for _, src := range categoryToSources[cat] {
			if !seen[src] {
				seen[src] = true
				// Only include active sources.
				if s := sourceByName(src); s != nil && s.Status == "active" {
					out = append(out, src)
				}
			}
		}
	}
	return out
}

// resolveSources determines which sources to query based on the user's
// --category, --room, and --source flags. Returns the deduplicated list of
// active source names and the resolved category list (for the envelope).
//
// Precedence: --source overrides everything; --room expands to categories;
// --category is used directly. When none are set, all active Tier 1 sources
// are returned.
func resolveSources(categoryFlag, roomFlag, sourceFlag string) (sources []string, categories []string, room string, err error) {
	// --source overrides category/room routing entirely.
	if sourceFlag != "" {
		for _, name := range splitCSV(sourceFlag) {
			s := sourceByName(name)
			if s == nil {
				return nil, nil, "", fmt.Errorf("unknown source %q; known sources: %s", name, knownSourceNames())
			}
			if s.Status != "active" {
				return nil, nil, "", fmt.Errorf("source %q is not active (status: %s)", name, s.Status)
			}
			sources = append(sources, name)
		}
		return sources, nil, "", nil
	}

	// --room expands to categories, then categories resolve to sources.
	if roomFlag != "" {
		cats, ok := roomToCategories[roomFlag]
		if !ok {
			return nil, nil, "", fmt.Errorf("unknown room %q; valid rooms: %s", roomFlag, knownRoomNames())
		}
		categories = cats
		room = roomFlag
		sources = resolveSourcesForCategories(categories)
		return sources, categories, room, nil
	}

	// --category: user specifies categories directly.
	if categoryFlag != "" {
		for _, cat := range splitCSV(categoryFlag) {
			if _, ok := categoryToSources[cat]; !ok {
				return nil, nil, "", fmt.Errorf("unknown category %q; valid categories: %s", cat, knownCategoryNames())
			}
			categories = append(categories, cat)
		}
		sources = resolveSourcesForCategories(categories)
		return sources, categories, "", nil
	}

	// Default: all active Tier 1 sources.
	for _, s := range activeSources() {
		sources = append(sources, s.Name)
	}
	return sources, nil, "", nil
}

// splitCSV splits a comma-separated string and trims whitespace.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func knownSourceNames() string {
	names := make([]string, len(sourceRegistry))
	for i, s := range sourceRegistry {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

func knownRoomNames() string {
	names := make([]string, 0, len(roomToCategories))
	for k := range roomToCategories {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func knownCategoryNames() string {
	names := make([]string, 0, len(categoryToSources))
	for k := range categoryToSources {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// newSourcesCmd creates the top-level `sources` command that lists all upstream
// data sources, their status, categories, and transport type.
func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "List all upstream data sources, their status, categories, and transport type.",
		Example: `  home-goat-pp-cli sources
  home-goat-pp-cli sources --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			type sourceRow struct {
				Name        string   `json:"name"`
				DisplayName string   `json:"display_name"`
				BaseURL     string   `json:"base_url"`
				Transport   string   `json:"transport"`
				Categories  []string `json:"categories"`
				Status      string   `json:"status"`
			}

			rows := make([]sourceRow, len(sourceRegistry))
			for i, s := range sourceRegistry {
				rows[i] = sourceRow{
					Name:        s.Name,
					DisplayName: s.DisplayName,
					BaseURL:     s.BaseURL,
					Transport:   s.Transport,
					Categories:  s.Categories,
					Status:      s.Status,
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"NAME", "DISPLAY", "STATUS", "TRANSPORT", "CATEGORIES"}
			tableRows := make([][]string, len(sourceRegistry))
			for i, s := range sourceRegistry {
				status := green(s.Status)
				if s.Status == "stub" {
					status = yellow(s.Status)
				}
				tableRows[i] = []string{
					s.Name,
					s.DisplayName,
					status,
					s.Transport,
					strings.Join(s.Categories, ", "),
				}
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	return cmd
}

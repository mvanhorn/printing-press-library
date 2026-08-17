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
		Name:        "ikea",
		DisplayName: "IKEA",
		BaseURL:     "https://sik.search.blue.cdtapps.com",
		Transport:   "sik_search",
		Categories:  []string{"foundational", "appliances", "furniture", "decor"},
		Status:      "active",
	},
	{
		Name:        "ge-appliances",
		DisplayName: "GE Appliances",
		BaseURL:     "https://q7rntw.a.searchspring.io",
		Transport:   "searchspring",
		Categories:  []string{"appliances"},
		Status:      "active",
	},
	{
		Name:        "bray-and-scarff",
		DisplayName: "Bray & Scarff",
		BaseURL:     "https://hasura.nmg-platform.com/v1/graphql",
		Transport:   "nmg_hasura_graphql",
		Categories:  []string{"appliances"},
		Status:      "active",
	},
	{
		Name:        "pc-richard",
		DisplayName: "PC Richard",
		BaseURL:     "https://www.pcrichard.com",
		Transport:   "demandware_embedded_json",
		Categories:  []string{"appliances"},
		Status:      "active",
	},
	{
		Name:        "appliance-factory",
		DisplayName: "Appliance Factory",
		BaseURL:     "https://www.appliancefactory.com/api/rest",
		Transport:   "avb_rest_http1",
		Categories:  []string{"appliances"},
		Status:      "active",
	},
	{
		Name:        "best-buy",
		DisplayName: "Best Buy",
		BaseURL:     "https://www.bestbuy.com",
		Transport:   "next_ssr_product_cards",
		Categories:  []string{"appliances"},
		Status:      "active",
	},
	{
		Name:        "abt",
		DisplayName: "Abt",
		BaseURL:     "https://www.abt.com",
		Transport:   "http1_search_and_product_schema",
		Categories:  []string{"appliances"},
		Status:      "active",
	},
	{
		Name:        "homewise-appliance",
		DisplayName: "Homewise Appliance",
		BaseURL:     "https://0qcofybyvd.execute-api.us-west-1.amazonaws.com/hw-prod",
		Transport:   "bloomreach_api",
		Categories:  []string{"appliances"},
		Status:      "active",
	},
	{
		Name:        "floor-and-decor",
		DisplayName: "Floor & Decor",
		BaseURL:     "https://AR91I5G1KF-dsn.algolia.net",
		Transport:   "algolia",
		Categories:  []string{"foundational"},
		Status:      "active",
	},
	{
		Name:        "superbrightleds",
		DisplayName: "Super Bright LEDs",
		BaseURL:     "https://VTAW7SB4LM-dsn.algolia.net",
		Transport:   "algolia",
		Categories:  []string{"foundational", "electrical"},
		Status:      "active",
	},
	{
		Name:        "prolighting",
		DisplayName: "PROLIGHTING",
		BaseURL:     "https://uscs34v2.ksearchnet.com",
		Transport:   "klevu",
		Categories:  []string{"foundational", "electrical"},
		Status:      "active",
	},
	{
		Name:        "1000bulbs",
		DisplayName: "1000Bulbs",
		BaseURL:     "https://www.1000bulbs.com",
		Transport:   "html_product_cards",
		Categories:  []string{"foundational", "electrical"},
		Status:      "active",
	},
	{
		Name:        "bees-lighting",
		DisplayName: "Bees Lighting",
		BaseURL:     "https://www.beeslighting.com",
		Transport:   "shopify_suggest",
		Categories:  []string{"electrical"},
		Status:      "active",
	},
	{
		Name:        "lighting-new-york",
		DisplayName: "Lighting New York",
		BaseURL:     "https://lightingnewyork.com",
		Transport:   "demandware_embedded_json",
		Categories:  []string{"electrical", "decor"},
		Status:      "active",
	},
	{
		Name:        "lightology",
		DisplayName: "Lightology",
		BaseURL:     "https://www.lightology.com",
		Transport:   "html_gtm_product_cards",
		Categories:  []string{"electrical", "decor"},
		Status:      "active",
	},
	{
		Name:        "plumbersstock",
		DisplayName: "PlumbersStock",
		BaseURL:     "https://www.plumbersstock.com",
		Transport:   "next_ssr_product_cards",
		Categories:  []string{"plumbing"},
		Status:      "active",
	},
	{
		Name:        "faucetdepot",
		DisplayName: "FaucetDepot",
		BaseURL:     "https://FSDN8N73JY-dsn.algolia.net",
		Transport:   "algolia",
		Categories:  []string{"foundational", "plumbing"},
		Status:      "active",
	},
	{
		Name:        "faucetlist",
		DisplayName: "FaucetList",
		BaseURL:     "https://faucetlist.com",
		Transport:   "shopify_suggest",
		Categories:  []string{"plumbing"},
		Status:      "active",
	},
	{
		Name:        "plumbtile",
		DisplayName: "PlumbTile",
		BaseURL:     "https://plumbtile.com",
		Transport:   "shopify_suggest",
		Categories:  []string{"plumbing"},
		Status:      "active",
	},
	{
		Name:        "modern-bathroom",
		DisplayName: "Modern Bathroom",
		BaseURL:     "https://www.modernbathroom.com",
		Transport:   "shopify_suggest",
		Categories:  []string{"plumbing"},
		Status:      "active",
	},
	{
		Name:        "kbauthority",
		DisplayName: "KBAuthority",
		BaseURL:     "https://api.searchspring.net",
		Transport:   "searchspring_autocomplete_html",
		Categories:  []string{"plumbing", "decor"},
		Status:      "active",
	},
	{
		Name:        "vintage-tub",
		DisplayName: "Vintage Tub",
		BaseURL:     "https://api.searchspring.net",
		Transport:   "searchspring",
		Categories:  []string{"plumbing", "decor"},
		Status:      "active",
	},
	{
		Name:        "signature-hardware",
		DisplayName: "Signature Hardware",
		BaseURL:     "https://www.signaturehardware.com",
		Transport:   "demandware_suggestions_html",
		Categories:  []string{"plumbing", "decor"},
		Status:      "active",
	},
	{
		Name:        "qualitybath",
		DisplayName: "QualityBath",
		BaseURL:     "https://www.qualitybath.com",
		Transport:   "react_query_hydrated_search",
		Categories:  []string{"plumbing", "decor"},
		Status:      "active",
	},
	{
		Name:        "pioneer-mini-split",
		DisplayName: "Pioneer Mini Split",
		BaseURL:     "https://www.pioneerminisplit.com",
		Transport:   "shopify_suggest",
		Categories:  []string{"hvac"},
		Status:      "active",
	},
	{
		Name:        "sylvane",
		DisplayName: "Sylvane",
		BaseURL:     "https://www.sylvane.com",
		Transport:   "shopify_suggest",
		Categories:  []string{"hvac"},
		Status:      "active",
	},
	{
		Name:        "iwae",
		DisplayName: "IWAe",
		BaseURL:     "https://iwae.com",
		Transport:   "hyva_product_cards",
		Categories:  []string{"hvac"},
		Status:      "active",
	},
	{
		Name:        "hardware-hut",
		DisplayName: "The Hardware Hut",
		BaseURL:     "https://hardwarehut.com",
		Transport:   "html_embedded_json",
		Categories:  []string{"foundational", "hardware", "materials"},
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
	// Lowe's: assessed via Printing Press printer protocol (probe-reachability +
	// browser-sniff on Firefox HAR, 2026-05-27). Autocomplete and store locator
	// are standard_http (User-Agent only), but product search is SSR behind Akamai.
	// The recommendation engine (pythia-recs-svc, 14 endpoints) requires browser
	// session cookies. Stubbed for product search; suggest and stores commands are
	// wired as standalone utilities (novel_lowes.go).
	{
		Name:        "lowes",
		DisplayName: "Lowe's",
		BaseURL:     "https://www.lowes.com",
		Transport:   "standard_http_partial",
		Categories:  []string{"foundational", "appliances"},
		Status:      "stub",
	},
	// Home Depot: assessed via Printing Press printer protocol (probe-reachability +
	// browser-sniff on Firefox HAR, 2026-05-27). Every endpoint — autocomplete
	// (/complete/search/), store finder (/StoreFinderServices/v2/), GraphQL search
	// (/federation-gateway/graphql) — returns 403 from both stdlib and surf-chrome.
	// Firefox HAR captured zero requests to www.homedepot.com; only Sprinklr chat
	// widget traffic. Home Depot renders all product/store data server-side (SSR);
	// there is no XHR/fetch API surface. No viable path to CLI activation without
	// an HTML scraping adapter.
	{
		Name:        "home-depot",
		DisplayName: "Home Depot",
		BaseURL:     "https://www.homedepot.com",
		Transport:   "ssr_only",
		Categories:  []string{"foundational", "appliances"},
		Status:      "stub",
	},
}

// categoryToSources maps a furnishing category to the source names that
// serve it. Mirrors spec.yaml category_routing.
var categoryToSources = map[string][]string{
	"foundational": {"ferguson", "rejuvenation", "ikea", "floor-and-decor", "superbrightleds", "prolighting", "1000bulbs", "faucetdepot", "hardware-hut"},
	"plumbing":     {"ferguson", "floor-and-decor", "plumbersstock", "faucetdepot", "faucetlist", "plumbtile", "modern-bathroom", "kbauthority", "vintage-tub", "signature-hardware", "qualitybath"},
	"electrical":   {"superbrightleds", "prolighting", "1000bulbs", "bees-lighting", "lighting-new-york", "lightology"},
	"hvac":         {"pioneer-mini-split", "sylvane", "iwae", "hardware-hut"},
	"flooring":     {"floor-and-decor"},
	"hardware":     {"hardware-hut", "rejuvenation", "ikea"},
	"materials":    {"floor-and-decor", "hardware-hut", "ikea"},
	"appliances":   {"ferguson", "ikea", "ge-appliances", "bray-and-scarff", "pc-richard", "appliance-factory", "best-buy", "abt", "homewise-appliance"},
	"furniture":    {"west-elm", "article", "shopify-dtc", "ikea"},
	"decor":        {"west-elm", "article", "rejuvenation", "shopify-dtc", "ikea", "lighting-new-york", "lightology", "kbauthority", "vintage-tub", "signature-hardware", "qualitybath"},
}

// roomToCategories maps a room type to the categories typically needed
// for that room. Mirrors spec.yaml room_templates.
var roomToCategories = map[string][]string{
	"bathroom": {"plumbing", "electrical", "flooring", "hardware", "materials", "decor"},
	"kitchen":  {"plumbing", "electrical", "flooring", "hardware", "materials", "appliances", "decor"},
	"bedroom":  {"furniture", "decor"},
	"living":   {"furniture", "decor"},
	"dining":   {"furniture", "decor"},
	"outdoor":  {"electrical", "hardware", "materials", "furniture", "decor"},
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
// --category is used directly. When none are set, all active sources
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

	// Default: all active sources.
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
		Example: `  reno-goat-pp-cli sources
  reno-goat-pp-cli sources --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
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

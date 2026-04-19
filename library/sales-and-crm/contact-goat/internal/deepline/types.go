// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Package deepline provides a hybrid Deepline API client that prefers the
// official `deepline` subprocess when present and falls back to direct HTTP.
// Deepline is a credit-priced contact-data API (https://code.deepline.com/);
// every execute call costs credits, so the client exposes a cost estimator
// and a local execution log for budget awareness.
package deepline

// BaseURL is the Deepline v2 API root.
const BaseURL = "https://code.deepline.com/api/v2"

// KeyPrefix is the required prefix for Deepline API keys. Keys starting with
// "dpl_" are Vercel tokens and are rejected at client construction.
const KeyPrefix = "dlp_"

// Known tool IDs referenced by the typed subcommands. The API is tool-based:
// every call is POST /integrations/{toolId}/execute with a tool-specific
// JSON payload.
// Tool IDs verified against `deepline tools list` on 2026-04-19. These are
// the ai_ark_* family served by Deepline's managed waterfall backend.
// Multiple CLI subcommands may map to the same underlying tool with
// different payload shapes (e.g. company search and enrich both use
// ai_ark_company_search).
const (
	ToolPersonSearchToEmailWaterfall = "ai_ark_find_emails"
	ToolApolloPeopleSearch           = "ai_ark_people_search"
	ToolPhoneFind                    = "ai_ark_mobile_phone_finder"
	ToolCompanySearch                = "ai_ark_company_search"
	ToolPersonEnrich                 = "ai_ark_personality_analysis"
	ToolReverseLookup                = "ai_ark_reverse_lookup"
)

// Aliases: several CLI subcommands map to the same backend tool.
const (
	ToolEmailFind     = ToolPersonSearchToEmailWaterfall // find-email + email-find share one backend
	ToolCompanyEnrich = ToolCompanySearch                // enrich-company uses company search with filters
)

// ToolInfo describes a known Deepline tool: its id, a short human-readable
// label, the rough shape of its payload, and the default credit cost per call.
//
// Costs here are conservative defaults; actual billing is determined
// server-side. `EstimateCost` uses these as a hint before spending credits.
type ToolInfo struct {
	ID             string
	Label          string
	PayloadHint    string
	DefaultCredits int
}

// Catalog is the static catalog of tools the CLI knows how to call. Unknown
// tool IDs are accepted via the generic `execute` command but default to a
// 1-credit estimate.
var Catalog = map[string]ToolInfo{
	ToolPersonSearchToEmailWaterfall: {
		ID:             ToolPersonSearchToEmailWaterfall,
		Label:          "Email finder (async via People Search trackId)",
		PayloadHint:    `{"name":"Patrick Collison","company":"stripe.com"}`,
		DefaultCredits: 4,
	},
	ToolApolloPeopleSearch: {
		ID:             ToolApolloPeopleSearch,
		Label:          "People search (by filters)",
		PayloadHint:    `{"title":"VP Engineering","location":"San Francisco","industry":"software","limit":25}`,
		DefaultCredits: 2,
	},
	ToolPhoneFind: {
		ID:             ToolPhoneFind,
		Label:          "Mobile phone finder (LinkedIn URL or name+company)",
		PayloadHint:    `{"linkedin_url":"https://www.linkedin.com/in/patrickcollison"}`,
		DefaultCredits: 3,
	},
	ToolCompanySearch: {
		ID:             ToolCompanySearch,
		Label:          "Company search / enrich",
		PayloadHint:    `{"industry":"fintech","size":"201-500","location":"United States"}`,
		DefaultCredits: 2,
	},
	ToolPersonEnrich: {
		ID:             ToolPersonEnrich,
		Label:          "Personality analysis (LinkedIn URL)",
		PayloadHint:    `{"linkedin_url":"https://www.linkedin.com/in/patrickcollison"}`,
		DefaultCredits: 2,
	},
	ToolReverseLookup: {
		ID:             ToolReverseLookup,
		Label:          "Reverse lookup (email or phone -> profile)",
		PayloadHint:    `{"email":"patrick@stripe.com"}`,
		DefaultCredits: 2,
	},
}

// LookupTool returns the ToolInfo for id, or a zero value and false if the id
// is not in the catalog.
func LookupTool(id string) (ToolInfo, bool) {
	t, ok := Catalog[id]
	return t, ok
}

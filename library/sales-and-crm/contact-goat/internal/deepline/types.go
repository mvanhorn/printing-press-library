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
//
// Tool IDs verified against `deepline tools list` on 2026-04-20. The previous
// revision of this file pointed ToolPersonEnrich at ai_ark_personality_analysis
// and ToolEmailFind at ai_ark_find_emails; neither is a direct email
// enrichment tool. Personality analysis returns personality traits (not
// contact info) and requires a `url` payload field; ai_ark_find_emails is an
// async follow-on to ai_ark_people_search that requires a trackId. Those
// constants are preserved here under clearer names for callers that genuinely
// want those behaviors, while ToolPersonEnrich and ToolEmailFind now point at
// real email-enrichment providers.
const (
	// --- ai_ark family: preserved for explicit callers ---
	// ToolPersonSearchToEmailWaterfall is the async follow-on to
	// ToolApolloPeopleSearch. It requires a trackId from a prior
	// ai_ark_people_search call and cannot be invoked directly with
	// name+company. Do NOT route the `find-email` subcommand here.
	ToolPersonSearchToEmailWaterfall = "ai_ark_find_emails"
	ToolApolloPeopleSearch           = "ai_ark_people_search"
	ToolPhoneFind                    = "ai_ark_mobile_phone_finder"
	ToolCompanySearch                = "ai_ark_company_search"
	ToolReverseLookup                = "ai_ark_reverse_lookup"
	// ToolPersonalityAnalysis analyzes a LinkedIn profile for personality
	// traits and outbound-messaging guidance. It returns personality data,
	// NOT contact info. Use ToolApolloPeopleMatch or ToolHunterPeopleFind
	// for person enrichment instead.
	ToolPersonalityAnalysis = "ai_ark_personality_analysis"

	// --- Email finders (name+domain -> work email) ---
	ToolDropleadsEmailFinder = "dropleads_email_finder"
	ToolHunterEmailFinder    = "hunter_email_finder"
	ToolDatagmaFindEmail     = "datagma_find_email"
	ToolIcypeasEmailSearch   = "icypeas_email_search"
	ToolHunterDomainSearch   = "hunter_domain_search"

	// --- Person enrichers (linkedin_url | email -> full record) ---
	ToolApolloPeopleMatch      = "apollo_people_match"
	ToolHunterPeopleFind       = "hunter_people_find"
	ToolContactOutEnrichPerson = "contactout_enrich_person"
)

// Primary aliases used by the CLI subcommands. The VALUE of each constant has
// changed in the 2026-04-20 fix; the NAME is preserved so external callers
// (MCP server, code that imports deepline.ToolEmailFind, etc.) keep compiling.
const (
	// ToolPersonEnrich is the default person-enrichment tool: given a
	// LinkedIn URL or an email, return name/title/company and any
	// contact fields the provider has on file. Routed to apollo_people_match
	// (broadest coverage, personal_emails[] + work email + employment
	// history in one call).
	ToolPersonEnrich = ToolApolloPeopleMatch

	// ToolEmailFind is the default "find a work email from name+domain"
	// tool. Routed to dropleads_email_finder (cheapest hit rate; returns
	// a single high-confidence email with MX status).
	ToolEmailFind = ToolDropleadsEmailFinder

	// ToolCompanyEnrich reuses the ai_ark_company_search backend; it
	// accepts a domain and returns firmographics.
	ToolCompanyEnrich = ToolCompanySearch
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
		Label:          "Email finder (async; requires ai_ark_people_search trackId)",
		PayloadHint:    `{"trackId":"<from ai_ark_people_search>"}`,
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
	ToolPersonalityAnalysis: {
		ID:             ToolPersonalityAnalysis,
		Label:          "Personality analysis (LinkedIn URL -> outbound guidance; NOT contact info)",
		PayloadHint:    `{"url":"https://www.linkedin.com/in/patrickcollison"}`,
		DefaultCredits: 2,
	},
	ToolReverseLookup: {
		ID:             ToolReverseLookup,
		Label:          "Reverse lookup (email or phone -> profile)",
		PayloadHint:    `{"email":"patrick@stripe.com"}`,
		DefaultCredits: 2,
	},

	ToolDropleadsEmailFinder: {
		ID:             ToolDropleadsEmailFinder,
		Label:          "Dropleads email finder (name+domain -> verified work email)",
		PayloadHint:    `{"first_name":"Mike","last_name":"Craig","company_domain":"stripe.com"}`,
		DefaultCredits: 1,
	},
	ToolHunterEmailFinder: {
		ID:             ToolHunterEmailFinder,
		Label:          "Hunter email finder (name+domain -> work email with confidence score)",
		PayloadHint:    `{"first_name":"Mike","last_name":"Craig","domain":"stripe.com"}`,
		DefaultCredits: 1,
	},
	ToolDatagmaFindEmail: {
		ID:             ToolDatagmaFindEmail,
		Label:          "Datagma email finder (name+domain -> verified work email)",
		PayloadHint:    `{"first_name":"Mike","last_name":"Craig","company_domain":"stripe.com"}`,
		DefaultCredits: 1,
	},
	ToolIcypeasEmailSearch: {
		ID:             ToolIcypeasEmailSearch,
		Label:          "Icypeas email search (async; poll read_results)",
		PayloadHint:    `{"firstname":"Mike","lastname":"Craig","domainOrCompany":"stripe.com"}`,
		DefaultCredits: 1,
	},
	ToolHunterDomainSearch: {
		ID:             ToolHunterDomainSearch,
		Label:          "Hunter domain search (domain -> all public emails)",
		PayloadHint:    `{"domain":"stripe.com"}`,
		DefaultCredits: 2,
	},
	ToolApolloPeopleMatch: {
		ID:             ToolApolloPeopleMatch,
		Label:          "Apollo people match (linkedin_url | email | name+domain -> full record)",
		PayloadHint:    `{"linkedin_url":"https://www.linkedin.com/in/patrickcollison","reveal_personal_emails":true}`,
		DefaultCredits: 1,
	},
	ToolHunterPeopleFind: {
		ID:             ToolHunterPeopleFind,
		Label:          "Hunter people find (email | linkedin_url -> role, socials, company)",
		PayloadHint:    `{"linkedin_url":"https://www.linkedin.com/in/patrickcollison"}`,
		DefaultCredits: 1,
	},
	ToolContactOutEnrichPerson: {
		ID:             ToolContactOutEnrichPerson,
		Label:          "ContactOut enrich person (linkedin_url | email | name+company -> email + phone)",
		PayloadHint:    `{"linkedin_url":"https://www.linkedin.com/in/patrickcollison"}`,
		DefaultCredits: 1,
	},
}

// LookupTool returns the ToolInfo for id, or a zero value and false if the id
// is not in the catalog.
func LookupTool(id string) (ToolInfo, bool) {
	t, ok := Catalog[id]
	return t, ok
}

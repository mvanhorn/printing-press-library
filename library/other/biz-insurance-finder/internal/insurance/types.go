// Package insurance contains the data model and pure, data-driven logic for the
// biz-insurance-finder CLI: the editable provider registry, the applicant profile,
// the importer-aware matching engine, the per-provider answer-sheet generator,
// the manual-actions checklist, and the underwriting warnings.
//
// Everything in this package is deterministic and side-effect free except the
// profile/registry file IO in profile.go and registry.go. The CLI layer
// (internal/cli) is the only place that reads stdin or touches the network.
package insurance

// Provider is one entry in the editable registry (providers.json). It carries
// both descriptive fields (shown to the user) and machine-readable matching
// metadata (Appetite, InstantQuote) so match.go can rank without hard-coded
// provider names.
type Provider struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	QuoteURL     string   `json:"quote_url,omitempty"` // quote-start URL if different from URL
	Type         string   `json:"type"`                // see ProviderType* below
	QuoteChannel string   `json:"quote_channel"`       // see QuoteChannel* below
	AMBest       string   `json:"am_best,omitempty"`
	Notes        string   `json:"notes,omitempty"`
	Appetite     Appetite `json:"appetite"`
	InstantQuote bool     `json:"instant_quote"`
	// Unverified marks an entry whose details have NOT been confirmed against
	// the carrier (e.g. Supersure). The CLI surfaces this prominently and the
	// matcher applies a penalty so unconfirmed providers never lead a shortlist.
	Unverified bool `json:"unverified,omitempty"`
	// SubmitNote flags WHERE the real submission/lead-capture happens, so the
	// user is not surprised by a multi-step wizard that captures the lead at the
	// contact step. Surfaced in the manual-actions checklist.
	SubmitNote string `json:"submit_note,omitempty"`
	// ManualNote is an optional provider-specific manual-action reminder.
	ManualNote string `json:"manual_note,omitempty"`
	// FieldHints are provider-specific answer-sheet additions/overrides (e.g.
	// "select the Product Liability program", "Class: Retail store").
	FieldHints []FieldHint `json:"field_hints,omitempty"`
}

// Provider type taxonomy (see the user spec).
const (
	ProviderTypeDirectCarrier    = "direct_carrier"
	ProviderTypeDigitalAgency    = "digital_agency"
	ProviderTypeBrokerMarket     = "broker_marketplace"
	ProviderTypeSurplusSpecialty = "surplus_lines_specialty"
	ProviderTypeWholesalerAgent  = "wholesaler_agent_only"
)

// Quote channel taxonomy.
const (
	QuoteChannelInstantOnline     = "instant_online"     // self-serve instant quote/bind
	QuoteChannelOnlineApplication = "online_application" // online app, quote returned later
	QuoteChannelQuoteRequestForm  = "quote_request_form" // form, a human follows up
	QuoteChannelPhoneAgent        = "phone_agent"        // phone / agent only, no consumer form
	QuoteChannelLeadMatch         = "lead_match"         // marketplace matches you to a carrier by phone
)

// Appetite is the carrier's willingness to write a given business class. Each
// field is a Fit rating string ("best" | "good" | "partial" | "poor" |
// "decline" | "" for unknown). The matcher converts these to scores.
type Appetite struct {
	Importer     string `json:"importer"`
	PrivateLabel string `json:"private_label"`
	Manufacturer string `json:"manufacturer"`
	Retail       string `json:"retail"`
	Service      string `json:"service"`
}

// Fit ratings.
const (
	FitBest    = "best"
	FitGood    = "good"
	FitPartial = "partial"
	FitPoor    = "poor"
	FitDecline = "decline"
)

// Registry is the loaded provider set plus its source provenance.
type Registry struct {
	Providers []Provider `json:"providers"`
	// Source records where the registry was loaded from ("embedded", or a file
	// path) for the doctor command and provenance.
	Source string `json:"source,omitempty"`
}

// Profile is the reusable applicant answer key. It is persisted to profile.json
// (gitignored — may contain PII) and reused across every provider.
type Profile struct {
	// Business identity
	LegalName       string `json:"legal_name"`
	DBA             string `json:"dba,omitempty"`
	EntityStructure string `json:"entity_structure"` // LLC, S-Corp, Sole Proprietor, ...
	FormationState  string `json:"formation_state"`
	BusinessAddress string `json:"business_address"`
	MailingAddress  string `json:"mailing_address,omitempty"`

	// Contact (named insured / applicant)
	ContactName  string `json:"contact_name"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`

	// Operations / class
	YearStarted     int    `json:"year_started"` // operating history, NOT rebrand date
	RevenueBand     string `json:"revenue_band"`
	EmployeesW2     int    `json:"employees_w2"`
	Contractors1099 int    `json:"contractors_1099"`
	IndustryClass   string `json:"industry_class"`

	// Importer / private-label / manufacturer status (the routing driver)
	Importer                 bool     `json:"importer"`
	PrivateLabel             bool     `json:"private_label"`
	Manufacturer             bool     `json:"manufacturer"`
	Products                 string   `json:"products,omitempty"`
	CountriesOfOrigin        []string `json:"countries_of_origin,omitempty"`
	SellsOnAmazon            bool     `json:"sells_on_amazon"`
	RetailerLimitRequirement string   `json:"retailer_limit_requirement,omitempty"`

	// Loss history / current coverage
	PriorClaims5yr    bool   `json:"prior_claims_5yr"`
	PriorClaimsDetail string `json:"prior_claims_detail,omitempty"`
	CurrentCoverage   string `json:"current_coverage,omitempty"`

	// Coverage requested
	GLPerOccurrence            string `json:"gl_per_occurrence"` // e.g. "$1,000,000"
	GLAggregate                string `json:"gl_aggregate"`      // e.g. "$2,000,000"
	ProductsCompletedOps       bool   `json:"products_completed_ops"`
	TradeShowAdditionalInsured bool   `json:"trade_show_additional_insured"`
	WantCyber                  bool   `json:"want_cyber"`
	WantBOP                    bool   `json:"want_bop"`
	EffectiveDate              string `json:"effective_date,omitempty"` // default: next business day
	BudgetAnnualUSD            int    `json:"budget_annual_usd,omitempty"`

	// Metadata
	SavedAt string `json:"saved_at,omitempty"`
}

// IsImporterClass reports whether the applicant is an importer, private-label
// brand, or manufacturer — i.e. a "deemed manufacturer" for US product
// liability. This single predicate drives the specialty-market routing.
func (p Profile) IsImporterClass() bool {
	return p.Importer || p.PrivateLabel || p.Manufacturer
}

// Recommendation is one ranked entry in the match output.
type Recommendation struct {
	Provider Provider `json:"provider"`
	Rank     int      `json:"rank"`  // 1-based, best first
	Score    int      `json:"score"` // higher is a better fit
	Tier     string   `json:"tier"`  // recommended | consider | avoid
	Reasons  []string `json:"reasons"`
	Cautions []string `json:"cautions,omitempty"`
}

// Recommendation tiers.
const (
	TierRecommended = "recommended"
	TierConsider    = "consider"
	TierAvoid       = "avoid"
)

// FieldHint is a provider-specific answer-sheet line.
type FieldHint struct {
	Field string `json:"field"`
	Value string `json:"value"`
	Note  string `json:"note,omitempty"`
}

// AnswerField is one paste-ready field/value pair on a provider answer sheet.
type AnswerField struct {
	Field string `json:"field"`
	Value string `json:"value"`
	Note  string `json:"note,omitempty"`
}

// AnswerSheet is the full set of paste-ready values for one provider.
type AnswerSheet struct {
	ProviderID   string        `json:"provider_id"`
	ProviderName string        `json:"provider_name"`
	QuoteURL     string        `json:"quote_url"`
	Fields       []AnswerField `json:"fields"`
}

// ChecklistItem is one manual action the tool must NOT perform for the user.
type ChecklistItem struct {
	Action   string `json:"action"`
	Detail   string `json:"detail"`
	Required bool   `json:"required"` // true = always applies; false = only if prompted by the form
}

// ProviderChecklist is the manual-actions checklist for one provider.
type ProviderChecklist struct {
	ProviderID   string          `json:"provider_id"`
	ProviderName string          `json:"provider_name"`
	QuoteURL     string          `json:"quote_url"`
	Items        []ChecklistItem `json:"items"`
}

// Warning is an underwriting landmine surfaced to the applicant.
type Warning struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	// Severity: "critical" | "important" | "info"
	Severity string `json:"severity"`
}

// Warning severities.
const (
	SeverityCritical  = "critical"
	SeverityImportant = "important"
	SeverityInfo      = "info"
)

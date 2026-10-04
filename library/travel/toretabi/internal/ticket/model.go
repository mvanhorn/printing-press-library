// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ticket

type Evidence struct {
	Scope     *string `json:"scope"`
	TextJA    string  `json:"text_ja"`
	Truncated bool    `json:"truncated"`
}
type Interval struct {
	Start      string `json:"start"`
	End        string `json:"end"`
	Derivation string `json:"derivation"`
}
type MonthDaySpan struct {
	StartMonth int      `json:"start_month"`
	StartDay   int      `json:"start_day"`
	EndMonth   int      `json:"end_month"`
	EndDay     int      `json:"end_day"`
	Year       *int     `json:"year"`
	Evidence   Evidence `json:"evidence"`
}
type Period struct {
	Evidence      Evidence   `json:"evidence"`
	Start         *string    `json:"start"`
	End           *string    `json:"end"`
	Intervals     []Interval `json:"intervals"`
	ExplicitDates []string   `json:"explicit_dates"`
	YearRound     bool       `json:"year_round"`
	Conditional   bool       `json:"conditional"`
}
type Money struct {
	CategoryJA    *string  `json:"category_ja"`
	CurrencyBasis *string  `json:"currency_basis"`
	Amount        string   `json:"amount"`
	Currency      string   `json:"currency"`
	Unit          string   `json:"unit"`
	Evidence      Evidence `json:"evidence"`
}
type Price struct {
	Status  string  `json:"status"`
	Amounts []Money `json:"amounts"`
	Reason  string  `json:"reason"`
}
type Summary struct {
	ID         string   `json:"id"`
	NameJA     string   `json:"name_ja"`
	SourceURL  string   `json:"source_url"`
	ObservedAt string   `json:"observed_at"`
	AreasJA    []string `json:"areas_ja"`
	TypeJA     *string  `json:"type_ja"`
	SeasonJA   *string  `json:"season_ja"`
	TagsJA     []string `json:"tags_ja"`
}
type Ticket struct {
	Operator *OperatorEvidence `json:"operator"`
	Stale    bool              `json:"stale"`
	Summary
	Description          Evidence       `json:"description"`
	Sales                Period         `json:"sales"`
	Use                  Period         `json:"use"`
	Validity             Evidence       `json:"validity"`
	ValidityDays         *int           `json:"validity_days"`
	Price                Price          `json:"price"`
	Benefits             []Money        `json:"benefits"`
	Eligibility          []Evidence     `json:"eligibility"`
	PurchaseChannels     []Evidence     `json:"purchase_channels"`
	Supplements          []Evidence     `json:"supplements"`
	Exceptions           []Evidence     `json:"exceptions"`
	BlackoutSpans        []MonthDaySpan `json:"blackout_spans"`
	Conditions           []Evidence     `json:"conditions"`
	OperatorURLs         []string       `json:"operator_urls"`
	EvidenceTruncated    bool           `json:"evidence_truncated"`
	InventoryStatus      string         `json:"inventory_status"`
	OperatorVerification string         `json:"operator_verification"`
	Unsupported          []string       `json:"unsupported"`
	CacheAgeSeconds      int64          `json:"cache_age_seconds"`
	Transport            string         `json:"transport"`
	SourceFailure        *string        `json:"source_failure"`
	CacheWarning         *string        `json:"cache_warning"`
}
type CatalogEntry struct {
	Code    string `json:"code"`
	LabelJA string `json:"label_ja"`
}
type Coverage struct {
	Routes         []string `json:"routes"`
	ScannedPages   int      `json:"scanned_pages"`
	ScannedTickets int      `json:"scanned_tickets"`
	MatchedTickets int      `json:"matched_tickets"`
	MaxPages       int      `json:"max_pages"`
	Returned       int      `json:"returned"`
	Continuation   *string  `json:"continuation"`
	Complete       bool     `json:"complete"`
	Note           string   `json:"note"`
}
type Listing struct {
	Tickets    []Summary      `json:"tickets"`
	Areas      []CatalogEntry `json:"areas"`
	Types      []CatalogEntry `json:"types"`
	Coverage   Coverage       `json:"coverage"`
	ObservedAt string         `json:"observed_at"`
}
type DateCheck struct {
	Date     *string  `json:"date"`
	State    string   `json:"state"`
	Reason   string   `json:"reason"`
	Evidence Evidence `json:"evidence"`
}
type Comparison struct {
	OperatorSalesCheck   DateCheck `json:"operator_sales_check"`
	OperatorUseCheck     DateCheck `json:"operator_use_check"`
	OperatorEditionState string    `json:"operator_edition_state"`
	Ticket               Ticket    `json:"ticket"`
	SalesCheck           DateCheck `json:"sales_check"`
	UseCheck             DateCheck `json:"use_check"`
	EditionState         string    `json:"edition_state"`
	ConfirmedEligibility string    `json:"confirmed_eligibility"`
}

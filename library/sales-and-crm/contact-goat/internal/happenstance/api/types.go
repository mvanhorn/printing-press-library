// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Package api provides a bearer-auth Go client for the Happenstance public
// REST API at https://api.happenstance.ai/v1. It is a peer to (not a
// replacement for) the cookie-sniff client in internal/client/. The two
// surfaces have different base URLs, different auth, different response
// shapes, and different rate-limit models (paid credits vs free monthly
// search allocation), so they live in separate packages and are routed
// between by internal/cli/source_selection.go.
//
// Auth: every request carries Authorization: Bearer <key>. The key is read
// from a private field on Client; it is never logged, written to a cache
// file, or echoed in --dry-run output. Dry-run renders it as the literal
// string "<HAPPENSTANCE_API_KEY>".
//
// Costs (from the public API docs, verified 2026-04-19): /v1/search costs
// 2 credits, /v1/research costs 1 credit on completion, /v1/search/{id}/find-more
// costs 2 credits. All other documented endpoints are free.
package api

import "fmt"

// DefaultBaseURL is the canonical Happenstance public-API root. Tests and
// callers can override via WithBaseURL.
const DefaultBaseURL = "https://api.happenstance.ai/v1"

// KeyEnvVar names the environment variable callers are expected to set with
// their bearer token. Surfaced verbatim in 401 error messages so the user has
// an actionable hint.
const KeyEnvVar = "HAPPENSTANCE_API_KEY"

// RotationURL is the upstream page where users provision and rotate keys.
// Surfaced in 401 error messages.
const RotationURL = "https://happenstance.ai/settings/api-keys"

// UsagePath is the public-API endpoint that returns the live credit balance.
// Surfaced in 402 error messages so callers know where to check the balance.
const UsagePath = "/v1/usage"

// RedactedBearerLine is the literal string emitted in dry-run output in
// place of the real bearer token. Tests grep for this exact string to
// confirm redaction works.
const RedactedBearerLine = "Bearer <HAPPENSTANCE_API_KEY>"

// RateLimitError is returned from Do (and therefore from every Client method)
// when the upstream API returns HTTP 429. Callers can type-assert with
// errors.As to implement custom backoff. RetryAfterSeconds is parsed from
// the Retry-After response header when present; zero means the server did
// not provide guidance and the caller should pick its own backoff.
type RateLimitError struct {
	RetryAfterSeconds int
	Body              string
}

func (e *RateLimitError) Error() string {
	if e.RetryAfterSeconds > 0 {
		return fmt.Sprintf("happenstance api: 429 rate limited (retry after %ds)", e.RetryAfterSeconds)
	}
	return "happenstance api: 429 rate limited"
}

// SearchEnvelope is the response from POST /v1/search. The asynchronous
// search is identified by Id; callers poll GET /v1/search/{id} for the
// full result list. URL is the human-facing happenstance.ai page.
type SearchEnvelope struct {
	Id      string         `json:"id"`
	URL     string         `json:"url,omitempty"`
	Status  string         `json:"status,omitempty"`
	Text    string         `json:"text,omitempty"`
	Results []SearchResult `json:"results,omitempty"`
	HasMore bool           `json:"has_more,omitempty"`
	NextPage string        `json:"next_page,omitempty"`
}

// SearchResult is one row of GET /v1/search/{id}.results. Field names match
// the OpenAPI shape verbatim. The normalizer in normalize.go (unit 3) maps
// these into the canonical client.Person consumed by every renderer.
type SearchResult struct {
	Name                 string  `json:"name"`
	CurrentTitle         string  `json:"current_title,omitempty"`
	CurrentCompany       string  `json:"current_company,omitempty"`
	WeightedTraitsScore  float64 `json:"weighted_traits_score,omitempty"`
}

// FindMoreEnvelope is the response from POST /v1/search/{id}/find-more.
// PageId is consumed by GET /v1/search/{id}?page_id=...
type FindMoreEnvelope struct {
	PageId         string `json:"page_id"`
	ParentSearchId string `json:"parent_search_id,omitempty"`
}

// ResearchEnvelope is the response from POST /v1/research and from
// GET /v1/research/{id}. Profile is populated only once Status is COMPLETED.
type ResearchEnvelope struct {
	Id      string           `json:"id"`
	URL     string           `json:"url,omitempty"`
	Status  string           `json:"status,omitempty"`
	Profile *ResearchProfile `json:"profile,omitempty"`
}

// ResearchProfile is the deep-research payload. The cookie surface does not
// return this shape; the normalizer collapses Employment[0] into
// CurrentTitle/CurrentCompany when projecting into client.Person.
type ResearchProfile struct {
	Employment []EmploymentEntry `json:"employment,omitempty"`
	Education  []EducationEntry  `json:"education,omitempty"`
	Projects   []ProjectEntry    `json:"projects,omitempty"`
	Writings   []WritingEntry    `json:"writings,omitempty"`
	Hobbies    []string          `json:"hobbies,omitempty"`
	Summary    string            `json:"summary,omitempty"`
}

// EmploymentEntry is one row of ResearchProfile.Employment.
type EmploymentEntry struct {
	Title       string `json:"title,omitempty"`
	Company     string `json:"company,omitempty"`
	StartDate   string `json:"start_date,omitempty"`
	EndDate     string `json:"end_date,omitempty"`
	Description string `json:"description,omitempty"`
}

// EducationEntry is one row of ResearchProfile.Education.
type EducationEntry struct {
	School    string `json:"school,omitempty"`
	Degree    string `json:"degree,omitempty"`
	Field     string `json:"field,omitempty"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// ProjectEntry is one row of ResearchProfile.Projects.
type ProjectEntry struct {
	Name        string `json:"name,omitempty"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

// WritingEntry is one row of ResearchProfile.Writings.
type WritingEntry struct {
	Title       string `json:"title,omitempty"`
	URL         string `json:"url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

// Group is one row of GET /v1/groups and the full payload of
// GET /v1/groups/{id}. Members is populated only on the singular endpoint.
type Group struct {
	Id          string        `json:"id"`
	Name        string        `json:"name"`
	MemberCount int           `json:"member_count,omitempty"`
	Members     []GroupMember `json:"members,omitempty"`
}

// GroupMember is one row of Group.Members. The public API only returns the
// member's display name; further hydration requires a separate Research call.
type GroupMember struct {
	Name string `json:"name"`
}

// User is the response from GET /v1/users/me. Friends is initialized as a
// non-nil empty slice when the API returns []; callers can range over it
// without nil-checking.
type User struct {
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	Friends []Friend `json:"friends"`
}

// Friend is one row of User.Friends. The public API exposes name + email
// only; richer data requires a Research call.
type Friend struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// Usage is the response from GET /v1/usage. BalanceCredits is the
// authoritative live balance; HasCredits is the upstream's pre-computed
// boolean view of "balance > 0" and is mirrored here for parity.
type Usage struct {
	BalanceCredits int             `json:"balance_credits"`
	HasCredits     bool            `json:"has_credits"`
	Purchases      []UsagePurchase `json:"purchases,omitempty"`
	Usage          []UsageEvent    `json:"usage,omitempty"`
	AutoReload     *AutoReload     `json:"auto_reload,omitempty"`
}

// UsagePurchase is one row of Usage.Purchases. Field names are kept generic
// because the OpenAPI spec is currently ambiguous on shape; this struct will
// be tightened once a real Usage payload is captured.
type UsagePurchase struct {
	Id        string `json:"id,omitempty"`
	Credits   int    `json:"credits,omitempty"`
	AmountUSD string `json:"amount_usd,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// UsageEvent is one row of Usage.Usage (a credit-spending event).
type UsageEvent struct {
	Id        string `json:"id,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Credits   int    `json:"credits,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// AutoReload mirrors the auto_reload object from GET /v1/usage. Nil when the
// user has not configured auto-reload.
type AutoReload struct {
	Enabled       bool `json:"enabled"`
	ThresholdCred int  `json:"threshold_credits,omitempty"`
	TopUpCredits  int  `json:"top_up_credits,omitempty"`
}

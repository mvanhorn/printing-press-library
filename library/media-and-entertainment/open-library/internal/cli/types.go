// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

type RequestInfo struct {
	Endpoint string         `json:"endpoint"`
	Query    map[string]any `json:"query,omitempty"`
}

type BookSearchResult struct {
	Source     string          `json:"source"`
	Configured bool            `json:"configured"`
	Query      map[string]any  `json:"query"`
	Request    RequestInfo     `json:"request"`
	Total      int             `json:"total"`
	Results    []BookCandidate `json:"results"`
	Freshness  string          `json:"freshness"`
	Caveats    []string        `json:"caveats"`
}

type BookCandidate struct {
	Key              string   `json:"key,omitempty"`
	Title            string   `json:"title,omitempty"`
	Authors          []string `json:"authors,omitempty"`
	AuthorKeys       []string `json:"author_keys,omitempty"`
	FirstPublishYear int      `json:"first_publish_year,omitempty"`
	EditionCount     int      `json:"edition_count,omitempty"`
	ISBNs            []string `json:"isbns,omitempty"`
	Languages        []string `json:"languages,omitempty"`
	SourceURL        string   `json:"source_url,omitempty"`
}

type EditionResult struct {
	Source     string         `json:"source"`
	Configured bool           `json:"configured"`
	Query      map[string]any `json:"query"`
	Request    RequestInfo    `json:"request"`
	Edition    EditionSummary `json:"edition"`
	SourceURL  string         `json:"source_url"`
	Freshness  string         `json:"freshness"`
	Caveats    []string       `json:"caveats"`
}

type EditionSummary struct {
	Key         string      `json:"key,omitempty"`
	Title       string      `json:"title,omitempty"`
	PublishDate string      `json:"publish_date,omitempty"`
	Publishers  []string    `json:"publishers,omitempty"`
	ISBN10      []string    `json:"isbn_10,omitempty"`
	ISBN13      []string    `json:"isbn_13,omitempty"`
	Works       []KeyRef    `json:"works,omitempty"`
	Authors     []KeyRef    `json:"authors,omitempty"`
	Covers      []int       `json:"covers,omitempty"`
	Identifiers interface{} `json:"identifiers,omitempty"`
}

type KeyRef struct {
	Key string `json:"key,omitempty"`
}

type AuthorResult struct {
	Source     string          `json:"source"`
	Configured bool            `json:"configured"`
	Query      map[string]any  `json:"query"`
	Request    RequestInfo     `json:"request"`
	Author     AuthorSummary   `json:"author"`
	Candidates []AuthorSummary `json:"candidates,omitempty"`
	Works      []WorkSummary   `json:"works,omitempty"`
	SourceURL  string          `json:"source_url,omitempty"`
	Freshness  string          `json:"freshness"`
	Caveats    []string        `json:"caveats"`
}

type AuthorSummary struct {
	Key            string   `json:"key,omitempty"`
	Name           string   `json:"name,omitempty"`
	TopWork        string   `json:"top_work,omitempty"`
	WorkCount      int      `json:"work_count,omitempty"`
	BirthDate      string   `json:"birth_date,omitempty"`
	DeathDate      string   `json:"death_date,omitempty"`
	AlternateNames []string `json:"alternate_names,omitempty"`
	SourceURL      string   `json:"source_url,omitempty"`
}

type WorkResult struct {
	Source     string         `json:"source"`
	Configured bool           `json:"configured"`
	Query      map[string]any `json:"query"`
	Request    RequestInfo    `json:"request"`
	Work       WorkSummary    `json:"work"`
	SourceURL  string         `json:"source_url"`
	Freshness  string         `json:"freshness"`
	Caveats    []string       `json:"caveats"`
}

type WorkSummary struct {
	Key              string   `json:"key,omitempty"`
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	FirstPublishDate string   `json:"first_publish_date,omitempty"`
	FirstPublishYear int      `json:"first_publish_year,omitempty"`
	EditionCount     int      `json:"edition_count,omitempty"`
	Authors          []KeyRef `json:"authors,omitempty"`
	Subjects         []string `json:"subjects,omitempty"`
	Covers           []int    `json:"covers,omitempty"`
	LatestRevision   int      `json:"latest_revision,omitempty"`
	SourceURL        string   `json:"source_url,omitempty"`
}

type EditionsResult struct {
	Source     string           `json:"source"`
	Configured bool             `json:"configured"`
	Query      map[string]any   `json:"query"`
	Request    RequestInfo      `json:"request"`
	Total      int              `json:"total"`
	Editions   []EditionSummary `json:"editions"`
	SourceURL  string           `json:"source_url"`
	Freshness  string           `json:"freshness"`
	Caveats    []string         `json:"caveats"`
}

type SubjectResult struct {
	Source     string          `json:"source"`
	Configured bool            `json:"configured"`
	Query      map[string]any  `json:"query"`
	Request    RequestInfo     `json:"request"`
	Subject    SubjectSummary  `json:"subject"`
	Works      []BookCandidate `json:"works"`
	Facets     *SubjectFacets  `json:"facets,omitempty"`
	SourceURL  string          `json:"source_url"`
	Freshness  string          `json:"freshness"`
	Caveats    []string        `json:"caveats"`
}

type SubjectSummary struct {
	Key         string `json:"key,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	WorkCount   int    `json:"work_count,omitempty"`
	EBookCount  int    `json:"ebook_count,omitempty"`
	DetailsUsed bool   `json:"details_used"`
}

type SubjectFacets struct {
	Subjects   []Facet `json:"subjects,omitempty"`
	Authors    []Facet `json:"authors,omitempty"`
	Publishers []Facet `json:"publishers,omitempty"`
}

type Facet struct {
	Name  string `json:"name,omitempty"`
	Key   string `json:"key,omitempty"`
	Count int    `json:"count,omitempty"`
}

type SourcesResult struct {
	Source      string       `json:"source"`
	Auth        string       `json:"auth"`
	Configured  bool         `json:"configured"`
	BaseURL     string       `json:"base_url"`
	Identified  bool         `json:"identified"`
	RateLimit   string       `json:"rate_limit"`
	Endpoints   []SourceInfo `json:"endpoints"`
	Freshness   string       `json:"freshness"`
	Caveats     []string     `json:"caveats"`
	Environment []EnvInfo    `json:"environment"`
}

type SourceInfo struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Use    string `json:"use"`
	Status string `json:"status,omitempty"`
}

type EnvInfo struct {
	Name        string `json:"name"`
	Configured  bool   `json:"configured"`
	Description string `json:"description"`
}

type DoctorResult struct {
	Source        string   `json:"source"`
	Auth          string   `json:"auth"`
	BaseURL       string   `json:"base_url"`
	UserAgentSet  bool     `json:"user_agent_set"`
	ContactSet    bool     `json:"contact_email_set"`
	Identified    bool     `json:"identified"`
	GoVersion     string   `json:"go_version"`
	OSArch        string   `json:"os_arch"`
	ReadyCommands []string `json:"ready_commands"`
	Caveats       []string `json:"caveats"`
}

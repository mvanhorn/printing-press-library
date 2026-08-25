// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

type GuidanceResult struct {
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
	EnvVars  []string `json:"env_vars,omitempty"`
	Sources  []string `json:"sources,omitempty"`
}

type FederalRegisterDocument struct {
	Title           string   `json:"title"`
	Type            string   `json:"type"`
	DocumentNumber  string   `json:"document_number"`
	PublicationDate string   `json:"publication_date"`
	Agencies        []string `json:"agencies,omitempty"`
	Abstract        string   `json:"abstract,omitempty"`
	HTMLURL         string   `json:"html_url,omitempty"`
	PDFURL          string   `json:"pdf_url,omitempty"`
	Excerpt         string   `json:"excerpt,omitempty"`
}

type FederalRegisterSearchResult struct {
	Kind        string                    `json:"kind"`
	Source      string                    `json:"source"`
	Query       map[string]string         `json:"query"`
	Count       int                       `json:"count"`
	TotalPages  int                       `json:"total_pages"`
	Results     []FederalRegisterDocument `json:"results"`
	Caveats     []string                  `json:"caveats,omitempty"`
	SourceLinks []string                  `json:"source_links,omitempty"`
}

type RegulationsDocument struct {
	ID                  string `json:"id"`
	AgencyID            string `json:"agency_id,omitempty"`
	Title               string `json:"title,omitempty"`
	DocumentType        string `json:"document_type,omitempty"`
	DocketID            string `json:"docket_id,omitempty"`
	FRDocumentNumber    string `json:"fr_document_number,omitempty"`
	PostedDate          string `json:"posted_date,omitempty"`
	CommentStartDate    string `json:"comment_start_date,omitempty"`
	CommentEndDate      string `json:"comment_end_date,omitempty"`
	OpenForComment      bool   `json:"open_for_comment"`
	WithinCommentPeriod bool   `json:"within_comment_period"`
}

type RegulationsListResult struct {
	Kind        string                `json:"kind"`
	Source      string                `json:"source"`
	Query       map[string]string     `json:"query"`
	PageSize    int                   `json:"page_size"`
	Total       int                   `json:"total"`
	HasNextPage bool                  `json:"has_next_page"`
	Results     []RegulationsDocument `json:"results"`
	Caveats     []string              `json:"caveats,omitempty"`
	SourceLinks []string              `json:"source_links,omitempty"`
}

type DocketResult struct {
	Kind        string            `json:"kind"`
	Source      string            `json:"source"`
	ID          string            `json:"id"`
	AgencyID    string            `json:"agency_id,omitempty"`
	Title       string            `json:"title,omitempty"`
	DocketType  string            `json:"docket_type,omitempty"`
	ModifyDate  string            `json:"modify_date,omitempty"`
	Abstract    string            `json:"abstract,omitempty"`
	RawFields   map[string]string `json:"raw_fields,omitempty"`
	SourceLinks []string          `json:"source_links,omitempty"`
}

type SourcesResult struct {
	Kind    string         `json:"kind"`
	Sources []SourceStatus `json:"sources"`
}

type SourceStatus struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Auth       string   `json:"auth"`
	EnvVars    []string `json:"env_vars,omitempty"`
	Notes      []string `json:"notes,omitempty"`
	SourceURLs []string `json:"source_urls,omitempty"`
}

package source

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/cliutil"
)

type semanticScholarSource struct {
	limiter *cliutil.AdaptiveLimiter
}

func init() {
	// Keyless Semantic Scholar is heavily throttled (HTTP 429). Pace very
	// conservatively; a key raises limits substantially.
	rate := 0.8
	if os.Getenv("SEMANTIC_SCHOLAR_API_KEY") != "" {
		rate = 5.0
	}
	Register(&semanticScholarSource{limiter: cliutil.NewAdaptiveLimiter(rate)})
}

func (s *semanticScholarSource) Name() string { return "semanticscholar" }
func (s *semanticScholarSource) Description() string {
	return "Semantic Scholar — TLDR summaries + publication types (optional; rate-limited without a key)"
}
func (s *semanticScholarSource) AuthRequired() bool     { return false }
func (s *semanticScholarSource) OptionalKeyEnv() string { return "SEMANTIC_SCHOLAR_API_KEY" }

type s2SearchResponse struct {
	Total int       `json:"total"`
	Data  []s2Paper `json:"data"`
}

type s2Paper struct {
	PaperID          string   `json:"paperId"`
	Title            string   `json:"title"`
	Year             int      `json:"year"`
	CitationCount    int      `json:"citationCount"`
	PublicationTypes []string `json:"publicationTypes"`
	ExternalIDs      struct {
		DOI string `json:"DOI"`
	} `json:"externalIds"`
	TLDR *struct {
		Text string `json:"text"`
	} `json:"tldr"`
}

func (s *semanticScholarSource) Sync(ctx context.Context, opts SyncOptions) ([]Work, error) {
	papers, _, err := s2Search(ctx, s.limiter, opts.Query, opts.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]Work, 0, len(papers))
	for _, p := range papers {
		out = append(out, Work{
			Source:   "semanticscholar",
			SourceID: p.PaperID,
			Title:    p.Title,
			Year:     p.Year,
			DOI:      p.ExternalIDs.DOI,
			URL:      "https://www.semanticscholar.org/paper/" + p.PaperID,
		})
	}
	return out, nil
}

func s2Search(ctx context.Context, lim *cliutil.AdaptiveLimiter, query string, limit int) ([]s2Paper, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	u := fmt.Sprintf("https://api.semanticscholar.org/graph/v1/paper/search?query=%s&limit=%d&fields=title,year,citationCount,publicationTypes,externalIds,tldr",
		url.QueryEscape(query), limit)
	var resp s2SearchResponse
	if err := getJSONWithKey(ctx, lim, u, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Data, resp.Total, nil
}

// S2Enrichment carries TLDR + publication types for a DOI.
type S2Enrichment struct {
	TLDR     string
	PubTypes []string
}

// SemanticScholarEnrich returns TLDR + publication types keyed by lowercased DOI
// for a query. Best-effort: on 429 (keyless throttle) it returns the
// RateLimitError so callers can note the degraded state rather than silently
// dropping the source.
func SemanticScholarEnrich(ctx context.Context, query string, sample int) (map[string]S2Enrichment, error) {
	var lim *cliutil.AdaptiveLimiter
	if s, ok := Lookup("semanticscholar"); ok {
		if ss, ok := s.(*semanticScholarSource); ok {
			lim = ss.limiter
		}
	}
	papers, _, err := s2Search(ctx, lim, query, sample)
	if err != nil {
		return nil, err
	}
	out := map[string]S2Enrichment{}
	for _, p := range papers {
		if p.ExternalIDs.DOI == "" {
			continue
		}
		e := S2Enrichment{PubTypes: p.PublicationTypes}
		if p.TLDR != nil {
			e.TLDR = p.TLDR.Text
		}
		out[normDOI(p.ExternalIDs.DOI)] = e
	}
	return out, nil
}

package source

import (
	"context"
	"fmt"
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/cliutil"
)

type europePMCSource struct {
	limiter *cliutil.AdaptiveLimiter
}

func init() {
	Register(&europePMCSource{limiter: cliutil.NewAdaptiveLimiter(5)})
}

func (e *europePMCSource) Name() string { return "europepmc" }
func (e *europePMCSource) Description() string {
	return "Europe PMC — full-text biomedical abstracts, open-access status, publication types"
}
func (e *europePMCSource) AuthRequired() bool     { return false }
func (e *europePMCSource) OptionalKeyEnv() string { return "" }

type europePMCResponse struct {
	HitCount   int `json:"hitCount"`
	ResultList struct {
		Result []europePMCResult `json:"result"`
	} `json:"resultList"`
}

type europePMCResult struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	PubYear      string `json:"pubYear"`
	DOI          string `json:"doi"`
	AbstractText string `json:"abstractText"`
	IsOpenAccess string `json:"isOpenAccess"`
	PubTypeList  struct {
		PubType []string `json:"pubType"`
	} `json:"pubTypeList"`
}

func (e *europePMCSource) Sync(ctx context.Context, opts SyncOptions) ([]Work, error) {
	res, _, err := europePMCSearch(ctx, e.limiter, opts.Query, opts.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]Work, 0, len(res))
	for _, r := range res {
		y := 0
		fmt.Sscanf(r.PubYear, "%d", &y)
		out = append(out, Work{
			Source:   "europepmc",
			SourceID: r.ID,
			Title:    r.Title,
			Year:     y,
			DOI:      r.DOI,
			URL:      "https://europepmc.org/article/MED/" + r.ID,
		})
	}
	return out, nil
}

func europePMCSearch(ctx context.Context, lim *cliutil.AdaptiveLimiter, query string, pageSize int) ([]europePMCResult, int, error) {
	if pageSize <= 0 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	u := fmt.Sprintf("https://www.ebi.ac.uk/europepmc/webservices/rest/search?query=%s&format=json&pageSize=%d&resultType=core",
		url.QueryEscape(query), pageSize)
	var resp europePMCResponse
	if err := getJSON(ctx, lim, u, &resp); err != nil {
		return nil, 0, err
	}
	return resp.ResultList.Result, resp.HitCount, nil
}

// EuropePMCAbstracts returns abstract text keyed by DOI for works that have one,
// best-effort, for a query. Used to enrich works lacking an OpenAlex abstract.
func EuropePMCAbstracts(ctx context.Context, query string, sample int) (map[string]string, error) {
	var lim *cliutil.AdaptiveLimiter
	if s, ok := Lookup("europepmc"); ok {
		if es, ok := s.(*europePMCSource); ok {
			lim = es.limiter
		}
	}
	res, _, err := europePMCSearch(ctx, lim, query, sample)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range res {
		if r.DOI != "" && r.AbstractText != "" {
			out[r.DOI] = r.AbstractText
		}
	}
	return out, nil
}

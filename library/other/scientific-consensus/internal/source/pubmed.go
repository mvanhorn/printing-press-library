package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/cliutil"
)

const eutils = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

type pubmedSource struct {
	limiter *cliutil.AdaptiveLimiter
}

func init() {
	// Keyless PubMed allows ~3 req/s; NCBI_API_KEY raises to ~10.
	rate := 3.0
	if os.Getenv("NCBI_API_KEY") != "" {
		rate = 9.0
	}
	Register(&pubmedSource{limiter: cliutil.NewAdaptiveLimiter(rate)})
}

func (p *pubmedSource) Name() string { return "pubmed" }
func (p *pubmedSource) Description() string {
	return "PubMed E-utilities — biomedical literature + authoritative MeSH publication types"
}
func (p *pubmedSource) AuthRequired() bool     { return false }
func (p *pubmedSource) OptionalKeyEnv() string { return "NCBI_API_KEY" }

func apiKeyParam() string {
	if k := os.Getenv("NCBI_API_KEY"); k != "" {
		return "&api_key=" + url.QueryEscape(k)
	}
	return ""
}

func (p *pubmedSource) Sync(ctx context.Context, opts SyncOptions) ([]Work, error) {
	if opts.Limit <= 0 {
		opts.Limit = 25
	}
	ids, err := pubmedSearch(ctx, p.limiter, opts.Query, opts.Limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	summaries, err := pubmedSummaries(ctx, p.limiter, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Work, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, Work{
			Source:   "pubmed",
			SourceID: s.UID,
			Title:    s.Title,
			Year:     s.Year(),
			DOI:      s.DOI(),
			URL:      "https://pubmed.ncbi.nlm.nih.gov/" + s.UID + "/",
		})
	}
	return out, nil
}

func pubmedSearch(ctx context.Context, lim *cliutil.AdaptiveLimiter, query string, retmax int) ([]string, error) {
	u := fmt.Sprintf("%s/esearch.fcgi?db=pubmed&retmode=json&retmax=%d&term=%s%s",
		eutils, retmax, url.QueryEscape(query), apiKeyParam())
	var resp struct {
		ESearchResult struct {
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	if err := getJSON(ctx, lim, u, &resp); err != nil {
		return nil, err
	}
	return resp.ESearchResult.IDList, nil
}

type pubmedSummary struct {
	UID        string   `json:"uid"`
	Title      string   `json:"title"`
	PubDate    string   `json:"pubdate"`
	PubType    []string `json:"pubtype"`
	ArticleIDs []struct {
		IDType string `json:"idtype"`
		Value  string `json:"value"`
	} `json:"articleids"`
}

func (s pubmedSummary) Year() int {
	f := strings.Fields(s.PubDate)
	if len(f) == 0 {
		return 0
	}
	var y int
	fmt.Sscanf(f[0], "%d", &y)
	return y
}

func (s pubmedSummary) DOI() string {
	for _, a := range s.ArticleIDs {
		if a.IDType == "doi" {
			return a.Value
		}
	}
	return ""
}

func pubmedSummaries(ctx context.Context, lim *cliutil.AdaptiveLimiter, ids []string) ([]pubmedSummary, error) {
	u := fmt.Sprintf("%s/esummary.fcgi?db=pubmed&retmode=json&id=%s%s",
		eutils, strings.Join(ids, ","), apiKeyParam())
	var raw struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := getJSON(ctx, lim, u, &raw); err != nil {
		return nil, err
	}
	uidsRaw, ok := raw.Result["uids"]
	if !ok {
		return nil, nil
	}
	var uids []string
	_ = json.Unmarshal(uidsRaw, &uids)
	out := make([]pubmedSummary, 0, len(uids))
	for _, uid := range uids {
		if r, ok := raw.Result[uid]; ok {
			var s pubmedSummary
			if json.Unmarshal(r, &s) == nil {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// PubMedPubTypes returns authoritative MeSH publication types keyed by PMID.
// Best-effort: returns whatever it can resolve. Uses the registered pubmed
// source's limiter when available.
func PubMedPubTypes(ctx context.Context, pmids []string) (map[string][]string, error) {
	if len(pmids) == 0 {
		return nil, nil
	}
	var lim *cliutil.AdaptiveLimiter
	if s, ok := Lookup("pubmed"); ok {
		if ps, ok := s.(*pubmedSource); ok {
			lim = ps.limiter
		}
	}
	summaries, err := pubmedSummaries(ctx, lim, pmids)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(summaries))
	for _, s := range summaries {
		out[s.UID] = s.PubType
	}
	return out, nil
}

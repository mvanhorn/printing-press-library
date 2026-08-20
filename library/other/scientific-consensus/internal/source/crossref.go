package source

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/cliutil"
)

type crossrefSource struct {
	limiter *cliutil.AdaptiveLimiter
}

func init() {
	Register(&crossrefSource{limiter: cliutil.NewAdaptiveLimiter(5)})
}

func (c *crossrefSource) Name() string { return "crossref" }
func (c *crossrefSource) Description() string {
	return "Crossref — DOI metadata, citation counts, and funder data (powers `funding`)"
}
func (c *crossrefSource) AuthRequired() bool     { return false }
func (c *crossrefSource) OptionalKeyEnv() string { return "" }

type crossrefMessage struct {
	Message struct {
		TotalResults int            `json:"total-results"`
		Items        []crossrefItem `json:"items"`
	} `json:"message"`
}

type crossrefItem struct {
	DOI                 string   `json:"DOI"`
	Title               []string `json:"title"`
	Type                string   `json:"type"`
	IsReferencedByCount int      `json:"is-referenced-by-count"`
	Published           struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"published"`
	Funder []struct {
		Name  string   `json:"name"`
		Award []string `json:"award"`
	} `json:"funder"`
}

func (i crossrefItem) year() int {
	if len(i.Published.DateParts) > 0 && len(i.Published.DateParts[0]) > 0 {
		return i.Published.DateParts[0][0]
	}
	return 0
}

func (i crossrefItem) title() string {
	if len(i.Title) > 0 {
		return i.Title[0]
	}
	return ""
}

func (c *crossrefSource) Sync(ctx context.Context, opts SyncOptions) ([]Work, error) {
	items, err := crossrefQuery(ctx, c.limiter, opts.Query, opts.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]Work, 0, len(items))
	for _, it := range items {
		out = append(out, Work{
			Source:   "crossref",
			SourceID: it.DOI,
			Title:    it.title(),
			Year:     it.year(),
			DOI:      it.DOI,
			URL:      "https://doi.org/" + it.DOI,
		})
	}
	return out, nil
}

func crossrefQuery(ctx context.Context, lim *cliutil.AdaptiveLimiter, query string, rows int) ([]crossrefItem, error) {
	if rows <= 0 {
		rows = 25
	}
	u := fmt.Sprintf("https://api.crossref.org/works?rows=%d&query=%s&mailto=%s",
		rows, url.QueryEscape(query), url.QueryEscape("scientific-consensus-pp-cli@users.noreply.github.com"))
	var resp crossrefMessage
	if err := getJSON(ctx, lim, u, &resp); err != nil {
		return nil, err
	}
	return resp.Message.Items, nil
}

// FunderStat is an aggregated funder count.
type FunderStat struct {
	Funder string `json:"funder"`
	Works  int    `json:"works"`
}

// CrossrefFunders returns funder concentration for a query, aggregated across
// the first `sample` Crossref works.
func CrossrefFunders(ctx context.Context, query string, sample int) ([]FunderStat, int, error) {
	var lim *cliutil.AdaptiveLimiter
	if s, ok := Lookup("crossref"); ok {
		if cs, ok := s.(*crossrefSource); ok {
			lim = cs.limiter
		}
	}
	items, err := crossrefQuery(ctx, lim, query, sample)
	if err != nil {
		return nil, 0, err
	}
	counts := map[string]int{}
	withFunder := 0
	for _, it := range items {
		seen := map[string]bool{}
		hadFunder := false
		for _, f := range it.Funder {
			if f.Name == "" || seen[f.Name] {
				continue
			}
			seen[f.Name] = true
			counts[f.Name]++
			hadFunder = true
		}
		if hadFunder {
			withFunder++
		}
	}
	stats := make([]FunderStat, 0, len(counts))
	for name, n := range counts {
		stats = append(stats, FunderStat{Funder: name, Works: n})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Works != stats[j].Works {
			return stats[i].Works > stats[j].Works
		}
		return stats[i].Funder < stats[j].Funder
	})
	return stats, withFunder, nil
}

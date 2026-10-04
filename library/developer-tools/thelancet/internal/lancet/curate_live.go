package lancet

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/cliutil"
)

// CurateLive answers a curate query directly from OpenAlex when the local store
// is empty or absent. It scopes to the Lancet family (or a single journal ISSN),
// searches the topic, and sorts by citations or date. This lets `curate` work
// out-of-the-box before any refresh.
func CurateLive(ctx context.Context, c Fetcher, topic, issn, sort string, openAccessOnly bool, limit int) ([]WorkRow, error) {
	if sort == "per-year" {
		return nil, fmt.Errorf("--sort per-year needs the local store (OpenAlex cannot rank by citations per year); run 'refresh' and use --data-source local, or sort by citations or date")
	}
	filter := FamilyISSNFilter()
	if issn != "" {
		filter = "primary_location.source.issn:" + issn
	}
	if openAccessOnly {
		filter += ",is_oa:true"
	}
	sortParam := "cited_by_count:desc"
	if sort == "date" {
		sortParam = "publication_date:desc"
	}
	if limit < 1 {
		limit = 25
	}
	params := map[string]string{
		"search":   topic,
		"filter":   filter,
		"sort":     sortParam,
		"per-page": strconv.Itoa(limit),
		"select":   "doi,title,publication_year,publication_date,cited_by_count,primary_topic,primary_location",
	}
	raw, err := c.Get(ctx, "/works", params)
	if err != nil {
		return nil, err
	}
	var page struct {
		Results []struct {
			DOI             string `json:"doi"`
			Title           string `json:"title"`
			PublicationYear int    `json:"publication_year"`
			PublicationDate string `json:"publication_date"`
			CitedByCount    int    `json:"cited_by_count"`
			PrimaryTopic    *struct {
				DisplayName string `json:"display_name"`
			} `json:"primary_topic"`
			PrimaryLocation *struct {
				Source *struct {
					DisplayName string `json:"display_name"`
				} `json:"source"`
			} `json:"primary_location"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]WorkRow, 0, len(page.Results))
	for _, r := range page.Results {
		w := WorkRow{
			Title: cliutil.CleanText(r.Title),
			DOI:   strings.TrimPrefix(r.DOI, "https://doi.org/"),
			Year:  r.PublicationYear,
			Cited: r.CitedByCount,
		}
		if _, err := time.Parse("2006-01-02", r.PublicationDate); err == nil {
			w.PubDate = r.PublicationDate
		}
		w.CitationsPerYear, _ = CitationsPerYear(now, w.PubDate, w.Year, w.Cited)
		if r.PrimaryTopic != nil {
			w.Topic = cliutil.CleanText(r.PrimaryTopic.DisplayName)
		}
		if r.PrimaryLocation != nil && r.PrimaryLocation.Source != nil {
			w.Journal = cliutil.CleanText(r.PrimaryLocation.Source.DisplayName)
		}
		out = append(out, w)
	}
	return out, nil
}

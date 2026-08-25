package lancet

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/cliutil"
)

// Fetcher is the minimal client surface the sync needs. The generated
// *client.Client satisfies it.
type Fetcher interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// worksSelect is the OpenAlex field projection the local store needs.
const worksSelect = "id,doi,title,publication_year,publication_date,cited_by_count,open_access,primary_topic,authorships"

type decodedAuthor struct {
	ID           string
	Name         string
	Institutions []decodedInstitution
}

type decodedInstitution struct {
	ID      string
	Name    string
	Country string
}

type decodedWork struct {
	ID      string
	DOI     string
	Title   string
	Year    int
	Date    string
	Cited   int
	IsOA    bool
	Topic   string
	Authors []decodedAuthor
}

// rawWork mirrors the subset of the OpenAlex work JSON we decode.
type rawWork struct {
	ID              string `json:"id"`
	DOI             string `json:"doi"`
	Title           string `json:"title"`
	PublicationYear int    `json:"publication_year"`
	PublicationDate string `json:"publication_date"`
	CitedByCount    int    `json:"cited_by_count"`
	OpenAccess      struct {
		IsOA bool `json:"is_oa"`
	} `json:"open_access"`
	PrimaryTopic *struct {
		DisplayName string `json:"display_name"`
	} `json:"primary_topic"`
	Authorships []struct {
		Author struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"author"`
		Institutions []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			CountryCode string `json:"country_code"`
		} `json:"institutions"`
	} `json:"authorships"`
}

type worksPage struct {
	Meta struct {
		Count      int    `json:"count"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
	Results []rawWork `json:"results"`
}

func decodeWork(r rawWork) decodedWork {
	w := decodedWork{
		ID:    shortID(r.ID),
		DOI:   strings.TrimPrefix(r.DOI, "https://doi.org/"),
		Title: cliutil.CleanText(r.Title),
		Year:  r.PublicationYear,
		Date:  r.PublicationDate,
		Cited: r.CitedByCount,
		IsOA:  r.OpenAccess.IsOA,
	}
	if r.PrimaryTopic != nil {
		w.Topic = cliutil.CleanText(r.PrimaryTopic.DisplayName)
	}
	for _, a := range r.Authorships {
		da := decodedAuthor{ID: shortID(a.Author.ID), Name: a.Author.DisplayName}
		for _, inst := range a.Institutions {
			da.Institutions = append(da.Institutions, decodedInstitution{
				ID:      shortID(inst.ID),
				Name:    inst.DisplayName,
				Country: inst.CountryCode,
			})
		}
		w.Authors = append(w.Authors, da)
	}
	return w
}

// shortID strips the OpenAlex URL prefix, keeping the bare W.../A.../I... id.
func shortID(u string) string {
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}

// RefreshResult reports per-journal sync outcomes.
type RefreshResult struct {
	Journal string `json:"journal"`
	ISSN    string `json:"issn"`
	Stored  int    `json:"stored"`
}

// Refresh syncs works for each journal into the local store using OpenAlex
// cursor pagination. fromYear of 0 means no lower bound. maxPages caps the
// number of 200-item pages fetched per journal (bounds cost and time).
func Refresh(ctx context.Context, c Fetcher, db *sql.DB, journ []Journal, fromYear, maxPages int, progress io.Writer) ([]RefreshResult, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	// When a lower-bound year is set the caller wants historical coverage for
	// the time-window analytics (drift, affiliation-growth), so fill oldest
	// works first. With no bound, newest-first is the useful default for
	// search/curate/rank-authors.
	sortOrder := "publication_date:desc"
	if fromYear > 0 {
		sortOrder = "publication_date:asc"
	}
	var results []RefreshResult
	for _, j := range journ {
		filter := "primary_location.source.issn:" + j.ISSN
		if fromYear > 0 {
			filter += ",from_publication_date:" + strconv.Itoa(fromYear) + "-01-01"
		}
		cursor := "*"
		stored := 0
		for page := 0; page < maxPages && cursor != ""; page++ {
			params := map[string]string{
				"filter":   filter,
				"select":   worksSelect,
				"per-page": "200",
				"cursor":   cursor,
				"sort":     sortOrder,
			}
			raw, err := c.Get(ctx, "/works", params)
			if err != nil {
				return results, fmt.Errorf("fetching %s page %d: %w", j.Slug, page+1, err)
			}
			var pg worksPage
			if err := json.Unmarshal(raw, &pg); err != nil {
				return results, fmt.Errorf("decoding %s page %d: %w", j.Slug, page+1, err)
			}
			if len(pg.Results) == 0 {
				break
			}
			batch := make([]decodedWork, 0, len(pg.Results))
			for _, r := range pg.Results {
				batch = append(batch, decodeWork(r))
			}
			n, err := StoreWorks(ctx, db, batch, j.ISSN, j.Display)
			if err != nil {
				return results, fmt.Errorf("storing %s: %w", j.Slug, err)
			}
			stored += n
			cursor = pg.Meta.NextCursor
			if progress != nil {
				fmt.Fprintf(progress, "  %s: %d works stored\n", j.Slug, stored)
			}
		}
		results = append(results, RefreshResult{Journal: j.Slug, ISSN: j.ISSN, Stored: stored})
	}
	return results, nil
}

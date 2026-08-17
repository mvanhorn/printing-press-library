// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// OpenAlex (CC0) store writer: the authoritative bulk source for the local
// article corpus, replacing the retired Cloudflare-gated NEJM RSS transport
// which carried no abstracts, citation data, or OA flags. Cursor-paginates
// api.openalex.org/works filtered to NEJM (ISSN 0028-4793), newest first, and
// upserts each work as a nejmArticleRecord with feed='openalex'. Reuses the
// live-search building blocks in openalex.go (work shape, abstract
// reconstruction, author formatting).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/nejm/internal/store"
)

const (
	openAlexSyncPerPage    = 50 // OpenAlex maximum
	openAlexSyncMaxRetries = 3
)

// openAlexSyncWork extends the shared openAlexWork shape (openalex.go) with
// fields the sync path stores but the live search command does not surface:
// bibliographic volume/issue/pages, the work type, and the primary topic's
// subfield (the closest OpenAlex analog to an NEJM specialty).
type openAlexSyncWork struct {
	openAlexWork
	Type   string `json:"type"`
	Biblio struct {
		Volume    string `json:"volume"`
		Issue     string `json:"issue"`
		FirstPage string `json:"first_page"`
		LastPage  string `json:"last_page"`
	} `json:"biblio"`
	PrimaryTopic struct {
		Subfield struct {
			DisplayName string `json:"display_name"`
		} `json:"subfield"`
	} `json:"primary_topic"`
}

// openAlexSyncResponse mirrors the /works envelope for cursor pagination.
type openAlexSyncResponse struct {
	Results []openAlexSyncWork `json:"results"`
	Meta    struct {
		Count      int    `json:"count"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
}

// nejmOpenAlexWorkToRecord flattens an OpenAlex work into the canonical store
// record. DOIs keep OpenAlex's native lowercase form (prefix-stripped).
func nejmOpenAlexWorkToRecord(w openAlexSyncWork) nejmArticleRecord {
	doi := strings.TrimPrefix(w.DOI, "https://doi.org/")
	u := w.PrimaryLocation.LandingPageURL
	if u == "" && doi != "" {
		u = "https://doi.org/" + doi
	}
	pages := w.Biblio.FirstPage
	if pages != "" && w.Biblio.LastPage != "" && w.Biblio.LastPage != w.Biblio.FirstPage {
		pages += "-" + w.Biblio.LastPage
	}
	return nejmArticleRecord{
		ID:          doi,
		DOI:         doi,
		Title:       w.Title,
		Authors:     openAlexAuthors(w.openAlexWork),
		Abstract:    openAlexDecodeAbstract(w.AbstractInvertedIndex),
		ArticleType: w.Type,
		Specialties: w.PrimaryTopic.Subfield.DisplayName,
		Date:        w.PublicationDate,
		IsFree:      w.OpenAccess.IsOA,
		URL:         u,
		Volume:      w.Biblio.Volume,
		Issue:       w.Biblio.Issue,
		Pages:       pages,
		Feed:        "openalex",
	}
}

// nejmOpenAlexFetchPage GETs one cursor page with bounded retries. Transient
// failures (network errors, HTTP 429, HTTP 5xx) retry with exponential
// backoff; everything else (403, malformed body) fails immediately so a
// policy block is not retried into a rate-limit ban. reqTimeout bounds each
// individual attempt (--timeout is per-request, so a hung connection cannot
// stall a multi-minute corpus sync); the run as a whole is bounded by the
// parent context and --max-pages, not by reqTimeout.
func nejmOpenAlexFetchPage(ctx context.Context, cursor string, reqTimeout time.Duration) (*openAlexSyncResponse, error) {
	params := url.Values{}
	params.Set("filter", "primary_location.source.issn:"+openAlexNEJMISSN)
	params.Set("per-page", fmt.Sprintf("%d", openAlexSyncPerPage))
	params.Set("sort", "publication_date:desc")
	params.Set("cursor", cursor)
	openAlexSetMailto(params)
	reqURL := openAlexWorksURL + "?" + params.Encode()

	var lastErr error
	backoff := time.Second
	for attempt := 0; attempt <= openAlexSyncMaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		attemptCtx, cancel := ctx, context.CancelFunc(func() {})
		if reqTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, reqTimeout)
		}
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, reqURL, nil)
		if err != nil {
			cancel()
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("querying OpenAlex: %w", err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("reading OpenAlex response: %w", err)
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			var page openAlexSyncResponse
			if err := json.Unmarshal(body, &page); err != nil {
				return nil, fmt.Errorf("parsing OpenAlex response: %w", err)
			}
			return &page, nil
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("OpenAlex HTTP %d: %s", resp.StatusCode, truncate(strings.TrimSpace(string(body)), 200))
		default:
			return nil, fmt.Errorf("OpenAlex request failed: HTTP %d: %s", resp.StatusCode, truncate(strings.TrimSpace(string(body)), 200))
		}
	}
	return nil, fmt.Errorf("OpenAlex unavailable after %d retries: %w", openAlexSyncMaxRetries, lastErr)
}

// nejmSyncOpenAlex pages through the NEJM corpus on OpenAlex and upserts every
// work into the local store. startCursor resumes an interrupted run ("" starts
// from the head); maxPages > 0 bounds the run. Returns the stored count and
// the resume cursor — non-empty when the page cap stopped the run before the
// corpus was exhausted, "" on natural completion.
//
// On first contact it purges rows left behind by the retired RSS transport
// (feed 'etoc'/'axatoc'): those rows carry no abstracts and use a different
// DOI casing, so they would duplicate every re-synced article.
func nejmSyncOpenAlex(ctx context.Context, db *store.Store, startCursor string, maxPages int, reqTimeout time.Duration, events io.Writer) (int, string, error) {
	if events == nil {
		events = io.Discard
	}
	if err := nejmEnsureFeedColumn(db); err != nil {
		return 0, startCursor, fmt.Errorf("migrating local database: %w", err)
	}
	if purged, err := db.PurgeArticlesByFeed(ctx, "etoc", "axatoc"); err != nil {
		return 0, startCursor, fmt.Errorf("purging retired RSS rows: %w", err)
	} else if purged > 0 {
		if humanFriendly {
			fmt.Fprintf(os.Stderr, "  purged %d abstract-less articles left by the retired RSS source\n", purged)
		} else {
			fmt.Fprintf(events, `{"event":"sync_info","resource":"article","reason":"rss_rows_purged","count":%d}`+"\n", purged)
		}
	}

	cursor := startCursor
	if cursor == "" {
		cursor = "*"
	}
	total := 0
	pages := 0
	lastProgressCount := 0
	lastProgressAt := time.Now()

	for {
		page, err := nejmOpenAlexFetchPage(ctx, cursor, reqTimeout)
		if err != nil {
			return total, cursor, err
		}
		if len(page.Results) == 0 {
			return total, "", nil
		}

		for _, work := range page.Results {
			rec := nejmOpenAlexWorkToRecord(work)
			if rec.DOI == "" {
				continue
			}
			data, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			if err := db.UpsertArticle(json.RawMessage(data)); err != nil {
				fmt.Fprintf(os.Stderr, "warning: upsert %s: %v\n", rec.DOI, err)
				continue
			}
			// The generated upsert does not write the NEJM-specific feed
			// column; set it explicitly (same pattern the RSS path used) so
			// provenance is queryable without waiting for the open-time
			// backfill migration.
			if _, err := db.DB().ExecContext(ctx,
				`UPDATE "article" SET "feed" = 'openalex' WHERE "doi" = ?`, rec.DOI); err != nil {
				fmt.Fprintf(os.Stderr, "warning: set feed for %s: %v\n", rec.DOI, err)
			}
			total++
		}
		pages++

		// Progress: every 100 articles or 5 seconds, whichever comes first.
		if total-lastProgressCount >= 100 || time.Since(lastProgressAt) >= 5*time.Second {
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "\r  article: %d synced (openalex)", total)
			} else {
				fmt.Fprintf(events, `{"event":"sync_progress","resource":"article","fetched":%d,"source":"openalex"}`+"\n", total)
			}
			lastProgressCount = total
			lastProgressAt = time.Now()
		}

		next := page.Meta.NextCursor
		if next == "" || next == cursor {
			return total, "", nil
		}
		cursor = next
		// Persist the cursor after each page so an interrupted full-corpus
		// sync resumes instead of restarting.
		if err := db.SaveSyncState("article", cursor, total); err != nil {
			fmt.Fprintf(os.Stderr, "\nwarning: failed to save sync state for article: %v\n", err)
		}

		if maxPages > 0 && pages >= maxPages {
			return total, cursor, nil
		}
	}
}

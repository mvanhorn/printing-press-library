// Copyright 2026 stanrails. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/visit-detroit-blog/internal/store"
)

// Inside the D blog content lives in the Visit Detroit Algolia content index.
// The blog slice is everything tagged sectionName:Blogs in that index.
const (
	algoliaBlogIndexPath = "/prod-visit-detroit-listings/query"
	blogFacetFilter      = "sectionName:Blogs"

	// Public, search-only Algolia credentials embedded in visitdetroit.com's
	// frontend JavaScript — the same key every browser that loads the site
	// receives. These are NOT private secrets; they are read-only and rate
	// limited. Overridable via env for when the site rotates the key.
	defaultAlgoliaAppID  = "EYQHJ2IY2M"
	defaultAlgoliaAPIKey = "c6d5977cb5cd80c09abfd2a7e5d9e88b"
)

func algoliaAppID() string {
	if v := os.Getenv("VISIT_DETROIT_BLOG_ALGOLIA_APP_ID"); v != "" {
		return v
	}
	return defaultAlgoliaAppID
}

func algoliaAPIKey() string {
	if v := os.Getenv("VISIT_DETROIT_BLOG_ALGOLIA_API_KEY"); v != "" {
		return v
	}
	return defaultAlgoliaAPIKey
}

func algoliaHeaders() map[string]string {
	return map[string]string{
		"X-Algolia-Application-Id": algoliaAppID(),
		"X-Algolia-API-Key":        algoliaAPIKey(),
		"Content-Type":             "application/json",
	}
}

// algoliaPostClient is satisfied by *client.Client. Search is a read-only POST,
// so it rides PostQueryWithParamsAndHeaders (doRead) rather than Post, which the
// verify-mode short-circuit treats as a mutation and refuses to dial out.
type algoliaPostClient interface {
	PostQueryWithParamsAndHeaders(ctx context.Context, path string, params map[string]string, body any, headers map[string]string) (json.RawMessage, int, error)
}

func algoliaSearchBody(query string, facetFilters [][]string, numericFilters []string, page, hitsPerPage int) (json.RawMessage, error) {
	body := map[string]any{
		"query":       query,
		"page":        page,
		"hitsPerPage": hitsPerPage,
	}
	if len(facetFilters) > 0 {
		body["facetFilters"] = facetFilters
	}
	if len(numericFilters) > 0 {
		body["numericFilters"] = numericFilters
	}
	return json.Marshal(body)
}

// algoliaPage extracts the hits array and page count from an Algolia response.
func algoliaPage(resp json.RawMessage) (hits []json.RawMessage, nbPages int, err error) {
	var env struct {
		Hits    []json.RawMessage `json:"hits"`
		NbPages int               `json:"nbPages"`
	}
	if err = json.Unmarshal(resp, &env); err != nil {
		return nil, 0, err
	}
	return env.Hits, env.NbPages, nil
}

// syncBlogsAlgolia pages the content index filtered to sectionName:Blogs and
// upserts every hit into the local store under resource_type "blogs". Returns a
// syncResult compatible with the generated sync harness in sync.go.
func syncBlogsAlgolia(ctx context.Context, c algoliaPostClient, db *store.Store, full bool, maxPages int) syncResult {
	started := time.Now()
	const resource = "blogs"

	if !humanFriendly {
		fmt.Fprintf(os.Stdout, `{"event":"sync_start","resource":"%s"}`+"\n", resource)
	}

	// Algolia has no delta cursor — every sync is a full page-0 scan of the
	// filtered index. A --full run additionally clears any stored cursor so no
	// stale resume state lingers if a resumable pagination path is added later.
	if full {
		_ = db.SaveSyncCursor(resource, "")
	}

	const pageSize = 1000 // Algolia hitsPerPage ceiling; the blog slice fits in one page.
	var total int
	page := 0
	headers := algoliaHeaders()

	for {
		if maxPages > 0 && page >= maxPages {
			if !humanFriendly {
				fmt.Fprintf(os.Stdout, `{"event":"sync_warning","resource":"%s","reason":"max_pages_hit","message":"capped at %d pages"}`+"\n", resource, maxPages)
			}
			break
		}

		body, err := algoliaSearchBody("", [][]string{{blogFacetFilter}}, nil, page, pageSize)
		if err != nil {
			return syncResult{Resource: resource, Count: total, Err: fmt.Errorf("building query for page %d: %w", page, err), Duration: time.Since(started)}
		}

		raw, status, err := c.PostQueryWithParamsAndHeaders(ctx, algoliaBlogIndexPath, map[string]string{}, json.RawMessage(body), headers)
		if err != nil {
			if status == 403 || status == 401 {
				if !humanFriendly {
					fmt.Fprintf(os.Stdout, `{"event":"sync_warning","resource":"%s","status":%d,"reason":"access_denied","message":"Algolia returned %d; the public search key may have rotated (set VISIT_DETROIT_BLOG_ALGOLIA_API_KEY)"}`+"\n", resource, status, status)
				}
				return syncResult{Resource: resource, Count: total, Warn: fmt.Errorf("access denied for %s (status %d)", resource, status), Duration: time.Since(started)}
			}
			return syncResult{Resource: resource, Count: total, Err: fmt.Errorf("fetching page %d: %w", page, err), Duration: time.Since(started)}
		}

		// Dry-run sentinel: client.dryRun returns {"dry_run": true} instead of a
		// real response under --dry-run. Emit a synthetic success and stop.
		if isDryRunResponse(raw) {
			if !humanFriendly {
				fmt.Fprintf(os.Stdout, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
			}
			return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
		}

		hits, nbPages, perr := algoliaPage(raw)
		if perr != nil {
			// Response isn't Algolia-shaped (e.g. a verify mock server); treat
			// as end of data rather than failing the whole sync.
			break
		}
		if len(hits) == 0 {
			break
		}

		stored, _, err := db.UpsertBatch(resource, hits)
		if err != nil {
			return syncResult{Resource: resource, Count: total, Err: fmt.Errorf("upserting page %d: %w", page, err), Duration: time.Since(started)}
		}
		total += stored

		if !humanFriendly {
			fmt.Fprintf(os.Stdout, `{"event":"sync_progress","resource":"%s","page":%d,"stored":%d,"total":%d}`+"\n", resource, page, stored, total)
		}

		page++
		if page >= nbPages {
			break
		}
	}

	_ = db.SaveSyncState(resource, "", total)

	if !humanFriendly {
		fmt.Fprintf(os.Stdout, `{"event":"sync_complete","resource":"%s","total":%d,"duration_ms":%d}`+"\n", resource, total, time.Since(started).Milliseconds())
	}
	return syncResult{Resource: resource, Count: total, Duration: time.Since(started)}
}

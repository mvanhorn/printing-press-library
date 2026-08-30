// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: sync — pull marketplace data (categories, collections, top APIs)
// into the local store with pagination and sync-state tracking.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

// defaultSyncResources lists the resource types synced by default.
func defaultSyncResources() []string {
	return []string{"category", "collection", "api"}
}

// maxSyncPages bounds the `api`/`collection` pagination loops in
// syncResource so a misbehaving or malicious `hasNextPage: true` response
// can't spin forever. It's a conservative internal constant rather than a
// CLI flag — promote it to a flag in a follow-up if users need to tune it.
const maxSyncPages = 200

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var resource string

	cmd := &cobra.Command{
		Use:         "sync",
		Short:       "Sync marketplace data (categories, collections, top APIs) into the local store",
		Long:        "Pull marketplace data from the RapidAPI hub into the local SQLite store for offline querying. Tracks pagination progress and last-synced state per resource; re-running resumes from the saved cursor.",
		Example:     "  rapidapi-pp-cli sync\n  rapidapi-pp-cli sync --resource category --limit 50",
		Annotations: map[string]string{"pp:endpoint": "sync.marketplace", "pp:method": "POST", "pp:path": "/gateway/graphql"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.OpenWithContext(cmd.Context(), learnDBPath(""))
			if err != nil {
				return err
			}
			defer s.Close()

			resources := defaultSyncResources()
			if resource != "" {
				resources = []string{resource}
			}
			if limit <= 0 {
				limit = 50
			}

			errCount := 0
			for _, res := range resources {
				count, _, err := syncResource(cmd, flags, s, res, limit, maxSyncPages)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "sync %s: %v\n", res, err)
					errCount++
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "synced %s: %d records\n", res, count)
			}
			if errCount > 0 {
				return fmt.Errorf("%d resource(s) failed to sync", errCount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "", "Sync a single resource type (category, collection, api)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max records per resource")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}

// syncResource pulls one resource type into the store, tracking sync state.
// maxPages bounds how many pages the `api`/`collection` loops below will
// fetch in this single call: the `sync` command passes maxSyncPages (a full
// sync), while autoRefreshIfStale passes a much smaller value so its
// background check stays a quick check, not a full sync (see auto_refresh.go).
// The returned bool reports whether the loop stopped because maxPages was
// hit rather than because the upstream genuinely ran out of data — callers
// that treat "fully refreshed" as a precondition (autoRefreshIfStale) must
// not ignore this: a capped resource is only partially synced even though
// no error occurred.
func syncResource(cmd *cobra.Command, flags *rootFlags, s *store.Store, resource string, limit int, maxPages int) (int, bool, error) {
	switch resource {
	case "category":
		// getCategoriesByCtx has no pagination construct at all — the hub
		// returns the full weighted list in one call. There is no cursor to
		// persist; last_synced_at (recorded regardless of the cursor value)
		// is what makes this sync's recency observable. Never capped: a
		// single call either succeeds or fails outright.
		variables := map[string]any{"limit": limit}
		data, err := gqlExec(cmd, flags, "getCategoriesByCtx", variables, gqlResponsePaths["getCategoriesByCtx"])
		if err != nil {
			return 0, false, err
		}
		count := cacheDomainRows(cmd, s, "category", data, s.UpsertCategories)
		return count, false, s.SaveSyncState(resource, "", count)

	case "collection":
		return syncCollectionResource(cmd, flags, s, resource, limit, maxPages)

	case "api":
		return syncAPIResource(cmd, flags, s, resource, limit, maxPages)

	default:
		return 0, false, fmt.Errorf("unknown resource %q (use category, collection, or api)", resource)
	}
}

// syncAPIResource paginates searchApis via its real GraphQL cursor
// (pageInfo.endCursor/hasNextPage), resuming from the persisted cursor and
// looping until the upstream reports no more pages or maxPages is hit.
func syncAPIResource(cmd *cobra.Command, flags *rootFlags, s *store.Store, resource string, limit int, maxPages int) (int, bool, error) {
	cursor, _, _, err := s.GetSyncState(resource)
	if err != nil {
		return 0, false, fmt.Errorf("reading sync state for %s: %w", resource, err)
	}

	total := 0
	pages := 0
	capped := false
	for {
		pagination := map[string]any{"first": limit}
		if cursor != "" {
			pagination["after"] = cursor
		}
		variables := map[string]any{
			"searchApiWhereInput":   map[string]any{"term": ""},
			"paginationInput":       pagination,
			"searchApiOrderByInput": map[string]any{"sortingFields": []map[string]any{{"by": "ASC", "fieldName": "ByRelevance"}}},
		}
		data, err := gqlExec(cmd, flags, "searchApis", variables, gqlResponsePaths["searchApisPage"])
		if err != nil {
			return total, false, fmt.Errorf("fetching %s: %w", resource, err)
		}

		var page struct {
			Nodes    json.RawMessage `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return total, false, fmt.Errorf("parsing %s page: %w", resource, err)
		}

		var rawNodes []json.RawMessage
		if err := json.Unmarshal(page.Nodes, &rawNodes); err != nil {
			return total, false, fmt.Errorf("parsing %s nodes: %w", resource, err)
		}
		if len(rawNodes) == 0 {
			break
		}

		total += cacheDomainRows(cmd, s, resource, page.Nodes, s.UpsertApis)
		if err := s.SaveSyncState(resource, page.PageInfo.EndCursor, total); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to save sync state for %s: %v\n", resource, err)
		}

		pages++
		if !page.PageInfo.HasNextPage {
			break
		}
		if pages >= maxPages {
			// Preserve the real, resumable cursor on a cap hit (deviates
			// from the Shopify CLI's unconditional post-loop clear — see
			// KTD3 in the plan) so the next call resumes past the cap
			// point instead of restarting from scratch.
			capped = true
			break
		}
		cursor = page.PageInfo.EndCursor
	}

	if !capped {
		if err := s.SaveSyncState(resource, "", total); err != nil {
			return total, false, fmt.Errorf("clearing sync cursor for %s: %w", resource, err)
		}
	}
	return total, capped, nil
}

// syncCollectionResource paginates GetCollectionsCollapsed via a real page
// number (this query has no pageInfo; a full page signals more data
// remains), resuming from the persisted page and looping until a partial
// page arrives or maxPages is hit.
func syncCollectionResource(cmd *cobra.Command, flags *rootFlags, s *store.Store, resource string, limit int, maxPages int) (int, bool, error) {
	cursor, _, _, err := s.GetSyncState(resource)
	if err != nil {
		return 0, false, fmt.Errorf("reading sync state for %s: %w", resource, err)
	}
	page := 1
	if cursor != "" {
		if p, convErr := strconv.Atoi(cursor); convErr == nil && p > 0 {
			page = p
		}
		// A non-numeric or non-positive cursor (including a pre-existing
		// legacy "page:<timestamp>" value from before this fix) is treated
		// as "no valid page" and resumes from page 1.
	}

	total := 0
	pages := 0
	capped := false
	for {
		variables := map[string]any{"page": page, "limit": limit}
		data, err := gqlExec(cmd, flags, "GetCollectionsCollapsed", variables, gqlResponsePaths["GetCollectionsCollapsed"])
		if err != nil {
			return total, false, fmt.Errorf("fetching %s: %w", resource, err)
		}

		var rawItems []json.RawMessage
		if err := json.Unmarshal(data, &rawItems); err != nil {
			return total, false, fmt.Errorf("parsing %s page: %w", resource, err)
		}
		if len(rawItems) == 0 {
			break
		}

		total += cacheDomainRows(cmd, s, resource, data, s.UpsertCollections)
		// Persist the NEXT page to fetch, not the page just consumed — the
		// cursor's contract is "resume from here", matching the api
		// resource's endCursor semantics. Saving the current page here
		// instead would make a capped resume re-fetch the same page
		// forever (the interim value is what survives on a cap-hit break
		// below; the final "fully synced" branch overwrites it with "").
		nextPage := page + 1
		if err := s.SaveSyncState(resource, strconv.Itoa(nextPage), total); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to save sync state for %s: %v\n", resource, err)
		}

		pages++
		if len(rawItems) < limit {
			break
		}
		if pages >= maxPages {
			capped = true
			break
		}
		page = nextPage
	}

	if !capped {
		if err := s.SaveSyncState(resource, "", total); err != nil {
			return total, false, fmt.Errorf("clearing sync cursor for %s: %w", resource, err)
		}
	}
	return total, capped, nil
}

// cacheDomainRows upserts a JSON array of records via a domain-specific
// store helper (e.g. UpsertApis) so records land in the typed domain tables.
// Per-item upsert failures are non-fatal by design (matching the Shopify
// CLI's anomaly-warning convention in this repo): a single malformed record
// must not fail the whole page. When any item in the page fails to store,
// a warning is emitted so the gap is visible instead of silent. This is a
// real, accepted tradeoff, not a false safety net: the caller's cursor
// still advances past this page regardless of per-item failures here
// (matching Shopify's own cursor semantics in this repo), so a failed
// item is not automatically retried by a later `sync` invocation — it
// requires clearing that resource's sync_state (e.g. via a future --full
// flag, not currently implemented) to force a full resync. Recorded here
// as a known limitation, not silently assumed away.
func cacheDomainRows(cmd *cobra.Command, s *store.Store, resource string, data json.RawMessage, upsert func(json.RawMessage) error) int {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return 0
	}
	count := 0
	for _, it := range items {
		if err := upsert(mustJSON(it)); err == nil {
			count++
		}
	}
	if count < len(items) {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s — %d of %d fetched record(s) failed to store\n", resource, len(items)-count, len(items))
	}
	return count
}

// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: shortlist hydrate — fetch full listing detail for board/IDs into SQLite.
// Pure HTTP GET /v1/listings/{id}; not part of search. HAR/live validated 2026-07-16.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newNovelShortlistHydrateCmd(flags *rootFlags) *cobra.Command {
	var (
		flagBoardID        string
		flagCollaboratorID string
		flagListingIDs     []string
		flagConcurrency    int
		flagForce          bool
		flagDB             string
	)

	cmd := &cobra.Command{
		Use:   "hydrate",
		Short: "Fetch full listing page details (about, rules, parking, amenities) for board or listing IDs into SQLite.",
		Long: `Pull richer listing documents after search/shortlist — not during search.

Uses pure HTTP GET /v1/listings/{id} (cookie session). Stores each payload under
resource_type=venues so shortlist compare/export and venues get see full sections:
description/about, rules, parking_info, cleaning, cancellation, amenities.

Provide --board-id (with --collaborator-id) and/or repeated --listing-id.`,
		Example: `  peerspace-pp-cli shortlist hydrate --board-id 6a590f4497e3495c4f756ee8 --collaborator-id 66915212d22cc89e3402c745
  peerspace-pp-cli shortlist hydrate --listing-id 68d468bb44492187e415d4a6 --listing-id 699c78bde7c8dc5e47480f0e --agent`,
		Annotations: map[string]string{
			"pp:endpoint":         "venues.detail",
			"pp:method":           "GET",
			"pp:path":             "/v1/listings/{id}",
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagConcurrency <= 0 {
				flagConcurrency = 3
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			ids := uniqueNonEmpty(flagListingIDs)
			if flagBoardID != "" {
				collab := stringsTrim(flagCollaboratorID)
				if collab == "" {
					// Fall back to PSUser cookie when present.
					if c, err := flags.newClient(); err == nil && c.Config != nil {
						collab = cookieValue(c.Config.CookieCredential(), "PSUser")
					}
				}
				if collab == "" {
					return fmt.Errorf("--collaborator-id is required with --board-id (or login so PSUser cookie is present)")
				}
				boardIDs, err := fetchBoardListingIDs(cmd, flags, collab, flagBoardID)
				if err != nil {
					return err
				}
				ids = uniqueNonEmpty(append(ids, boardIDs...))
			}
			if len(ids) == 0 {
				return fmt.Errorf("provide --listing-id and/or --board-id with listings")
			}

			// Skip already-hydrated unless --force
			if !flagForce {
				if s, err := openNovelStoreRO(ctx, flagDB); err == nil && s != nil {
					defer s.Close()
					filtered := make([]string, 0, len(ids))
					for _, id := range ids {
						if l, _, ok, _ := findListingByID(ctx, s, id); ok && l.Hydrated {
							continue
						}
						filtered = append(filtered, id)
					}
					// keep skipped count
					ids = filtered
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type result struct {
				ID      string `json:"id"`
				OK      bool   `json:"ok"`
				Title   string `json:"title,omitempty"`
				Fit     string `json:"format_fit,omitempty"`
				Error   string `json:"error,omitempty"`
				Skipped bool   `json:"skipped,omitempty"`
			}

			// Recompute skip list for reporting when force=false emptied ids
			if len(ids) == 0 {
				out := map[string]any{
					"hydrated": 0,
					"failed":   0,
					"skipped":  "all already hydrated (use --force to refresh)",
					"results":  []any{},
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			results := make([]result, len(ids))
			var wg sync.WaitGroup
			sem := make(chan struct{}, flagConcurrency)
			var okN, failN int64

			for i, id := range ids {
				wg.Add(1)
				go func(i int, id string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					data, err := fetchListingDetail(ctx, c, id)
					if err != nil {
						results[i] = result{ID: id, OK: false, Error: err.Error()}
						atomic.AddInt64(&failN, 1)
						return
					}
					l, err := parseAndUpsertListingDetail(ctx, id, data)
					if err != nil {
						results[i] = result{ID: id, OK: false, Error: err.Error()}
						atomic.AddInt64(&failN, 1)
						return
					}
					results[i] = result{ID: l.ID, OK: true, Title: l.Title, Fit: l.FormatFit}
					atomic.AddInt64(&okN, 1)
				}(i, id)
			}
			wg.Wait()

			summary := map[string]any{
				"hydrated": okN,
				"failed":   failN,
				"results":  results,
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && !wantsMachineOutput(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "hydrate: ok=%d failed=%d\n", okN, failN)
				for _, r := range results {
					if r.OK {
						fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s  %s  fit=%s\n", r.ID, r.Title, r.Fit)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s  %s\n", r.ID, r.Error)
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}

	cmd.Flags().StringVar(&flagBoardID, "board-id", "", "Favorite board / project id (loads listing ids from fav_board)")
	cmd.Flags().StringVar(&flagCollaboratorID, "collaborator-id", "", "SSO/collaborator id for fav-board (defaults to PSUser cookie)")
	cmd.Flags().StringArrayVar(&flagListingIDs, "listing-id", nil, "Listing id to hydrate (repeatable)")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 3, "Parallel detail fetches")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Re-fetch even if listing already hydrated in SQLite")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}

func stringsTrim(s string) string { return strings.TrimSpace(s) }

func uniqueNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func cookieValue(cookieHeader, name string) string {
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		k, v, ok := strings.Cut(part, "=")
		if ok && strings.EqualFold(strings.TrimSpace(k), name) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// fetchBoardListingIDs returns listing ids attached to a specific board.
func fetchBoardListingIDs(cmd *cobra.Command, flags *rootFlags, collaboratorID, boardID string) ([]string, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := replacePathParam("/v1/projects/attachments/collaborator/{collaborator_id}/xr/fav_board", "collaborator_id", collaboratorID)
	data, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "projects", false, path, map[string]string{"limit": "500"}, listingAuthHeaders(c), "", cmd.ErrOrStderr())
	if err != nil {
		return nil, err
	}
	// Prefer attachments filtered by project_id == boardID
	var env struct {
		Attachments []struct {
			Value     string `json:"value"`
			ProjectID string `json:"project_id"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(data, &env); err == nil && len(env.Attachments) > 0 {
		out := make([]string, 0, len(env.Attachments))
		for _, a := range env.Attachments {
			if boardID != "" && a.ProjectID != "" && a.ProjectID != boardID {
				continue
			}
			if a.Value != "" {
				out = append(out, a.Value)
			}
		}
		return uniqueNonEmpty(out), nil
	}
	// Fallback: all favorite ids
	return venuex.ExtractFavoriteIDs(data), nil
}

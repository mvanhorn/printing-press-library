// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: mirror amazon.jobs listings into local SQLite.
// Preserved across `generate --force`.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/store"
)

const syncPageSize = 100

type syncView struct {
	Synced      int    `json:"synced"`
	Pages       int    `json:"pages"`
	TotalHits   int    `json:"total_hits"`
	SavedSearch string `json:"saved_search,omitempty"`
	DB          string `json:"db"`
	Curtailed   bool   `json:"curtailed,omitempty"`
}

func newNovelSyncCmd(flags *rootFlags) *cobra.Command {
	var country, state, city, sort, savedName, dbFlag string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "sync [query]",
		Short: "Mirror Amazon job listings into the local SQLite store for offline commands",
		Long: strings.Trim(`
Fetch amazon.jobs listings for a query/location and upsert their full records
into the local SQLite store (keyed by id). Offline commands ('stats', 'skills')
read this store. Pass --saved <name> to sync a saved search's query and update
its new-since cursor.

This is the store populator; run it before 'stats' or 'skills'.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli sync engineer --max-pages 5
  amazon-jobs-pp-cli sync --city Seattle --max-pages 3
  amazon-jobs-pp-cli sync --saved sde-seattle`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync amazon.jobs listings into the local store")
				return nil
			}
			if err := guardDataSource(flags, "live"); err != nil {
				return err
			}
			if maxPages < 1 {
				maxPages = 5
			}
			curtailed := false
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
				curtailed = true
			}

			query := strings.TrimSpace(strings.Join(args, " "))
			dbPath := resolveDBPath(dbFlag)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			// A saved search's stored config overrides ad-hoc flags/query.
			if savedName != "" {
				s, gerr := getSavedSearch(ctx, db, savedName)
				if gerr != nil {
					return gerr
				}
				if s == nil {
					return usageErr(fmt.Errorf("no saved search named %q (see: amazon-jobs-pp-cli searches)", savedName))
				}
				query, country, state, city = s.Query, s.Country, s.State, s.City
				if s.Sort != "" {
					sort = s.Sort
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var synced, pages, totalHits int
			for p := 0; p < maxPages; p++ {
				values := buildSearchValues(query, country, state, city, sort, syncPageSize, p*syncPageSize)
				hits, raw, ferr := searchPage(ctx, c, values)
				if ferr != nil {
					return classifyAPIError(ferr, flags)
				}
				totalHits = hits
				pages++
				if len(raw) == 0 {
					break
				}
				stored, _, uerr := db.UpsertBatch("postings", raw)
				if uerr != nil {
					return fmt.Errorf("storing jobs: %w", uerr)
				}
				synced += stored
				if len(raw) < syncPageSize {
					break
				}
			}

			// Note: the new-since cursor is owned by `new`, not `sync`.
			// `sync --saved` only reuses the saved query/filters to mirror
			// records; advancing the cursor here would make `new` show nothing.

			view := syncView{
				Synced:      synced,
				Pages:       pages,
				TotalHits:   totalHits,
				SavedSearch: savedName,
				DB:          dbPath,
				Curtailed:   curtailed,
			}
			return emitResult(cmd, flags, view, func(w io.Writer) {
				fmt.Fprintf(w, "synced %d job(s) across %d page(s) into %s\n", synced, pages, dbPath)
				if totalHits > synced {
					fmt.Fprintf(w, "%d total hits upstream; raise --max-pages to mirror more\n", totalHits)
				}
				if savedName != "" {
					fmt.Fprintf(w, "mirrored saved search %q; its new-since cursor is unchanged (run `new` to advance it)\n", savedName)
				}
			})
		},
	}

	cmd.Flags().StringVar(&country, "country", "", "Filter by ISO alpha-3 country code (USA, GBR, IND, ...)")
	cmd.Flags().StringVar(&state, "state", "", "Filter by full state/region name")
	cmd.Flags().StringVar(&city, "city", "", "Filter by city name")
	cmd.Flags().StringVar(&sort, "sort", "recent", "Sort order: recent or relevant")
	cmd.Flags().StringVar(&savedName, "saved", "", "Sync a saved search by name (overrides query/filters)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum pages (100 jobs each) to mirror")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local SQLite store path (default: platform data dir)")

	return cmd
}

// jobIDFromRaw extracts the stable id from a raw job record (id_icims preferred,
// then id).
func jobIDFromRaw(raw json.RawMessage) string {
	var probe struct {
		IDIcims string `json:"id_icims"`
		ID      string `json:"id"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	if probe.IDIcims != "" {
		return probe.IDIcims
	}
	return probe.ID
}

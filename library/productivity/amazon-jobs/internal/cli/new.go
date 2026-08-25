// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: reqs unseen since the last check of a saved search.
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

const newScanPageSize = 100

type newView struct {
	SavedSearch string `json:"saved_search"`
	NewCount    int    `json:"new_count"`
	TotalNow    int    `json:"total_now"`
	// TotalHits is what upstream says the saved query matches. TotalNow only
	// counts what this run actually scanned, so the two diverging is the
	// signal that the page cap cut the scan short.
	TotalHits int    `json:"total_hits,omitempty"`
	Baseline  bool   `json:"baseline_established"`
	Curtailed bool   `json:"curtailed,omitempty"`
	Note      string `json:"note,omitempty"`
	Results   []Job  `json:"results"`
}

func newNovelNewCmd(flags *rootFlags) *cobra.Command {
	var dbFlag string
	var limit, maxPages int

	cmd := &cobra.Command{
		Use:   "new <saved-search>",
		Short: "Show Amazon reqs that appeared since your last check of a saved search",
		Long: strings.Trim(`
Fetch a saved search live, diff its current job ids against the saved cursor,
and print only the reqs you have not seen — then advance the cursor.

Use this command for reqs unseen since your last check of a saved search. Use
'find --sort recent' for newest-by-posted_date regardless of what you have seen.
Create a saved search first with 'save', and list them with 'searches'.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli new sde-seattle
  amazon-jobs-pp-cli new sde-seattle --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff a saved search against its new-since cursor")
				return nil
			}
			if err := guardDataSource(flags, "live"); err != nil {
				return err
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a saved-search name is required (see: amazon-jobs-pp-cli searches)"))
			}
			name := strings.TrimSpace(args[0])
			if limit < 1 {
				limit = 50
			}
			if maxPages < 1 {
				maxPages = 5
			}
			curtailed := false
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
				curtailed = true
			}
			dbPath := resolveDBPath(dbFlag)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			saved, err := getSavedSearch(ctx, db, name)
			if err != nil {
				return err
			}
			if saved == nil {
				return usageErr(fmt.Errorf("no saved search named %q (create one with: amazon-jobs-pp-cli save %s <query>)", name, name))
			}

			seenSet := make(map[string]struct{}, len(saved.LastSeen))
			for _, id := range saved.LastSeen {
				seenSet[id] = struct{}{}
			}
			// LastSynced (set only by a prior `new`) is the "has a baseline ever
			// been taken" marker. Using it instead of len(LastSeen) means an
			// always-empty search still transitions out of baseline after the
			// first check.
			hadBaseline := saved.LastSynced != ""

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			currentIDs := make([]string, 0, 256)
			currentSet := make(map[string]struct{}, 256)
			newJobs := make([]Job, 0)
			allRaw := make([]json.RawMessage, 0, 256)
			var totalHits, scannedPages int
			for p := 0; p < maxPages; p++ {
				values := buildSearchValues(saved.Query, saved.Country, saved.State, saved.City, saved.Sort, newScanPageSize, p*newScanPageSize)
				hits, raw, ferr := searchPage(ctx, c, values)
				if ferr != nil {
					return classifyAPIError(ferr, flags)
				}
				totalHits = hits
				if len(raw) == 0 {
					break
				}
				scannedPages++
				allRaw = append(allRaw, raw...)
				for _, r := range raw {
					id := jobIDFromRaw(r)
					if id == "" {
						continue
					}
					if _, dup := currentSet[id]; dup {
						continue
					}
					currentSet[id] = struct{}{}
					currentIDs = append(currentIDs, id)
					if _, seen := seenSet[id]; !seen {
						if j, perr := parseJob(r); perr == nil {
							newJobs = append(newJobs, j)
						}
					}
				}
				if len(raw) < newScanPageSize {
					break
				}
				// A full final page means the loop stopped at --max-pages, not
				// at the end of the result set.
				if p == maxPages-1 {
					curtailed = true
				}
			}
			// The upstream count is authoritative when the API reports one.
			if totalHits > len(currentIDs) {
				curtailed = true
			}

			// Keep the mirror fresh while we have the records in hand. A failed
			// mirror write must abort before the cursor moves: advancing it would
			// leave `stats`/`skills` reading an incomplete mirror while these
			// jobs never surface as new again.
			if len(allRaw) > 0 {
				if _, _, uerr := db.UpsertBatch("postings", allRaw); uerr != nil {
					return fmt.Errorf("mirroring jobs: %w", uerr)
				}
			}

			// Capture the true new count BEFORE truncating for display, then
			// advance the cursor to the full current set. Counting after
			// truncation would report only --limit and silently mark the
			// remaining new reqs seen — they would never surface again.
			trueNewCount := len(newJobs)
			// A complete scan replaces the cursor, so reqs that dropped out of
			// the result set stop being tracked. A curtailed scan must not do
			// that: discarding ids we simply never reached would resurface them
			// as "new" once they drift back into an earlier page.
			cursorIDs := currentIDs
			if curtailed {
				cursorIDs = unionUnreachedIDs(currentIDs, currentSet, saved.LastSeen)
			}
			if err := updateSavedSeen(ctx, db, name, cursorIDs, nowISO()); err != nil {
				return err
			}

			truncated := false
			if len(newJobs) > limit {
				newJobs = newJobs[:limit]
				truncated = true
			}
			// Always convert HTML to readable text for this curated command.
			for i := range newJobs {
				newJobs[i] = cleanJob(newJobs[i])
			}
			annotateFreshness(newJobs)

			view := newView{
				SavedSearch: name,
				TotalNow:    len(currentIDs),
				TotalHits:   totalHits,
				Curtailed:   curtailed,
				Results:     newJobs,
			}
			if !hadBaseline {
				// First check: establish a baseline instead of reporting the
				// entire current set as "new".
				view.Baseline = true
				view.NewCount = 0
				view.Results = []Job{}
				view.Note = fmt.Sprintf("baseline established: now tracking %d job(s); future runs show reqs new since now", len(currentIDs))
			} else {
				view.NewCount = trueNewCount
				if truncated {
					view.Note = fmt.Sprintf("showing first %d of %d new reqs; raise --limit to see the rest", limit, trueNewCount)
				}
			}
			if curtailed {
				scanned := fmt.Sprintf("scanned only %d req(s) across %d page(s)", len(currentIDs), scannedPages)
				if totalHits > len(currentIDs) {
					scanned = fmt.Sprintf("scanned only %d of %d upstream req(s) across %d page(s)", len(currentIDs), totalHits, scannedPages)
				}
				view.Note = joinNotes(view.Note,
					scanned+"; reqs beyond that window are not counted here and stay untracked until --max-pages covers them")
			}

			return emitLiveResult(cmd, flags, view, func(w io.Writer) {
				if view.Baseline {
					fmt.Fprintln(w, view.Note)
					return
				}
				if view.NewCount == 0 {
					fmt.Fprintf(w, "no new reqs for %q since last check (%d tracked)\n", name, view.TotalNow)
				} else {
					fmt.Fprintf(w, "%d new req(s) for %q since last check:\n\n", view.NewCount, name)
					for _, j := range view.Results {
						printJobLine(w, j)
					}
				}
				if view.Note != "" {
					fmt.Fprintf(w, "\n%s\n", view.Note)
				}
			})
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum new reqs to display")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum pages (100 jobs each) to scan for the current set")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local SQLite store path (default: platform data dir)")

	return cmd
}

// unionUnreachedIDs returns the scanned ids plus any previously-seen id the
// scan never reached, so a page-capped run cannot walk the cursor backwards.
func unionUnreachedIDs(scanned []string, scannedSet map[string]struct{}, prior []string) []string {
	out := make([]string, 0, len(scanned)+len(prior))
	out = append(out, scanned...)
	for _, id := range prior {
		if _, reached := scannedSet[id]; reached {
			continue
		}
		out = append(out, id)
	}
	return out
}

// joinNotes concatenates two human notes, tolerating an empty first note.
func joinNotes(existing, added string) string {
	if existing == "" {
		return added
	}
	return existing + "; " + added
}

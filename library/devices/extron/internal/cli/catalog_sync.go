// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored catalog sync: fetches the Extron literature index pages and
// stores the parsed catalog in the local resources table.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source live

// letterFailure records one letter bucket the crawl could not finish.
type letterFailure struct {
	Letter string `json:"letter"`
	Error  string `json:"error"`
}

// newCatalogClient is a seam so the letter-loop tests can point the crawl at a
// local test server instead of www.extron.com.
var newCatalogClient = extron.New

func newNovelCatalogSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var lettersCSV string
	var full bool
	var maxPages int
	var maxDuration time.Duration
	var retries int
	var strict bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch the Extron literature catalog into the local store",
		Long: `Fetch the Extron literature catalog into the local store. Run this before
relying on local search or catalog commands — it is what builds the catalog.

By default it fetches the first alphabetical index page per letter bucket
(0-9, A-Z); --full follows each category's pagination for a complete catalog.

This is not the same command as top-level 'sync'. Top-level 'sync' walks the
generated literature endpoint and refreshes entity lookups; only 'catalog sync'
populates the catalog that search, catalog completeness, catalog verify,
literature recent, and literature updates read.

A letter that fails is retried (--retries) and then skipped, so one bad bucket
no longer discards the other 35. The root --timeout bounds each letter bucket;
--max-duration bounds the whole crawl. Pass --strict to fail the run when any
letter was skipped.`,
		Example: "  extron-pp-cli catalog sync\n  extron-pp-cli catalog sync --full\n  extron-pp-cli catalog sync --full --max-duration 2h --retries 3",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync catalog")
			}
			// PATCH(amend-2026-08-24: budget the whole crawl separately from each
			// letter) — the root --timeout used to bound the entire 36-letter walk,
			// so a long --full run died mid-crawl no matter how large --timeout was.
			runCtx := cmd.Context()
			var cancelRun context.CancelFunc = func() {}
			if maxDuration > 0 {
				runCtx, cancelRun = context.WithTimeout(runCtx, maxDuration)
			}
			defer cancelRun()

			dbPath = resolveCatalogDB(flags, dbPath)
			db, err := store.OpenWithContext(runCtx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			letters := parseLettersCSV(lettersCSV)
			if cliutil.IsDogfoodEnv() && len(letters) > 1 {
				letters = letters[:1]
			}

			client := newCatalogClient()
			total := 0
			letterCounts := make(map[string]int, len(letters))
			failures := make([]letterFailure, 0, len(letters))
			succeeded := 0

			for _, letter := range letters {
				if runCtx.Err() != nil {
					failures = append(failures, letterFailure{Letter: letter, Error: runCtx.Err().Error()})
					continue
				}
				n, err := syncLetter(runCtx, flags, client, db, letter, full, maxPages, retries, cmd.ErrOrStderr())
				total += n
				letterCounts[letter] += n
				if err != nil {
					failures = append(failures, letterFailure{Letter: letter, Error: err.Error()})
					fmt.Fprintf(cmd.ErrOrStderr(), "catalog: letter %s: skipped after %d attempt(s): %v\n", letter, retries+1, err)
					continue
				}
				succeeded++
				fmt.Fprintf(cmd.ErrOrStderr(), "catalog: letter %s: %d docs (running total %d)\n", letter, letterCounts[letter], total)
			}

			// Record what actually landed even when some buckets were skipped, so a
			// partial catalog is not reported as never-synced.
			cursor := "partial"
			if full && maxPages <= 0 && len(failures) == 0 {
				cursor = "full"
			}
			if succeeded > 0 {
				if err := db.SaveSyncState(catalogResource, cursor, total); err != nil {
					return fmt.Errorf("recording sync state: %w", err)
				}
			}

			summary := struct {
				LettersFetched int             `json:"letters_fetched"`
				LettersFailed  int             `json:"letters_failed"`
				Docs           int             `json:"docs"`
				Full           bool            `json:"full"`
				Database       string          `json:"database"`
				PerLetter      map[string]int  `json:"per_letter,omitempty"`
				Errors         []letterFailure `json:"errors,omitempty"`
			}{
				LettersFetched: succeeded,
				LettersFailed:  len(failures),
				Docs:           total,
				Full:           full,
				Database:       dbPath,
				PerLetter:      letterCounts,
				Errors:         failures,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "Synced %d docs from %d letter bucket(s)%s into %s\n",
					total, succeeded, fullOpt(full), dbPath)
				if len(failures) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipped %d letter bucket(s): %s\n",
						len(failures), strings.Join(failedLetters(failures), ", "))
				}
			} else if err := printJSONFiltered(cmd.OutOrStdout(), summary, flags); err != nil {
				return err
			}

			if succeeded == 0 && len(failures) > 0 {
				return fmt.Errorf("catalog sync: every letter bucket failed (%d); first error: %s", len(failures), failures[0].Error)
			}
			if strict && len(failures) > 0 {
				return fmt.Errorf("catalog sync: --strict and %d letter bucket(s) skipped: %s", len(failures), strings.Join(failedLetters(failures), ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&lettersCSV, "letters", "", "letter buckets to fetch as a CSV list (default: 0-9,A-Z)")
	cmd.Flags().BoolVar(&full, "full", false, "follow per-category pagination for a complete catalog")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "maximum category pages per letter in --full mode (0 = unlimited)")
	cmd.Flags().DurationVar(&maxDuration, "max-duration", 30*time.Minute, "overall crawl budget; the root --timeout applies per letter bucket, not to the whole sync (0 = unlimited)")
	cmd.Flags().IntVar(&retries, "retries", 2, "retry attempts per letter bucket before skipping it")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero when any letter bucket was skipped")
	return cmd
}

// syncLetter fetches one letter bucket, retrying transient failures before
// giving up on it. It returns the docs stored even when the bucket ultimately
// failed, so a partial letter still counts toward the catalog.
//
// PATCH(amend-2026-08-24: continue past a failed letter bucket) — a single
// letter error used to abort the whole 0-9,A-Z crawl.
func syncLetter(parent context.Context, flags *rootFlags, client *extron.Client, db *store.Store, letter string, full bool, maxPages, retries int, logw io.Writer) (int, error) {
	if retries < 0 {
		retries = 0
	}
	stored := 0
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if parent.Err() != nil {
			return stored, parent.Err()
		}
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
			fmt.Fprintf(logw, "catalog: letter %s: retry %d/%d in %v (%v)\n", letter, attempt, retries, backoff, lastErr)
			select {
			case <-time.After(backoff):
			case <-parent.Done():
				return stored, parent.Err()
			}
		}

		// Each attempt gets its own root --timeout budget.
		ctx, cancel := boundCtx(parent, flags)
		n, err := fetchLetter(ctx, client, db, letter, full, maxPages)
		cancel()
		if n > stored {
			// Upserts are keyed by document URL, so the best attempt's count is
			// the accurate figure for what this bucket contributed.
			stored = n
		}
		if err == nil {
			return stored, nil
		}
		lastErr = err
		// The overall crawl budget is spent — no later attempt can succeed.
		if parent.Err() != nil {
			return stored, err
		}
	}
	return stored, lastErr
}

// fetchLetter performs one attempt at a letter bucket.
func fetchLetter(ctx context.Context, client *extron.Client, db *store.Store, letter string, full bool, maxPages int) (int, error) {
	docs, refs, err := client.FetchIndex(ctx, letter)
	if err != nil {
		return 0, fmt.Errorf("letter %s: %w", letter, err)
	}
	stored, err := upsertDocs(db, docs)
	if err != nil {
		return stored, fmt.Errorf("letter %s: %w", letter, err)
	}
	if full && len(refs) > 0 {
		paged, err := syncCategoryPages(ctx, client, db, letter, refs, maxPages)
		stored += paged
		if err != nil {
			return stored, fmt.Errorf("letter %s (full): %w", letter, err)
		}
	}
	return stored, nil
}

func failedLetters(failures []letterFailure) []string {
	out := make([]string, 0, len(failures))
	for _, f := range failures {
		out = append(out, f.Letter)
	}
	return out
}

func fullOpt(full bool) string {
	if full {
		return " (full)"
	}
	return ""
}

func parseLettersCSV(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return extron.DefaultLetters
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToUpper(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func upsertDocs(db *store.Store, docs []extron.Doc) (int, error) {
	n := 0
	for _, d := range docs {
		data, err := json.Marshal(d)
		if err != nil {
			return n, err
		}
		if err := db.Upsert(catalogResource, d.URL, data); err != nil {
			return n, fmt.Errorf("storing %s: %w", d.Title, err)
		}
		n++
	}
	return n, nil
}

// syncCategoryPages follows each category's pagination for one letter.
func syncCategoryPages(ctx context.Context, client *extron.Client, db *store.Store, letter string, refs map[string]extron.PageRef, maxPages int) (int, error) {
	total := 0
	for _, ref := range refs {
		page := ref.Page
		pagesSeen := 0
		for {
			if maxPages > 0 && pagesSeen >= maxPages {
				break
			}
			docs, hasNext, err := client.FetchCategoryPage(ctx, letter, ref)
			if err != nil {
				if strings.Contains(err.Error(), "no literature rows") {
					break
				}
				return total, err
			}
			n, err := upsertDocs(db, docs)
			if err != nil {
				return total, err
			}
			total += n
			pagesSeen++
			if !hasNext {
				break
			}
			ref.Page = page + pagesSeen
			page = ref.Page
		}
	}
	return total, nil
}

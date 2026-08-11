// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored catalog sync: fetches the Extron literature index pages and
// stores the parsed catalog in the local resources table.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source live

func newNovelCatalogSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var lettersCSV string
	var full bool
	var maxPages int

	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Fetch the Extron literature catalog into the local store",
		Long:    "Fetch the Extron literature catalog into the local store. By default fetches the first alphabetical index page per letter bucket (0-9, A-Z); --full follows each category's pagination for a complete catalog. Run this before relying on local search or catalog commands.",
		Example: "  extron-pp-cli catalog sync\n  extron-pp-cli catalog sync --full",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync catalog")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = resolveCatalogDB(flags, dbPath)
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			letters := parseLettersCSV(lettersCSV)
			if cliutil.IsDogfoodEnv() && len(letters) > 1 {
				letters = letters[:1]
			}

			client := extron.New()
			total := 0
			letterCounts := make(map[string]int, len(letters))
			for _, letter := range letters {
				docs, refs, err := client.FetchIndex(ctx, letter)
				if err != nil {
					return fmt.Errorf("letter %s: %w", letter, err)
				}
				n, err := upsertDocs(db, docs)
				if err != nil {
					return fmt.Errorf("letter %s: %w", letter, err)
				}
				total += n
				letterCounts[letter] += n

				if full && len(refs) > 0 {
					paged, err := syncCategoryPages(ctx, client, db, letter, refs, maxPages)
					if err != nil {
						return fmt.Errorf("letter %s (full): %w", letter, err)
					}
					total += paged
					letterCounts[letter] += paged
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "catalog: letter %s: %d docs (running total %d)\n", letter, letterCounts[letter], total)
			}

			cursor := "partial"
			if full && maxPages <= 0 {
				cursor = "full"
			}
			if err := db.SaveSyncState(catalogResource, cursor, total); err != nil {
				return fmt.Errorf("recording sync state: %w", err)
			}

			summary := struct {
				LettersFetched int            `json:"letters_fetched"`
				Docs           int            `json:"docs"`
				Full           bool           `json:"full"`
				Database       string         `json:"database"`
				PerLetter      map[string]int `json:"per_letter,omitempty"`
			}{
				LettersFetched: len(letters),
				Docs:           total,
				Full:           full,
				Database:       dbPath,
				PerLetter:      letterCounts,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d docs from %d letter bucket(s)%s into %s\n",
				total, len(letters), fullOpt(full), dbPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&lettersCSV, "letters", "", "letter buckets to fetch as a CSV list (default: 0-9,A-Z)")
	cmd.Flags().BoolVar(&full, "full", false, "follow per-category pagination for a complete catalog")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "maximum category pages per letter in --full mode (0 = unlimited)")
	return cmd
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

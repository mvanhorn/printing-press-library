// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: pull attachment full text into the local cache.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/productivity/zotero-research-library/internal/store"
	"github.com/spf13/cobra"
)

// fulltextCursorResource keys the incremental watermark in sync_state.
const fulltextCursorResource = "fulltext-content"

func newNovelCacheIndexFulltextCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "index-fulltext",
		Short: "Fetch attachment full text into the cache so 'ground --fulltext' works offline",
		Long: `Pulls the server-extracted attachment text (Zotero indexes PDFs server-side)
into the local cache: GET /fulltext lists which attachments changed since the
last run, then each one's text is fetched and stored. Incremental — re-runs
only fetch what changed. Expect one API call per changed attachment.`,
		Example: "  zotero-research-library-pp-cli cache index-fulltext",
		// pp:happy-args bounds the dogfood probe: unbounded first-run indexing
		// is one API call per attachment and exceeds any probe timeout.
		Annotations: map[string]string{"pp:happy-args": "--limit=10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			dbPath := defaultDBPath("zotero-research-library-pp-cli")
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				return fmt.Errorf("local cache is empty — run 'zotero-research-library-pp-cli sync --full' first")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening cache: %w", err)
			}
			defer db.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			since := db.GetSyncCursor(fulltextCursorResource)
			if since == "" {
				since = "0"
			}
			changedRaw, err := c.Get(ctx, "/fulltext", map[string]string{"since": since})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var changed map[string]json.Number
			if err := json.Unmarshal(changedRaw, &changed); err != nil {
				return fmt.Errorf("decoding /fulltext response: %w", err)
			}
			if len(changed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Full text is up to date — nothing changed since the last run.")
				return nil
			}

			keys := make([]string, 0, len(changed))
			maxVersion := int64(0)
			for k, v := range changed {
				keys = append(keys, k)
				if n, err := v.Int64(); err == nil && n > maxVersion {
					maxVersion = n
				}
			}
			sort.Strings(keys)
			truncated := false
			if flagLimit > 0 && len(keys) > flagLimit {
				truncated = true
				fmt.Fprintf(cmd.ErrOrStderr(), "note: indexing %d of %d changed attachments (--limit); re-run to continue\n", flagLimit, len(keys))
				keys = keys[:flagLimit]
			}

			indexed, missing, failed := 0, 0, 0
			for i, key := range keys {
				body, err := c.Get(ctx, "/items/"+key+"/fulltext", nil)
				if err != nil {
					// 404 = attachment has no extracted text; anything else
					// is a real failure worth counting separately.
					if apiErrStatus(err) == 404 {
						missing++
						continue
					}
					failed++
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: fulltext for %s: %v\n", key, err)
					continue
				}
				var ft struct {
					Content      string `json:"content"`
					IndexedChars int    `json:"indexedChars"`
					TotalChars   int    `json:"totalChars"`
					IndexedPages int    `json:"indexedPages"`
					TotalPages   int    `json:"totalPages"`
				}
				if err := json.Unmarshal(body, &ft); err != nil || ft.Content == "" {
					missing++
					continue
				}
				indexedChars := ft.IndexedChars
				totalChars := ft.TotalChars
				if indexedChars == 0 && ft.IndexedPages > 0 {
					indexedChars = ft.IndexedPages
					totalChars = ft.TotalPages
				}
				// items.id is the item key; the update trigger keeps
				// items_fts in step automatically.
				if _, err := db.DB().ExecContext(ctx,
					`UPDATE "items" SET "content" = ?, "indexed_chars" = ?, "total_chars" = ? WHERE "id" = ?`,
					ft.Content, indexedChars, totalChars, key); err != nil {
					failed++
					continue
				}
				indexed++
				if (i+1)%25 == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %d/%d attachments processed\n", i+1, len(keys))
				}
			}

			// Advance the incremental cursor only when every changed
			// attachment was actually processed — a truncated (--limit) or
			// partially-failed run must not skip the remainder forever.
			if failed == 0 && !truncated {
				_ = db.SaveSyncCursor(fulltextCursorResource, strconv.FormatInt(maxVersion, 10))
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"changed": len(changed),
					"indexed": indexed,
					"missing": missing,
					"failed":  failed,
					"cursor":  maxVersion,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Indexed full text for %d attachments (%d had none, %d failed) — 'ground --fulltext' is ready.\n", indexed, missing, failed)
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Cap attachments fetched this run (0 = all changed)")
	return cmd
}

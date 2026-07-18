// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: archive a Substack publication into the local SQLite corpus.
// Hand-implemented (proof-of-concept, Tier-0 keyless). generate --force
// preserves implemented bodies.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/store"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/substack"
)

// tagSourceHost annotates a stored post with the publication host it was
// archived from (_pp_source_host), the join key digest/author-compare use to
// group posts by publication. Best-effort: on any JSON error the item is
// stored unchanged.
func tagSourceHost(raw json.RawMessage, host string) json.RawMessage {
	if host == "" {
		return raw
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	hb, err := json.Marshal(host)
	if err != nil {
		return raw
	}
	m["_pp_source_host"] = hb
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// pp:data-source live
func newNovelArchiveCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sort string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "archive <publication>",
		Short: "Archive a whole Substack publication into a local SQLite mirror you can read, search",
		Long: "Archive a Substack publication's posts into the local SQLite corpus for offline read, " +
			"search, and SQL. Accepts a handle (astralcodexten), a bare host (blog.bytebytego.com), or a " +
			"full URL. Keyless (Tier 0): archives post metadata + previews for any public publication; the " +
			"full body of paid posts requires your own session (see the read command).",
		Example: "  substack-reader-pp-cli archive astralcodexten --limit 200",
		// No mcp:read-only annotation: archive WRITES the local SQLite corpus
		// (db.Upsert + db.SaveSyncState below), so per the MCP readOnlyHint
		// contract ("does not modify its environment") it is not read-only. It
		// inherits the conservative default, matching the other store-writing
		// command (sync). read/digest/author-compare stay read-only — they only
		// read Substack or the local corpus, never write it.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would archive publication posts into the local corpus")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a <publication> (handle, host, or URL) is required"))
			}
			base := substack.ResolveHost(args[0])
			if base == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("could not resolve publication %q", args[0]))
			}
			sourceHost := strings.TrimPrefix(base, "https://")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("substack-reader-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			const pageSize = 50
			if cliutil.IsDogfoodEnv() && limit > pageSize {
				limit = pageSize
			}

			sc := substack.NewClient()
			archived, skipped := 0, 0
			for offset := 0; archived < limit; offset += pageSize {
				n := pageSize
				if remaining := limit - archived; remaining < n {
					n = remaining
				}
				items, err := sc.FetchArchivePage(ctx, base, sort, n, offset)
				if err != nil {
					return fmt.Errorf("fetching archive: %w", err)
				}
				if len(items) == 0 {
					break
				}
				for _, raw := range items {
					s, _ := substack.Summarize(raw)
					id := s.ID.String()
					if id == "" {
						skipped++
						continue
					}
					// Tag each post with the publication host we archived it
					// from, so digest/author-compare can attribute posts to a
					// publication without relying on canonical_url parsing. The
					// key is stable across runs because the same user argument
					// resolves to the same host (playbook B5: store the
					// user-facing identifier alongside the record).
					raw = tagSourceHost(raw, sourceHost)
					if err := db.Upsert("posts", id, raw); err != nil {
						return fmt.Errorf("storing post %s: %w", id, err)
					}
					archived++
				}
				if len(items) < n {
					break
				}
			}

			// Stamp sync state so the framework's local-first commands (search,
			// digest) trust the corpus. The generated store ships SaveSyncState
			// but nothing on a hand-built populate path calls it — the playbook's
			// meta-pattern 8 ("sync_state never stamped on a novel populate path").
			if archived > 0 {
				_ = db.SaveSyncState("posts", "", archived)
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"publication": base,
					"archived":    archived,
					"skipped":     skipped,
					"db":          dbPath,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived %d posts from %s into %s\n", archived, base, dbPath)
			if skipped > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  (%d items skipped: missing id)\n", skipped)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Search them offline: substack-reader-pp-cli search \"<term>\"\n")
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum posts to archive")
	cmd.Flags().StringVar(&sort, "sort", "new", "archive sort order: new | top | community")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path (default: standard location)")
	return cmd
}

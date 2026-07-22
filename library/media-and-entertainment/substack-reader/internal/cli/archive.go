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

// archiveWalk pages through a publication archive until `limit` posts have been
// stored or the archive is exhausted. fetch is FetchArchivePage bound to a
// (base, sort); storeItem persists one raw item and reports whether it counted
// toward the limit (false = skipped, e.g. missing id).
//
// Two pagination rules, both verified against the live API (dougshapiro
// .substack.com, 2026-07-19):
//   - A SHORT page is not an end-of-archive signal. The endpoint returned 23
//     items for limit=50 at offset=0 even though ~70 older posts existed;
//     only an EMPTY page terminates the walk.
//   - offset advances by the number of items actually returned, not by the
//     requested page size, because the server's offset indexes the visible
//     post list. Advancing by the requested size would skip the posts behind
//     a short page.
func archiveWalk(limit, pageSize int, fetch func(n, offset int) ([]json.RawMessage, error), storeItem func(raw json.RawMessage) (bool, error)) (archived, skipped int, err error) {
	offset := 0
	for archived < limit {
		n := pageSize
		if remaining := limit - archived; remaining < n {
			n = remaining
		}
		items, err := fetch(n, offset)
		if err != nil {
			return archived, skipped, err
		}
		if len(items) == 0 {
			break
		}
		for _, raw := range items {
			if archived >= limit {
				break
			}
			stored, err := storeItem(raw)
			if err != nil {
				return archived, skipped, err
			}
			if stored {
				archived++
			} else {
				skipped++
			}
		}
		offset += len(items)
	}
	return archived, skipped, nil
}

// bodyTextToStore decides what body text a post's stored record keeps. A body
// already in the corpus always wins — Upsert replaces the whole stored JSON,
// so dropping it would silently degrade an FTS-indexed corpus back to titles
// and previews (exactly what a --metadata-only re-run over a full archive
// would otherwise do). Only when the corpus holds no body does the mode
// matter: metadata-only stores none, the default asks fetch for one.
func bodyTextToStore(db *store.Store, id string, metadataOnly bool, fetch func() (string, error)) (string, error) {
	if body := storedBodyText(db, id); body != "" {
		return body, nil
	}
	if metadataOnly {
		return "", nil
	}
	return fetch()
}

// storedBodyText returns the body text a previous archive run stored for this
// post id, so re-runs never refetch bodies the corpus already holds.
func storedBodyText(db *store.Store, id string) string {
	prev, err := db.Get("posts", id)
	if err != nil || len(prev) == 0 {
		return ""
	}
	var m struct {
		Body string `json:"_pp_body_text"`
	}
	if json.Unmarshal(prev, &m) != nil {
		return ""
	}
	return m.Body
}

// attachBodyText annotates a stored post with its rendered plain-text body
// (_pp_body_text), which the store's FTS indexer picks up like any other
// string field. Best-effort like tagSourceHost: on any JSON error the item is
// stored unchanged.
func attachBodyText(raw json.RawMessage, body string) json.RawMessage {
	if body == "" {
		return raw
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	bb, err := json.Marshal(body)
	if err != nil {
		return raw
	}
	m["_pp_body_text"] = bb
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

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
	var metadataOnly bool

	cmd := &cobra.Command{
		Use:   "archive <publication>",
		Short: "Archive a whole Substack publication into a local SQLite mirror you can read, search",
		Long: "Archive a Substack publication's posts into the local SQLite corpus for offline read, " +
			"search, and SQL. Accepts a handle (astralcodexten), a bare host (blog.bytebytego.com), or a " +
			"full URL.\n\n" +
			"Walks the archive newest-first until --limit posts are stored or the archive is exhausted " +
			"(the endpoint serves short pages mid-archive; only an empty page ends the walk). By default " +
			"each new post's body is also fetched — one keyless request per post, rate-limited — and " +
			"indexed for full-text search: free posts index their full text, paid posts the public " +
			"preview (a full paid body requires your own session; see the read command). Bodies already " +
			"in the corpus are never refetched, so re-runs are cheap and incremental. --metadata-only " +
			"skips body fetching; search then covers titles and previews only.",
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

			// A full archive walk is one command but MANY requests (pages plus
			// one body fetch per new post), so the 60s default of --timeout
			// ("Request timeout") must not cap the whole walk — a ~100-post
			// first archive takes longer than that by design (rate-limited).
			// Each request stays bounded by the client's own per-request
			// timeout; an EXPLICIT --timeout opts into a whole-walk deadline.
			ctx := cmd.Context()
			cancel := func() {}
			if cmd.Flags().Changed("timeout") {
				ctx, cancel = boundCtx(cmd.Context(), flags)
			}
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
			bodies, bodyErrs := 0, 0
			archived, skipped, err := archiveWalk(limit, pageSize,
				func(n, offset int) ([]json.RawMessage, error) {
					items, err := sc.FetchArchivePage(ctx, base, sort, n, offset)
					if err != nil {
						return nil, fmt.Errorf("fetching archive: %w", err)
					}
					return items, nil
				},
				func(raw json.RawMessage) (bool, error) {
					s, _ := substack.Summarize(raw)
					id := s.ID.String()
					if id == "" {
						return false, nil
					}
					// Tag each post with the publication host we archived it
					// from, so digest/author-compare can attribute posts to a
					// publication without relying on canonical_url parsing. The
					// key is stable across runs because the same user argument
					// resolves to the same host (playbook B5: store the
					// user-facing identifier alongside the record).
					raw = tagSourceHost(raw, sourceHost)
					body, bodyErr := bodyTextToStore(db, id, metadataOnly, func() (string, error) {
						if s.Slug == "" {
							return "", nil
						}
						post, ferr := sc.FetchPost(ctx, base, s.Slug)
						if ferr != nil {
							// A canceled/expired context means every further
							// fetch fails too — abort instead of spraying
							// one warning per remaining post.
							if ctx.Err() != nil {
								return "", fmt.Errorf("fetching body for %s: %w", s.Slug, ferr)
							}
							bodyErrs++
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetching body for %s: %v\n", s.Slug, ferr)
							return "", nil
						}
						meta, perr := substack.ParsePostMeta(post)
						if perr != nil {
							bodyErrs++
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: parsing body for %s: %v\n", s.Slug, perr)
							return "", nil
						}
						b := substack.HTMLToText(meta.BodyHTML)
						if b != "" {
							bodies++
						}
						return b, nil
					})
					if bodyErr != nil {
						return false, bodyErr
					}
					raw = attachBodyText(raw, body)
					if err := db.Upsert("posts", id, raw); err != nil {
						return false, fmt.Errorf("storing post %s: %w", id, err)
					}
					return true, nil
				})
			if err != nil {
				return err
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
					"publication":    base,
					"archived":       archived,
					"skipped":        skipped,
					"bodies_fetched": bodies,
					"body_errors":    bodyErrs,
					"db":             dbPath,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived %d posts from %s into %s\n", archived, base, dbPath)
			if !metadataOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "  (%d post bodies fetched for full-text search", bodies)
				if bodyErrs > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), ", %d body fetches failed", bodyErrs)
				}
				fmt.Fprintln(cmd.OutOrStdout(), ")")
			}
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
	cmd.Flags().BoolVar(&metadataOnly, "metadata-only", false, "skip fetching post bodies (faster; search covers titles and previews only)")
	return cmd
}

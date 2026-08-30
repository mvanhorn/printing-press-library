// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: offline research-grounding search.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/zotero-research-library/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if !hasSubcommandNamed(root, "ground") {
			root.AddCommand(newNovelGroundCmd(flags))
		}
	})
}

type groundHit struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Creators    string `json:"creators,omitempty"`
	Date        string `json:"date,omitempty"`
	Publication string `json:"publication,omitempty"`
	DOI         string `json:"doi,omitempty"`
	ItemType    string `json:"item_type,omitempty"`
	Snippet     string `json:"snippet,omitempty"`
	MatchedIn   string `json:"matched_in"`
	// AttachmentKey/Title identify the child attachment or note the match
	// came from when the hit was resolved to its parent paper (which then
	// occupies the top-level key/title fields).
	AttachmentKey   string  `json:"attachment_key,omitempty"`
	AttachmentTitle string  `json:"attachment_title,omitempty"`
	rank            float64 `json:"-"`
}

func newNovelGroundCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagFulltext bool
	var flagCollection string
	var flagLive bool

	cmd := &cobra.Command{
		Use:   "ground <query>...",
		Short: "Ranked full-text search over your synced library — offline, with snippets and citation-ready metadata",
		Long: `Answers "what does my library say about X?" from the local cache: ranked
FTS5 search over titles, abstracts, creators, and tags, with snippets and
citation-ready metadata. No network, no rate limit.

--fulltext matches indexed attachment text (populate it once with
'cache index-fulltext') and resolves each hit to its parent paper. --live
bypasses the cache and runs the query server-side via qmode=everything.`,
		Example: `  zotero-research-library-pp-cli ground "hamstring injury prevention" --limit 10 --agent
  zotero-research-library-pp-cli ground "nordic hamstring" --fulltext --agent
  zotero-research-library-pp-cli ground "acl injury" --collection HAMSTRING --limit 5`,
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<query>=hamstring"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return fmt.Errorf("query must not be empty")
			}
			if flagLimit <= 0 {
				flagLimit = 10
			}

			if flagLive {
				return groundLive(cmd, flags, query, flagLimit)
			}

			ctx := cmd.Context()
			db, err := requireLocalCache(ctx, "zotero-research-library-pp-cli")
			if err != nil {
				return err
			}
			defer db.Close()

			collectionKey := ""
			if flagCollection != "" {
				collectionKey, err = resolveCollectionKey(ctx, db, flagCollection)
				if err != nil {
					return err
				}
			}

			hits, err := groundSearch(ctx, db, query, flagLimit, flagFulltext, collectionKey)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"query":   query,
					"source":  "local",
					"count":   len(hits),
					"results": hits,
				}, flags)
			}
			out := cmd.OutOrStdout()
			if len(hits) == 0 {
				fmt.Fprintf(out, "No matches for %q in the local cache.\n", query)
				if flagFulltext {
					fmt.Fprintln(out, "Attachment text must be indexed first: run 'cache index-fulltext'.")
				} else {
					fmt.Fprintln(out, "If the cache may be stale, run 'sync' — or retry with --live for a server-side search.")
				}
				return nil
			}
			for i, h := range hits {
				fmt.Fprintf(out, "%2d. %s", i+1, h.Title)
				if h.Creators != "" {
					fmt.Fprintf(out, " — %s", h.Creators)
				}
				if h.Date != "" {
					fmt.Fprintf(out, " (%s)", h.Date)
				}
				fmt.Fprintf(out, "\n    key=%s", h.Key)
				if h.DOI != "" {
					fmt.Fprintf(out, "  doi=%s", h.DOI)
				}
				if h.Publication != "" {
					fmt.Fprintf(out, "  %s", h.Publication)
				}
				fmt.Fprintln(out)
				if h.Snippet != "" {
					fmt.Fprintf(out, "    %s\n", h.Snippet)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum results to return")
	cmd.Flags().BoolVar(&flagFulltext, "fulltext", false, "Match indexed attachment full text (see 'cache index-fulltext'); resolve hits to parent papers")
	cmd.Flags().StringVar(&flagCollection, "collection", "", "Restrict to a collection (key or name)")
	cmd.Flags().BoolVar(&flagLive, "live", false, "Run the query server-side (qmode=everything) instead of the local cache")
	return cmd
}

// groundSearch runs the ranked local search. The primary source is
// resources_fts, whose content column holds every searchable leaf value of
// the item JSON (title, abstract, creators, tags). A second pass over
// items_fts covers attachment full text populated by 'cache index-fulltext';
// in --fulltext mode that pass is the only source.
func groundSearch(ctx context.Context, db *store.Store, query string, limit int, fulltextOnly bool, collectionKey string) ([]groundHit, error) {
	match := ftsQuote(query)

	collectionFilter := ""
	var collectionArgs []any
	if collectionKey != "" {
		collectionFilter = ` AND EXISTS (SELECT 1 FROM json_each(COALESCE(json_extract(i."data", '$.data.collections'), '[]')) je WHERE je.value = ?)`
		collectionArgs = append(collectionArgs, collectionKey)
	}

	seen := map[string]bool{}
	var hits []groundHit

	if !fulltextOnly {
		// resources_fts carries id/resource_type as stored columns with
		// hashed rowids — join on the id column, never on rowid.
		primarySQL := `SELECT i."data", snippet("resources_fts", 2, '[', ']', ' … ', 12), bm25("resources_fts")
			FROM "resources_fts"
			JOIN "items" i ON i."id" = "resources_fts".id
			WHERE "resources_fts".resource_type = 'items' AND "resources_fts" MATCH ?` + collectionFilter + `
			ORDER BY bm25("resources_fts") LIMIT ?`
		args := append([]any{"content: (" + match + ")"}, collectionArgs...)
		args = append(args, limit*2)
		rows, err := db.DB().QueryContext(ctx, primarySQL, args...)
		if err != nil {
			return nil, fmt.Errorf("local search: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var raw []byte
			var snip string
			var rank float64
			if err := rows.Scan(&raw, &snip, &rank); err != nil {
				return nil, err
			}
			h, ok := buildGroundHit(ctx, db, raw, snip, rank, "metadata", false)
			if !ok || seen[h.Key] {
				continue
			}
			seen[h.Key] = true
			hits = append(hits, h)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Attachment full text lives in the items.content column (populated by
	// 'cache index-fulltext') and is indexed by items_fts. No SQL collection
	// filter here: attachment rows carry no collections array — membership is
	// checked against the resolved parent below.
	fulltextSQL := `SELECT i."data", snippet("items_fts", 1, '[', ']', ' … ', 12), bm25("items_fts")
		FROM "items_fts" JOIN "items" i ON i.rowid = "items_fts".rowid
		WHERE "items_fts" MATCH ?
		ORDER BY bm25("items_fts") LIMIT ?`
	frows, err := db.DB().QueryContext(ctx, fulltextSQL, "content: ("+match+")", limit*4)
	if err != nil {
		return nil, fmt.Errorf("fulltext search: %w", err)
	}
	defer frows.Close()
	for frows.Next() {
		var raw []byte
		var snip string
		var rank float64
		if err := frows.Scan(&raw, &snip, &rank); err != nil {
			return nil, err
		}
		// Full-text matches rank slightly behind exact metadata matches
		// unless --fulltext asked for them specifically.
		adj := rank
		if !fulltextOnly {
			adj += 0.5
		}
		h, ok := buildGroundHit(ctx, db, raw, snip, adj, "attachment fulltext", true)
		if !ok || seen[h.Key] {
			continue
		}
		if collectionKey != "" {
			in, err := itemInCollection(ctx, db, h.Key, collectionKey)
			if err != nil {
				return nil, err
			}
			if !in {
				continue
			}
		}
		seen[h.Key] = true
		hits = append(hits, h)
	}
	if err := frows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(hits, func(a, b int) bool { return hits[a].rank < hits[b].rank })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// buildGroundHit decodes a stored item row into a result. Attachment and note
// rows resolve to their parent paper when one exists, keeping the attachment's
// snippet while citation metadata comes from the parent.
func buildGroundHit(ctx context.Context, db *store.Store, raw []byte, snippet string, rank float64, matchedIn string, resolveParent bool) (groundHit, bool) {
	d, err := decodeItemData(raw)
	if err != nil || d.Key == "" {
		return groundHit{}, false
	}
	h := groundHit{
		Key:         d.Key,
		Title:       d.Title,
		Creators:    creatorSummary(d.Creators),
		Date:        d.Date,
		Publication: d.PublicationTitle,
		DOI:         d.DOI,
		ItemType:    d.ItemType,
		Snippet:     strings.TrimSpace(snippet),
		MatchedIn:   matchedIn,
		rank:        rank,
	}
	if (d.ItemType == "attachment" || d.ItemType == "note") && d.ParentItem != "" {
		var parentRaw []byte
		row := db.DB().QueryRowContext(ctx, `SELECT "data" FROM "items" WHERE "id" = ?`, d.ParentItem)
		if err := row.Scan(&parentRaw); err == nil {
			if p, perr := decodeItemData(parentRaw); perr == nil && p.Key != "" {
				h.AttachmentKey = d.Key
				h.AttachmentTitle = d.Title
				h.Key = p.Key
				h.Title = p.Title
				h.Creators = creatorSummary(p.Creators)
				h.Date = p.Date
				h.Publication = p.PublicationTitle
				h.DOI = p.DOI
				h.ItemType = p.ItemType
			}
		} else if resolveParent {
			h.MatchedIn = matchedIn + " (parent not in cache)"
		}
	}
	// Attachments matched only on filename noise aren't research results.
	if h.Title == "" {
		return groundHit{}, false
	}
	return h, true
}

// groundLive runs the query server-side via the generated items endpoint's
// full search grammar (qmode=everything covers server-side full text).
func groundLive(cmd *cobra.Command, flags *rootFlags, query string, limit int) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	params := map[string]string{
		"q":     query,
		"qmode": "everything",
		"limit": strconv.Itoa(limit),
	}
	data, err := c.Get(cmd.Context(), "/items", params)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"query":   query,
			"source":  "live",
			"results": data,
		}, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

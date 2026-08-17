// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: aggregate new uploads from every followed designer across
// sources, reporting only items not already seen by a prior `feed` run.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

type feedItem struct {
	Source     string `json:"source"`
	ModelID    string `json:"model_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Designer   string `json:"designer"`
	AliasGroup string `json:"alias_group,omitempty"`
}

// pp:data-source computed
func newNovelFeedCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "feed",
		Short:       "Follow a designer across all the sites they post to and see new uploads from any of them in one feed.",
		Example:     "  printgoat-pp-cli feed --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			rows, err := s.DB().QueryContext(ctx, `SELECT alias_group, source, handle FROM printgoat_designer_links ORDER BY alias_group, source, handle`)
			if err != nil {
				return fmt.Errorf("reading followed designers: %w", err)
			}
			var links []followedDesigner
			for rows.Next() {
				var d followedDesigner
				var alias sql.NullString
				if serr := rows.Scan(&alias, &d.Source, &d.Handle); serr != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning followed designers: %w", serr)
				}
				d.AliasGroup = alias.String
				links = append(links, d)
			}
			closeErr := rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading followed designers: %w", err)
			}
			if closeErr != nil {
				return fmt.Errorf("reading followed designers: %w", closeErr)
			}

			if len(links) == 0 {
				out := map[string]any{
					"followed_designers": 0,
					"new_items":          []feedItem{},
					"new_count":          0,
					"message":            "not following any designers yet; use 'follow designer add <source> <handle>' first",
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var newItems []feedItem
			var notes []map[string]any
			now := time.Now().UTC().Format(time.RFC3339)

			for _, l := range links {
				switch l.Source {
				case "thingiverse":
					path := "https://api.thingiverse.com/users/" + url.PathEscape(l.Handle) + "/things"
					data, ferr := c.Get(ctx, path, nil)
					if ferr != nil {
						notes = append(notes, map[string]any{"source": l.Source, "handle": l.Handle, "error": ferr.Error()})
						continue
					}
					var items []map[string]any
					if json.Unmarshal(data, &items) != nil {
						notes = append(notes, map[string]any{"source": l.Source, "handle": l.Handle, "error": "unexpected response shape"})
						continue
					}
					for _, it := range items {
						id := getString(it, "id")
						if id == "" {
							id = fmt.Sprintf("%v", it["id"])
						}
						if id == "" || id == "<nil>" {
							continue
						}
						res, ierr := s.DB().ExecContext(ctx,
							`INSERT OR IGNORE INTO printgoat_feed_seen (source, model_id, seen_at) VALUES (?, ?, ?)`,
							l.Source, id, now,
						)
						if ierr != nil {
							continue
						}
						if n, _ := res.RowsAffected(); n == 0 {
							continue // already seen on a prior feed run
						}
						newItems = append(newItems, feedItem{
							Source: l.Source, ModelID: id, Name: getString(it, "name"),
							URL: getString(it, "public_url"), Designer: l.Handle, AliasGroup: l.AliasGroup,
						})
					}
				default:
					// Printables and Cults3D have no confirmed "recent uploads
					// by user" query in this CLI's researched API surface.
					notes = append(notes, map[string]any{"source": l.Source, "handle": l.Handle, "note": "not yet supported for this source"})
				}
			}

			out := map[string]any{
				"followed_designers": len(links),
				"new_items":          newItems,
				"new_count":          len(newItems),
			}
			if len(notes) > 0 {
				out["notes"] = notes
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type engagementEntry struct {
	MemberID   string `json:"member_id,omitempty"`
	Name       string `json:"name"`
	Headline   string `json:"headline,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
	Reactions  int    `json:"reactions"`
	Comments   int    `json:"comments"`
	Connected  bool   `json:"connected"`
}

type engagementView struct {
	Entries      []engagementEntry `json:"entries"`
	ScannedRows  int               `json:"scanned_rows"`
	NotConnected int               `json:"not_connected"`
	Note         string            `json:"note,omitempty"`
}

func newNovelEngagementCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath        string
		limit         int
		onlyStrangers bool
	)
	cmd := &cobra.Command{
		Use:   "engagement",
		Short: "Who engaged with your posts, flagged by whether they are already a connection",
		Long: strings.Trim(`
Cross-references synced post reactions and comments against your connection
list, so warm engagers who are not yet connections surface as outreach targets.

Requires post engagement in the local mirror: run
'unipile-pp-cli sync --resources posts,reactions,comments' first.

Use this command to turn engagement into an invitation list. Do NOT use it for
raw post metrics; use 'posts reactions'.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli engagement --agent --limit 20
  unipile-pp-cli engagement --only-strangers
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--limit=5",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "engagement")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := engagementView{Entries: make([]engagementEntry, 0)}
			db, ok, err := novelStore(cmd, flags, dbPath, view)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			relations, err := loadRelations(ctx, db)
			if err != nil {
				return err
			}
			byName := make(map[string]relationRow, len(relations))
			for _, r := range relations {
				if r.Name != "" {
					byName[strings.ToLower(r.Name)] = r
				}
			}

			agg := make(map[string]*engagementEntry)
			for _, src := range []struct {
				resource string
				field    string
			}{
				{"reactions", "reactions"},
				{"posts-reactions", "reactions"},
				{"comments", "comments"},
				{"posts-comments", "comments"},
			} {
				rows, qerr := db.QueryContext(ctx, `
					SELECT COALESCE(json_extract(data,'$.author.name'), json_extract(data,'$.author_name'), json_extract(data,'$.name'), ''),
					       COALESCE(json_extract(data,'$.author.id'),   json_extract(data,'$.author_id'),   json_extract(data,'$.id'),   '')
					FROM resources WHERE resource_type = ?`, src.resource)
				if qerr != nil {
					continue
				}
				for rows.Next() {
					var name, id sql.NullString
					if serr := rows.Scan(&name, &id); serr != nil {
						_ = rows.Close()
						return fmt.Errorf("scanning %s: %w", src.resource, serr)
					}
					view.ScannedRows++
					key := strings.ToLower(strings.TrimSpace(name.String))
					if key == "" {
						key = id.String
					}
					if key == "" {
						continue
					}
					e, hit := agg[key]
					if !hit {
						e = &engagementEntry{Name: strings.TrimSpace(name.String), MemberID: id.String}
						if r, known := byName[key]; known {
							e.Connected = true
							e.Headline = r.Headline
							e.ProfileURL = r.ProfileURL
							if e.MemberID == "" {
								e.MemberID = r.MemberID
							}
						}
						agg[key] = e
					}
					if src.field == "reactions" {
						e.Reactions++
					} else {
						e.Comments++
					}
				}
				if rerr := rows.Err(); rerr != nil {
					_ = rows.Close()
					return fmt.Errorf("iterating %s: %w", src.resource, rerr)
				}
				if cerr := rows.Close(); cerr != nil {
					return fmt.Errorf("closing %s rows: %w", src.resource, cerr)
				}
			}

			for _, e := range agg {
				if !e.Connected {
					view.NotConnected++
				}
				if onlyStrangers && e.Connected {
					continue
				}
				view.Entries = append(view.Entries, *e)
			}
			sort.Slice(view.Entries, func(i, j int) bool {
				li := view.Entries[i].Reactions + view.Entries[i].Comments
				lj := view.Entries[j].Reactions + view.Entries[j].Comments
				if li != lj {
					return li > lj
				}
				return view.Entries[i].Name < view.Entries[j].Name
			})
			if limit > 0 && len(view.Entries) > limit {
				view.Entries = view.Entries[:limit]
			}
			if view.ScannedRows == 0 {
				view.Note = "no post engagement in the local mirror; run 'unipile-pp-cli sync --resources posts,reactions,comments' after publishing a post"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Entries) == 0 {
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No post engagement matched the current filters.")
				}
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tREACTIONS\tCOMMENTS\tCONNECTED\tHEADLINE")
			for _, e := range view.Entries {
				connected := "no"
				if e.Connected {
					connected = "yes"
				}
				fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\n", truncate(e.Name, 26), e.Reactions, e.Comments, connected, truncate(e.Headline, 40))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum engagers to return")
	cmd.Flags().BoolVar(&onlyStrangers, "only-strangers", false, "only show engagers who are not already connections")
	return cmd
}

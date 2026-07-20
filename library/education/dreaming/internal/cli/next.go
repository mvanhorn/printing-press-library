// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// next: roadmap-aware next-video picker. Joins your derived level (cumulative
// hours) to the cached catalog minus the watched playlist, ranked by the
// fine-grained 1-100 difficulty. Hand-built novel feature. Works offline.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type nextVideo struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Level      string `json:"level"`
	Difficulty int    `json:"difficulty"`
	Guide      string `json:"guide"`
	Dialect    string `json:"dialect"`
	Duration   int    `json:"duration_seconds"`
}

func newNovelNextCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagLevel string
	var flagGuide string
	var flagDialect string
	var includeWatched bool

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Get the next unwatched videos tuned to where you are on the fluency roadmap, sorted by fine-grained difficulty.",
		Long: "Pick the next videos to watch: unwatched catalog entries at the difficulty\n" +
			"band for your current roadmap level (derived from your cumulative hours),\n" +
			"sorted by the fine-grained 1-100 difficulty rating. Works offline against\n" +
			"the synced catalog. Override the band with --level.",
		Example: strings.Trim(`
  dreaming-pp-cli next --limit 5
  dreaming-pp-cli next --level intermediate --dialect rioplatense
  dreaming-pp-cli next --guide "Agustina" --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openDreamingStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			var count int
			_ = db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM videos`).Scan(&count)
			if count == 0 {
				return notFoundErr(fmt.Errorf("video catalog is empty — run 'dreaming-pp-cli sync' first"))
			}

			// Determine the level band.
			var bands []string
			if flagLevel != "" && flagLevel != "auto" {
				bands = []string{strings.ToLower(flagLevel)}
			} else {
				u, ok, err := ensureUser(cmd.Context(), flags, db)
				if err != nil {
					return err
				}
				if !ok {
					return notFoundErr(fmt.Errorf("no user stats cached — run 'dreaming-pp-cli sync' first or specify --level"))
				}
				bands = videoLevelBands(levelForHours(u.TotalHours()))
			}

			q := `SELECT id, title, level, COALESCE(difficulty,0), COALESCE(guide,''), COALESCE(dialect,''), COALESCE(duration,0)
				FROM videos WHERE level IN (` + placeholders(len(bands)) + `)`
			argv := make([]any, 0, len(bands)+3)
			for _, b := range bands {
				argv = append(argv, b)
			}
			if !includeWatched {
				q += ` AND id NOT IN (SELECT video_id FROM playlist)`
			}
			if flagGuide != "" {
				q += ` AND guide = ?`
				argv = append(argv, flagGuide)
			}
			if flagDialect != "" {
				q += ` AND dialect = ?`
				argv = append(argv, flagDialect)
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 10
			}
			// Unrated videos (NULL or 0 difficulty) sort last so rated picks lead.
			q += ` ORDER BY (COALESCE(difficulty,0) <= 0) ASC, COALESCE(difficulty,0) ASC, title ASC LIMIT ?`
			argv = append(argv, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return err
			}
			defer rows.Close()
			var out []nextVideo
			for rows.Next() {
				var v nextVideo
				if err := rows.Scan(&v.ID, &v.Title, &v.Level, &v.Difficulty, &v.Guide, &v.Dialect, &v.Duration); err != nil {
					return err
				}
				out = append(out, v)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No unwatched videos at level [%s] matched. Try --level or sync more of the catalog.\n", strings.Join(bands, ", "))
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DIFF\tLEVEL\tMIN\tGUIDE\tTITLE\tID")
			for _, v := range out {
				fmt.Fprintf(tw, "%d\t%s\t%d\t%s\t%s\t%s\n", v.Difficulty, v.Level, v.Duration/60, truncate(v.Guide, 16), truncate(v.Title, 40), v.ID)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum videos to return")
	cmd.Flags().StringVar(&flagLevel, "level", "auto", "Catalog level band: auto (from your hours) or superbeginner|beginner|intermediate|advanced")
	cmd.Flags().StringVar(&flagGuide, "guide", "", "Only videos by this guide/teacher")
	cmd.Flags().StringVar(&flagDialect, "dialect", "", "Only videos in this dialect/accent")
	cmd.Flags().BoolVar(&includeWatched, "include-watched", false, "Include videos already in your watched playlist")
	return cmd
}

func placeholders(n int) string {
	if n <= 0 {
		return "''"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

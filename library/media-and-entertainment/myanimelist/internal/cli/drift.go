// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type driftRow struct {
	Kind          string  `json:"kind"`
	ID            int     `json:"id"`
	Title         string  `json:"title,omitempty"`
	From          string  `json:"from"`
	To            string  `json:"to"`
	ScoreDelta    float64 `json:"score_delta"`
	MemberDelta   int     `json:"members_delta"`
	FavoriteDelta int     `json:"favorites_delta"`
	RankDelta     int     `json:"rank_delta"`
	ScoreNow      float64 `json:"score_now"`
	MembersNow    int     `json:"members_now"`
}

type driftView struct {
	Rows      []driftRow `json:"movers"`
	Since     string     `json:"since"`
	Snapshots int        `json:"snapshots_examined"`
	Note      string     `json:"note,omitempty"`
	Recorded  *driftRow  `json:"recorded,omitempty"`
}

// newNovelDriftCmd reports score/member/favorites/rank movement from the local
// snapshot table. MyAnimeList publishes no history anywhere, so this question
// is only answerable from snapshots this CLI recorded itself.
func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since string
	var limit int
	var record bool
	var kind string
	cmd := &cobra.Command{
		Use:   "drift [id]",
		Short: "Show how a title's score, members, favorites, and rank moved over time",
		Long: "Use this command for how a title's score, members, favorites, or rank changed over time from local snapshots.\n" +
			"Do NOT use this command for the static shape of the current score distribution; use 'anime divisive' instead.\n\n" +
			"Snapshots are recorded with `drift <id> --record` or by running it on a schedule; with no id it prints the largest movers across every recorded title.",
		Example: "  myanimelist-pp-cli drift --since 30d --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "auto",
			"pp:happy-args":       "id=52991;--db=:memory:;--since=30d",
			"pp:typed-exit-codes": "0,3",
			"pp:novel-scaffold":   "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if kind != "anime" && kind != "manga" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--kind must be anime or manga, got %q", kind))
			}
			dbPath = malDBPath(flags, dbPath)
			if !malStoreExists(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: myanimelist-pp-cli drift 52991 --record --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), driftView{Rows: make([]driftRow, 0), Since: since}, flags)
				}
				return nil
			}
			db, err := malOpenStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			if record {
				if len(args) < 1 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("recording a snapshot needs an id, e.g. myanimelist-pp-cli drift 52991 --record"))
				}
				// Reporting is local-only, but recording a snapshot reads the
				// live title page. The command therefore declares "auto" and
				// refuses the one combination that cannot work offline instead
				// of silently hitting the network under --data-source local.
				if flags != nil && flags.dataSource == "local" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--record needs the live title page, which --data-source local forbids; drop the flag or use --data-source auto"))
				}
				id, err := malIntArg(args[0], "id")
				if err != nil {
					return err
				}
				detail, err := malDetail(ctx, flags, kind, id)
				if err != nil {
					return err
				}
				if err := malRecordSnapshot(ctx, db, detail); err != nil {
					return err
				}
				row := driftRow{Kind: detail.Kind, ID: detail.ID, Title: detail.Title, To: time.Now().UTC().Format(time.RFC3339), ScoreNow: detail.Score, MembersNow: detail.Members}
				view := driftView{Rows: make([]driftRow, 0), Since: since, Snapshots: 1, Recorded: &row}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "recorded snapshot for %s %d (%s)\n", detail.Kind, detail.ID, detail.Title)
				return nil
			}

			window, err := parseSinceWindow(since)
			if err != nil {
				return err
			}
			idFilter := 0
			if len(args) > 0 {
				idFilter, err = malIntArg(args[0], "id")
				if err != nil {
					return err
				}
			}
			rows, err := db.DB().QueryContext(ctx, `
				SELECT kind, id, captured_at, COALESCE(title,''), score, members, favorites, rank
				FROM mal_snapshots
				WHERE kind = ? AND (? = 0 OR id = ?)
				ORDER BY id, captured_at`, kind, idFilter, idFilter)
			if err != nil {
				return fmt.Errorf("reading snapshots: %w", err)
			}
			type snap struct {
				kind, captured, title        string
				id, members, favorites, rank int
				score                        float64
			}
			all := make([]snap, 0, 64)
			for rows.Next() {
				var s snap
				if err := rows.Scan(&s.kind, &s.id, &s.captured, &s.title, &s.score, &s.members, &s.favorites, &s.rank); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning snapshots: %w", err)
				}
				all = append(all, s)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("reading snapshots: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing snapshot rows: %w", err)
			}

			cutoff := time.Now().UTC().Add(-window)
			byID := map[int][]snap{}
			for _, s := range all {
				byID[s.id] = append(byID[s.id], s)
			}
			out := make([]driftRow, 0, len(byID))
			for id, snaps := range byID {
				if len(snaps) < 2 {
					continue
				}
				sort.Slice(snaps, func(i, j int) bool { return snaps[i].captured < snaps[j].captured })
				latest := snaps[len(snaps)-1]
				earlier := snaps[0]
				for _, s := range snaps {
					if t, perr := time.Parse(time.RFC3339, s.captured); perr == nil && t.Before(cutoff) {
						earlier = s
					}
				}
				if earlier.captured == latest.captured {
					continue
				}
				out = append(out, driftRow{
					Kind: latest.kind, ID: id, Title: latest.title,
					From: earlier.captured, To: latest.captured,
					ScoreDelta:    math.Round((latest.score-earlier.score)*100) / 100,
					MemberDelta:   latest.members - earlier.members,
					FavoriteDelta: latest.favorites - earlier.favorites,
					RankDelta:     earlier.rank - latest.rank,
					ScoreNow:      latest.score, MembersNow: latest.members,
				})
			}
			sort.Slice(out, func(i, j int) bool { return math.Abs(out[i].ScoreDelta) > math.Abs(out[j].ScoreDelta) })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			view := driftView{Rows: out, Since: since, Snapshots: len(all)}
			if len(out) == 0 {
				view.Note = fmt.Sprintf("no title has two snapshots inside the %s window; run `myanimelist-pp-cli drift <id> --record` periodically to build history", since)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			table := make([]map[string]any, 0, len(out))
			for _, r := range out {
				table = append(table, map[string]any{
					"title": r.Title, "id": r.ID, "score_now": r.ScoreNow,
					"score_delta": r.ScoreDelta, "members_delta": r.MemberDelta,
					"rank_delta": r.RankDelta,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&since, "since", "30d", "Comparison window (e.g. 7d, 24h, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum movers to return (0 for all)")
	cmd.Flags().BoolVar(&record, "record", false, "Record a fresh snapshot for the given id instead of reporting drift")
	cmd.Flags().StringVar(&kind, "kind", "anime", "Entity kind to compare (anime or manga)")
	return cmd
}

// parseSinceWindow accepts Go durations plus the day/week shorthand the rest of
// the CLI uses.
func parseSinceWindow(s string) (time.Duration, error) {
	if s == "" {
		return 30 * 24 * time.Hour, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	var n int
	var unit string
	if _, err := fmt.Sscanf(s, "%d%s", &n, &unit); err != nil {
		return 0, usageErr(fmt.Errorf("--since %q is not a duration; try 7d, 24h, or 30d", s))
	}
	switch unit {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case "h":
		return time.Duration(n) * time.Hour, nil
	case "m":
		return time.Duration(n) * time.Minute, nil
	}
	return 0, usageErr(fmt.Errorf("--since unit %q is not supported; use d, w, h, or m", unit))
}

var _ = os.Stat

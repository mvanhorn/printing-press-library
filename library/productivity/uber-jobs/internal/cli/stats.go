// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	// A literal rootCmd.AddCommand is what the press's static scanners
	// (dogfood, verify-skill) walk; without it they resolve "stats" to the
	// framework leaf of the same name (#4567). It adds only when nothing
	// registered the command already, so the runtime tree never doubles.
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		if !hasChildCommand(rootCmd, "stats") {
			rootCmd.AddCommand(newNovelStatsCmd(flags))
		}
	})
}

// statsRow is one group's hiring counts. opened_30d and closed_30d are null
// until the local history covers the whole 30-day window.
type statsRow struct {
	ID        string  `json:"id"`
	Group     string  `json:"group"`
	Label     *string `json:"label"`
	Open      int     `json:"open"`
	Posted7d  int     `json:"posted_7d"`
	Posted30d int     `json:"posted_30d"`
	Opened30d *int    `json:"opened_30d"`
	Closed30d *int    `json:"closed_30d"`
}

// statsBy maps accepted --by spellings to the grouping key.
var statsBy = map[string]string{
	"country": "country", "countries": "country",
	"team": "team", "category": "team", "job_category": "team",
	"subteam": "subteam", "sub-team": "subteam", "sub_team": "subteam",
}

const historyWindow = 30 * 24 * time.Hour

func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var by, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Count open, newly posted, opened and closed Uber postings per country, team, subteam or category",
		Long: strings.Trim(`
Count Uber postings per country, team, subteam, or category, with churn once history allows

Columns: open (postings listed now), posted_7d and posted_30d (open postings
whose true posting date falls in the window; migration-floor dates never
count), opened_30d (first seen locally in the last 30 days) and closed_30d
(closed in the last 30 days). The last two stay null until the local store's
first complete sync is at least 30 days old, because before that every posting
looks newly opened. --by category is the same grouping as team (job_category).
A posting with locations in several countries counts once in each country.

auto counts from the local store when it holds a complete sync, and otherwise
reads the corpus live once (two requests; opened_30d and closed_30d are then
null). Rows sort by open, highest first, then by group.`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli stats --by country --json
  uber-jobs-pp-cli stats --by category --limit 40
  uber-jobs-pp-cli stats --by subteam --data-source local --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("stats takes no arguments; use --by country, team, subteam, or category"))
			}
			key, ok := statsBy[strings.ToLower(strings.TrimSpace(by))]
			if !ok {
				return usageErr(fmt.Errorf("--by must be country, team, subteam, or category, got %q", by))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be zero or positive"))
			}
			ds, err := resolveDataSource(cmd, flags, "auto", "live", "local")
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			now := nowUTC()
			resolved, last, err := resolveLocalFirst(ctx, ds, dbPath, 0, now)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, describeSource(flags, resolved, dbPath, uberjobs.Filters{})+", then count postings by "+key)
			}
			env, rows, err := computeStats(ctx, flags, resolved, dbPath, key, last, now)
			if err != nil {
				return err
			}
			env.Hits = len(rows)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			env.Returned = len(rows)
			env.Results = rows
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, strings.ToUpper(key)+"\tOPEN\tPOSTED 7D\tPOSTED 30D\tOPENED 30D\tCLOSED 30D")
				for _, r := range rows {
					name := r.Group
					if r.Label != nil && *r.Label != r.Group {
						name += " (" + *r.Label + ")"
					}
					fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\t%s\n", name, r.Open, r.Posted7d, r.Posted30d, intOrDash(r.Opened30d), intOrDash(r.Closed30d))
				}
				if err := tw.Flush(); err != nil {
					return err
				}
				fmt.Fprintf(w, "\n%d of %d groups (source: %s)\n", len(rows), env.Hits, env.Meta.Source)
				if env.Meta.Note != "" {
					fmt.Fprintln(w, "note:", env.Meta.Note)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&by, "by", "country", "Group by: country, team, subteam, or category (an alias of team)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum groups to return after sorting (0 returns all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI data dir)")
	addDataSourceFlag(cmd, "auto", "Where to read: auto (the local store when it holds a complete sync, else live), live, or local")
	return cmd
}

// statsInput is one posting with its local history (empty when live).
type statsInput struct {
	posting   uberjobs.Posting
	firstSeen string
	closedOn  *string
}

// computeStats groups postings from the resolved source.
func computeStats(ctx context.Context, flags *rootFlags, resolved, dbPath, key string, last *uberjobs.SyncRun, now time.Time) (uberjobs.Envelope, []statsRow, error) {
	env := uberjobs.NewEnvelope("stats", "", []statsRow{}, 0)
	var inputs []statsInput
	history := false
	if resolved == "local" {
		// Read-only: a missing store is an empty one and is never created.
		var stored []uberjobs.StoredPosting
		var first time.Time
		var ok bool
		_, err := withStoreRO(ctx, dbPath, func(s *store.Store) error {
			var err error
			if stored, err = uberjobs.LoadPostings(ctx, s.DB(), false); err != nil {
				return err
			}
			first, ok = uberjobs.FirstSyncAt(ctx, s.DB())
			return nil
		})
		if err != nil && !isMissingTable(err) {
			return env, nil, err
		}
		for _, sp := range stored {
			inputs = append(inputs, statsInput{posting: sp.Posting, firstSeen: sp.FirstSeen, closedOn: sp.ClosedOn})
		}
		history = ok && !first.After(now.Add(-historyWindow))
		env.Meta.Source = uberjobs.SourceLocal
		env.Meta.Complete = last != nil
		env.Scanned = len(stored)
		switch {
		case len(stored) == 0:
			env.Meta.Note = "the local store is empty; run: uber-jobs-pp-cli sync"
		case !history && ok:
			env.Meta.Note = joinNote(localFirstNote("local", last), fmt.Sprintf("local history starts %s, less than 30 days ago, so opened_30d and closed_30d are null", first.Format("2006-01-02")))
		case !ok:
			env.Meta.Note = "no complete sync recorded, so opened_30d and closed_30d are null"
		default:
			env.Meta.Note = localFirstNote("local", last)
		}
	} else {
		genv, rows, err := gatherPostings(ctx, flags, resolved, dbPath, uberjobs.Filters{}, now)
		if err != nil {
			return env, nil, err
		}
		env.Meta = genv.Meta
		env.Meta.Command = "stats"
		env.Scanned = genv.Scanned
		env.ScanCapHit = genv.ScanCapHit
		for _, p := range rows {
			inputs = append(inputs, statsInput{posting: p})
		}
		note := "live counts have no local history, so opened_30d and closed_30d are null"
		if env.Meta.Note != "" {
			note = env.Meta.Note + "; " + note
		}
		env.Meta.Note = note
	}
	if key == "country" {
		env.Meta.Note = joinNote(env.Meta.Note, "a posting listed in several countries counts in each")
	}
	rows := groupStats(inputs, key, history, now)
	env.Meta.Extra = map[string]any{"by": key, "history_covers_30d": history}
	return env, rows, nil
}

// groupStats counts postings per group and sorts deterministically.
func groupStats(inputs []statsInput, key string, history bool, now time.Time) []statsRow {
	byID := map[string]*statsRow{}
	get := func(id string, label *string) *statsRow {
		r, ok := byID[id]
		if !ok {
			r = &statsRow{ID: id, Group: id, Label: label}
			if history {
				zero1, zero2 := 0, 0
				r.Opened30d, r.Closed30d = &zero1, &zero2
			}
			byID[id] = r
		}
		return r
	}
	cutoff := now.Add(-historyWindow)
	for _, in := range inputs {
		p := in.posting
		open := in.closedOn == nil
		for _, g := range statsGroups(p, key) {
			r := get(g.id, g.label)
			if open {
				r.Open++
				if uberjobs.WithinWindow(p, 7*24*time.Hour, now) {
					r.Posted7d++
				}
				if uberjobs.WithinWindow(p, historyWindow, now) {
					r.Posted30d++
				}
			}
			if !history {
				continue
			}
			if t, err := time.Parse(time.RFC3339, in.firstSeen); err == nil && !t.Before(cutoff) {
				*r.Opened30d++
			}
			if in.closedOn != nil {
				if t, err := time.Parse(time.RFC3339, *in.closedOn); err == nil && !t.Before(cutoff) {
					*r.Closed30d++
				}
			}
		}
	}
	rows := make([]statsRow, 0, len(byID))
	for _, r := range byID {
		if r.Open == 0 && (r.Closed30d == nil || *r.Closed30d == 0) {
			continue
		}
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Open != rows[j].Open {
			return rows[i].Open > rows[j].Open
		}
		return rows[i].ID < rows[j].ID
	})
	return rows
}

type statsGroup struct {
	id    string
	label *string
}

// statsGroups returns the groups one posting counts in: every distinct
// country of its locations, or its single team or subteam.
func statsGroups(p uberjobs.Posting, key string) []statsGroup {
	switch key {
	case "team":
		return []statsGroup{{id: deref(p.JobCategory, "(none)")}}
	case "subteam":
		return []statsGroup{{id: deref(p.SubTeam, "(none)")}}
	}
	seen := map[string]bool{}
	var out []statsGroup
	for _, l := range p.Locations {
		var g statsGroup
		switch {
		case l.CountryCode != nil && *l.CountryCode != "":
			g = statsGroup{id: *l.CountryCode, label: l.Country}
		case l.Country != nil && *l.Country != "":
			g = statsGroup{id: "unmapped:" + *l.Country, label: l.Country}
		default:
			continue
		}
		if !seen[g.id] {
			seen[g.id] = true
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		out = []statsGroup{{id: "(none)"}}
	}
	return out
}

func intOrDash(v *int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprint(*v)
}

// joinNote appends a clause to a note with "; " only when both are non-empty.
func joinNote(note, clause string) string {
	switch {
	case strings.TrimSpace(note) == "":
		return clause
	case strings.TrimSpace(clause) == "":
		return note
	default:
		return note + "; " + clause
	}
}

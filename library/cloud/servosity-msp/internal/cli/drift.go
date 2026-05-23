// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/snapshot"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/store"
)

// driftCompany mirrors the per-company shape inside an `attention` snapshot
// payload. Kept private to this file; the diff logic is the substance.
type driftCompany struct {
	CompanyID        int64  `json:"company_id"`
	CompanyName      string `json:"company_name"`
	Score            int    `json:"score"`
	OpenIssues       int    `json:"open_issues"`
	StaleBackups     int    `json:"stale_backups"`
	DRBackupInFlight int    `json:"drbackup_in_flight"`
}

// driftSnapshotPayload is the envelope written by `attention`.
type driftSnapshotPayload struct {
	TakenAt   time.Time      `json:"taken_at"`
	Companies []driftCompany `json:"companies"`
	Totals    map[string]int `json:"totals"`
}

// driftChange is one row in the WORSE or RECOVERED sections of the diff.
type driftChange struct {
	CompanyID        int64  `json:"company_id"`
	CompanyName      string `json:"company_name"`
	ScoreFrom        int    `json:"score_from"`
	ScoreTo          int    `json:"score_to"`
	ScoreDelta       int    `json:"score_delta"`
	OpenIssuesFrom   int    `json:"open_issues_from"`
	OpenIssuesTo     int    `json:"open_issues_to"`
	StaleBackupsFrom int    `json:"stale_backups_from"`
	StaleBackupsTo   int    `json:"stale_backups_to"`
	NewCompany       bool   `json:"new_company,omitempty"`
	Dropped          bool   `json:"dropped,omitempty"`
}

// driftResult is the JSON envelope emitted on stdout in machine mode.
type driftResult struct {
	From           time.Time     `json:"from"`
	To             time.Time     `json:"to"`
	Metric         string        `json:"metric"`
	Worse          []driftChange `json:"worse"`
	Recovered      []driftChange `json:"recovered"`
	UnchangedCount int           `json:"unchanged_count"`
}

// newDriftCmd builds the trend-awareness command: it diffs two `attention`
// snapshots (or any other metric series) so you can see what got worse and
// what recovered between two moments in time. Read-only on the local store;
// the snapshots themselves are written by `attention`.
func newDriftCmd(flags *rootFlags) *cobra.Command {
	var metric string
	var fromAnchor string
	var toAnchor string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Diff two snapshots: what got worse, what recovered",
		Long: `Diff two snapshots the CLI itself collected over time. Shows what got
worse (new issues, new stale backups) and what recovered between two
timestamps. Snapshots are recorded by 'attention' (and other commands that
opt in via the snapshot package).

Anchors accept the same vocabulary as everywhere else: "now", "today",
"yesterday", "2h ago", "7d ago", "2026-05-21", RFC3339.

If either anchor has no snapshot recorded, drift will tell you so and
suggest 'attention' as the way to record one.`,
		Example: `  # Default: attention metric, yesterday vs now
  servosity-msp-cli drift

  # Explicit window
  servosity-msp-cli drift --from "7d ago" --to now

  # Pin to a specific historical pair
  servosity-msp-cli drift --from 2026-05-20 --to 2026-05-22`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			now := time.Now()
			fromTime, ok := snapshot.ResolveAnchor(now, fromAnchor)
			if !ok {
				return usageErr(fmt.Errorf("invalid --from anchor %q (try 'yesterday', '2h ago', '2026-05-21')", fromAnchor))
			}
			toTime, ok := snapshot.ResolveAnchor(now, toAnchor)
			if !ok {
				return usageErr(fmt.Errorf("invalid --to anchor %q (try 'now', '2h ago', '2026-05-22')", toAnchor))
			}

			ctx := cmd.Context()
			db, err := store.Open(defaultDBPath("servosity-msp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w\nRun 'servosity-msp-cli sync' first.", err)
			}
			defer db.Close()

			fromSnap, err := snapshot.At(ctx, db.DB(), metric, fromTime)
			if err != nil {
				return fmt.Errorf("reading --from snapshot: %w", err)
			}
			if fromSnap == nil {
				return fmt.Errorf("No snapshot found for metric '%s' at %s. Run `servosity-msp-cli attention` first to record one.",
					metric, fromAnchor)
			}
			toSnap, err := snapshot.At(ctx, db.DB(), metric, toTime)
			if err != nil {
				return fmt.Errorf("reading --to snapshot: %w", err)
			}
			if toSnap == nil {
				return fmt.Errorf("No snapshot found for metric '%s' at %s. Run `servosity-msp-cli attention` first to record one.",
					metric, toAnchor)
			}

			var fromPayload, toPayload driftSnapshotPayload
			if err := json.Unmarshal(fromSnap.Data, &fromPayload); err != nil {
				return fmt.Errorf("decoding --from snapshot payload: %w", err)
			}
			if err := json.Unmarshal(toSnap.Data, &toPayload); err != nil {
				return fmt.Errorf("decoding --to snapshot payload: %w", err)
			}

			result := diffSnapshots(metric, fromSnap.TakenAt, toSnap.TakenAt, fromPayload, toPayload)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return renderDriftHuman(cmd, result)
		},
	}

	cmd.Flags().StringVar(&metric, "metric", "attention", "Which snapshot series to diff (snapshots are tagged by metric name)")
	cmd.Flags().StringVar(&fromAnchor, "from", "yesterday", "Earlier anchor (yesterday, now, 2h ago, 7d ago, 2026-05-21, RFC3339)")
	cmd.Flags().StringVar(&toAnchor, "to", "now", "Later anchor (same vocabulary as --from)")

	return cmd
}

// diffSnapshots compares two attention payloads and returns the WORSE,
// RECOVERED, and unchanged sets. Companies are keyed by company_id; missing
// IDs in 0 (decode failures) are skipped — a row with no id can't be diffed.
//
// WORSE: score increased OR open_issues increased OR stale_backups increased.
// RECOVERED: score decreased OR open_issues went from non-zero to zero.
// NEW in the later snapshot: marked NEW; if score > 0 they also count as
// worse (something appeared that needs attention).
// DROPPED from the later snapshot: included as recovered with dropped=true
// and score_to=0, but only when their from-score was > 0 (otherwise it's
// noise, not recovery).
func diffSnapshots(metric string, fromTaken, toTaken time.Time, from, to driftSnapshotPayload) driftResult {
	fromByID := map[int64]driftCompany{}
	for _, c := range from.Companies {
		if c.CompanyID == 0 {
			continue
		}
		fromByID[c.CompanyID] = c
	}
	toByID := map[int64]driftCompany{}
	for _, c := range to.Companies {
		if c.CompanyID == 0 {
			continue
		}
		toByID[c.CompanyID] = c
	}

	var worse []driftChange
	var recovered []driftChange
	unchanged := 0

	// Walk the later snapshot first so NEW companies surface.
	for id, t := range toByID {
		f, existedBefore := fromByID[id]
		change := driftChange{
			CompanyID:        id,
			CompanyName:      pickName(t.CompanyName, f.CompanyName, id),
			ScoreFrom:        f.Score,
			ScoreTo:          t.Score,
			ScoreDelta:       t.Score - f.Score,
			OpenIssuesFrom:   f.OpenIssues,
			OpenIssuesTo:     t.OpenIssues,
			StaleBackupsFrom: f.StaleBackups,
			StaleBackupsTo:   t.StaleBackups,
			NewCompany:       !existedBefore,
		}

		gotWorse := t.Score > f.Score ||
			t.OpenIssues > f.OpenIssues ||
			t.StaleBackups > f.StaleBackups
		gotBetter := t.Score < f.Score ||
			(t.OpenIssues == 0 && f.OpenIssues > 0)

		switch {
		case gotWorse && !gotBetter:
			worse = append(worse, change)
		case gotBetter && !gotWorse:
			recovered = append(recovered, change)
		case gotWorse && gotBetter:
			// Mixed signal (e.g. open_issues up, stale_backups down): defer to
			// the score delta as the tiebreaker — score is the rolled-up bar.
			if change.ScoreDelta > 0 {
				worse = append(worse, change)
			} else if change.ScoreDelta < 0 {
				recovered = append(recovered, change)
			} else {
				unchanged++
			}
		default:
			unchanged++
		}
	}

	// Dropped companies: present in `from`, absent in `to`. Only count as
	// recovery when they actually had something to recover from.
	for id, f := range fromByID {
		if _, stillThere := toByID[id]; stillThere {
			continue
		}
		if f.Score == 0 && f.OpenIssues == 0 && f.StaleBackups == 0 {
			// They were zero-everything in `from` and are gone in `to`;
			// that's not recovery, it's just absence. Don't count it.
			continue
		}
		recovered = append(recovered, driftChange{
			CompanyID:        id,
			CompanyName:      pickName(f.CompanyName, "", id),
			ScoreFrom:        f.Score,
			ScoreTo:          0,
			ScoreDelta:       -f.Score,
			OpenIssuesFrom:   f.OpenIssues,
			OpenIssuesTo:     0,
			StaleBackupsFrom: f.StaleBackups,
			StaleBackupsTo:   0,
			Dropped:          true,
		})
	}

	// Sort: worse by biggest delta first; recovered by biggest improvement first.
	sort.Slice(worse, func(i, j int) bool {
		if worse[i].ScoreDelta != worse[j].ScoreDelta {
			return worse[i].ScoreDelta > worse[j].ScoreDelta
		}
		return worse[i].CompanyName < worse[j].CompanyName
	})
	sort.Slice(recovered, func(i, j int) bool {
		if recovered[i].ScoreDelta != recovered[j].ScoreDelta {
			return recovered[i].ScoreDelta < recovered[j].ScoreDelta
		}
		return recovered[i].CompanyName < recovered[j].CompanyName
	})

	return driftResult{
		From:           fromTaken,
		To:             toTaken,
		Metric:         metric,
		Worse:          worse,
		Recovered:      recovered,
		UnchangedCount: unchanged,
	}
}

func pickName(primary, fallback string, id int64) string {
	if primary != "" {
		return primary
	}
	if fallback != "" {
		return fallback
	}
	return fmt.Sprintf("company:%d", id)
}

func renderDriftHuman(cmd *cobra.Command, r driftResult) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Drift in %s from %s to %s:\n\n",
		r.Metric,
		r.From.Local().Format("2006-01-02 15:04"),
		r.To.Local().Format("2006-01-02 15:04"))

	if len(r.Worse) == 0 && len(r.Recovered) == 0 {
		fmt.Fprintf(w, "No change across %d companies.\n", r.UnchangedCount)
		return nil
	}

	if len(r.Worse) > 0 {
		fmt.Fprintf(w, "WORSE (%d %s):\n", len(r.Worse), pluralize(len(r.Worse), "company", "companies"))
		for _, c := range r.Worse {
			fmt.Fprintf(w, "  %s\n", formatDriftLine(c))
		}
		fmt.Fprintln(w)
	}

	if len(r.Recovered) > 0 {
		fmt.Fprintf(w, "RECOVERED (%d %s):\n", len(r.Recovered), pluralize(len(r.Recovered), "company", "companies"))
		for _, c := range r.Recovered {
			fmt.Fprintf(w, "  %s\n", formatDriftLine(c))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "NO CHANGE: %d %s\n", r.UnchangedCount, pluralize(r.UnchangedCount, "company", "companies"))
	return nil
}

func formatDriftLine(c driftChange) string {
	label := fmt.Sprintf("%s (%d):", c.CompanyName, c.CompanyID)
	deltaSign := "+"
	if c.ScoreDelta < 0 {
		deltaSign = "" // negative already prints its own sign
	}
	head := fmt.Sprintf("%-28s score %d → %d (%s%d)",
		label, c.ScoreFrom, c.ScoreTo, deltaSign, c.ScoreDelta)

	var detail []string
	if c.OpenIssuesFrom != c.OpenIssuesTo {
		detail = append(detail, fmt.Sprintf("open_issues %d→%d", c.OpenIssuesFrom, c.OpenIssuesTo))
	}
	if c.StaleBackupsFrom != c.StaleBackupsTo {
		detail = append(detail, fmt.Sprintf("stale_backups %d→%d", c.StaleBackupsFrom, c.StaleBackupsTo))
	}
	tags := []string{}
	if c.NewCompany {
		tags = append(tags, "NEW")
	}
	if c.Dropped {
		tags = append(tags, "DROPPED")
	}

	out := head
	if len(detail) > 0 {
		out += " — " + strings.Join(detail, ", ")
	}
	if len(tags) > 0 {
		out += " (" + strings.Join(tags, ", ") + ")"
	}
	return out
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

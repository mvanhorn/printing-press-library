// Copyright 2026 melanson633. Licensed under Apache-2.0. See LICENSE.
// Transcendence feature: offline "where did my time go" recap, aggregating
// synced entries by project, client, or tag.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/store"
	"github.com/spf13/cobra"
)

func newRecapCmd(flags *rootFlags) *cobra.Command {
	var rangeFlag, byFlag, workspace, dbPath string

	cmd := &cobra.Command{
		Use:   "recap",
		Short: "Where your tracked time went, grouped by project, client, or tag",
		Long: `Aggregate synced time entries over a date range into a ranked
breakdown with the billable vs non-billable split and each group's
share of total tracked time. Reads the local store; run sync first.`,
		Example: `  # This month, by project
  clockify-pp-cli recap

  # Last month, by client, as JSON
  clockify-pp-cli recap --range last-month --by client --json

  # Last 7 days, by tag
  clockify-pp-cli recap --range 7d --by tag`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("clockify-pp-cli")
			}
			start, end, label, err := resolveRange(rangeFlag, time.Now())
			if err != nil {
				return usageErr(err)
			}
			switch byFlag {
			case "project", "client", "tag":
			default:
				return usageErr(fmt.Errorf("--by must be project, client, or tag (got %q)", byFlag))
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clockify-pp-cli sync' first.", err)
			}
			defer db.Close()

			// recap aggregates already-synced entries. Under --dry-run the
			// HTTP layer suppresses requests, so the live fallback inside
			// ensureTimeEntries cannot resolve a user/workspace on a cold
			// cache; force local-only reads so the dry run reports the
			// offline recap instead of erroring.
			efFlags := flags
			if flags.dryRun {
				local := *flags
				local.dataSource = "local"
				efFlags = &local
			}
			entries, err := ensureTimeEntries(db, efFlags, start, end, workspace)
			if err != nil {
				return fmt.Errorf("loading time entries: %w", err)
			}
			projects := loadProjects(db)
			clients := loadClientNames(db)
			tags := loadTagNames(db)

			type bucket struct {
				Key         string
				Billable    time.Duration
				NonBillable time.Duration
			}
			buckets := map[string]*bucket{}
			var grand time.Duration
			counted := 0
			get := func(key string) *bucket {
				b := buckets[key]
				if b == nil {
					b = &bucket{Key: key}
					buckets[key] = b
				}
				return b
			}
			add := func(key string, e timeEntry) {
				b := get(key)
				if e.Billable {
					b.Billable += e.Duration
				} else {
					b.NonBillable += e.Duration
				}
			}

			for _, e := range entries {
				if e.Start.IsZero() || e.Start.Before(start) || !e.Start.Before(end) {
					continue
				}
				if workspace != "" && e.WorkspaceID != workspace {
					continue
				}
				counted++
				grand += e.Duration
				switch byFlag {
				case "project":
					name := "(no project)"
					if p, ok := projects[e.ProjectID]; ok && p.Name != "" {
						name = p.Name
					} else if e.ProjectID != "" {
						name = e.ProjectID
					}
					add(name, e)
				case "client":
					name := "(no client)"
					if p, ok := projects[e.ProjectID]; ok && p.ClientID != "" {
						if cn, ok := clients[p.ClientID]; ok && cn != "" {
							name = cn
						} else {
							name = p.ClientID
						}
					}
					add(name, e)
				case "tag":
					if len(e.TagIDs) == 0 {
						add("(untagged)", e)
						break
					}
					for _, tid := range e.TagIDs {
						name := tid
						if tn, ok := tags[tid]; ok && tn != "" {
							name = tn
						}
						add(name, e)
					}
				}
			}

			rows := make([]*bucket, 0, len(buckets))
			for _, b := range buckets {
				rows = append(rows, b)
			}
			sort.Slice(rows, func(i, j int) bool {
				ti := rows[i].Billable + rows[i].NonBillable
				tj := rows[j].Billable + rows[j].NonBillable
				if ti != tj {
					return ti > tj
				}
				return rows[i].Key < rows[j].Key
			})

			// For --by tag, a single entry can land in multiple buckets (one per
			// tag), so sum(bucket totals) > grand and per-tag shares against
			// grand can exceed 100%. Use the sum of bucket totals as the share
			// denominator in that mode; grand remains the honest total-tracked
			// figure used for the JSON total_hours and the header line.
			shareDenom := grand
			if byFlag == "tag" {
				shareDenom = 0
				for _, b := range rows {
					shareDenom += b.Billable + b.NonBillable
				}
			}

			if flags.asJSON {
				type jrow struct {
					Group            string  `json:"group"`
					TotalHours       float64 `json:"total_hours"`
					BillableHours    float64 `json:"billable_hours"`
					NonBillableHours float64 `json:"non_billable_hours"`
					SharePct         float64 `json:"share_pct"`
				}
				jrows := make([]jrow, 0, len(rows))
				for _, b := range rows {
					tot := b.Billable + b.NonBillable
					jrows = append(jrows, jrow{
						Group:            b.Key,
						TotalHours:       round2(tot.Hours()),
						BillableHours:    round2(b.Billable.Hours()),
						NonBillableHours: round2(b.NonBillable.Hours()),
						SharePct:         sharePct(tot, shareDenom),
					})
				}
				return flags.printJSON(cmd, map[string]any{
					"range":       label,
					"range_start": start.Format("2006-01-02"),
					"grouped_by":  byFlag,
					"entry_count": counted,
					"total_hours": round2(grand.Hours()),
					"groups":      jrows,
				})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Recap — %s, by %s (%d entries, %.2fh total)\n\n", label, byFlag, counted, grand.Hours())
			if len(rows) == 0 {
				fmt.Fprintln(out, "No time entries in range.")
				fmt.Fprintf(out, "(%s)\n", emptyStoreHint)
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "GROUP\tTOTAL\tBILLABLE\tNON-BILL\tSHARE")
			for _, b := range rows {
				tot := b.Billable + b.NonBillable
				fmt.Fprintf(tw, "%s\t%.2fh\t%.2fh\t%.2fh\t%.1f%%\n",
					truncate(b.Key, 32), tot.Hours(), b.Billable.Hours(), b.NonBillable.Hours(), sharePct(tot, shareDenom))
			}
			tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&rangeFlag, "range", "this-month", "Date range: today, this-week, last-week, this-month, last-month, all, or Nd")
	cmd.Flags().StringVar(&byFlag, "by", "project", "Group by: project, client, or tag")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter to one workspace id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// loadTagNames returns a tagId -> tag name index from the local store.
func loadTagNames(db *store.Store) map[string]string {
	idx := nameIndex(db, []string{"tags"}, []string{"tags", "tag"})
	out := make(map[string]string, len(idx))
	for id, raw := range idx {
		var t struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &t) == nil {
			out[id] = t.Name
		}
	}
	return out
}

// sharePct returns part as a percentage of whole, rounded to 1 decimal.
func sharePct(part, whole time.Duration) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(int64(part.Hours()/whole.Hours()*1000+0.5)) / 10
}

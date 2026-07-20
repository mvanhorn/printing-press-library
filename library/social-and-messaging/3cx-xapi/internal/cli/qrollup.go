// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Novel command: queue rollup. One cross-queue table from the local mirror —
// per-queue agent counts, ring strategy, and any synced queue-performance
// stats merged by queue number. Reads local data only.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/internal/cliutil"
	"github.com/spf13/cobra"
)

type queueRow struct {
	Number       string                       `json:"number"`
	Name         string                       `json:"name"`
	AgentCount   int                          `json:"agent_count"`
	RingStrategy string                       `json:"ring_strategy,omitempty"`
	Stats        map[string][]json.RawMessage `json:"stats,omitempty"`
}

type qrollupReport struct {
	Window      string     `json:"window"`
	Queues      []queueRow `json:"queues"`
	QueueCount  int        `json:"queue_count"`
	TotalAgents int        `json:"total_agents"`
	StatsSynced bool       `json:"stats_synced"`
	Note        string     `json:"note,omitempty"`
}

// candidate queue-statistics resource types; merged by queue number when synced.
var queueStatResources = []string{
	"report-queue-performance-overview",
	"report-detailed-queue-statistics",
	"report-abandoned-queue-calls",
	"report-breaches-sla",
}

func buildQueueRows(queues []map[string]json.RawMessage, statsByNumber map[string]map[string][]json.RawMessage) ([]queueRow, int) {
	rows := make([]queueRow, 0, len(queues))
	totalAgents := 0
	for _, q := range queues {
		num := jsonString(q, "Number")
		agents := memberNumbers(q, "Agents", "Members")
		totalAgents += len(agents)
		row := queueRow{
			Number:       num,
			Name:         firstString(q, "Name", "Number"),
			AgentCount:   len(agents),
			RingStrategy: jsonString(q, "RingStrategy"),
		}
		if s, ok := statsByNumber[num]; ok {
			row.Stats = s
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Number < rows[j].Number })
	return rows, totalAgents
}

func qrollupWithinWindow(raw json.RawMessage, cutoff time.Time) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	for _, field := range []string{"CallTime", "CallTimeForCsv", "StartTime", "AnsweredTime", "Timestamp", "CreatedAt"} {
		value := firstString(obj, field)
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, value)
		return err == nil && !parsed.Before(cutoff)
	}
	return true
}

func newNovelQrollupCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "qrollup",
		Short: "Cross-queue rollup: per-queue agents, ring strategy, and synced performance stats",
		Long: "One table across all queues from the local mirror — agent counts, ring strategy, and\n" +
			"any synced queue-performance statistics merged by queue number.\n\n" +
			"Use this command for cross-queue aggregate rollups and week-over-week comparison. For\n" +
			"live calls right now use the active-calls command; for raw per-call rows use the\n" +
			"call-history list.\n\n" +
			"Sync first (expand agents for accurate counts):\n" +
			"  3cx-xapi-pp-cli sync --resources queues,report-queue-performance-overview,report-detailed-queue-statistics,report-abandoned-queue-calls,report-breaches-sla --resource-param 'queues:$expand=Agents'",
		Example:     "  3cx-xapi-pp-cli qrollup --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would roll up queue performance across all queues")
				return nil
			}
			if flagSince == "" {
				flagSince = "7d"
			}
			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}

			db, ok, err := openLocalMirror(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, rtQueues) {
				hintIfStale(cmd, db, rtQueues, flags.maxAge)
			}

			queues, err := listObjects(db, rtQueues)
			if err != nil {
				return err
			}

			// Merge any synced queue-stat resources by queue number.
			cutoff := time.Now().UTC().Add(-window)
			statsByNumber := map[string]map[string][]json.RawMessage{}
			statsSynced := false
			for _, rt := range queueStatResources {
				raws, err := db.List(rt, 0)
				if err != nil || len(raws) == 0 {
					continue
				}
				statsSynced = true
				for _, raw := range raws {
					if !qrollupWithinWindow(raw, cutoff) {
						continue
					}
					var obj map[string]json.RawMessage
					if json.Unmarshal(raw, &obj) != nil {
						continue
					}
					num := firstString(obj, "Number", "QueueDn", "Queue", "Dn")
					if num != "" {
						if statsByNumber[num] == nil {
							statsByNumber[num] = map[string][]json.RawMessage{}
						}
						statsByNumber[num][rt] = append(statsByNumber[num][rt], raw)
					}
				}
			}

			hintMembersNotExpanded(cmd, nil, queues)
			rows, totalAgents := buildQueueRows(queues, statsByNumber)
			report := qrollupReport{
				Window:      flagSince,
				Queues:      rows,
				QueueCount:  len(rows),
				TotalAgents: totalAgents,
				StatsSynced: statsSynced,
			}
			if len(rows) == 0 {
				report.Note = "no queues in the local mirror; run sync --resources queues"
			} else if !statsSynced {
				report.Note = "showing queue config + agent counts; sync queue-statistics report resources for performance/SLA columns"
			}

			if machineOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Queue rollup (window %s) — %d queues, %d agents\n\n", report.Window, report.QueueCount, report.TotalAgents)
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "NUMBER\tNAME\tAGENTS\tRING STRATEGY")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.Number, r.Name, r.AgentCount, r.RingStrategy)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if report.Note != "" {
				fmt.Fprintln(w, report.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Stats window label (e.g. 7d, 24h, 4w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

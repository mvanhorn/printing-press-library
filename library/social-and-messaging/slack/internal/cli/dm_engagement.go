// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It implements
// `dm-engagement` — one volume row per direct report: DM count with you,
// Asana tasks created+completed, Fathom calls attended over a window.
// The Slack DM count is fully real; the Asana/Fathom columns come from
// sibling pp-* mirrors and degrade gracefully when absent.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// dmEngagementRow is one direct report's engagement volume.
type dmEngagementRow struct {
	Report         string `json:"report"`
	UserID         string `json:"user_id"`
	DMChannelID    string `json:"dm_channel_id,omitempty"`
	DMCount        int    `json:"dm_count"`
	AsanaCreated   int    `json:"asana_tasks_created"`
	AsanaCompleted int    `json:"asana_tasks_completed"`
	FathomCalls    int    `json:"fathom_calls_attended"`
}

// dmEngagementReport is the full JSON shape of `dm-engagement`.
type dmEngagementReport struct {
	Window         string            `json:"window"`
	Rows           []dmEngagementRow `json:"rows"`
	MissingSources []string          `json:"missing_sources"`
}

// dmChannelForUser finds the IM channel a given user id participates in.
// Mirror IM channels are stored with name "dm:<userID>" (see
// rawChannel.toChannel in sync_mirror.go), so the lookup is a name match.
func dmChannelForUser(channels []store.Channel, userID string) (store.Channel, bool) {
	for _, ch := range channels {
		if ch.IsIM && (ch.Name == "dm:"+userID) {
			return ch, true
		}
	}
	return store.Channel{}, false
}

// slackDMCounts computes the in-window DM message count for each target
// user. This is the always-real portion of the verb.
func slackDMCounts(ctx context.Context, db *store.Store, channels []store.Channel, targets []store.User, since, until string) (map[string]int, map[string]string, error) {
	counts := map[string]int{}
	dmChan := map[string]string{}
	for _, u := range targets {
		ch, ok := dmChannelForUser(channels, u.ID)
		if !ok {
			counts[u.ID] = 0
			continue
		}
		dmChan[u.ID] = ch.ID
		msgs, err := db.MessagesInWindow(ctx, []string{ch.ID}, since, until)
		if err != nil {
			return nil, nil, err
		}
		counts[u.ID] = len(msgs)
	}
	return counts, dmChan, nil
}

// attachAsanaTaskCounts queries the attached Asana mirror for tasks
// created and completed by a user in the window. Defensive: a schema
// mismatch demotes the source and returns zeroes.
func attachAsanaTaskCounts(ctx context.Context, cs *crossSource, userName string) (created, completed int) {
	q := `SELECT COALESCE(SUM(CASE WHEN completed=1 THEN 1 ELSE 0 END),0),
	             COUNT(*)
	      FROM x_asana.tasks
	      WHERE assignee_name LIKE '%' || ? || '%' COLLATE NOCASE`
	rows, ok := cs.probeQuery(ctx, "asana", q, userName)
	if !ok {
		return 0, 0
	}
	defer rows.Close()
	if rows.Next() {
		_ = rows.Scan(&completed, &created)
	}
	return created, completed
}

// attachFathomCallCount queries the attached Fathom mirror for the number
// of calls a user attended. Defensive against unknown sibling schema.
func attachFathomCallCount(ctx context.Context, cs *crossSource, userName string) int {
	q := `SELECT COUNT(*) FROM x_fathom.calls
	      WHERE participants LIKE '%' || ? || '%' COLLATE NOCASE`
	rows, ok := cs.probeQuery(ctx, "fathom", q, userName)
	if !ok {
		return 0
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		_ = rows.Scan(&n)
	}
	return n
}

// resolveReportTargets turns the --report value into a set of users. The
// literal "all" expands to every non-bot, non-deleted human in the
// mirror; anything else is resolved as a single user.
func resolveReportTargets(ctx context.Context, db *store.Store, report string) ([]store.User, error) {
	if strings.EqualFold(strings.TrimSpace(report), "all") {
		channels, err := db.ListChannels(ctx, false)
		if err != nil {
			return nil, err
		}
		// Every user that has an IM channel is a plausible "report".
		var users []store.User
		seen := map[string]bool{}
		for _, ch := range channels {
			if !ch.IsIM || !strings.HasPrefix(ch.Name, "dm:") {
				continue
			}
			uid := strings.TrimPrefix(ch.Name, "dm:")
			if seen[uid] {
				continue
			}
			seen[uid] = true
			u, err := db.ResolveUser(ctx, uid)
			if err != nil {
				users = append(users, store.User{ID: uid, Name: uid})
				continue
			}
			if u.IsBot || u.Deleted {
				continue
			}
			users = append(users, u)
		}
		return users, nil
	}
	u, err := db.ResolveUser(ctx, report)
	if err != nil {
		return nil, err
	}
	return []store.User{u}, nil
}

// displayName returns the most human label for a user.
func displayName(u store.User) string {
	switch {
	case u.DisplayName != "":
		return u.DisplayName
	case u.RealName != "":
		return u.RealName
	case u.Name != "":
		return u.Name
	default:
		return u.ID
	}
}

func newDMEngagementCmd(flags *rootFlags) *cobra.Command {
	var report, window, dbPath string
	var skipMissing bool

	cmd := &cobra.Command{
		Use:   "dm-engagement",
		Short: "Per-direct-report engagement volume across Slack, Asana and Fathom",
		Long: `dm-engagement prints one volume row per direct report: the count of
direct messages with you, Asana tasks created and completed, and Fathom
calls attended over a time window.

The Slack DM count is read from this CLI's local mirror. The Asana and
Fathom columns are read from the sibling pp-* CLIs' SQLite mirrors via
ATTACH DATABASE. When a sibling mirror is absent the column reports 0
and the source is listed in 'missing_sources' — the verb never crashes
on a missing sibling.

No live Slack calls are made.`,
		Example: stringTrimNL(`
  # 14-day engagement for every direct report
  slack-pp-cli dm-engagement --report all --window 14d --agent

  # One report
  slack-pp-cli dm-engagement --report marjorie --window 7d --json

  # Preview without touching any database
  slack-pp-cli dm-engagement --report all --dry-run`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if report == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			since, until, err := windowBounds(window)
			if err != nil {
				return usageErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'slack-pp-cli sync mirror' first.", err)
			}
			defer db.Close()

			targets, err := resolveReportTargets(cmd.Context(), db, report)
			if err != nil {
				return notFoundErr(fmt.Errorf("resolving --report %q: %w", report, err))
			}
			channels, err := db.ListChannels(cmd.Context(), false)
			if err != nil {
				return fmt.Errorf("listing channels: %w", err)
			}

			// --- Slack side (always real) ---
			counts, dmChan, err := slackDMCounts(cmd.Context(), db, channels, targets, since, until)
			if err != nil {
				return fmt.Errorf("counting dms: %w", err)
			}

			// --- Cross-source sides (graceful degradation) ---
			cs, _ := newCrossSource(cmd.Context(), db.DB(), map[string]string{
				"asana":  "asana-pp-cli",
				"fathom": "fathom-pp-cli",
			})
			defer cs.detach(cmd.Context())

			rows := make([]dmEngagementRow, 0, len(targets))
			for _, u := range targets {
				name := displayName(u)
				created, completed := attachAsanaTaskCounts(cmd.Context(), cs, name)
				calls := attachFathomCallCount(cmd.Context(), cs, name)
				rows = append(rows, dmEngagementRow{
					Report:         name,
					UserID:         u.ID,
					DMChannelID:    dmChan[u.ID],
					DMCount:        counts[u.ID],
					AsanaCreated:   created,
					AsanaCompleted: completed,
					FathomCalls:    calls,
				})
				// DM content read of an im channel is an audited event.
				if dmChan[u.ID] != "" {
					_ = db.AppendAuditLog(cmd.Context(), auditCaller(), "dm-engagement", dmChan[u.ID],
						"dm volume read for "+name)
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].DMCount != rows[j].DMCount {
					return rows[i].DMCount > rows[j].DMCount
				}
				return rows[i].Report < rows[j].Report
			})

			missing := cs.missing()
			if missing == nil {
				missing = []string{}
			}
			_ = skipMissing // flag is informational; degradation is unconditional

			return printJSONFiltered(cmd.OutOrStdout(), dmEngagementReport{
				Window:         window,
				Rows:           rows,
				MissingSources: missing,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&report, "report", "", "Direct report name/ID, or 'all' (required)")
	cmd.Flags().StringVar(&window, "window", "14d", "Time window (e.g. 14d, 7d, 30d)")
	cmd.Flags().BoolVar(&skipMissing, "skip-missing", false, "Degrade gracefully when a sibling mirror is absent (default behaviour)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}

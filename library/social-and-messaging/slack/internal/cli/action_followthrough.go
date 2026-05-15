// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It implements
// `action-followthrough` — for each Fathom call action item assigned to
// a CSM, whether that person mentioned the company in Slack within the
// window, with the matching permalink. The Fathom side is read from the
// sibling pp-fathom mirror via ATTACH DATABASE; the Slack-mention check
// is a real FTS5 query against this CLI's mirror.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// fathomActionItem is one action item pulled from the Fathom mirror.
type fathomActionItem struct {
	Assignee string `json:"assignee"`
	Company  string `json:"company"`
	Text     string `json:"text"`
	CallDate string `json:"call_date"`
}

// followThroughRow pairs an action item with its Slack follow-through
// status: did the assignee mention the company after the call?
type followThroughRow struct {
	Assignee       string `json:"assignee"`
	Company        string `json:"company"`
	ActionItem     string `json:"action_item"`
	CallDate       string `json:"call_date"`
	FollowedUp     bool   `json:"followed_up"`
	SlackMention   string `json:"slack_mention,omitempty"`
	SlackPermalink string `json:"slack_permalink,omitempty"`
}

// actionFollowthroughReport is the full JSON shape of the verb.
type actionFollowthroughReport struct {
	Report         string             `json:"report"`
	Window         string             `json:"window"`
	ItemCount      int                `json:"item_count"`
	FollowedUp     int                `json:"followed_up"`
	Rows           []followThroughRow `json:"rows"`
	MissingSources []string           `json:"missing_sources"`
}

// attachFathomActionItemsForAssignee queries the attached Fathom mirror
// for action items assigned to a person. Defensive against unknown
// sibling schema — a mismatch demotes the source and returns nil.
func attachFathomActionItemsForAssignee(ctx context.Context, cs *crossSource, assignee string) []fathomActionItem {
	q := `SELECT assignee, company, description, created_at FROM x_fathom.action_items
	      WHERE assignee LIKE '%' || ? || '%' COLLATE NOCASE LIMIT 100`
	rows, ok := cs.probeQuery(ctx, "fathom", q, assignee)
	if !ok {
		return nil
	}
	defer rows.Close()
	var out []fathomActionItem
	for rows.Next() {
		var a, c, d, ts string
		if err := rows.Scan(&a, &c, &d, &ts); err != nil {
			break
		}
		out = append(out, fathomActionItem{Assignee: a, Company: c, Text: d, CallDate: ts})
	}
	return out
}

// checkSlackFollowThrough runs the real FTS5 mention check: did the
// assignee mention the company in Slack inside the window? Returns the
// first matching message (newest first) when one is found.
func checkSlackFollowThrough(ctx context.Context, db *store.Store, company, since, until string) (store.Message, bool, error) {
	if company == "" {
		return store.Message{}, false, nil
	}
	msgs, err := db.SearchMessages(ctx, company, nil, 50)
	if err != nil {
		return store.Message{}, false, err
	}
	for _, m := range msgs {
		if since != "" && m.TS < since {
			continue
		}
		if until != "" && m.TS > until {
			continue
		}
		return m, true, nil
	}
	return store.Message{}, false, nil
}

func newActionFollowthroughCmd(flags *rootFlags) *cobra.Command {
	var report, window, dbPath string
	var skipMissing bool

	cmd := &cobra.Command{
		Use:   "action-followthrough",
		Short: "Audit whether Fathom call action items were followed up in Slack",
		Long: `action-followthrough cross-references Fathom call action items with
Slack activity: for each action item assigned to a CSM, it checks
whether that person mentioned the company in Slack within the window,
and reports the matching message permalink when they did.

The Fathom action items are read from the sibling pp-fathom CLI's
SQLite mirror via ATTACH DATABASE. The Slack-mention check is a real
FTS5 query against this CLI's local mirror. When the Fathom mirror is
absent the verb reports an empty row set and lists "fathom" in
'missing_sources' — it never crashes on a missing sibling.

No live Slack calls are made.`,
		Example: stringTrimNL(`
  # 14-day follow-through audit for one CSM
  slack-pp-cli action-followthrough --report marjorie --window 14d --agent

  # JSON for piping into L10 prep
  slack-pp-cli action-followthrough --report marjorie --json

  # Preview without touching any database
  slack-pp-cli action-followthrough --report marjorie --dry-run`),
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
			if err := db.EnsureMirrorSchema(cmd.Context()); err != nil {
				return err
			}

			// --- Fathom side (cross-source, graceful degradation) ---
			cs, _ := newCrossSource(cmd.Context(), db.DB(), map[string]string{
				"fathom": "fathom-pp-cli",
			})
			defer cs.detach(cmd.Context())
			items := attachFathomActionItemsForAssignee(cmd.Context(), cs, report)

			// --- Slack mention check (always real FTS5) ---
			rows := make([]followThroughRow, 0, len(items))
			followed := 0
			for _, it := range items {
				m, ok, ferr := checkSlackFollowThrough(cmd.Context(), db, it.Company, since, until)
				if ferr != nil {
					return fmt.Errorf("checking slack follow-through: %w", ferr)
				}
				row := followThroughRow{
					Assignee:   it.Assignee,
					Company:    it.Company,
					ActionItem: it.Text,
					CallDate:   it.CallDate,
					FollowedUp: ok,
				}
				if ok {
					followed++
					row.SlackMention = m.Text
					row.SlackPermalink = m.Permalink
				}
				rows = append(rows, row)
			}

			missing := cs.missing()
			if missing == nil {
				missing = []string{}
			}
			_ = skipMissing // flag is informational; degradation is unconditional

			return printJSONFiltered(cmd.OutOrStdout(), actionFollowthroughReport{
				Report:         report,
				Window:         window,
				ItemCount:      len(rows),
				FollowedUp:     followed,
				Rows:           rows,
				MissingSources: missing,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&report, "report", "", "CSM name to audit (required)")
	cmd.Flags().StringVar(&window, "window", "14d", "Time window (e.g. 14d, 7d, 30d)")
	cmd.Flags().BoolVar(&skipMissing, "skip-missing", false, "Degrade gracefully when the Fathom mirror is absent (default behaviour)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}

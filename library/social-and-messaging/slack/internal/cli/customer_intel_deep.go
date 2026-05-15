// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It implements
// `customer-intel-deep` — a one-screen, time-ordered timeline for a
// customer joining Slack mentions, the Attio deal stage, open Asana
// tasks and Fathom call action items. The Slack side is fully real; the
// Attio/Asana/Fathom sides come from sibling pp-* SQLite mirrors via
// ATTACH DATABASE and degrade gracefully when a sibling DB is absent.

package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// timelineEvent is one cited line on the customer timeline.
type timelineEvent struct {
	TS        string `json:"ts"`          // Slack-style epoch string for sorting
	When      string `json:"when"`        // RFC3339 human-readable
	Source    string `json:"source"`      // "slack" | "attio" | "asana" | "fathom"
	Kind      string `json:"kind"`        // "mention" | "deal_stage" | "task" | "action_item"
	Summary   string `json:"summary"`
	Permalink string `json:"permalink,omitempty"`
}

// customerIntelReport is the full JSON shape of `customer-intel-deep`.
type customerIntelReport struct {
	Customer       string          `json:"customer"`
	Window         string          `json:"window"`
	EventCount     int             `json:"event_count"`
	Timeline       []timelineEvent `json:"timeline"`
	MissingSources []string        `json:"missing_sources"`
}

// slackTimelineEvents converts FTS message hits for the customer name
// into timeline events. This is the always-real portion of the verb.
func slackTimelineEvents(msgs []store.Message) []timelineEvent {
	out := make([]timelineEvent, 0, len(msgs))
	for _, m := range msgs {
		summary := m.Text
		if summary == "" {
			summary = "(no text)"
		}
		out = append(out, timelineEvent{
			TS:        m.TS,
			When:      tsToTime(m.TS).Format("2006-01-02T15:04:05Z"),
			Source:    "slack",
			Kind:      "mention",
			Summary:   summary,
			Permalink: m.Permalink,
		})
	}
	return out
}

// attachAttioStage queries the attached Attio mirror for the customer's
// current deal stage. The Attio schema is not known precisely, so the
// query is defensive: probeQuery demotes the source on any schema
// mismatch and the verb continues with Slack-only data.
func attachAttioStage(ctx context.Context, cs *crossSource, customer string) []timelineEvent {
	// Best-effort: a deals table keyed by company name with a stage
	// column. If the sibling schema differs, probeQuery fails cleanly.
	q := `SELECT name, stage, updated_at FROM x_attio.deals
	      WHERE name LIKE '%' || ? || '%' COLLATE NOCASE LIMIT 25`
	rows, ok := cs.probeQuery(ctx, "attio", q, customer)
	if !ok {
		return nil
	}
	defer rows.Close()
	var out []timelineEvent
	for rows.Next() {
		var name, stage, updated string
		if err := rows.Scan(&name, &stage, &updated); err != nil {
			break
		}
		out = append(out, timelineEvent{
			TS:      updated,
			When:    updated,
			Source:  "attio",
			Kind:    "deal_stage",
			Summary: fmt.Sprintf("%s — stage: %s", name, stage),
		})
	}
	return out
}

// attachAsanaTasks queries the attached Asana mirror for open tasks
// mentioning the customer. Defensive against unknown sibling schema.
func attachAsanaTasks(ctx context.Context, cs *crossSource, customer string) []timelineEvent {
	q := `SELECT name, completed, created_at FROM x_asana.tasks
	      WHERE name LIKE '%' || ? || '%' COLLATE NOCASE
	        AND (completed = 0 OR completed IS NULL) LIMIT 25`
	rows, ok := cs.probeQuery(ctx, "asana", q, customer)
	if !ok {
		return nil
	}
	defer rows.Close()
	var out []timelineEvent
	for rows.Next() {
		var name, created string
		var completed interface{}
		if err := rows.Scan(&name, &completed, &created); err != nil {
			break
		}
		out = append(out, timelineEvent{
			TS:      created,
			When:    created,
			Source:  "asana",
			Kind:    "task",
			Summary: "open task: " + name,
		})
	}
	return out
}

// attachFathomActionItems queries the attached Fathom mirror for call
// action items mentioning the customer. Defensive against unknown schema.
func attachFathomActionItems(ctx context.Context, cs *crossSource, customer string) []timelineEvent {
	q := `SELECT description, created_at FROM x_fathom.action_items
	      WHERE description LIKE '%' || ? || '%' COLLATE NOCASE LIMIT 25`
	rows, ok := cs.probeQuery(ctx, "fathom", q, customer)
	if !ok {
		return nil
	}
	defer rows.Close()
	var out []timelineEvent
	for rows.Next() {
		var desc, created string
		if err := rows.Scan(&desc, &created); err != nil {
			break
		}
		out = append(out, timelineEvent{
			TS:      created,
			When:    created,
			Source:  "fathom",
			Kind:    "action_item",
			Summary: "call action item: " + desc,
		})
	}
	return out
}

func newCustomerIntelDeepCmd(flags *rootFlags) *cobra.Command {
	var window, dbPath string
	var skipMissing bool
	var limit int

	cmd := &cobra.Command{
		Use:   "customer-intel-deep [name]",
		Short: "One-screen time-ordered cross-source timeline for a customer",
		Long: `customer-intel-deep builds a single time-ordered timeline for a
customer: Slack mentions, Attio deal stage, open Asana tasks and Fathom
call action items — each line cited with a permalink where available.

The Slack portion is read from this CLI's local mirror. The Attio,
Asana and Fathom portions are read from the sibling pp-* CLIs' SQLite
mirrors via ATTACH DATABASE. When a sibling mirror is absent the verb
degrades gracefully: it emits the Slack-only timeline and lists the
unavailable sources in 'missing_sources'. Use --skip-missing to make
that degradation explicit (it is the default behaviour either way).

No live Slack calls are made.`,
		Example: stringTrimNL(`
  # 14-day cross-source timeline for Sonria
  slack-pp-cli customer-intel-deep "Sonria" --window 14d --agent

  # Slack-only when sibling mirrors are not present
  slack-pp-cli customer-intel-deep "Sonria" --skip-missing --json

  # Preview without touching any database
  slack-pp-cli customer-intel-deep "Sonria" --dry-run`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			customer := args[0]
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

			// --- Slack side (always real) ---
			msgs, err := db.SearchMessages(cmd.Context(), customer, nil, 200)
			if err != nil {
				return fmt.Errorf("searching slack mirror: %w", err)
			}
			// Window-filter the FTS hits.
			var inWindow []store.Message
			for _, m := range msgs {
				if since != "" && m.TS < since {
					continue
				}
				if until != "" && m.TS > until {
					continue
				}
				inWindow = append(inWindow, m)
			}
			events := slackTimelineEvents(inWindow)

			// --- Cross-source sides (graceful degradation) ---
			cs, _ := newCrossSource(cmd.Context(), db.DB(), map[string]string{
				"attio":  "attio-pp-cli",
				"asana":  "asana-pp-cli",
				"fathom": "fathom-pp-cli",
			})
			defer cs.detach(cmd.Context())
			events = append(events, attachAttioStage(cmd.Context(), cs, customer)...)
			events = append(events, attachAsanaTasks(cmd.Context(), cs, customer)...)
			events = append(events, attachFathomActionItems(cmd.Context(), cs, customer)...)

			// Time-order the merged timeline. tsToTime tolerates the
			// non-Slack ts formats; events with an unparseable ts sort
			// to the front, which is acceptable for a digest.
			sort.SliceStable(events, func(i, j int) bool {
				return tsToTime(events[i].TS).Before(tsToTime(events[j].TS))
			})
			if limit > 0 && len(events) > limit {
				events = events[len(events)-limit:]
			}

			missing := cs.missing()
			if missing == nil {
				missing = []string{}
			}
			_ = skipMissing // flag is informational; degradation is unconditional

			report := customerIntelReport{
				Customer:       customer,
				Window:         window,
				EventCount:     len(events),
				Timeline:       events,
				MissingSources: missing,
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "14d", "Time window (e.g. 14d, 7d, 30d)")
	cmd.Flags().BoolVar(&skipMissing, "skip-missing", false, "Degrade gracefully when a sibling mirror is absent (default behaviour)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum timeline events to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}

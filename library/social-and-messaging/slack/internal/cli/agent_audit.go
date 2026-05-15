// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It implements
// `agent-audit` — a privacy-governance report over the CLI's own
// append-only m_audit_log: which agents/cron jobs read which Slack
// DMs/channels over a window.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// auditCallerSummary aggregates one caller's reads inside the window.
type auditCallerSummary struct {
	Caller    string   `json:"caller"`
	Reads     int      `json:"reads"`
	Channels  []string `json:"channels"`
	Verbs     []string `json:"verbs"`
	LastReadAt string  `json:"last_read_at"`
}

// agentAuditReport is the full JSON shape of `agent-audit`.
type agentAuditReport struct {
	Window      string               `json:"window"`
	TotalReads  int                  `json:"total_reads"`
	Callers     []auditCallerSummary `json:"callers"`
	Entries     []store.AuditEntry   `json:"entries"`
}

// summarizeAuditLog rolls up audit entries into per-caller summaries.
// since is a wall-clock cutoff; entries older than it are dropped. Pure
// logic — unit-tested independently of the store.
func summarizeAuditLog(entries []store.AuditEntry, since time.Time) agentAuditReport {
	var kept []store.AuditEntry
	for _, e := range entries {
		if !since.IsZero() && e.TS.Before(since) {
			continue
		}
		kept = append(kept, e)
	}

	byCaller := map[string]*auditCallerSummary{}
	chanSeen := map[string]map[string]bool{}
	verbSeen := map[string]map[string]bool{}
	for _, e := range kept {
		c := e.Caller
		if c == "" {
			c = "(unknown)"
		}
		s := byCaller[c]
		if s == nil {
			s = &auditCallerSummary{Caller: c}
			byCaller[c] = s
			chanSeen[c] = map[string]bool{}
			verbSeen[c] = map[string]bool{}
		}
		s.Reads++
		if ts := e.TS.UTC().Format(time.RFC3339); ts > s.LastReadAt {
			s.LastReadAt = ts
		}
		if e.ChannelID != "" && !chanSeen[c][e.ChannelID] {
			chanSeen[c][e.ChannelID] = true
			s.Channels = append(s.Channels, e.ChannelID)
		}
		if e.Verb != "" && !verbSeen[c][e.Verb] {
			verbSeen[c][e.Verb] = true
			s.Verbs = append(s.Verbs, e.Verb)
		}
	}

	callers := make([]auditCallerSummary, 0, len(byCaller))
	for _, s := range byCaller {
		sort.Strings(s.Channels)
		sort.Strings(s.Verbs)
		callers = append(callers, *s)
	}
	// Most-active caller first; ties broken by name for stability.
	sort.SliceStable(callers, func(i, j int) bool {
		if callers[i].Reads != callers[j].Reads {
			return callers[i].Reads > callers[j].Reads
		}
		return callers[i].Caller < callers[j].Caller
	})

	return agentAuditReport{
		TotalReads: len(kept),
		Callers:    callers,
		Entries:    kept,
	}
}

func newAgentAuditCmd(flags *rootFlags) *cobra.Command {
	var window, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "agent-audit",
		Short: "Show which agents/cron jobs read which Slack DMs and channels",
		Long: `agent-audit reports privacy governance: every agent or cron job that
read DM/MPIM content, sourced from this CLI's own append-only audit log
(m_audit_log). Each P2 verb that reads a DM/MPIM appends a row, so the
log is a faithful record of automated access to private conversations.

All data is read from the local mirror — no live Slack calls are made.`,
		Example: stringTrimNL(`
  # Audit reads in the last 7 days
  slack-pp-cli agent-audit --window 7d --agent

  # Last 30 days, JSON for piping
  slack-pp-cli agent-audit --window 30d --json

  # Preview without touching the database
  slack-pp-cli agent-audit --dry-run`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			var since time.Time
			if window != "" {
				t, err := parseSinceDuration(window)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --window value %q: %w", window, err))
				}
				since = t
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'slack-pp-cli sync mirror' first.", err)
			}
			defer db.Close()

			entries, err := db.AuditLog(cmd.Context(), limit)
			if err != nil {
				return fmt.Errorf("reading audit log: %w", err)
			}
			report := summarizeAuditLog(entries, since)
			report.Window = window
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "7d", "Time window (e.g. 7d, 30d, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum audit-log rows to scan")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}

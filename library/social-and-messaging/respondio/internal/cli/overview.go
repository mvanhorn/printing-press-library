// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command overview: inbox workload summary from the local mirror.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/internal/store"
	"github.com/spf13/cobra"
)

type agentLoad struct {
	Agent string `json:"agent"`
	Count int    `json:"count"`
}

type overviewView struct {
	TotalContacts       int         `json:"total_contacts"`
	OpenConversations   int         `json:"open_conversations"`
	ClosedConversations int         `json:"closed_conversations"`
	UnassignedOpen      int         `json:"unassigned_open"`
	WithAssignee        int         `json:"with_assignee"`
	RecentActivity      int         `json:"recent_activity"`
	ActivityWindowDays  int         `json:"activity_window_days"`
	PerAgent            []agentLoad `json:"per_agent"`
}

// pp:data-source local

func newNovelOverviewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var limit int

	cmd := &cobra.Command{
		Use:         "overview",
		Short:       "One-command summary of open conversations, unassigned contacts, per-agent distribution, and recent activity.",
		Long:        "Summarizes the inbox from the local mirror: total contacts, open/closed conversations, unassigned open contacts, per-agent load, and recent activity within a window.",
		Example:     "  respondio-pp-cli overview --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "overview")
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("respondio-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: respondio-pp-cli sync --resources contact --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), emptyOverview(days), flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No synced contacts yet.")
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			view := emptyOverview(days)
			now := time.Now().Unix()
			window := time.Duration(days) * 24 * time.Hour
			agentIdx := map[string]int{}

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'contact'`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			var datas [][]byte
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan contact: %w", err)
				}
				datas = append(datas, data)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate contacts: %w", err)
			}
			_ = rows.Close()

			for _, raw := range datas {
				var c map[string]any
				if err := json.Unmarshal(raw, &c); err != nil {
					continue
				}
				view.TotalContacts++
				status, _ := c["status"].(string)
				switch status {
				case "open":
					view.OpenConversations++
				case "close", "closed":
					view.ClosedConversations++
				}
				if assignee, ok := c["assignee"].(map[string]any); ok && assignee != nil {
					view.WithAssignee++
					agent := agentName(assignee)
					agentIdx[agent]++
					if status == "open" {
						// counted under per-agent below
					}
				} else if status == "open" {
					view.UnassignedOpen++
				}
				if lmt, ok := c["last_message_time"].(float64); ok && int64(lmt) > now-int64(window.Seconds()) {
					view.RecentActivity++
				}
			}

			for agent, count := range agentIdx {
				view.PerAgent = append(view.PerAgent, agentLoad{Agent: agent, Count: count})
			}
			sort.Slice(view.PerAgent, func(i, j int) bool { return view.PerAgent[i].Count > view.PerAgent[j].Count })
			if limit > 0 && len(view.PerAgent) > limit {
				view.PerAgent = view.PerAgent[:limit]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Contacts: %d  Open: %d  Closed: %d  UnassignedOpen: %d  WithAssignee: %d\n", view.TotalContacts, view.OpenConversations, view.ClosedConversations, view.UnassignedOpen, view.WithAssignee)
			for _, a := range view.PerAgent {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %d\n", a.Agent, a.Count)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "recent activity window in days")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum agents to list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func emptyOverview(days int) overviewView {
	return overviewView{PerAgent: make([]agentLoad, 0), ActivityWindowDays: days}
}

func agentName(a map[string]any) string {
	if email, ok := a["email"].(string); ok && email != "" {
		return email
	}
	fn, _ := a["firstName"].(string)
	ln, _ := a["lastName"].(string)
	return fmt.Sprintf("%s %s", fn, ln)
}

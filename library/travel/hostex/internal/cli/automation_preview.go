// Hand-authored transcendence command. generate --force preserves this file.
// pp:data-source local
package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelAutomationPreviewCmd(flags *rootFlags) *cobra.Command {
	var flagDay string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "automation-preview",
		Short: "List pending automated messages/reviews and flag any whose thread shows recent activity.",
		Long: "Lists locally synced pending automation actions (scheduled messages and\n" +
			"reviews) and joins each to its stay's conversation so you can spot queued bot\n" +
			"actions on threads that already saw recent activity before they fire.\n\n" +
			"Reads the local mirror; run `hostex-pp-cli sync --resources automation,reservations,conversations`\n" +
			"first. To find slow human threads use `inbox-sla` instead.",
		Example:     "  hostex-pp-cli automation-preview --day tomorrow --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}

			now := nowUTC()
			var dayFilter string // "" means no filter
			switch strings.ToLower(strings.TrimSpace(flagDay)) {
			case "":
				dayFilter = ""
			case "today":
				dayFilter = now.Format("2006-01-02")
			case "tomorrow":
				dayFilter = now.Add(24 * time.Hour).Format("2006-01-02")
			default:
				if t, err := time.Parse("2006-01-02", strings.TrimSpace(flagDay)); err == nil {
					dayFilter = t.Format("2006-01-02")
				} else {
					return usageErr(fmt.Errorf("--day must be 'today', 'tomorrow', or YYYY-MM-DD"))
				}
			}

			db, done, err := openLocalMirror(cmd, flags, flagDB)
			if done {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "automation") {
				hintIfStale(cmd, db, "automation", flags.maxAge)
			}

			actions, err := listObjs(db, "automation")
			if err != nil {
				return err
			}
			reservations, _ := listObjs(db, "reservations")
			conversations, _ := listObjs(db, "conversations")

			// stay_code -> conversation_id (via reservations)
			stayConv := map[string]string{}
			for _, r := range reservations {
				if sc := novStr(r, "stay_code"); sc != "" {
					if cid := novStr(r, "conversation_id"); cid != "" {
						stayConv[sc] = cid
					}
				}
			}
			// conversation_id -> last_message_at
			convLast := map[string]time.Time{}
			for _, c := range conversations {
				if id := novStr(c, "id"); id != "" {
					if lm, ok := novTime(c["last_message_at"]); ok {
						convLast[id] = lm
					}
				}
			}

			type actRow struct {
				ID                any    `json:"id"`
				Type              string `json:"type,omitempty"`
				Name              string `json:"name,omitempty"`
				StayCode          string `json:"stay_code,omitempty"`
				PropertyTitle     string `json:"property_title,omitempty"`
				ChannelType       string `json:"channel_type,omitempty"`
				ScheduledAt       string `json:"scheduled_at,omitempty"`
				ThreadLastMessage string `json:"thread_last_message_at,omitempty"`
				PossiblyHandled   bool   `json:"possibly_handled"`
			}

			rows := make([]actRow, 0)
			scanned := 0
			for _, a := range actions {
				scanned++
				if dayFilter != "" {
					if st, ok := novTime(a["scheduled_at"]); !ok || st.Format("2006-01-02") != dayFilter {
						continue
					}
				}
				sc := novStr(a, "stay_code")
				threadLast := ""
				possibly := false
				if cid, ok := stayConv[sc]; ok {
					if lm, ok2 := convLast[cid]; ok2 {
						threadLast = lm.Format(time.RFC3339)
						// Recent thread activity (last 24h) suggests a human may
						// already be handling this conversation.
						if d := now.Sub(lm); d >= 0 && d <= 24*time.Hour {
							possibly = true
						}
					}
				}
				rows = append(rows, actRow{
					ID:                a["id"],
					Type:              novStr(a, "type"),
					Name:              novStr(a, "name"),
					StayCode:          sc,
					PropertyTitle:     novStr(a, "property_title"),
					ChannelType:       novStr(a, "channel_type"),
					ScheduledAt:       novStr(a, "scheduled_at"),
					ThreadLastMessage: threadLast,
					PossiblyHandled:   possibly,
				})
			}

			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].ScheduledAt < rows[j].ScheduledAt
			})

			view := struct {
				Day            string   `json:"day,omitempty"`
				ScannedActions int      `json:"scanned_actions"`
				PendingShown   int      `json:"pending_shown"`
				Actions        []actRow `json:"actions"`
			}{
				Day:            flagDay,
				ScannedActions: scanned,
				PendingShown:   len(rows),
				Actions:        rows,
			}
			return novEmit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagDay, "day", "", "Restrict to actions scheduled on 'today', 'tomorrow', or YYYY-MM-DD")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite mirror (default: per-user data dir)")
	return cmd
}

// Hand-built (Phase 3): four narrowly-scoped "view" commands over the local
// messages store: since, flagged, stale-unread, conversations. They share
// the loadMessages helper from novel_messages.go and report total counts
// snapshotted before any --top/--limit truncation (PR #408 P1 lesson).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// --- since ---

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		windowArg string
		focused   bool
		other     bool
		folder    string
		sender    string
		top       int
		groupBy   string
	)
	cmd := &cobra.Command{
		Use:   "since [when]",
		Short: "Show what arrived since a timestamp, grouped by focused/other, sender, or folder",
		Long: strings.TrimSpace(`
Reads the local SQLite store populated by ` + "`sync`" + ` and reports messages whose
receivedDateTime is after the given window. Useful for "catch-up": when an agent
joins a session, run ` + "`since '2 hours ago'`" + ` and surface only what landed in that
window, grouped by inferenceClassification, sender, or parent folder.

The window accepts:
- relative durations: "30d", "12h", "2 hours ago", "30 days ago"
- ISO timestamps: "2026-05-10T08:00:00Z"
- a date: "2026-05-10"
- "last-sync" — reads store.GetLastSyncedAt("messages") rather than hardcoding 24h.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli since "2 hours ago" --agent
  outlook-email-pp-cli since 12h --focused --agent
  outlook-email-pp-cli since 2026-05-10 --group-by sender --top 20 --agent
  outlook-email-pp-cli since last-sync --agent
`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && windowArg == "" {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			value := windowArg
			if value == "" {
				value = args[0]
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			st, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer st.Close()
			cutoff, err := resolveSinceWindow(value, st, "messages")
			if err != nil {
				return usageErr(err)
			}
			f := loadMessagesFilter{
				ReceivedAfter: cutoff,
				ExcludeDrafts: true,
				OrderBy:       "received_date_time DESC",
			}
			if focused {
				f.Inference = "focused"
			} else if other {
				f.Inference = "other"
			}
			if folder != "" {
				f.Folders = []string{folder}
			}
			if sender != "" {
				f.Senders = []string{sender}
			}
			rows, err := loadMessages(ctx, st.DB(), f)
			if err != nil {
				return apiErr(err)
			}
			totalCount := len(rows)
			// PATCH: compute groups across the full result set BEFORE --top truncation; otherwise the groups map silently disagrees with total_count.
			grouped := groupSince(rows, groupBy)
			if top > 0 && len(rows) > top {
				rows = rows[:top]
			}
			out := map[string]any{
				"window_start": cutoff.UTC().Format(time.RFC3339),
				"total_count":  totalCount,
				"items":        rows,
				"groups":       grouped,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&windowArg, "window", "", "Window (relative duration, ISO timestamp, date, or 'last-sync'); overrides positional")
	cmd.Flags().BoolVar(&focused, "focused", false, "Only Focused-inbox messages")
	cmd.Flags().BoolVar(&other, "other", false, "Only Other-inbox messages")
	cmd.Flags().StringVar(&folder, "folder", "", "Restrict to a specific parent folder id")
	cmd.Flags().StringVar(&sender, "sender", "", "Restrict to a specific sender address")
	cmd.Flags().IntVar(&top, "top", 0, "Cap the items[] list (does not affect total_count)")
	cmd.Flags().StringVar(&groupBy, "group-by", "inference", "Group by 'inference' | 'sender' | 'folder' | 'none'")
	return cmd
}

func groupSince(rows []messageRow, mode string) map[string]int {
	out := map[string]int{}
	for _, r := range rows {
		var k string
		switch strings.ToLower(mode) {
		case "sender":
			k = r.FromEmail
		case "folder":
			k = r.ParentFolderID
		case "none":
			continue
		default: // inference
			k = strings.ToLower(r.InferenceClassification)
			if k == "" {
				k = "unknown"
			}
		}
		if k == "" {
			k = "unknown"
		}
		out[k]++
	}
	return out
}

// --- flagged ---

func newFlaggedCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		overdueOnly bool
		top       int
		windowArg string
	)
	cmd := &cobra.Command{
		Use:   "flagged",
		Short: "List flagged messages with open due dates, days overdue, age",
		Long: strings.TrimSpace(`
Reports messages whose flag.flagStatus is "flagged" AND completedDateTime is
unset. Includes due-date diff (days_overdue: positive when past due, negative
when upcoming) and total age. The inbox-zero work list that Outlook's own UI
never aggregates.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli flagged --agent
  outlook-email-pp-cli flagged --overdue --agent
  outlook-email-pp-cli flagged --since 30d --top 50 --agent
`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			st, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer st.Close()
			cutoff, err := resolveSinceWindow(windowArg, st, "messages")
			if err != nil {
				return usageErr(err)
			}
			f := loadMessagesFilter{
				IncompleteFlag: true,
				ReceivedAfter:  cutoff,
				ExcludeDrafts:  true,
				OrderBy:        "received_date_time DESC",
			}
			rows, err := loadMessages(ctx, st.DB(), f)
			if err != nil {
				return apiErr(err)
			}
			now := time.Now().UTC()
			type item struct {
				ID             string    `json:"id"`
				Subject        string    `json:"subject"`
				From           string    `json:"from"`
				ReceivedAt     time.Time `json:"received_at"`
				FlagStartAt    time.Time `json:"flag_start_at,omitempty"`
				FlagDueAt      time.Time `json:"flag_due_at,omitempty"`
				DaysOverdue    *int      `json:"days_overdue,omitempty"`
				AgeDays        int       `json:"age_days"`
				IsRead         bool      `json:"is_read"`
				ParentFolderID string    `json:"parent_folder_id,omitempty"`
				WebLink        string    `json:"web_link,omitempty"`
			}
			items := make([]item, 0, len(rows))
			for _, r := range rows {
				age := int(now.Sub(r.ReceivedAt).Hours() / 24)
				if r.ReceivedAt.IsZero() {
					age = 0
				}
				it := item{
					ID:             r.ID,
					Subject:        r.Subject,
					From:           r.FromEmail,
					ReceivedAt:     r.ReceivedAt,
					FlagStartAt:    r.FlagStartAt,
					FlagDueAt:      r.FlagDueAt,
					AgeDays:        age,
					IsRead:         r.IsRead,
					ParentFolderID: r.ParentFolderID,
					WebLink:        r.WebLink,
				}
				if !r.FlagDueAt.IsZero() {
					od := int(now.Sub(r.FlagDueAt).Hours() / 24)
					it.DaysOverdue = &od
					if overdueOnly && od < 0 {
						continue
					}
				} else if overdueOnly {
					continue
				}
				items = append(items, it)
			}
			// Sort by days_overdue DESC (most overdue first), then age DESC.
			sort.Slice(items, func(i, j int) bool {
				di, dj := 0, 0
				if items[i].DaysOverdue != nil {
					di = *items[i].DaysOverdue
				}
				if items[j].DaysOverdue != nil {
					dj = *items[j].DaysOverdue
				}
				if di != dj {
					return di > dj
				}
				return items[i].AgeDays > items[j].AgeDays
			})
			totalCount := len(items)
			if top > 0 && len(items) > top {
				items = items[:top]
			}
			out := map[string]any{
				"count":  totalCount, // PR #408 P1: snapshot BEFORE truncation
				"items":  items,
				"as_of":  now.Format(time.RFC3339),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&overdueOnly, "overdue", false, "Only items past their dueDateTime")
	cmd.Flags().IntVar(&top, "top", 0, "Cap the items[] list (does not affect count)")
	cmd.Flags().StringVar(&windowArg, "since", "", "Only flagged messages received after this point (e.g. '30d', '2026-04-01', 'last-sync')")
	return cmd
}

// --- stale-unread ---

func newStaleUnreadCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		days   int
		top    int
	)
	cmd := &cobra.Command{
		Use:   "stale-unread",
		Short: "Unread messages older than N days, grouped by folder",
		Long: strings.TrimSpace(`
Reports messages where is_read=0 AND received_date_time < now - N days, grouped
by parent_folder_id. Surfaces unread debt hiding in subfolders that the UI's
"by date" sort never lets you see.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli stale-unread --days 14 --agent
  outlook-email-pp-cli stale-unread --days 30 --top 200 --agent
`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if days < 0 {
				return usageErr(fmt.Errorf("--days must be >= 0"))
			}
			ctx := cmd.Context()
			st, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer st.Close()
			isFalse := false
			cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
			f := loadMessagesFilter{
				ReceivedBefore: cutoff,
				IsRead:         &isFalse,
				ExcludeDrafts:  true,
				OrderBy:        "received_date_time ASC",
			}
			rows, err := loadMessages(ctx, st.DB(), f)
			if err != nil {
				return apiErr(err)
			}
			perFolder := map[string]int{}
			for _, r := range rows {
				k := r.ParentFolderID
				if k == "" {
					k = "(unknown)"
				}
				perFolder[k]++
			}
			totalCount := len(rows)
			if top > 0 && len(rows) > top {
				rows = rows[:top]
			}
			out := map[string]any{
				"count":         totalCount,
				"cutoff":        cutoff.Format(time.RFC3339),
				"folder_counts": perFolder,
				"items":         rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 14, "Age threshold in days; messages older than this are reported")
	cmd.Flags().IntVar(&top, "top", 0, "Cap the items[] list (does not affect count)")
	return cmd
}

// --- conversations ---

func newConversationsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		window string
		top    int
	)
	cmd := &cobra.Command{
		Use:   "conversations",
		Short: "Top conversations by message count and unread tail",
		Long: strings.TrimSpace(`
Aggregates messages by conversation_id over the given window and ranks them by
message count (then unread count, then last-received). Useful for finding the
threads burning an inbox-zero attention budget. No Graph endpoint aggregates
conversations — this is local-store-only.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli conversations --top 20 --window 30d --agent
  outlook-email-pp-cli conversations --window 7d --agent
`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			st, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer st.Close()
			cutoff, err := resolveSinceWindow(window, st, "messages")
			if err != nil {
				return usageErr(err)
			}
			f := loadMessagesFilter{
				ReceivedAfter: cutoff,
				ExcludeDrafts: true,
				OrderBy:       "received_date_time DESC",
			}
			rows, err := loadMessages(ctx, st.DB(), f)
			if err != nil {
				return apiErr(err)
			}
			type convo struct {
				ID            string    `json:"conversation_id"`
				Subject       string    `json:"subject"`
				MessageCount  int       `json:"message_count"`
				UnreadCount   int       `json:"unread_count"`
				Participants  []string  `json:"participants"`
				FirstAt       time.Time `json:"first_at"`
				LastAt        time.Time `json:"last_at"`
				LastFrom      string    `json:"last_from"`
				LastIsFromMe  bool      `json:"last_is_from_me"`
				FlaggedAny    bool      `json:"flagged_any"`
			}
			byID := map[string]*convo{}
			pset := map[string]map[string]struct{}{}
			me, _ := myAddress(st.DB())
			for _, r := range rows {
				if r.ConversationID == "" {
					continue
				}
				c, ok := byID[r.ConversationID]
				if !ok {
					c = &convo{ID: r.ConversationID, Subject: r.Subject, FirstAt: r.ReceivedAt}
					byID[r.ConversationID] = c
					pset[r.ConversationID] = map[string]struct{}{}
				}
				c.MessageCount++
				if !r.IsRead {
					c.UnreadCount++
				}
				if r.FlagStatus == "flagged" {
					c.FlaggedAny = true
				}
				if r.ReceivedAt.Before(c.FirstAt) || c.FirstAt.IsZero() {
					c.FirstAt = r.ReceivedAt
				}
				if r.ReceivedAt.After(c.LastAt) {
					c.LastAt = r.ReceivedAt
					c.LastFrom = r.FromEmail
					c.LastIsFromMe = (me != "" && strings.EqualFold(r.FromEmail, me))
				}
				if r.FromEmail != "" {
					pset[r.ConversationID][r.FromEmail] = struct{}{}
				}
			}
			out := make([]*convo, 0, len(byID))
			for id, c := range byID {
				for p := range pset[id] {
					c.Participants = append(c.Participants, p)
				}
				sort.Strings(c.Participants)
				out = append(out, c)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].MessageCount != out[j].MessageCount {
					return out[i].MessageCount > out[j].MessageCount
				}
				if out[i].UnreadCount != out[j].UnreadCount {
					return out[i].UnreadCount > out[j].UnreadCount
				}
				return out[i].LastAt.After(out[j].LastAt)
			})
			totalCount := len(out)
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			env := map[string]any{
				"count":        totalCount,
				"window_start": cutoff.Format(time.RFC3339),
				"items":        out,
				"me":           me,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&window, "window", "30d", "Window (relative duration, ISO timestamp, date, or 'last-sync')")
	cmd.Flags().IntVar(&top, "top", 20, "Cap the items[] list (does not affect count)")
	return cmd
}

// ensure JSON-encoder import is exercised in this file (used indirectly via
// printJSONFiltered's encoder). Keeps imports tidy across the four helpers.
var _ = json.Marshal
var _ = context.Canceled

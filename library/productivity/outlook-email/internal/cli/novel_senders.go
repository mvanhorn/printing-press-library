// Hand-built (Phase 3): senders + quiet — sender-centric aggregates over the
// local messages store. Both push the window into SQL; both snapshot total
// counts before any --top truncation.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newSendersCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		window string
		min    int
		top    int
	)
	cmd := &cobra.Command{
		Use:   "senders",
		Short: "Volume rollup by sender over a window (count, unread, last-received, dominant folder)",
		Long: strings.TrimSpace(`
Groups the local messages store by from.emailAddress.address over the
specified window and reports per-sender:
  count, unread_count, last_received_at, first_received_at, dominant_folder,
  has_unsubscribe_header (heuristic: subject or body_preview mentions
  unsubscribe/list-unsubscribe).
Sort order: count DESC, then unread DESC.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli senders --window 30d --min 5 --agent
  outlook-email-pp-cli senders --window 7d --top 10 --agent
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
			rows, err := loadMessages(ctx, st.DB(), loadMessagesFilter{
				ReceivedAfter: cutoff,
				ExcludeDrafts: true,
				OrderBy:       "received_date_time DESC",
			})
			if err != nil {
				return apiErr(err)
			}
			type senderRow struct {
				Sender          string    `json:"sender"`
				Name            string    `json:"display_name,omitempty"`
				Count           int       `json:"count"`
				Unread          int       `json:"unread_count"`
				FirstReceivedAt time.Time `json:"first_received_at"`
				LastReceivedAt  time.Time `json:"last_received_at"`
				DominantFolder  string    `json:"dominant_folder"`
				LikelyBulk      bool      `json:"likely_bulk"`
				FolderCounts    map[string]int `json:"folder_counts,omitempty"`
			}
			agg := map[string]*senderRow{}
			for _, r := range rows {
				if r.FromEmail == "" {
					continue
				}
				k := r.FromEmail
				row, ok := agg[k]
				if !ok {
					row = &senderRow{
						Sender:       k,
						Name:         r.FromName,
						FolderCounts: map[string]int{},
					}
					agg[k] = row
				}
				row.Count++
				if !r.IsRead {
					row.Unread++
				}
				if !r.ReceivedAt.IsZero() {
					if row.FirstReceivedAt.IsZero() || r.ReceivedAt.Before(row.FirstReceivedAt) {
						row.FirstReceivedAt = r.ReceivedAt
					}
					if r.ReceivedAt.After(row.LastReceivedAt) {
						row.LastReceivedAt = r.ReceivedAt
					}
				}
				if r.ParentFolderID != "" {
					row.FolderCounts[r.ParentFolderID]++
				}
				if !row.LikelyBulk {
					if hasUnsubscribeHints(r.Subject) || hasUnsubscribeHints(r.BodyPreview) {
						row.LikelyBulk = true
					}
				}
			}
			out := make([]*senderRow, 0, len(agg))
			for _, row := range agg {
				row.DominantFolder = topKey(row.FolderCounts)
				if row.Count >= min {
					out = append(out, row)
				}
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Count != out[j].Count {
					return out[i].Count > out[j].Count
				}
				if out[i].Unread != out[j].Unread {
					return out[i].Unread > out[j].Unread
				}
				return out[i].LastReceivedAt.After(out[j].LastReceivedAt)
			})
			totalCount := len(out)
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			env := map[string]any{
				"count":        totalCount,
				"window_start": cutoff.Format(time.RFC3339),
				"min":          min,
				"items":        out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&window, "window", "30d", "Window (relative duration, ISO timestamp, date, or 'last-sync')")
	cmd.Flags().IntVar(&min, "min", 5, "Minimum messages in window to include the sender")
	cmd.Flags().IntVar(&top, "top", 0, "Cap the items[] list (does not affect count)")
	return cmd
}

func newQuietCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		baseline string
		silent   string
		minMsgs  int
		top      int
	)
	cmd := &cobra.Command{
		Use:   "quiet",
		Short: "Senders you used to hear from but who've gone silent for N days",
		Long: strings.TrimSpace(`
Self-join over the local messages store: senders with at least --min-msgs
messages received in the baseline window (e.g. last 90 days) AND zero messages
received in the trailing silent window (e.g. last 30 days). Useful for
relationship-lapse detection (sales/PM/recruiter pattern).

Silent window must end at "now" and be no larger than the baseline window.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli quiet --baseline 90d --silent 30d --agent
  outlook-email-pp-cli quiet --baseline 180d --silent 60d --min-msgs 5 --agent
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
			baseCutoff, err := resolveSinceWindow(baseline, st, "messages")
			if err != nil {
				return usageErr(fmt.Errorf("--baseline: %w", err))
			}
			silentCutoff, err := resolveSinceWindow(silent, st, "messages")
			if err != nil {
				return usageErr(fmt.Errorf("--silent: %w", err))
			}
			if baseCutoff.IsZero() || silentCutoff.IsZero() {
				return usageErr(fmt.Errorf("--baseline and --silent must both resolve to a real window"))
			}
			if baseCutoff.After(silentCutoff) {
				return usageErr(fmt.Errorf("--baseline must reach further back in time than --silent (baseline=%s, silent=%s)", baseCutoff.Format(time.RFC3339), silentCutoff.Format(time.RFC3339)))
			}
			// Two queries, both bounded — first window is [baseline, silent),
			// second window is [silent, now]. Anything in the second window
			// disqualifies the sender from "quiet".
			activeRows, err := loadMessages(ctx, st.DB(), loadMessagesFilter{
				ReceivedAfter:  baseCutoff,
				ReceivedBefore: silentCutoff,
				ExcludeDrafts:  true,
				OrderBy:        "received_date_time DESC",
			})
			if err != nil {
				return apiErr(err)
			}
			silentRows, err := loadMessages(ctx, st.DB(), loadMessagesFilter{
				ReceivedAfter: silentCutoff,
				ExcludeDrafts: true,
				OrderBy:       "received_date_time DESC",
			})
			if err != nil {
				return apiErr(err)
			}
			active := map[string]struct {
				name string
				count int
				last  time.Time
			}{}
			for _, r := range activeRows {
				if r.FromEmail == "" {
					continue
				}
				e := active[r.FromEmail]
				e.name = r.FromName
				e.count++
				if r.ReceivedAt.After(e.last) {
					e.last = r.ReceivedAt
				}
				active[r.FromEmail] = e
			}
			silentSet := map[string]struct{}{}
			for _, r := range silentRows {
				if r.FromEmail != "" {
					silentSet[r.FromEmail] = struct{}{}
				}
			}
			type quietRow struct {
				Sender             string    `json:"sender"`
				Name               string    `json:"display_name,omitempty"`
				BaselineCount      int       `json:"baseline_count"`
				LastReceivedAt     time.Time `json:"last_received_at"`
				DaysSinceLast      int       `json:"days_since_last"`
			}
			now := time.Now().UTC()
			out := []quietRow{}
			for email, e := range active {
				if _, recentlyHeard := silentSet[email]; recentlyHeard {
					continue
				}
				if e.count < minMsgs {
					continue
				}
				dsl := 0
				if !e.last.IsZero() {
					dsl = int(now.Sub(e.last).Hours() / 24)
				}
				out = append(out, quietRow{Sender: email, Name: e.name, BaselineCount: e.count, LastReceivedAt: e.last, DaysSinceLast: dsl})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].BaselineCount != out[j].BaselineCount {
					return out[i].BaselineCount > out[j].BaselineCount
				}
				return out[i].DaysSinceLast > out[j].DaysSinceLast
			})
			totalCount := len(out)
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			env := map[string]any{
				"count":          totalCount,
				"baseline_start": baseCutoff.Format(time.RFC3339),
				"silent_start":   silentCutoff.Format(time.RFC3339),
				"items":          out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&baseline, "baseline", "90d", "Baseline window for active senders (e.g. '90d', '180d')")
	cmd.Flags().StringVar(&silent, "silent", "30d", "Trailing silent window (must be shorter than baseline)")
	cmd.Flags().IntVar(&minMsgs, "min-msgs", 3, "Minimum baseline messages to consider a sender 'active'")
	cmd.Flags().IntVar(&top, "top", 0, "Cap the items[] list (does not affect count)")
	return cmd
}

// --- shared helpers ---

func hasUnsubscribeHints(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "unsubscribe") || strings.Contains(s, "manage subscriptions") || strings.Contains(s, "list-unsubscribe")
}

func topKey(m map[string]int) string {
	best := ""
	bestN := 0
	for k, n := range m {
		if n > bestN {
			best = k
			bestN = n
		}
	}
	return best
}

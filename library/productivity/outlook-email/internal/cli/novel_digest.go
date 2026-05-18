// Hand-built (Phase 3): digest — one-shot daily summary aggregating received,
// sent, unread, flagged counts, top senders, top conversations, focused/other
// ratio for a specific calendar date. Pure aggregation over the local store;
// no live API calls.

package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		date   string
		topN   int
	)
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "One-shot daily summary: counts, top senders, top conversations, focused/other ratio",
		Long: strings.TrimSpace(`
Aggregates the local messages store for a single calendar date (default: today
in the local time zone). Output covers:

  - received_count / sent_count / unread_count / flagged_count
  - top_senders[--top]  (sender, count, unread)
  - top_conversations[--top]  (subject, message_count, last_from)
  - focused_ratio       (count_focused / received_count)
  - has_attachments_count
  - flagged_completed_count

Use as a cron-shaped morning brief. Run after `+"`sync`"+`.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli digest --agent
  outlook-email-pp-cli digest --date 2026-05-12 --agent
  outlook-email-pp-cli digest --top 10 --agent
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
			now := time.Now()
			day := now
			if date != "" {
				d, err := time.ParseInLocation("2006-01-02", date, now.Location())
				if err != nil {
					return usageErr(err)
				}
				day = d
			}
			start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, now.Location()).UTC()
			end := start.Add(24 * time.Hour)

			received, err := loadMessages(ctx, st.DB(), loadMessagesFilter{
				ReceivedAfter:  start,
				ReceivedBefore: end,
				ExcludeDrafts:  true,
				OrderBy:        "received_date_time DESC",
			})
			if err != nil {
				return apiErr(err)
			}
			sent, err := loadMessages(ctx, st.DB(), loadMessagesFilter{
				SentAfter:  start,
				SentBefore: end,
				OrderBy:    "sent_date_time DESC",
			})
			if err != nil {
				return apiErr(err)
			}
			me, _ := myAddress(st.DB())
			sentByMe := 0
			if me != "" {
				for _, r := range sent {
					if strings.EqualFold(r.FromEmail, me) {
						sentByMe++
					}
				}
			} else {
				sentByMe = len(sent)
			}

			type senderTop struct {
				Sender string `json:"sender"`
				Count  int    `json:"count"`
				Unread int    `json:"unread"`
			}
			type convTop struct {
				ConversationID string `json:"conversation_id"`
				Subject        string `json:"subject"`
				MessageCount   int    `json:"message_count"`
				LastFrom       string `json:"last_from"`
			}
			senderAgg := map[string]*senderTop{}
			convAgg := map[string]*convTop{}
			focused, otherCount, unread, flagged, flaggedDone, attachments := 0, 0, 0, 0, 0, 0
			for _, r := range received {
				if !r.IsRead {
					unread++
				}
				if r.HasAttachments {
					attachments++
				}
				switch strings.ToLower(r.InferenceClassification) {
				case "focused":
					focused++
				case "other":
					otherCount++
				}
				if r.FlagStatus == "flagged" {
					if r.FlagCompletedAt.IsZero() {
						flagged++
					} else {
						flaggedDone++
					}
				}
				if r.FromEmail != "" {
					s := senderAgg[r.FromEmail]
					if s == nil {
						s = &senderTop{Sender: r.FromEmail}
						senderAgg[r.FromEmail] = s
					}
					s.Count++
					if !r.IsRead {
						s.Unread++
					}
				}
				if r.ConversationID != "" {
					c := convAgg[r.ConversationID]
					if c == nil {
						c = &convTop{ConversationID: r.ConversationID, Subject: r.Subject, LastFrom: r.FromEmail}
						convAgg[r.ConversationID] = c
					}
					c.MessageCount++
					// Keep subject from the latest (rows are received DESC)
				}
			}
			senders := make([]*senderTop, 0, len(senderAgg))
			for _, v := range senderAgg {
				senders = append(senders, v)
			}
			sort.Slice(senders, func(i, j int) bool {
				if senders[i].Count != senders[j].Count {
					return senders[i].Count > senders[j].Count
				}
				return senders[i].Unread > senders[j].Unread
			})
			if topN > 0 && len(senders) > topN {
				senders = senders[:topN]
			}
			conversations := make([]*convTop, 0, len(convAgg))
			for _, v := range convAgg {
				conversations = append(conversations, v)
			}
			sort.Slice(conversations, func(i, j int) bool { return conversations[i].MessageCount > conversations[j].MessageCount })
			if topN > 0 && len(conversations) > topN {
				conversations = conversations[:topN]
			}

			ratio := 0.0
			if len(received) > 0 {
				ratio = float64(focused) / float64(len(received))
			}
			out := map[string]any{
				"date":                    day.Format("2006-01-02"),
				"received_count":          len(received),
				"sent_count":              sentByMe,
				"unread_count":            unread,
				"flagged_count":           flagged,
				"flagged_completed_count": flaggedDone,
				"focused_count":           focused,
				"other_count":             otherCount,
				"focused_ratio":           ratio,
				"has_attachments_count":   attachments,
				"top_senders":             senders,
				"top_conversations":       conversations,
				"me":                      me,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&date, "date", "", "Calendar date YYYY-MM-DD (default: today, local time)")
	cmd.Flags().IntVar(&topN, "top", 5, "Cap for top_senders and top_conversations lists")
	return cmd
}

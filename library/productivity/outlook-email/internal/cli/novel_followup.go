// Hand-built (Phase 3): followup + waiting — symmetric "who owes whom a
// reply" reports over conversations in the local messages store.
//
// PR #408 lesson #4 (feature parity with advertised behavior):
// followup's help text says "the recipient never replied" — implement that
// exactly. We don't just list messages-I-sent-N-days-ago; we verify that no
// later message from that recipient exists in the same conversation.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newFollowupCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		days      int
		to        string
		top       int
		windowArg string
	)
	cmd := &cobra.Command{
		Use:   "followup",
		Short: "Emails you sent more than N days ago that the recipient never replied to",
		Long: strings.TrimSpace(`
Joins local messages where from.emailAddress.address = me (resolved from the
sent folder) against later messages in the SAME conversation_id whose from
address matches one of the original recipients.

A sent message qualifies as "needs followup" when:
  - it was sent more than --days ago (default 7)
  - to a specific recipient (or any if --to is empty)
  - and there is NO later message in the same conversation from any of its
    recipients

No Graph endpoint exposes this; the local store is what makes the cross-thread
join possible.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli followup --days 7 --agent
  outlook-email-pp-cli followup --to person@example.com --days 14 --agent
  outlook-email-pp-cli followup --since 30d --top 50 --agent
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
			me, err := myAddress(st.DB())
			if err != nil {
				return apiErr(err)
			}
			if me == "" {
				// Local store has no sent-folder messages yet; report empty
				// rather than erroring so callers can chain this command
				// safely in pipelines. The note tells the agent how to fix it.
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"count": 0,
					"me":    "",
					"items": []any{},
					"note":  "no sent-folder messages in local store; run `outlook-email-pp-cli sync` first to populate followup data",
				}, flags)
			}
			cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
			// Coarse SQL window: any messages I sent within the broader --since
			// envelope (default: matches --days). The Go-side filter then
			// enforces "no later reply" using per-conversation lookups.
			windowStart, err := resolveSinceWindow(windowArg, st, "messages")
			if err != nil {
				return usageErr(err)
			}
			if windowStart.IsZero() {
				windowStart = cutoff.AddDate(-1, 0, 0) // up to one year back of sent mail
			}
			// PATCH: drop redundant ExcludeDrafts:true (IsDraft=&false already adds the more restrictive `is_draft = 0`) and rename the misnamed `isTrue` to `isFalse`.
			isFalse := false
			rows, err := loadMessages(ctx, st.DB(), loadMessagesFilter{
				SentAfter:  windowStart,
				SentBefore: cutoff,
				IsDraft:    &isFalse, // exclude drafts: from.address may not match me yet
				Senders:    []string{me},
				OrderBy:    "sent_date_time DESC",
			})
			if err != nil {
				return apiErr(err)
			}
			toLower := strings.ToLower(strings.TrimSpace(to))
			type item struct {
				ID             string    `json:"id"`
				Subject        string    `json:"subject"`
				ConversationID string    `json:"conversation_id"`
				SentAt         time.Time `json:"sent_at"`
				DaysQuiet      int       `json:"days_quiet"`
				Recipients     []string  `json:"recipients"`
				WebLink        string    `json:"web_link,omitempty"`
			}
			out := []item{}
			now := time.Now().UTC()
			for _, r := range rows {
				if r.SentAt.IsZero() {
					continue
				}
				recipients := append([]string{}, r.ToEmails...)
				recipients = append(recipients, r.CcEmails...)
				if len(recipients) == 0 {
					continue
				}
				if toLower != "" {
					found := false
					for _, rec := range recipients {
						if strings.EqualFold(rec, toLower) {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
				// "No later reply" — check the local store for a message in
				// the same conversation with from address in `recipients` and
				// received_date_time > r.SentAt.
				replied, err := hasLaterReply(ctx, st.DB(), r.ConversationID, recipients, r.SentAt)
				if err != nil {
					return apiErr(err)
				}
				if replied {
					continue
				}
				out = append(out, item{
					ID:             r.ID,
					Subject:        r.Subject,
					ConversationID: r.ConversationID,
					SentAt:         r.SentAt,
					DaysQuiet:      int(now.Sub(r.SentAt).Hours() / 24),
					Recipients:     recipients,
					WebLink:        r.WebLink,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].DaysQuiet > out[j].DaysQuiet })
			totalCount := len(out)
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			env := map[string]any{
				"count":  totalCount,
				"me":     me,
				"cutoff": cutoff.Format(time.RFC3339),
				"items":  out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 7, "Minimum age (in days) of the original sent message")
	cmd.Flags().StringVar(&to, "to", "", "Limit to followups owed by a specific recipient address")
	cmd.Flags().IntVar(&top, "top", 0, "Cap the items[] list (does not affect count)")
	cmd.Flags().StringVar(&windowArg, "since", "", "Limit search to messages sent after this point (default: 1y back of sent mail)")
	return cmd
}

func hasLaterReply(ctx context.Context, db *sql.DB, conversationID string, recipients []string, sentAt time.Time) (bool, error) {
	if conversationID == "" || len(recipients) == 0 {
		return false, nil
	}
	lower := make([]string, len(recipients))
	for i, r := range recipients {
		lower[i] = strings.ToLower(strings.TrimSpace(r))
	}
	ph, args := inPlaceholders(lower)
	q := `SELECT 1 FROM messages
	       WHERE conversation_id = ?
	         AND received_date_time > ?
	         AND LOWER(json_extract(data, '$.from.emailAddress.address')) IN (` + ph + `)
	       LIMIT 1`
	queryArgs := append([]any{conversationID, sentAt.UTC().Format(time.RFC3339)}, args...)
	var dummy int
	err := db.QueryRowContext(ctx, q, queryArgs...).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// --- waiting ---

func newWaitingCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		days   int
		top    int
	)
	cmd := &cobra.Command{
		Use:   "waiting",
		Short: "Conversations where the last message is NOT from you, unread/unanswered for N days",
		Long: strings.TrimSpace(`
For every conversation in the local store, find the most-recent message. If
that message is from someone other than you AND it's been at least N days
since it arrived AND you haven't replied (i.e. no later message from you),
the conversation is "waiting on me".

Symmetric to followup. Pair both at start-of-day to see your half and their
half of every open thread.
`),
		Example: strings.TrimSpace(`
  outlook-email-pp-cli waiting --days 3 --agent
  outlook-email-pp-cli waiting --days 1 --top 20 --agent
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
			me, err := myAddress(st.DB())
			if err != nil {
				return apiErr(err)
			}
			if me == "" {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"count": 0,
					"me":    "",
					"items": []any{},
					"note":  "no sent-folder messages in local store; run `outlook-email-pp-cli sync` first to populate waiting data",
				}, flags)
			}
			cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
			// Coarse SQL window: messages received in the last ~year — keeps
			// scan bounded for active mailboxes. Conversations older than that
			// rarely need follow-up.
			windowStart := time.Now().UTC().AddDate(-1, 0, 0)
			rows, err := loadMessages(ctx, st.DB(), loadMessagesFilter{
				ReceivedAfter: windowStart,
				ExcludeDrafts: true,
				OrderBy:       "received_date_time DESC",
			})
			if err != nil {
				return apiErr(err)
			}
			// Group by conversation_id; remember the latest message per convo.
			type lastMsg struct {
				row messageRow
			}
			byConv := map[string]lastMsg{}
			for _, r := range rows {
				if r.ConversationID == "" {
					continue
				}
				cur, ok := byConv[r.ConversationID]
				if !ok || r.ReceivedAt.After(cur.row.ReceivedAt) {
					byConv[r.ConversationID] = lastMsg{row: r}
				}
			}
			now := time.Now().UTC()
			type item struct {
				ConversationID string    `json:"conversation_id"`
				Subject        string    `json:"subject"`
				LastFrom       string    `json:"last_from"`
				LastAt         time.Time `json:"last_at"`
				DaysWaiting    int       `json:"days_waiting"`
				IsRead         bool      `json:"is_read"`
				WebLink        string    `json:"web_link,omitempty"`
			}
			out := []item{}
			for _, lm := range byConv {
				r := lm.row
				if r.FromEmail == "" {
					continue
				}
				if strings.EqualFold(r.FromEmail, me) {
					continue // last message is from me; nothing to wait on
				}
				if !r.ReceivedAt.Before(cutoff) {
					continue // arrived too recently
				}
				out = append(out, item{
					ConversationID: r.ConversationID,
					Subject:        r.Subject,
					LastFrom:       r.FromEmail,
					LastAt:         r.ReceivedAt,
					DaysWaiting:    int(now.Sub(r.ReceivedAt).Hours() / 24),
					IsRead:         r.IsRead,
					WebLink:        r.WebLink,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].DaysWaiting > out[j].DaysWaiting })
			totalCount := len(out)
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			env := map[string]any{
				"count":  totalCount,
				"me":     me,
				"cutoff": cutoff.Format(time.RFC3339),
				"items":  out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 3, "Minimum days since the last message arrived")
	cmd.Flags().IntVar(&top, "top", 0, "Cap the items[] list (does not affect count)")
	return cmd
}

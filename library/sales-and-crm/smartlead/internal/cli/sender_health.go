// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// senderHealth ranks one email sender account by deliverability signals.
type senderHealth struct {
	AccountID     string   `json:"account_id"`
	FromEmail     string   `json:"from_email"`
	Connected     bool     `json:"connected"`
	InboxRate     float64  `json:"inbox_rate"`
	WarmupReply   float64  `json:"warmup_reply_rate"`
	DailySent     int      `json:"daily_sent"`
	DailyCap      int      `json:"daily_cap"`
	CampaignCount int      `json:"campaign_count"`
	Score         int      `json:"score"`
	Flags         []string `json:"flags,omitempty"`
}

func newSenderHealthCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "sender-health",
		Short: "Rank email sender accounts by a deliverability composite",
		Long: strings.Trim(`
Rank every email sender account by a composite of inbox-warmup landing rate,
SMTP/IMAP connection health, and sending utilization. For each account the
warmup-stats endpoint is fetched and the fleet is scored and sorted worst
first. The SmartLead API returns raw per-account stats only; the cross-account
ranking is computed here.`, "\n"),
		Example: strings.Trim(`
  smartlead-pp-cli sender-health
  smartlead-pp-cli sender-health --json
  smartlead-pp-cli sender-health --limit 25`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			accounts, err := fetchAllPaged(c, "/email-accounts", 100)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(accounts) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []senderHealth{}, flags)
			}

			var out []senderHealth
			for _, raw := range accounts {
				var acct map[string]any
				if json.Unmarshal(raw, &acct) != nil {
					continue
				}
				id := asString(acct["id"])
				if id == "" {
					continue
				}
				sh := senderHealth{
					AccountID:     id,
					FromEmail:     asString(acct["from_email"]),
					Connected:     asBool(acct["is_smtp_success"]) && asBool(acct["is_imap_success"]),
					DailySent:     asInt(acct["daily_sent_count"]),
					DailyCap:      asInt(acct["message_per_day"]),
					CampaignCount: asInt(acct["campaign_count"]),
				}

				// Warmup stats are per-account; a missing/empty response is
				// treated as "no warmup data" rather than a hard failure.
				inboxRate, warmupReply, _, haveWarmup := fetchWarmup(c, id)
				sh.InboxRate = inboxRate
				sh.WarmupReply = warmupReply

				score := inboxRate * 100
				if !sh.Connected {
					score -= 45
					sh.Flags = append(sh.Flags, "disconnected")
				}
				if !haveWarmup {
					score -= 15
					sh.Flags = append(sh.Flags, "no-warmup-data")
				} else if inboxRate < 0.85 {
					sh.Flags = append(sh.Flags, "low-inbox")
				}
				if sh.DailyCap > 0 && sh.DailySent > sh.DailyCap {
					sh.Flags = append(sh.Flags, "over-cap")
				}
				if score < 0 {
					score = 0
				}
				sh.Score = int(score + 0.5)
				out = append(out, sh)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Score != out[j].Score {
					return out[i].Score < out[j].Score
				}
				return out[i].FromEmail < out[j].FromEmail
			})

			// --limit selects the worst N accounts across the whole fleet,
			// so it applies after scoring and sorting. Filtering during the
			// build loop above would sample in arbitrary API order.
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SCORE\tACCOUNT\tCONN\tINBOX%\tWARMUP-REPLY%\tSENT/CAP\tFLAGS")
			for _, s := range out {
				conn := "down"
				if s.Connected {
					conn = "ok"
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%.1f\t%.1f\t%d/%d\t%s\n",
					s.Score, truncate(s.FromEmail, 36), conn, s.InboxRate*100,
					s.WarmupReply*100, s.DailySent, s.DailyCap, strings.Join(s.Flags, ","))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of accounts to score (0 = all)")
	return cmd
}

// fetchWarmup retrieves an account's warmup-stats and returns the inbox
// landing rate, the warmup reply rate, the number of days of warmup history,
// and whether warmup data was present.
func fetchWarmup(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, accountID string) (inboxRate, warmupReply float64, historyDays int, ok bool) {
	raw, err := c.Get("/email-accounts/"+accountID+"/warmup-stats", nil)
	if err != nil {
		return 0, 0, 0, false
	}
	var w map[string]any
	if json.Unmarshal(raw, &w) != nil {
		return 0, 0, 0, false
	}
	inbox := asInt(w["inbox_count"])
	spam := asInt(w["spam_count"])

	var totalSent, totalReply int
	if days, isArr := w["stats_by_date"].([]any); isArr {
		historyDays = len(days)
		for _, d := range days {
			if row, isMap := d.(map[string]any); isMap {
				totalSent += asInt(row["sent_count"])
				totalReply += asInt(row["reply_count"])
			}
		}
	}
	if inbox+spam == 0 && historyDays == 0 {
		return 0, 0, 0, false
	}
	inboxRate = warmupInboxRate(inbox, spam)
	warmupReply = rate(totalReply, totalSent)
	return inboxRate, warmupReply, historyDays, true
}

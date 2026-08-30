// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `unsub verify`: the accountability read. Joins the
// unsubscribe attempt ledger (successful 2xx one-click POSTs) against
// mail_meta arrivals landing after a 2-day grace window and reports every
// violator with a ready-to-run escalation query. Pure local read.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// unsubVerifyGrace is how long after a 2xx POST a sender's mail is still
// forgiven (RFC 8058 allows processing delay; in-flight mail also lands).
const unsubVerifyGrace = 48 * time.Hour

// unsubVerifyRow is one sender that kept mailing after a successful
// one-click unsubscribe.
type unsubVerifyRow struct {
	Sender          string `json:"sender"`
	URL             string `json:"url"`
	PostedAt        string `json:"posted_at"`
	ArrivalsSince   int    `json:"arrivals_since"`
	NewestSubject   string `json:"newest_subject"`
	NewestDate      string `json:"newest_date"`
	EscalationQuery string `json:"escalation_query"`
}

func newNovelUnsubVerifyCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "See which senders kept mailing you after a one-click unsubscribe, with an escalation query per violator",
		Long: `Join the unsubscribe attempt ledger against what actually arrived.

For every sender whose one-click POST returned a 2xx within the --since
window, count messages that landed AFTER the post plus a 2-day grace
period. Senders with post-grace arrivals are violators: each row carries
the newest offending subject/date and an escalation_query ("from:<sender>")
ready to paste into 'cleanup plan --q'.

Reads only the local store — run 'sync' first so arrivals are current.
Attempts recorded 'skipped:*' or 'unknown' never count as successful posts.`,
		Example: `  # Who ignored last week's unsubscribes?
  gmail-pp-cli unsub verify --account personal

  # Escalate a violator into the cleanup engine
  gmail-pp-cli cleanup plan --account personal --q "from:deals@shop.example" --action trash`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if since == "" {
				return usageErr(fmt.Errorf("--since must not be empty (default 7d)"))
			}
			postedSince, err := parseSinceDuration(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			violations, err := db.UnsubViolations(account, postedSince, unsubVerifyGrace)
			if err != nil {
				return fmt.Errorf("joining unsubscribe ledger against arrivals: %w", err)
			}
			rows := make([]unsubVerifyRow, 0, len(violations))
			for _, v := range violations {
				rows = append(rows, unsubVerifyRow{
					Sender:          v.Sender,
					URL:             v.URL,
					PostedAt:        v.PostedAt,
					ArrivalsSince:   v.ArrivalsSince,
					NewestSubject:   v.NewestSubject,
					NewestDate:      msToRFC3339(v.NewestDateMs),
					EscalationQuery: fmt.Sprintf("from:%s", v.Sender),
				})
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no violators: every sender with a successful one-click POST in the last %s has stayed quiet past the %s grace\n",
					since, unsubVerifyGrace)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Consider one-click POSTs made within this window (e.g. 7d, 4w)")
	return cmd
}

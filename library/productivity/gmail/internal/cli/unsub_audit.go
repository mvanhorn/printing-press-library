// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `unsub audit`: classify every unsubscribe-capable sender in
// the local store against the one-click ladder (unsub_support.go) and
// stamp mail_meta.list_unsub_domain for the alignment check. Pure local
// read + local-store write; no network.

package cli

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// unsubAuditRow is one sender's audit verdict.
type unsubAuditRow struct {
	Sender         string  `json:"sender"`
	Count          int     `json:"count"`
	UnreadRate     float64 `json:"unread_rate"`
	Classification string  `json:"classification"`
	URLOrMailto    string  `json:"url_or_mailto"`
	Aligned        bool    `json:"aligned"`
	UnsubDomain    string  `json:"unsub_domain,omitempty"`
	FromDomain     string  `json:"from_domain,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	MessageID      string  `json:"message_id,omitempty"`
}

// buildUnsubAuditRow classifies one sender from its newest
// unsubscribe-bearing stored message and computes the domain alignment.
func buildUnsubAuditRow(agg store.UnsubSenderAgg, newest store.MailMeta) unsubAuditRow {
	cls := classifyUnsubSender(newest.ListUnsubscribe, newest.ListUnsubscribePost, newest.AuthResults)
	row := unsubAuditRow{
		Sender:         agg.FromEmail,
		Count:          agg.Count,
		Classification: cls.Class,
		Reason:         cls.Reason,
		MessageID:      newest.ID,
		FromDomain:     registrableDomain(emailDomain(agg.FromEmail)),
	}
	if agg.Count > 0 {
		row.UnreadRate = float64(agg.UnreadCount) / float64(agg.Count)
	}
	switch {
	case cls.URL != "":
		row.URLOrMailto = cls.URL
		if u := parsedURLHost(cls.URL); u != "" {
			row.UnsubDomain = registrableDomain(u)
		}
		row.Aligned = row.UnsubDomain != "" && row.UnsubDomain == row.FromDomain
	case cls.Mailto != "":
		row.URLOrMailto = cls.Mailto
	}
	return row
}

func newNovelUnsubAuditCmd(flags *rootFlags) *cobra.Command {
	var minCount int
	var since string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Classify unsubscribe-capable senders: one-click-verified (RFC 8058, Gmail dmarc=pass), plain-url (click manually), or mailto-only (desk list, never acted on)",
		Long: `Group the synced mail_meta rows per sender and classify every sender that
carries a List-Unsubscribe header:

  one-click-verified(pending-header-check)
      ALL of: exactly one https unsubscribe URL (internal duplicates are
      ambiguity and downgrade), List-Unsubscribe-Post equal to
      "List-Unsubscribe=One-Click", and a stored Authentication-Results
      header that is Gmail's own (authserv-id mx.google.com) carrying
      dmarc=pass. The DKIM-Signature header is not stored offline, so the
      final DKIM coverage check happens live inside 'unsub run' — hence
      "(pending-header-check)".
  plain-url     an https URL is present but a one-click condition failed;
                click it manually.
  mailto-only   only mailto: entries; surfaced as a desk list and NEVER
                acted on (nothing sends mail from this binary).
  unusable      a List-Unsubscribe value with neither an https URL nor a
                mailto entry (e.g. plain http).

Also stamps mail_meta.list_unsub_domain with the registrable domain of the
https URL (an eTLD+1 approximation: last two labels, or three under common
two-level suffixes like co.uk/com.au) for the aligned column: aligned means
the unsubscribe URL's registrable domain equals the sender's.

Reads only the local store — run 'sync' first.`,
		Example: `  # Classify heavy senders (3+ messages in the last 180 days)
  gmail-pp-cli unsub audit --account personal

  # Everything, JSON for an agent
  gmail-pp-cli unsub audit --account ads --min-count 1 --since 365d --agent`,
		Annotations: map[string]string{
			"mcp:read-only":   "true",
			"mcp:local-write": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if minCount <= 0 {
				return usageErr(fmt.Errorf("--min-count must be positive, got %d", minCount))
			}
			sinceMs, err := sinceFlagToMs(since)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			aggs, err := db.UnsubSenderAggregates(account, sinceMs, minCount)
			if err != nil {
				return fmt.Errorf("aggregating unsubscribe senders: %w", err)
			}
			rows := make([]unsubAuditRow, 0, len(aggs))
			for _, agg := range aggs {
				newest, err := db.NewestUnsubMeta(account, agg.FromEmail)
				if err != nil {
					return fmt.Errorf("reading newest unsubscribe message for %s: %w", agg.FromEmail, err)
				}
				row := buildUnsubAuditRow(agg, newest)
				if row.UnsubDomain != "" {
					if _, err := db.SetMailListUnsubDomain(account, agg.FromEmail, row.UnsubDomain); err != nil {
						return fmt.Errorf("stamping list_unsub_domain for %s: %w", agg.FromEmail, err)
					}
				}
				rows = append(rows, row)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no unsubscribe-capable senders matched for account %q — populate the store with: gmail-pp-cli sync --account %s\n",
					account, account)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().IntVar(&minCount, "min-count", 3, "Only audit senders with at least this many messages in the window")
	cmd.Flags().StringVar(&since, "since", "180d", "Only count messages newer than this (e.g. 180d, 12w); empty = no bound")
	return cmd
}

// parsedURLHost returns a URL's hostname (” when unparseable).
func parsedURLHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// sinceFlagToMs converts a --since duration flag (” = unbounded) to a
// millisecond lower bound, wrapping parse failures as usage errors.
func sinceFlagToMs(since string) (int64, error) {
	if since == "" {
		return 0, nil
	}
	ts, err := parseSinceDuration(since)
	if err != nil {
		return 0, usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
	}
	return ts.UnixMilli(), nil
}

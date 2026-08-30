// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `digest`: the flagship per-category summary over the local
// store. Every derived category (primary/promotions/social/updates/forums
// plus uncategorized) is always present — zero-count categories included —
// so agents can rely on a fixed shape. Pure local read; run `sync` first.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// digestCategories is the fixed category vocabulary digest always emits,
// in this order. "" is mail outside the inbox with no CATEGORY_* label.
var digestCategories = []string{"primary", "promotions", "social", "updates", "forums", ""}

// digestCategoryRow is one category's summary.
type digestCategoryRow struct {
	Category            string              `json:"category"`
	Total               int                 `json:"total"`
	Unread              int                 `json:"unread"`
	OldestUnreadAgeDays *int                `json:"oldest_unread_age_days"` // null when no unread
	TopSenders          []store.SenderCount `json:"top_senders"`
	TotalSize           int64               `json:"total_size"`
}

// digestOutput is the digest JSON envelope.
type digestOutput struct {
	Account    string              `json:"account"`
	Since      string              `json:"since,omitempty"`
	Categories []digestCategoryRow `json:"categories"`
	Rollup     struct {
		Total               int   `json:"total"`
		Unread              int   `json:"unread"`
		TotalSize           int64 `json:"total_size"`
		OldestUnreadAgeDays *int  `json:"oldest_unread_age_days"`
	} `json:"rollup"`
}

// ageDaysFromMs converts a ms-epoch timestamp to full days elapsed as of
// now (nil for ms<=0).
func ageDaysFromMs(ms int64, now time.Time) *int {
	if ms <= 0 {
		return nil
	}
	d := int(now.Sub(time.UnixMilli(ms)).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return &d
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Per-category mailbox summary: totals, unread, oldest-unread age, top senders, and size — plus an account rollup",
		Long: `Summarize the synced mailbox per derived category (primary, promotions,
social, updates, forums, and uncategorized ""): message total, unread
count, the age in days of the oldest unread message (null when nothing is
unread), the top 3 senders by count, and total size. A rollup across all
categories closes the report.

Every category is always present — zero-count categories included — so the
output shape is fixed for agents. Reads only the local store; run 'sync'
first.`,
		Example: `  # This week's inbox picture
  gmail-pp-cli digest --account personal

  # Full history, JSON for an agent
  gmail-pp-cli digest --account ads --since 365d --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
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

			rows, err := db.CategoryDigest(account, sinceMs)
			if err != nil {
				return fmt.Errorf("aggregating categories: %w", err)
			}
			topSenders, err := db.CategoryTopSenders(account, sinceMs, 3)
			if err != nil {
				return fmt.Errorf("aggregating top senders: %w", err)
			}

			byCat := map[string]store.CategoryDigestRow{}
			for _, r := range rows {
				byCat[r.Category] = r
			}
			now := time.Now()
			out := digestOutput{Account: account, Since: since}
			var rollupOldestMs int64
			for _, cat := range digestCategories {
				r := byCat[cat]
				senders := topSenders[cat]
				if senders == nil {
					senders = []store.SenderCount{}
				}
				out.Categories = append(out.Categories, digestCategoryRow{
					Category:            cat,
					Total:               r.Total,
					Unread:              r.Unread,
					OldestUnreadAgeDays: ageDaysFromMs(r.OldestUnreadMs, now),
					TopSenders:          senders,
					TotalSize:           r.TotalSize,
				})
				out.Rollup.Total += r.Total
				out.Rollup.Unread += r.Unread
				out.Rollup.TotalSize += r.TotalSize
				if r.OldestUnreadMs > 0 && (rollupOldestMs == 0 || r.OldestUnreadMs < rollupOldestMs) {
					rollupOldestMs = r.OldestUnreadMs
				}
			}
			out.Rollup.OldestUnreadAgeDays = ageDaysFromMs(rollupOldestMs, now)
			if out.Rollup.Total == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local data matched for account %q — populate the store with: gmail-pp-cli sync --account %s\n",
					account, account)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Only count messages newer than this (e.g. 7d, 24h, 4w); empty = all history")
	return cmd
}

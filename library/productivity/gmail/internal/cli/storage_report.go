// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `storage report`: storage attribution over the local store
// — by sender, by category, by year, and the largest single messages —
// with a ready-to-run cleanup query per sender row. Pure local read.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// storageSenderRow is one sender's attribution plus its ready query.
type storageSenderRow struct {
	FromEmail  string `json:"from_email"`
	Count      int    `json:"count"`
	TotalSize  int64  `json:"total_size"`
	ReadyQuery string `json:"ready_query"`
}

// storageLargestRow is one large message with a human-readable date.
type storageLargestRow struct {
	ID           string `json:"id"`
	FromEmail    string `json:"from_email"`
	Subject      string `json:"subject"`
	SizeEstimate int64  `json:"size_estimate"`
	Date         string `json:"date"`
}

// storageReportOutput is the storage report JSON envelope.
type storageReportOutput struct {
	Account    string                     `json:"account"`
	BySender   []storageSenderRow         `json:"by_sender"`
	ByCategory []store.StorageCategoryRow `json:"by_category"`
	ByYear     []store.StorageYearRow     `json:"by_year"`
	Largest    []storageLargestRow        `json:"largest"`
}

func newNovelStorageReportCmd(flags *rootFlags) *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Which senders, categories, years, and single messages own your storage — with a ready-to-run cleanup query per sender",
		Long: `Attribute mailbox storage four ways from the local store: total size per
sender (top N, with a ready_query like "from:<sender> larger:1m" you can
paste straight into 'cleanup plan --q'), per derived category, per calendar
year of arrival (UTC), and the N largest single messages.

Sizes are Gmail's sizeEstimate in bytes. Reads only the local store — run
'sync' first.`,
		Example: `  # Who owns my storage?
  gmail-pp-cli storage report --account personal

  # Top 50 senders, JSON for an agent
  gmail-pp-cli storage report --account ads --top 50 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if top <= 0 {
				return usageErr(fmt.Errorf("--top must be positive, got %d", top))
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			bySender, err := db.StorageBySender(account, top)
			if err != nil {
				return fmt.Errorf("attributing by sender: %w", err)
			}
			byCategory, err := db.StorageByCategory(account)
			if err != nil {
				return fmt.Errorf("attributing by category: %w", err)
			}
			byYear, err := db.StorageByYear(account)
			if err != nil {
				return fmt.Errorf("attributing by year: %w", err)
			}
			largest, err := db.StorageLargest(account, top)
			if err != nil {
				return fmt.Errorf("finding largest messages: %w", err)
			}

			out := storageReportOutput{
				Account:    account,
				BySender:   make([]storageSenderRow, 0, len(bySender)),
				ByCategory: byCategory,
				ByYear:     byYear,
				Largest:    make([]storageLargestRow, 0, len(largest)),
			}
			for _, r := range bySender {
				out.BySender = append(out.BySender, storageSenderRow{
					FromEmail:  r.FromEmail,
					Count:      r.Count,
					TotalSize:  r.TotalSize,
					ReadyQuery: fmt.Sprintf("from:%s larger:1m", r.FromEmail),
				})
			}
			for _, r := range largest {
				out.Largest = append(out.Largest, storageLargestRow{
					ID:           r.ID,
					FromEmail:    r.FromEmail,
					Subject:      r.Subject,
					SizeEstimate: r.SizeEstimate,
					Date:         msToRFC3339(r.InternalDate),
				})
			}
			if len(out.BySender) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local data matched for account %q — populate the store with: gmail-pp-cli sync --account %s\n",
					account, account)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&top, "top", 15, "How many sender rows and largest messages to return")
	return cmd
}

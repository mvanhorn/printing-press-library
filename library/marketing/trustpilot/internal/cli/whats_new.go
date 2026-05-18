package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPWhatsNewCmd(flags *rootFlags) *cobra.Command {
	var sinceFlag string
	cmd := &cobra.Command{
		Use:   "whats-new <domain>",
		Short: "List reviews added since a given timestamp (or since last sync)",
		Example: `  trustpilot-pp-cli whats-new www.thriftbooks.com --json
  trustpilot-pp-cli whats-new bookshop.org --since 2026-05-01T00:00:00Z`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			domain := normalizeDomain(args[0])
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			_, cursorSince, err := tpkg.GetCursor(ctx, db, domain)
			if err != nil {
				return err
			}
			since := cursorSince
			if sinceFlag != "" {
				since, err = time.Parse(time.RFC3339, sinceFlag)
				if err != nil {
					return fmt.Errorf("parse --since as RFC3339: %w", err)
				}
			}
			if since.IsZero() {
				return fmt.Errorf("no --since provided and no sync cursor for %s; run `trustpilot-pp-cli sync-trustpilot %s` first", domain, domain)
			}
			reviews, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{Domain: domain, MinDate: since})
			if err != nil {
				return err
			}
			byRating := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
			for _, r := range reviews {
				if r.Rating >= 1 && r.Rating <= 5 {
					byRating[fmt.Sprint(r.Rating)]++
				}
			}
			payload := map[string]any{
				"domain":     domain,
				"since":      since.UTC().Format(time.RFC3339),
				"newReviews": reviews,
				"byRating":   byRating,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d reviews since %s\n", len(reviews), since.UTC().Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceFlag, "since", "", "RFC3339 timestamp; defaults to the sync cursor's lastSyncedAt")
	return cmd
}

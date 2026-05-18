package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPTopicsCmd(flags *rootFlags) *cobra.Command {
	var local bool
	cmd := &cobra.Command{
		Use:   "topics <domain>",
		Short: "Trustpilot's per-topic AI summaries (shipping, price, quality, etc.) for a company",
		Example: `  trustpilot-pp-cli topics www.thriftbooks.com --json
  trustpilot-pp-cli topics bookshop.org --local`,
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
			var bu tpkg.BusinessUnit
			source := "live"
			if local {
				source = "local"
				bu, err = tpkg.LoadCompany(ctx, db, domain)
				if err != nil {
					return fmt.Errorf("no cached topics for %s; run `trustpilot-pp-cli topics %s` first", domain, domain)
				}
			} else {
				sess, _, err := loadOrHarvestSession(ctx, db, true)
				if err != nil {
					return err
				}
				page, err := fetchPageWithRetry(ctx, db, &sess, domain, tpkg.PageFilters{Page: 1, Sort: "recency"})
				if err != nil {
					return err
				}
				bu = page.BusinessUnit
				_ = tpkg.UpsertCompany(ctx, db, bu)
			}
			// PATCH: Include a standard meta envelope on topics JSON output.
			meta := NewMeta(source)
			addBusinessUnitMeta(&meta, bu)
			// PATCH: Explain empty Trustpilot-provided topic and summary fields in meta notices.
			if len(bu.TopicAISummaries) == 0 {
				meta.AddNotice(NoticeTopicsEmpty)
			}
			if bu.AISummary == "" {
				meta.AddNotice(NoticeAISummaryEmpty)
			}
			payload := map[string]any{"domain": bu.IdentifyingName, "topics": bu.TopicAISummaries, "overallAiSummary": bu.AISummary}
			attachMeta(payload, meta)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			for _, t := range bu.TopicAISummaries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", t.Topic, t.Summary)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "Read cached company topics from the local store")
	return cmd
}

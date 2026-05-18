// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli filings-by-ticker <TICKER> --type 10-K` — top-level wrapper
// around the resource-level filings browse, with ticker→CIK resolution.
//
// Named `filings-by-ticker` to avoid colliding with the existing `filings`
// subcommand group (browse, get). LODESTAR users get a positional ticker
// without having to first run `companies lookup`.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

func newFilingsByTickerCmd(flags *rootFlags) *cobra.Command {
	var formType string
	var since string
	var count int
	cmd := &cobra.Command{
		Use:         "filings-by-ticker <ticker-or-cik>",
		Short:       "Per-form-type filing list for a ticker (10-K, 10-Q, 8-K, 4, DEF 14A)",
		Example:     "  edgar-pp-cli filings-by-ticker AAPL --type 10-K --count 5",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Aliases:     []string{"filings-for"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(cmd.Context(), edgarDBPath())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureEdgarSchema(cmd.Context()); err != nil {
				return err
			}
			ec, err := resolveCIKOrTicker(cmd.Context(), c, db, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			sinceISO, perr := parseSinceDate(since)
			if perr != nil {
				return usageErr(perr)
			}
			if _, ferr := fetchSubmissions(cmd.Context(), c, db, ec.CIK); ferr != nil {
				return classifyAPIError(ferr, flags)
			}
			var formTypes []string
			if formType != "" {
				formTypes = []string{formType}
			}
			if count <= 0 {
				count = 40
			}
			filings, err := db.ListEdgarFilings(cmd.Context(), ec.CIK, formTypes, sinceISO, count)
			if err != nil {
				return err
			}
			return emitJSON(cmd, flags, filings)
		},
	}
	cmd.Flags().StringVar(&formType, "type", "", "Form type (10-K, 10-Q, 8-K, 4, DEF 14A)")
	cmd.Flags().StringVar(&since, "since", "", "Earliest filing date")
	cmd.Flags().IntVar(&count, "count", 40, "Max results")
	return cmd
}

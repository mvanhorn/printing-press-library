// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli since <TICKER> --as-of TS [--types FORMS]` — return all
// filings filed since `as-of` for a ticker, optionally filtered by form
// type. LODESTAR /$recheck core loop.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var asOf string
	var types string

	cmd := &cobra.Command{
		Use:   "since <ticker-or-cik>",
		Short: "List filings filed since a given date for a ticker (LODESTAR /$recheck loop)",
		Long: `Return only filings filed since --as-of for the given ticker (or CIK).
Reads from the local SQLite cache; on cold cache for this CIK, performs one
submissions sync to populate it.`,
		Example:     "  edgar-pp-cli since AAPL --as-of 2026-05-08 --types 8-K,4",
		Annotations: map[string]string{"mcp:read-only": "true"},
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
			since := ""
			if asOf != "" {
				s, perr := parseSinceDate(asOf)
				if perr != nil {
					return usageErr(perr)
				}
				since = s
			}
			var formTypes []string
			if types != "" {
				for _, t := range strings.Split(types, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						formTypes = append(formTypes, t)
					}
				}
			}
			// Warm cache if empty for this CIK
			filings, err := db.ListEdgarFilings(cmd.Context(), ec.CIK, formTypes, since, 200)
			if err != nil {
				return err
			}
			if len(filings) == 0 {
				// Try fetching submissions index once
				if _, ferr := fetchSubmissions(cmd.Context(), c, db, ec.CIK); ferr != nil {
					return classifyAPIError(ferr, flags)
				}
				filings, err = db.ListEdgarFilings(cmd.Context(), ec.CIK, formTypes, since, 200)
				if err != nil {
					return err
				}
			}
			return emitJSON(cmd, flags, filings)
		},
	}
	cmd.Flags().StringVar(&asOf, "as-of", "", "ISO date (or 90d/12mo/1y) — return filings on/after this date")
	cmd.Flags().StringVar(&types, "types", "", "Comma-separated form types to include (e.g., 8-K,4,10-K)")
	return cmd
}

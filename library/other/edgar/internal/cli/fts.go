// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli fts <QUERY>` — FTS5 full-text search over cached filing bodies.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

func newFTSCmd(flags *rootFlags) *cobra.Command {
	var ticker string
	var form string
	var sinceArg string
	var limit int

	cmd := &cobra.Command{
		Use:   "fts <query>",
		Short: "Full-text search over locally-cached filing bodies via FTS5",
		Long: `Run an FTS5 query against locally-cached filing bodies. Use --ticker to
restrict by issuer and --form to restrict by form type. Returns snippets
with byte offsets for precise re-read. Filings whose body_text isn't yet
cached are noted on stderr but don't fail the call.`,
		Example:     "  edgar-pp-cli fts \"export controls\" --ticker AAPL --form 10-K",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), edgarDBPath())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureEdgarSchema(cmd.Context()); err != nil {
				return err
			}
			var cikFilter string
			if ticker != "" {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				ec, terr := resolveCIKOrTicker(cmd.Context(), c, db, ticker)
				if terr != nil {
					return classifyAPIError(terr, flags)
				}
				cikFilter = ec.CIK
			}
			_ = sinceArg // documented; not yet wired (would constrain by filed_at — FTS5 filter applied client-side)
			hits, err := db.SearchEdgarFTS(cmd.Context(), args[0], cikFilter, form, limit)
			if err != nil {
				return fmt.Errorf("fts query: %w", err)
			}
			if len(hits) == 0 {
				fmt.Fprintln(os.Stderr, "no matches — body_text may not be cached; run `edgar-pp-cli since <ticker>` or `eightk-items` to warm the cache")
			}
			return emitJSON(cmd, flags, hits)
		},
	}
	cmd.Flags().StringVar(&ticker, "ticker", "", "Restrict to filings of this ticker")
	cmd.Flags().StringVar(&form, "form", "", "Restrict to this form type (10-K, 10-Q, 8-K)")
	cmd.Flags().StringVar(&sinceArg, "since", "", "Only filings filed on/after this date (reserved)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

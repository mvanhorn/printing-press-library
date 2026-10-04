// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"errors"

	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/ticket"
	"github.com/spf13/cobra"
)

func newTicketsListCmd(flags *rootFlags) *cobra.Command {
	var area, kind, query string
	var pages, limit int
	cmd := &cobra.Command{Use: "list", Short: "Discover a bounded regional ticket listing with source coverage", Long: "Read native Toretabi area/type listing pages. --query checks Japanese ticket names and listing tags locally. Inspect coverage before interpreting zero matches. No seat inventory is queried.", Example: "  toretabi-pp-cli tickets list --area 1 --ticket-type 4 --max-pages 1 --limit 3 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--area=1;--ticket-type=4;--max-pages=1;--limit=3"}}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "tickets list")
		}
		if len(args) > 0 {
			return ticketCommandError(cmd, flags, usageErr(errors.New("tickets list takes no positional arguments; use --query")))
		}
		if flags.dataSource == "local" {
			return ticketCommandError(cmd, flags, usageErr(errors.New("tickets list is live-only; use tickets cached for saved details")))
		}
		if pages < 1 || pages > 5 || limit < 1 || limit > 50 {
			return ticketCommandError(cmd, flags, usageErr(errors.New("--max-pages must be 1..5 and --limit 1..50")))
		}
		if cliutil.IsDogfoodEnv() && pages > 1 {
			pages = 1
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		out, e := ticket.NewClient(flags.rateLimit).List(ctx, ticket.ListOptions{Area: area, Type: kind, Query: query, MaxPages: pages, Limit: limit})
		if e != nil {
			if strings.Contains(e.Error(), "--area") || strings.Contains(e.Error(), "--ticket-type") {
				e = usageErr(e)
			}
			return ticketCommandError(cmd, flags, e)
		}
		return ticketOutput(cmd, flags, out)
	}
	cmd.Flags().StringVar(&area, "area", "", "Native source area code 1..11; output includes Japanese catalog")
	cmd.Flags().StringVar(&kind, "ticket-type", "", "Native type code: 1 early purchase, 2 multiple tickets, 3 return discount, 4 free area")
	cmd.Flags().StringVar(&query, "query", "", "Literal substring in Japanese ticket names or listing tags")
	cmd.Flags().IntVar(&pages, "max-pages", 2, "Maximum source listing pages to inspect, from 1 to 5")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum returned tickets, separate from scan pages, from 1 to 50")
	return cmd
}

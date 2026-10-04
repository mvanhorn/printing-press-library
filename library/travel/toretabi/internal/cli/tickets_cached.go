// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/ticket"
)

func newNovelTicketsCachedCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	cmd := &cobra.Command{Use: "cached", Short: "Read a bounded saved ticket shortlist with original observation clocks", Example: "  toretabi-pp-cli tickets cached --query 北海道 --limit 3 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--limit=3"}}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "tickets cached")
		}
		if len(args) > 0 {
			return ticketCommandError(cmd, flags, usageErr(errors.New("tickets cached takes no positional arguments; use --query")))
		}
		if flags.dataSource == "live" || flags.noCache {
			return ticketCommandError(cmd, flags, usageErr(errors.New("tickets cached reads local evidence only; omit --data-source live and --no-cache")))
		}
		if limit < 1 || limit > 50 {
			return ticketCommandError(cmd, flags, usageErr(errors.New("--limit must be 1..50")))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		path, e := ticketCachePath()
		if e != nil {
			return ticketCommandError(cmd, flags, e)
		}
		xs, e := ticket.Cached(ctx, path, query, limit)
		if e != nil {
			return ticketCommandError(cmd, flags, e)
		}
		for i := range xs {
			markTicketFreshness(&xs[i], flags.maxAge)
		}
		return ticketOutput(cmd, flags, map[string]any{"tickets": xs, "count": len(xs), "capacity": ticket.MaxCached, "note": "Only previously inspected saved tickets are covered. Publisher/operator observation clocks are retained; neither source was refreshed.", "source_boundary": "Saved publisher and linked operator evidence with independently evaluated freshness; rules and original clocks are not refreshed."})
	}
	cmd.Flags().StringVar(&query, "query", "", "Literal substring in saved Japanese name, or an exact source ID")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum saved observations to return, from 1 to 50")
	return cmd
}

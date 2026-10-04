// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/ticket"
	"github.com/spf13/cobra"
)

func newNovelTicketsCompareCmd(flags *rootFlags) *cobra.Command {
	var useOn, asOf, ids string
	cmd := &cobra.Command{Use: "compare [tickets...]", Short: "Compare four tickets' published periods, supplements and purchase conditions", Long: "Compare 2..4 source IDs. --use-on is the intended first use date; --as-of is the sale/edition-check date (default Asia/Tokyo today). Publisher and bounded linked official operator rules are checked separately, with independent clocks and explicit corroboration/conflicts. Eligibility and seat availability remain unknown. Month/day-only blackout spans, holidays, missing fares and compound conditions require operator confirmation.", Example: "  toretabi-pp-cli tickets compare tokai_043 east_027 --use-on 2026-10-10 --as-of 2026-10-04 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "left=tokai_043;right=east_027;--use-on=2026-10-10;--as-of=2026-10-04;--data-source=live"}}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "tickets compare")
		}
		if ids != "" {
			if len(args) > 0 {
				return ticketCommandError(cmd, flags, usageErr(errors.New("use positional ticket IDs or --ids, not both")))
			}
			args = strings.Split(ids, ",")
			for i := range args {
				args[i] = strings.TrimSpace(args[i])
			}
		}
		if len(args) == 1 && strings.Contains(args[0], ",") {
			args = strings.Split(args[0], ",")
			for i := range args {
				args[i] = strings.TrimSpace(args[i])
			}
		}
		if len(args) < 2 || len(args) > 4 {
			return ticketCommandError(cmd, flags, usageErr(errors.New("tickets compare needs 2..4 source IDs")))
		}
		if asOf == "" {
			asOf = tokyoToday()
		}
		for key, v := range map[string]string{"use-on": useOn, "as-of": asOf} {
			if e := validateTicketDate(key, v); e != nil {
				return ticketCommandError(cmd, flags, e)
			}
		}
		seen := map[string]bool{}
		for _, id := range args {
			if !ticket.ValidID(id) || seen[id] {
				return ticketCommandError(cmd, flags, usageErr(fmt.Errorf("invalid or repeated ticket ID %q", id)))
			}
			seen[id] = true
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c := ticket.NewClient(flags.rateLimit)
		rows := []ticket.Comparison{}
		failures := []map[string]any{}
		var last error
		for _, id := range args {
			t, e := ticketGet(ctx, c, id, flags, asOf)
			if e != nil {
				var rate *cliutil.RateLimitError
				if errors.As(e, &rate) {
					return ticketCommandError(cmd, flags, e)
				}
				last = e
				failures = append(failures, map[string]any{"id": id, "error": e.Error()})
				continue
			}
			warnTicket(cmd, t)
			rows = append(rows, ticket.Compare(t, useOn, asOf))
		}
		if len(rows) == 0 {
			return ticketCommandError(cmd, flags, fmt.Errorf("all %d ticket detail reads failed: %w", len(args), last))
		}
		if len(failures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d detail reads failed; comparison includes %d successful tickets\n", len(failures), len(args), len(rows))
		}
		return ticketOutput(cmd, flags, map[string]any{"comparisons": rows, "fetch_failures": failures, "coverage": map[string]any{"requested": len(args), "inspected": len(rows), "complete": len(failures) == 0}, "source_boundary": "Publisher and bounded linked-operator rule evidence are compared separately; missing/conflicting rules and traveler eligibility remain unresolved.", "use_on": strOrNil(useOn), "as_of": asOf})
	}
	cmd.Flags().StringVar(&ids, "ids", "", "Comma-separated 2..4 Toretabi ticket IDs; use this field for MCP")
	cmd.Flags().StringVar(&useOn, "use-on", "", "Intended first use date in YYYY-MM-DD, optional")
	cmd.Flags().StringVar(&asOf, "as-of", "", "Sale and archival-evidence check date; default Asia/Tokyo today")
	return cmd
}

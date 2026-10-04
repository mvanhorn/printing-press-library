// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/ticket"
)

func newTicketsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "get [id]", Short: "Inspect ticket evidence and bounded linked-operator rule checks", Example: "  toretabi-pp-cli tickets get hokkaido_028 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "id=hokkaido_028;--data-source=live"}}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "tickets get")
		}
		if len(args) != 1 {
			return ticketCommandError(cmd, flags, usageErr(errors.New("tickets get needs one source ID, for example tokai_043")))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		t, e := ticketGet(ctx, ticket.NewClient(flags.rateLimit), args[0], flags, tokyoToday())
		if e != nil {
			return ticketCommandError(cmd, flags, e)
		}
		warnTicket(cmd, t)
		flags.agentSource = t.Transport
		return ticketOutput(cmd, flags, map[string]any{"ticket": t, "source_boundary": "Publisher and linked operator observations are separate; unresolved rules, eligibility and inventory remain unknown."})
	}
	return cmd
}

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// These commands are wired up but ship as honest "coming next" stubs in the
// initial release. The infrastructure (HTML scraper, bid client, store) all
// exists; what's missing is the per-command glue.

func newBidGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "bid-group",
		Short:       "Coordinated multi-item snipe groups (single-win, multi-win, contingency)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(novelStub(flags, "create", "Create a new bid group", "bid-group create <name> --type single-win|multi-win=N|contingency"))
	cmd.AddCommand(novelStub(flags, "list", "List bid groups", "bid-group list"))
	cmd.AddCommand(novelStub(flags, "delete", "Delete a bid group", "bid-group delete <name>"))
	return cmd
}

func newFeedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "feed",
		Short:       "Stream new listings matching a saved search, with sold-comp context appended",
		Example:     "  ebay-pp-cli feed cards-watch",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE:        novelStubRun(flags, "feed", "feed <saved-search>"),
	}
	return cmd
}

func newOfferHunterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "offer-hunter",
		Short:       "Auto-submit best offers across a saved search at a percentage of asking",
		Example:     "  ebay-pp-cli offer-hunter cards-watch --at-percent 80",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE:        novelStubRun(flags, "offer-hunter", "offer-hunter <saved-search> --at-percent 80"),
	}
	return cmd
}

// novelStubRun returns a RunE function that emits an honest "not yet
// implemented" message in both human and JSON modes, and respects --dry-run.
func novelStubRun(flags *rootFlags, name, usage string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return nil
		}
		payload := map[string]any{
			"command":     name,
			"status":      "not-implemented",
			"message":     "command is wired in the CLI but not yet implemented; see the absorb manifest for the planned shape",
			"planned_use": usage,
			"track":       "https://github.com/mvanhorn/printing-press-library/issues",
		}
		data, _ := json.Marshal(payload)
		if flags.asJSON || flags.agent || flags.selectFields != "" {
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: not yet implemented\n  Planned: %s\n", name, usage)
		fmt.Fprintf(cmd.OutOrStdout(), "  See `ebay-pp-cli comp`, `ebay-pp-cli snipe`, and `ebay-pp-cli auctions` for the implemented features.\n")
		return nil
	}
}

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "history",
		Short:       "Buying history (won, lost, paid)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(novelStub(flags, "won", "List items you won", "history won --days 90"))
	cmd.AddCommand(novelStub(flags, "lost", "List auctions you lost", "history lost --days 90"))
	cmd.AddCommand(novelStub(flags, "paid", "List items you paid for", "history paid --days 90"))
	return cmd
}

func newSavedSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "saved-search",
		Short:       "Local saved-search CRUD (independent of eBay's saved-search UI)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(novelStub(flags, "create", "Create a saved search", `saved-search create <name> --query "Steph Curry" --has-bids --ending-within 1h`))
	cmd.AddCommand(novelStub(flags, "list", "List saved searches", "saved-search list"))
	cmd.AddCommand(novelStub(flags, "run", "Run a saved search", "saved-search run <name>"))
	cmd.AddCommand(novelStub(flags, "delete", "Delete a saved search", "saved-search delete <name>"))
	return cmd
}

// novelStub returns a Cobra command that emits a structured "coming next"
// message to JSON consumers and a friendly note to humans, keeping --dry-run
// and --json contracts honest. The mcp:hidden annotation excludes the stub
// from the agent-facing MCP surface so agents never see commands that don't
// produce useful output.
func novelStub(flags *rootFlags, name, short, usage string) *cobra.Command {
	return &cobra.Command{
		Use:     name,
		Short:   short,
		Example: "  ebay-pp-cli " + usage,
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			payload := map[string]any{
				"command":     name,
				"status":      "not-implemented",
				"message":     "command is wired in the CLI but not yet implemented; see the absorb manifest for the planned shape",
				"planned_use": usage,
				"track":       "https://github.com/mvanhorn/printing-press-library/issues",
			}
			data, _ := json.Marshal(payload)
			if flags.asJSON || flags.agent || flags.selectFields != "" {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: not yet implemented\n  Planned: %s\n", name, usage)
			fmt.Fprintf(cmd.OutOrStdout(), "  See `ebay-pp-cli comp`, `ebay-pp-cli snipe`, and `ebay-pp-cli auctions` for the implemented features.\n")
			return nil
		},
	}
}

// Hand-authored — NOT generated. Survives regen as a whole hand-authored unit.
// Resolves the duplicate `ship` Cobra registration that arises when a
// novel-features parent (newNovelShipCmd) and a promoted endpoint
// (newShipPromotedCmd) share the same root word. Composes both into a single
// parent with `get <imo>` plus the hand-authored novel subcommands.
//
// It calls newNovelShipCmd (which builds the generated stub subcommands) only
// to keep those generated constructors referenced — the `unused` linter would
// otherwise flag them — then drops the TODO stubs via ResetCommands and
// registers the real implementations.
package cli

import "github.com/spf13/cobra"

func newShipParentCmd(flags *rootFlags) *cobra.Command {
	cmd := newNovelShipCmd(flags)
	cmd.Short = "Ship particulars from GISIS, plus a local cache and watchlist."
	cmd.Long = "Use 'ship get <imo>' to fetch authoritative particulars from GISIS (cached locally on every fetch). Use 'ship list', 'ship history', 'ship stale', 'ship pin/unpin/refresh', and 'ship batch' to build and query a compounding local vessel index."
	cmd.ResetCommands()

	getCmd := newShipPromotedCmd(flags)
	getCmd.Use = "get <imo>"
	getCmd.Short = "Get ship particulars by IMO number from GISIS."
	getCmd.Long = "Fetches the IMO Ship and Company Particulars module for the given IMO number, parses the HTML into typed JSON, and caches the result in the local SQLite store. Pair with 'ship history', 'ship list', and 'owner fleet' to query accumulated lookups."
	getCmd.Example = "  gisis-pp-cli ship get 9866641 --json"
	// Override the auto-generated RunE (which returned raw HTML via html_extract:page)
	// with one that uses goquery to parse the GISIS Ship Particulars page into a
	// typed Ship struct and caches it. See ship_get_handler.go.
	getCmd.RunE = runShipGet(flags)

	cmd.AddCommand(getCmd)
	cmd.AddCommand(newShipHistoryCmd(flags))
	cmd.AddCommand(newShipListCmd(flags))
	cmd.AddCommand(newShipStaleCmd(flags))
	cmd.AddCommand(newShipBatchCmd(flags))
	cmd.AddCommand(newShipPinCmd(flags))
	cmd.AddCommand(newShipUnpinCmd(flags))
	cmd.AddCommand(newShipRefreshCmd(flags))
	return cmd
}

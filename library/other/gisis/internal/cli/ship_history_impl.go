// Hand-authored — NOT generated. Implements `gisis-pp-cli ship history <imo>`.
// GISIS embeds full flag/name/type/owner change history inline on the Ship
// Particulars page, so one fetch yields the whole history — no snapshot-over-
// time accumulation needed. This command reuses the ship get parser and emits
// only the history-relevant fields. The fetch also warms the local cache.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type shipHistoryView struct {
	IMONumber              string             `json:"imo_number"`
	Name                   string             `json:"name,omitempty"`
	CurrentFlag            string             `json:"current_flag,omitempty"`
	CurrentShipType        string             `json:"current_ship_type,omitempty"`
	CurrentRegisteredOwner string             `json:"current_registered_owner,omitempty"`
	NameHistory            []shipHistoryEntry `json:"name_history,omitempty"`
	FlagHistory            []shipHistoryEntry `json:"flag_history,omitempty"`
	ShipTypeHistory        []shipHistoryEntry `json:"ship_type_history,omitempty"`
	RegisteredOwnerHistory []shipHistoryEntry `json:"registered_owner_history,omitempty"`
	SourceURL              string             `json:"source_url"`
	FetchedAt              string             `json:"fetched_at"`
}

func newShipHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "history <imo>",
		Short:       "Show a vessel's flag, name, type, and owner change history.",
		Long:        "Fetches the GISIS Ship Particulars page for an IMO number and returns only the change history GISIS records inline — flag hops, renamings, type reclassifications, and ownership transfers — all from a single request. Also refreshes the local cache for that vessel.",
		Example:     "  gisis-pp-cli ship history 9866641 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s\n(dry run - no request sent)\n", shipSourceURL(strings.TrimSpace(args[0])))
				return nil
			}
			imo := strings.TrimSpace(args[0])
			if imo == "" {
				return usageErr(fmt.Errorf("IMO number is required"))
			}
			// PATCH(pr-953 greptile): reject malformed IMOs before the GISIS fetch.
			if !isValidIMOFormat(imo) {
				return usageErr(fmt.Errorf("invalid IMO %q: expected a 7-digit number", imo))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ship, err := fetchShipParticulars(cmd.Context(), c, imo)
			if err != nil {
				return mapShipFetchError(imo, err, flags)
			}
			if cerr := cacheShipParticulars(cmd.Context(), flags, ship); cerr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetched %s but failed to cache it locally: %v\n", imo, cerr)
			}

			view := shipHistoryView{
				IMONumber:              ship.IMONumber,
				Name:                   ship.Name,
				CurrentFlag:            ship.Flag,
				CurrentShipType:        ship.ShipType,
				CurrentRegisteredOwner: ship.RegisteredOwner,
				NameHistory:            ship.NameHistory,
				FlagHistory:            ship.FlagHistory,
				ShipTypeHistory:        ship.ShipTypeHistory,
				RegisteredOwnerHistory: ship.RegisteredOwnerHistory,
				SourceURL:              ship.SourceURL,
				FetchedAt:              ship.FetchedAt,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

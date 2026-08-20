// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "door list" command: a live, joined valid-holder list for
// an event — joins tickets against returns and transfers so a door operator
// sees who is actually entitled to enter, including the new holder for any
// transferred ticket.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// doorEntry is one valid ticket holder row in the door list.
type doorEntry struct {
	TicketID      string `json:"ticket_id"`
	Code          string `json:"code"`
	HolderName    string `json:"holder_name"`
	HolderEmail   string `json:"holder_email"`
	Claimed       bool   `json:"claimed"`
	Transferred   bool   `json:"transferred"`
	TransferredAt string `json:"transferred_at,omitempty"`
}

// doorTicket / doorReturn / doorTransfer are the slim node shapes the door
// join needs; the GraphQL selections in dice_query.go return supersets.
type doorTicket struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	ClaimedAt string `json:"claimedAt"`
	Holder    struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
	} `json:"holder"`
}

type doorReturn struct {
	TicketID string `json:"ticketId"`
}

type doorTransfer struct {
	TransferredAt string `json:"transferredAt"`
	Tickets       []struct {
		ID string `json:"id"`
	} `json:"tickets"`
}

func newDoorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "door",
		Short: "Build a door-ready valid-holder list for an event",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newDoorListCmd(flags))
	return cmd
}

// pp:data-source live
func newDoorListCmd(flags *rootFlags) *cobra.Command {
	var event string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List valid ticket holders for an event (excludes returns, shows transfers)",
		Example:     "  dice-fm-pp-cli door list --event RXZlbnQ6MTIzNDU= --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if event == "" {
				return fmt.Errorf("--event is required (the event whose door list you want)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			where := eqWhere("eventId", event)

			ticketNodes, _, _, err := fetchConnection(ctx, c, "tickets", where, dicePerPage, 0, "", false)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			returnNodes, _, _, err := fetchConnection(ctx, c, "returns", where, dicePerPage, 0, "", false)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			transferNodes, _, _, err := fetchConnection(ctx, c, "transfers", where, dicePerPage, 0, "", false)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			entries := buildDoorList(ticketNodes, returnNodes, transferNodes)
			return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Event ID to build the door list for (required)")
	return cmd
}

// buildDoorList joins ticket nodes against returns (to drop refunded tickets)
// and transfers (to mark and date re-holdered tickets), returning one entry per
// still-valid ticket. Returns a non-nil empty slice when there are no valid
// holders so the output renders as [] rather than null.
func buildDoorList(ticketNodes, returnNodes, transferNodes []json.RawMessage) []doorEntry {
	returned := make(map[string]bool, len(returnNodes))
	for _, n := range returnNodes {
		var r doorReturn
		if err := json.Unmarshal(n, &r); err != nil {
			continue
		}
		if r.TicketID != "" {
			returned[r.TicketID] = true
		}
	}

	transferredAt := make(map[string]string)
	for _, n := range transferNodes {
		var tr doorTransfer
		if err := json.Unmarshal(n, &tr); err != nil {
			continue
		}
		for _, t := range tr.Tickets {
			if t.ID != "" {
				transferredAt[t.ID] = tr.TransferredAt
			}
		}
	}

	entries := make([]doorEntry, 0, len(ticketNodes))
	for _, n := range ticketNodes {
		var t doorTicket
		if err := json.Unmarshal(n, &t); err != nil {
			continue
		}
		if t.ID == "" || returned[t.ID] {
			continue
		}
		at, transferred := transferredAt[t.ID]
		entries = append(entries, doorEntry{
			TicketID:      t.ID,
			Code:          t.Code,
			HolderName:    joinName(t.Holder.FirstName, t.Holder.LastName),
			HolderEmail:   t.Holder.Email,
			Claimed:       t.ClaimedAt != "",
			Transferred:   transferred,
			TransferredAt: at,
		})
	}
	return entries
}

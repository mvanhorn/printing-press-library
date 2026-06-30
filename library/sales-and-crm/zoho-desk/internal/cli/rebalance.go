// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented RunE body. Mutates state under --apply, so this
// command is intentionally not annotated mcp:read-only.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelRebalanceCmd(flags *rootFlags) *cobra.Command {
	var flagPlan bool
	var flagApply bool
	var dbPath string

	type move struct {
		TicketID    string `json:"ticketId"`
		FromAgentID string `json:"fromAgentId"`
		FromName    string `json:"fromName"`
		ToAgentID   string `json:"toAgentId"`
		ToName      string `json:"toName"`
	}

	cmd := &cobra.Command{
		Use:     "rebalance",
		Short:   "Propose ticket moves from overloaded agents to idle ones, then optionally apply them in bulk.",
		Example: "  zoho-desk-pp-cli rebalance --plan --json\n  zoho-desk-pp-cli rebalance --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()

			tickets, err := loadTickets(cmd.Context(), db)
			if err != nil {
				return fmt.Errorf("reading tickets: %w", err)
			}
			names := agentNames(cmd.Context(), db)

			// Open-ticket load per assigned agent (unassigned excluded).
			loads := map[string]int{}
			byAgent := map[string][]string{}
			for _, t := range tickets {
				if isClosedStatus(str(t, "status")) {
					continue
				}
				aid := str(t, "assigneeId")
				if aid == "" {
					continue
				}
				loads[aid]++
				byAgent[aid] = append(byAgent[aid], str(t, "id"))
			}

			nameOf := func(aid string) string {
				if n := names[aid]; n != "" {
					return n
				}
				return aid
			}

			moves := make([]move, 0)
			// Greedy: while the gap between the most- and least-loaded agents
			// exceeds 1, shift one ticket from max to min.
			for len(loads) >= 2 {
				var maxA, minA string
				for aid := range loads {
					if maxA == "" || loads[aid] > loads[maxA] {
						maxA = aid
					}
					if minA == "" || loads[aid] < loads[minA] {
						minA = aid
					}
				}
				if maxA == minA || loads[maxA]-loads[minA] <= 1 {
					break
				}
				if len(byAgent[maxA]) == 0 {
					break
				}
				tid := byAgent[maxA][len(byAgent[maxA])-1]
				byAgent[maxA] = byAgent[maxA][:len(byAgent[maxA])-1]
				byAgent[minA] = append(byAgent[minA], tid)
				loads[maxA]--
				loads[minA]++
				moves = append(moves, move{
					TicketID:    tid,
					FromAgentID: maxA,
					FromName:    nameOf(maxA),
					ToAgentID:   minA,
					ToName:      nameOf(minA),
				})
			}

			mode := "plan"
			movesApplied := 0
			failures := make([]map[string]string, 0)
			if flagApply {
				mode = "apply"
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				applyCap := len(moves)
				if cliutil.IsDogfoodEnv() && applyCap > 1 {
					applyCap = 1
				}
				// Accumulate per-move failures instead of bailing on the first,
				// so movesApplied and the failure list are always reported and
				// partial progress is visible to the caller.
				for i := 0; i < applyCap; i++ {
					m := moves[i]
					body := map[string]any{"assigneeId": m.ToAgentID}
					if _, _, err := c.Patch(cmd.Context(), "/tickets/"+m.TicketID, body); err != nil {
						failures = append(failures, map[string]string{"ticketId": m.TicketID, "error": err.Error()})
						continue
					}
					movesApplied++
				}
				if len(failures) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d reassignments failed; %d applied\n", len(failures), applyCap, movesApplied)
				}
			}

			view := struct {
				Mode           string              `json:"mode"`
				Moves          []move              `json:"moves"`
				MovesApplied   int                 `json:"movesApplied"`
				Failures       []map[string]string `json:"failures,omitempty"`
				ScannedTickets int                 `json:"scanned_tickets"`
			}{
				Mode:           mode,
				Moves:          moves,
				MovesApplied:   movesApplied,
				Failures:       failures,
				ScannedTickets: len(tickets),
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().BoolVar(&flagPlan, "plan", true, "Compute and print the rebalance plan without writing (default)")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Actually reassign tickets via the live API")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

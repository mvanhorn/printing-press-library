// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE resource list/get commands (not generated). These replace
// the generator's per-endpoint GraphQL command stubs, whose root-level `nodes`
// query shape does not match DICE's `viewer { conn { edges { node } } }` API.
// They issue correct GraphQL via the read-only transport path (see
// dice_query.go) and reuse the generated output pipeline for --json/--csv/etc.
package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// runList is the shared body for a live connection-list command: short-circuit
// under --dry-run, fetch the viewer connection with the given where-filter, and
// emit the nodes through the standard output pipeline.
func runList(cmd *cobra.Command, flags *rootFlags, resource string, where map[string]any, limit int) error {
	if dryRunOK(flags) {
		return nil
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	nodes, _, err := fetchConnection(cmd.Context(), c, resource, where, dicePerPage, limit, "")
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return outputNodes(cmd, flags, nodes)
}

func newEventsListCmd(flags *rootFlags) *cobra.Command {
	var state string
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your DICE events, optionally filtered by state (APPROVED, DRAFT, CANCELLED, ...); returns id, name, date, and venue",
		Example:     "  dice-fm-pp-cli events list --state APPROVED --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var where map[string]any
			if state != "" {
				where = eqWhere("state", strings.ToUpper(state))
			}
			return runList(cmd, flags, "events", where, limit)
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Filter by state: APPROVED, ARCHIVED, CANCELLED, DECLINED, DRAFT, REVIEW, SUBMITTED")
	cmd.Flags().IntVar(&limit, "limit", diceDefaultListLimit, "Max events to return (0 = all pages)")
	return cmd
}

func newEventsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <id>",
		Short:       "Get a single event by ID",
		Example:     "  dice-fm-pp-cli events get RXZlbnQ6MTIzNDU= --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			node, err := fetchNodeByID(cmd.Context(), c, "Event", eventSelection, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), node, flags)
		},
	}
	return cmd
}

func newTicketsPromotedCmd(flags *rootFlags) *cobra.Command {
	var event, fanPhone string
	var limit int
	cmd := &cobra.Command{
		Use:         "tickets",
		Short:       "List sold tickets with holder details, pricing, and claim status",
		Example:     "  dice-fm-pp-cli tickets --event RXZlbnQ6MTIzNDU= --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var clauses []map[string]any
			if event != "" {
				clauses = append(clauses, eqWhere("eventId", event))
			}
			if fanPhone != "" {
				clauses = append(clauses, eqWhere("fanPhoneNumber", fanPhone))
			}
			return runList(cmd, flags, "tickets", mergeWhere(clauses...), limit)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Filter by event ID")
	cmd.Flags().StringVar(&fanPhone, "fan-phone", "", "Filter by fan phone number")
	cmd.Flags().IntVar(&limit, "limit", diceDefaultListLimit, "Max tickets to return (0 = all pages)")
	return cmd
}

func newOrdersPromotedCmd(flags *rootFlags) *cobra.Command {
	var event string
	var limit int
	cmd := &cobra.Command{
		Use:         "orders",
		Short:       "List ticket purchase orders with financial and geographic data",
		Example:     "  dice-fm-pp-cli orders --event RXZlbnQ6MTIzNDU= --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var where map[string]any
			if event != "" {
				where = eqWhere("eventId", event)
			}
			return runList(cmd, flags, "orders", where, limit)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Filter by event ID")
	cmd.Flags().IntVar(&limit, "limit", diceDefaultListLimit, "Max orders to return (0 = all pages)")
	return cmd
}

func newReturnsPromotedCmd(flags *rootFlags) *cobra.Command {
	var event string
	var limit int
	cmd := &cobra.Command{
		Use:         "returns",
		Short:       "List ticket returns and refunds",
		Example:     "  dice-fm-pp-cli returns --event RXZlbnQ6MTIzNDU= --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var where map[string]any
			if event != "" {
				where = eqWhere("eventId", event)
			}
			return runList(cmd, flags, "returns", where, limit)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Filter by event ID")
	cmd.Flags().IntVar(&limit, "limit", diceDefaultListLimit, "Max returns to return (0 = all pages)")
	return cmd
}

func newTransfersPromotedCmd(flags *rootFlags) *cobra.Command {
	var event string
	var limit int
	cmd := &cobra.Command{
		Use:         "transfers",
		Short:       "List ticket transfers between fans",
		Example:     "  dice-fm-pp-cli transfers --event RXZlbnQ6MTIzNDU= --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var where map[string]any
			if event != "" {
				where = eqWhere("eventId", event)
			}
			return runList(cmd, flags, "transfers", where, limit)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Filter by event ID")
	cmd.Flags().IntVar(&limit, "limit", diceDefaultListLimit, "Max transfers to return (0 = all pages)")
	return cmd
}

func newExtrasPromotedCmd(flags *rootFlags) *cobra.Command {
	var event string
	var separateBarcode bool
	var limit int
	cmd := &cobra.Command{
		Use:         "extras",
		Short:       "List extras and add-ons sold with tickets",
		Example:     "  dice-fm-pp-cli extras --event RXZlbnQ6MTIzNDU= --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var clauses []map[string]any
			if event != "" {
				clauses = append(clauses, eqWhere("eventId", event))
			}
			if separateBarcode {
				clauses = append(clauses, eqWhere("hasSeparateAccessBarcode", true))
			}
			return runList(cmd, flags, "extras", mergeWhere(clauses...), limit)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Filter by event ID")
	cmd.Flags().BoolVar(&separateBarcode, "separate-barcode", false, "Only extras that have a separate access barcode")
	cmd.Flags().IntVar(&limit, "limit", diceDefaultListLimit, "Max extras to return (0 = all pages)")
	return cmd
}

func newGenresPromotedCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "genres",
		Short:       "List event genre types and their child genres",
		Example:     "  dice-fm-pp-cli genres --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, flags, "genres", nil, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", diceDefaultListLimit, "Max genre types to return (0 = all pages)")
	return cmd
}

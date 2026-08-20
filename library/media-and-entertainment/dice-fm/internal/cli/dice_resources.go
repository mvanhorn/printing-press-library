// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE resource list/get commands (not generated). These replace
// the generator's per-endpoint GraphQL command stubs, whose root-level `nodes`
// query shape does not match DICE's `viewer { conn { edges { node } } }` API.
// They issue correct GraphQL via the read-only transport path (see
// dice_query.go) and reuse the generated output pipeline for --json/--csv/etc.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// fetchListNodes fetches a connection's nodes, emitting a stderr warning when the
// result was capped by limit so a partial result is never silent.
func fetchListNodes(cmd *cobra.Command, flags *rootFlags, resource string, where map[string]any, limit int) ([]json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	nodes, _, truncated, err := fetchConnection(cmd.Context(), c, resource, where, dicePerPage, limit, "", false)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	if truncated {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: showing first %d %s; more records exist — pass --limit 0 to fetch all\n", len(nodes), resource)
	}
	return nodes, nil
}

// runList is the shared body for a live connection-list command: short-circuit
// under --dry-run, fetch the viewer connection with the given where-filter, and
// emit the nodes through the standard output pipeline.
func runList(cmd *cobra.Command, flags *rootFlags, resource string, where map[string]any, limit int) error {
	if dryRunOK(flags) {
		return nil
	}
	nodes, err := fetchListNodes(cmd, flags, resource, where, limit)
	if err != nil {
		return err
	}
	return outputNodes(cmd, flags, nodes)
}

// effectiveLimit returns 0 (all pages) when a command is scoped to a single event
// and the user did not explicitly set --limit, so per-event queries return every
// record rather than silently capping at the browse default.
func effectiveLimit(cmd *cobra.Command, event string, limit int) int {
	if event != "" && !cmd.Flags().Changed("limit") {
		return 0
	}
	return limit
}

// parseDateFlag validates a YYYY-MM-DD flag value, returning it unchanged. An
// empty value is allowed (open bound).
func parseDateFlag(name, v string) (string, error) {
	if v == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return "", fmt.Errorf("--%s must be a YYYY-MM-DD date: %w", name, err)
	}
	return v, nil
}

// filterByShowDate keeps event nodes whose startDatetime falls in the [from, to]
// inclusive date window (YYYY-MM-DD). Empty bounds are open. Comparison is on the
// date prefix, which is correct for ISO-8601 timestamps.
func filterByShowDate(nodes []json.RawMessage, from, to string) []json.RawMessage {
	if from == "" && to == "" {
		return nodes
	}
	out := make([]json.RawMessage, 0, len(nodes))
	for _, n := range nodes {
		var e struct {
			StartDatetime string `json:"startDatetime"`
		}
		_ = json.Unmarshal(n, &e)
		d := e.StartDatetime
		if len(d) >= 10 {
			d = d[:10]
		}
		if from != "" && d < from {
			continue
		}
		if to != "" && d > to {
			continue
		}
		out = append(out, n)
	}
	return out
}

func newEventsListCmd(flags *rootFlags) *cobra.Command {
	var state, from, to string
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your DICE events, optionally filtered by state and show-date window; returns id, name, date, and venue",
		Example:     "  dice-fm-pp-cli events list --state APPROVED --from 2026-04-26 --to 2026-05-26 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if from, err = parseDateFlag("from", from); err != nil {
				return err
			}
			if to, err = parseDateFlag("to", to); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			var where map[string]any
			if state != "" {
				where = eqWhere("state", strings.ToUpper(state))
			}
			nodes, err := fetchListNodes(cmd, flags, "events", where, limit)
			if err != nil {
				return err
			}
			return outputNodes(cmd, flags, filterByShowDate(nodes, from, to))
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Filter by state: APPROVED, ARCHIVED, CANCELLED, DECLINED, DRAFT, REVIEW, SUBMITTED")
	cmd.Flags().StringVar(&from, "from", "", "Only include shows on or after this date (YYYY-MM-DD, by show date)")
	cmd.Flags().StringVar(&to, "to", "", "Only include shows on or before this date (YYYY-MM-DD, by show date)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max events to fetch (0 = all pages; default fetches all)")
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
			return runList(cmd, flags, "tickets", mergeWhere(clauses...), effectiveLimit(cmd, event, limit))
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
			return runList(cmd, flags, "orders", where, effectiveLimit(cmd, event, limit))
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
			return runList(cmd, flags, "returns", where, effectiveLimit(cmd, event, limit))
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
			return runList(cmd, flags, "transfers", where, effectiveLimit(cmd, event, limit))
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
			return runList(cmd, flags, "extras", mergeWhere(clauses...), effectiveLimit(cmd, event, limit))
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

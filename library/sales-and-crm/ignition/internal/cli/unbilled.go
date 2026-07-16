// Copyright 2026 corben-tech and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type unbilledClientView struct {
	ClientID   string   `json:"client_id"`
	ClientName string   `json:"client_name"`
	Amount     float64  `json:"amount"`
	ItemCount  int      `json:"item_count"`
	Services   []string `json:"services"`
}

type unbilledView struct {
	Unbilled   []unbilledClientView `json:"unbilled"`
	GrandTotal float64              `json:"grand_total"`
	ItemCount  int                  `json:"item_count"`
	Scanned    int                  `json:"scanned"`
}

func newNovelUnbilledCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "unbilled",
		Short:       "Show agreed work not yet invoiced (BILLING_ITEM status UNBILLED) — the invoicing to-do list.",
		Example:     "  ignition-pp-cli unbilled --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch billing items")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			billingItems, err := fetchSearchIndex(ctx, c, "BILLING_ITEM", billingSearchQuery, "pagedBillingItems")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := buildUnbilledView(billingItems)
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printUnbilledTable(cmd.OutOrStdout(), view)
			return nil
		},
	}
	return cmd
}

func buildUnbilledView(billingItems []searchNode) unbilledView {
	byClient := map[string]*unbilledClientView{}
	view := unbilledView{Unbilled: []unbilledClientView{}, Scanned: len(billingItems)}
	for _, node := range billingItems {
		if !strings.EqualFold(strings.TrimSpace(node.BillingItemStatus), "UNBILLED") {
			continue
		}
		clientID, clientName := nodeClient(node)
		key := clientID
		if key == "" {
			key = clientName
		}
		if key == "" {
			key = "(unknown)"
		}
		row := byClient[key]
		if row == nil {
			row = &unbilledClientView{ClientID: clientID, ClientName: clientName, Services: []string{}}
			byClient[key] = row
		}
		amount := 0.0
		if node.Amount != nil {
			amount = parseMoneyFormat(node.Amount.Format)
		}
		row.Amount += amount
		row.ItemCount++
		if node.ServiceName != "" {
			row.Services = append(row.Services, node.ServiceName)
		}
		view.GrandTotal += amount
		view.ItemCount++
	}
	for _, row := range byClient {
		view.Unbilled = append(view.Unbilled, *row)
	}
	sort.Slice(view.Unbilled, func(i, j int) bool {
		return view.Unbilled[i].ClientName < view.Unbilled[j].ClientName
	})
	return view
}

func printUnbilledTable(w io.Writer, view unbilledView) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CLIENT\tAMOUNT\tITEMS\tSERVICES")
	for _, row := range view.Unbilled {
		name := row.ClientName
		if name == "" {
			name = row.ClientID
		}
		if name == "" {
			name = "(unknown)"
		}
		fmt.Fprintf(tw, "%s\t%.2f\t%d\t%s\n", name, row.Amount, row.ItemCount, strings.Join(row.Services, ", "))
	}
	fmt.Fprintf(tw, "TOTAL\t%.2f\t%d\t\n", view.GrandTotal, view.ItemCount)
	_ = tw.Flush()
}

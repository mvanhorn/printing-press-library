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

type outstandingClientView struct {
	ClientID         string  `json:"client_id"`
	ClientName       string  `json:"client_name"`
	OutstandingTotal float64 `json:"outstanding_total"`
	ItemCount        int     `json:"item_count"`
}

type outstandingView struct {
	Clients      []outstandingClientView `json:"clients"`
	GrandTotal   float64                 `json:"grand_total"`
	InvoiceCount int                     `json:"invoice_count"`
	Scanned      int                     `json:"scanned"`
}

func newNovelOutstandingCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "outstanding",
		Short:       "Show genuine open/failed accounts receivable (unpaid invoices), not scheduled billing.",
		Example:     "  ignition-pp-cli outstanding --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch invoices")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			invoices, err := fetchSearchIndex(ctx, c, "INVOICE", invoiceSearchQuery, "pagedInvoices")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := buildOutstandingView(invoices)
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printOutstandingTable(cmd.OutOrStdout(), view)
			return nil
		},
	}
	return cmd
}

func buildOutstandingView(nodes []searchNode) outstandingView {
	byClient := map[string]*outstandingClientView{}
	view := outstandingView{Clients: []outstandingClientView{}, Scanned: len(nodes)}
	for _, node := range nodes {
		if !isInvoiceSearchNode(node) {
			continue
		}
		if !isOutstandingStatus(nodeStatus(node)) {
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
			row = &outstandingClientView{ClientID: clientID, ClientName: clientName}
			byClient[key] = row
		}
		row.OutstandingTotal += nodeAmount(node)
		row.ItemCount++
		view.GrandTotal += nodeAmount(node)
		view.InvoiceCount++
	}
	for _, row := range byClient {
		view.Clients = append(view.Clients, *row)
	}
	sort.Slice(view.Clients, func(i, j int) bool {
		return view.Clients[i].ClientName < view.Clients[j].ClientName
	})
	return view
}

func isOutstandingStatus(status string) bool {
	token := strings.ToUpper(strings.TrimSpace(status))
	token = strings.NewReplacer("-", "_").Replace(token)
	token = strings.Join(strings.Fields(token), "_")
	switch token {
	case "", "PAID_OUT", "REFUNDED", "CANCELED", "DISPUTE_LOST":
		return false
	default:
		return true
	}
}

func isInvoiceSearchNode(node searchNode) bool {
	return strings.EqualFold(node.TypeName, "InvoiceResult") ||
		node.PaymentProgress != nil ||
		node.PaymentStatus != nil ||
		node.ExternalNumber != ""
}

func nodeClient(node searchNode) (string, string) {
	if node.Client == nil {
		return "", ""
	}
	return node.Client.ID, node.Client.Name
}

func printOutstandingTable(w io.Writer, view outstandingView) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CLIENT\tOUTSTANDING\tITEMS")
	for _, client := range view.Clients {
		name := client.ClientName
		if name == "" {
			name = client.ClientID
		}
		if name == "" {
			name = "(unknown)"
		}
		fmt.Fprintf(tw, "%s\t%.2f\t%d\n", name, client.OutstandingTotal, client.ItemCount)
	}
	fmt.Fprintf(tw, "TOTAL\t%.2f\t%d\n", view.GrandTotal, view.InvoiceCount)
	_ = tw.Flush()
}

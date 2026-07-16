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

type rejectedPaymentItemView struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
}

type rejectedPaymentClientView struct {
	ClientID   string                    `json:"client_id,omitempty"`
	ClientName string                    `json:"client_name,omitempty"`
	Items      []rejectedPaymentItemView `json:"items"`
}

type rejectedPaymentsView struct {
	Clients []rejectedPaymentClientView `json:"clients"`
	Count   int                         `json:"count"`
}

func newNovelRejectedPaymentsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "rejected-payments",
		Short:       "List clients with a rejected or failed payment so collections can follow up.",
		Example:     "  ignition-pp-cli rejected-payments --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch invoices and billing items")
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
			billingItems, err := fetchSearchIndex(ctx, c, "BILLING_ITEM", billingSearchQuery, "pagedBillingItems")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := buildRejectedPaymentsView(append(invoices, billingItems...))
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printRejectedPaymentsTable(cmd.OutOrStdout(), view)
			return nil
		},
	}
	return cmd
}

func buildRejectedPaymentsView(nodes []searchNode) rejectedPaymentsView {
	byClient := map[string]*rejectedPaymentClientView{}
	var view rejectedPaymentsView
	for _, node := range nodes {
		status := nodeStatus(node)
		if !isRejectedPaymentStatus(status) {
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
			row = &rejectedPaymentClientView{ClientID: clientID, ClientName: clientName}
			byClient[key] = row
		}
		row.Items = append(row.Items, rejectedPaymentItemView{ID: nodeDisplayID(node), Status: status})
		view.Count++
	}
	for _, row := range byClient {
		view.Clients = append(view.Clients, *row)
	}
	sort.Slice(view.Clients, func(i, j int) bool {
		return view.Clients[i].ClientName < view.Clients[j].ClientName
	})
	return view
}

func isRejectedPaymentStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return strings.Contains(s, "reject") ||
		strings.Contains(s, "fail") ||
		strings.Contains(s, "declin") ||
		strings.Contains(s, "overdue")
}

func printRejectedPaymentsTable(w io.Writer, view rejectedPaymentsView) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CLIENT\tITEMS")
	for _, client := range view.Clients {
		name := client.ClientName
		if name == "" {
			name = client.ClientID
		}
		if name == "" {
			name = "(unknown)"
		}
		fmt.Fprintf(tw, "%s\t%d\n", name, len(client.Items))
	}
	_ = tw.Flush()
}

// Copyright 2026 corben-tech and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type clientBillingView struct {
	ClientID         string       `json:"client_id"`
	ClientName       string       `json:"client_name,omitempty"`
	Proposals        []searchNode `json:"proposals"`
	Invoices         []searchNode `json:"invoices"`
	BillingItems     []searchNode `json:"billing_items"`
	ProposedCount    int          `json:"proposed_count"`
	InvoicedTotal    float64      `json:"invoiced_total"`
	OutstandingTotal float64      `json:"outstanding_total"`
}

func newNovelClientBillingCmd(flags *rootFlags) *cobra.Command {
	var flagClientId string

	cmd := &cobra.Command{
		Use:         "client-billing",
		Short:       "One client's full money picture: what was proposed, invoiced, and still outstanding.",
		Example:     "  ignition-pp-cli client-billing --client-id cli_example --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch proposals, invoices, and billing items")
				return nil
			}
			if strings.TrimSpace(flagClientId) == "" {
				return usageErr(fmt.Errorf("--client-id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			proposals, err := fetchSearchIndex(ctx, c, "PROPOSAL", proposalSearchQuery, "pagedProposals")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			invoices, err := fetchSearchIndex(ctx, c, "INVOICE", invoiceSearchQuery, "pagedInvoices")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			billingItems, err := fetchSearchIndex(ctx, c, "BILLING_ITEM", billingSearchQuery, "pagedBillingItems")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := buildClientBillingView(flagClientId, proposals, invoices, billingItems)
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printClientBillingTable(cmd.OutOrStdout(), view)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagClientId, "client-id", "", "Ignition client ID to summarize")
	return cmd
}

func buildClientBillingView(clientID string, proposals, invoices, billingItems []searchNode) clientBillingView {
	view := clientBillingView{ClientID: clientID}
	for _, node := range proposals {
		if node.Client == nil || node.Client.ID != clientID {
			continue
		}
		view.Proposals = append(view.Proposals, node)
		view.ProposedCount++
		if view.ClientName == "" {
			view.ClientName = node.Client.Name
		}
	}
	for _, node := range invoices {
		if node.Client == nil || node.Client.ID != clientID {
			continue
		}
		view.Invoices = append(view.Invoices, node)
		view.InvoicedTotal += nodeAmount(node)
		if isOutstandingStatus(nodeStatus(node)) {
			view.OutstandingTotal += nodeAmount(node)
		}
		if view.ClientName == "" {
			view.ClientName = node.Client.Name
		}
	}
	for _, node := range billingItems {
		if node.Client == nil || node.Client.ID != clientID {
			continue
		}
		view.BillingItems = append(view.BillingItems, node)
		if view.ClientName == "" {
			view.ClientName = node.Client.Name
		}
	}
	return view
}

func printClientBillingTable(w io.Writer, view clientBillingView) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CLIENT\tPROPOSALS\tINVOICED\tOUTSTANDING")
	name := view.ClientName
	if name == "" {
		name = view.ClientID
	}
	fmt.Fprintf(tw, "%s\t%d\t%.2f\t%.2f\n", name, view.ProposedCount, view.InvoicedTotal, view.OutstandingTotal)
	_ = tw.Flush()
}

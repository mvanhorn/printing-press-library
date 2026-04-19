// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written promoted command. Spec-driven shape declared in spec.yaml.

package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newTransactionsPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transactions <sport> <league>",
		Short: "Recent league transactions (trades, signings, waivers)",
		Example: `  espn-pp-cli transactions baseball mlb
  espn-pp-cli transactions football nfl --agent
  espn-pp-cli transactions hockey nhl --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: transactions <sport> <league>"))
			}
			sport, league := args[0], args[1]

			// Transactions live on the core API host.
			url := fmt.Sprintf("https://sports.core.api.espn.com/v2/sports/%s/leagues/%s/transactions", sport, league)

			body, err := espnHTTPGet(flags.timeout, url)
			if err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				var raw json.RawMessage
				if err := json.Unmarshal(body, &raw); err != nil {
					return err
				}
				return enc.Encode(raw)
			}

			return renderTransactions(cmd.OutOrStdout(), body)
		},
	}
	return cmd
}

func renderTransactions(w io.Writer, data []byte) error {
	var resp struct {
		Count int `json:"count"`
		Items []struct {
			Date        string `json:"date"`
			Type        string `json:"type"`
			Description string `json:"description"`
			From        struct {
				Ref string `json:"$ref"`
			} `json:"from"`
			To struct {
				Ref string `json:"$ref"`
			} `json:"to"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing transactions: %w", err)
	}

	if len(resp.Items) == 0 {
		fmt.Fprintln(w, "No transactions found.")
		return nil
	}

	tw := newTabWriter(w)
	fmt.Fprintf(tw, "%s\t%s\t%s\n",
		bold("DATE"), bold("TYPE"), bold("DESCRIPTION"))
	for _, t := range resp.Items {
		date := t.Date
		if len(date) > 10 {
			date = date[:10]
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n",
			date, t.Type, truncate(t.Description, 80))
	}
	return tw.Flush()
}

// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written promoted command. Spec-driven shape declared in spec.yaml.

package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newPlaysPromotedCmd(flags *rootFlags) *cobra.Command {
	var event string
	var limit int

	cmd := &cobra.Command{
		Use:   "plays <sport> <league>",
		Short: "Play-by-play feed for a specific event",
		Example: `  espn-pp-cli plays football nfl --event 401547417
  espn-pp-cli plays basketball nba --event 401584793 --limit 50
  espn-pp-cli plays baseball mlb --event 401569551 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: plays <sport> <league> --event <id>"))
			}
			if event == "" {
				return usageErr(fmt.Errorf("--event is required"))
			}
			sport, league := args[0], args[1]

			if limit <= 0 {
				limit = 200
			}

			// Plays endpoint lives on the core API host.
			url := fmt.Sprintf(
				"https://sports.core.api.espn.com/v2/sports/%s/leagues/%s/events/%s/competitions/%s/plays?limit=%d",
				sport, league, event, event, limit)

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

			return renderPlays(cmd.OutOrStdout(), body)
		},
	}

	cmd.Flags().StringVar(&event, "event", "", "ESPN event/game id (required)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum number of plays to return")
	return cmd
}

func renderPlays(w io.Writer, data []byte) error {
	var resp struct {
		Count int `json:"count"`
		Items []struct {
			Sequence string `json:"sequenceNumber"`
			Type     struct {
				Text string `json:"text"`
			} `json:"type"`
			Text   string `json:"text"`
			Period struct {
				Number int `json:"number"`
			} `json:"period"`
			Clock struct {
				DisplayValue string `json:"displayValue"`
			} `json:"clock"`
			HomeScore int `json:"homeScore"`
			AwayScore int `json:"awayScore"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing plays: %w", err)
	}

	if len(resp.Items) == 0 {
		fmt.Fprintln(w, "No plays found.")
		return nil
	}

	tw := newTabWriter(w)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
		bold("PERIOD"), bold("CLOCK"), bold("SCORE"), bold("TYPE"), bold("PLAY"))
	for _, p := range resp.Items {
		score := fmt.Sprintf("%d-%d", p.AwayScore, p.HomeScore)
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			p.Period.Number,
			p.Clock.DisplayValue,
			score,
			truncate(p.Type.Text, 20),
			truncate(p.Text, 80))
	}
	return tw.Flush()
}

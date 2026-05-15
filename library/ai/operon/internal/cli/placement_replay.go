// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/store"
)

type replayDiff struct {
	ImpressionID        string   `json:"impression_id"`
	OriginalWinner      string   `json:"original_winner_advertiser_id"`
	ReplayedWinner      string   `json:"replayed_winner_advertiser_id"`
	WinnerMatches       bool     `json:"winner_matches"`
	OriginalScoutScore  *float64 `json:"original_scout_score,omitempty"`
	ReplayedScoutScore  *float64 `json:"replayed_scout_score,omitempty"`
	OriginalDecision    string   `json:"original_decision"`
	ReplayedDecision    string   `json:"replayed_decision"`
	OriginalRankingSize int      `json:"original_ranking_size"`
	ReplayedRankingSize int      `json:"replayed_ranking_size"`
	Notes               string   `json:"notes,omitempty"`
}

func newPlacementReplayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "replay <impression-id>",
		Short: "Re-issue a stored placement request and diff the new auction outcome against the original.",
		Long: `Look up the stored placement by impression id, replay the original request
context to api.operon.so/placement, and diff:

  - winner advertiser id (match? yes/no)
  - winner scoutScore (drift?)
  - decision (filled vs blocked)
  - ranking size

Useful for catching stealth ranking changes, scoutScore drift, or behavior
shifts after a publisher's category gets re-classified.`,
		Example: strings.Trim(`
  operon-pp-cli placement replay imp_a1b2c3d4e5f60718
  operon-pp-cli placement replay imp_a1b2c3d4e5f60718 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			impID := strings.TrimSpace(args[0])
			if impID == "" {
				return cmd.Help()
			}

			path := dbPath
			if path == "" {
				path = store.DefaultPath("operon-pp-cli")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query store: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch placement: %s\n", impID)
				fmt.Fprintf(cmd.OutOrStdout(), "would re-POST /placement with stored request_context\n")
				return nil
			}

			ctx := context.Background()
			st, err := store.Open(ctx, path)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer st.Close()

			original, err := st.GetPlacement(ctx, impID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf(
						"placement %q not found in local store\nhint: only placements logged via 'operon-pp-cli sync' or earlier replays are searchable. The /placement API does not expose a listing endpoint.",
						impID,
					))
				}
				return apiErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var requestBody map[string]any
			if err := json.Unmarshal([]byte(original.RequestContextJSON), &requestBody); err != nil {
				return apiErr(fmt.Errorf("parsing stored request context: %w", err))
			}

			data, _, err := c.Post("/placement", requestBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var replayed map[string]any
			if err := json.Unmarshal(data, &replayed); err != nil {
				return apiErr(fmt.Errorf("parsing replayed placement response: %w", err))
			}

			diff := replayDiff{
				ImpressionID:        impID,
				OriginalWinner:      original.WinnerAdvertiserID,
				OriginalScoutScore:  original.ScoutScore,
				OriginalDecision:    original.ResponseDecision,
				OriginalRankingSize: rankingSize(original.AuctionJSON),
			}

			if pl, ok := replayed["placement"].(map[string]any); ok {
				if v, ok := pl["advertiserId"].(string); ok {
					diff.ReplayedWinner = v
				}
				if v, ok := pl["scoutScore"].(float64); ok {
					f := v
					diff.ReplayedScoutScore = &f
				}
			}
			if v, ok := replayed["decision"].(string); ok {
				diff.ReplayedDecision = v
			}
			if a, ok := replayed["auction"].(map[string]any); ok {
				if r, ok := a["ranking"].([]any); ok {
					diff.ReplayedRankingSize = len(r)
				}
			}

			diff.WinnerMatches = diff.OriginalWinner == diff.ReplayedWinner

			if !diff.WinnerMatches {
				diff.Notes = fmt.Sprintf("winner changed: %s -> %s", diff.OriginalWinner, diff.ReplayedWinner)
			} else if diff.OriginalDecision != diff.ReplayedDecision {
				diff.Notes = fmt.Sprintf("decision changed: %s -> %s", diff.OriginalDecision, diff.ReplayedDecision)
			}

			if flags.asJSON || flags.csv || flags.compact || flags.selectFields != "" {
				return printJSONFiltered(cmd.OutOrStdout(), diff, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "impression_id    : %s\n", diff.ImpressionID)
			fmt.Fprintf(w, "winner_orig      : %s\n", diff.OriginalWinner)
			fmt.Fprintf(w, "winner_replayed  : %s\n", diff.ReplayedWinner)
			fmt.Fprintf(w, "winner_matches   : %t\n", diff.WinnerMatches)
			fmt.Fprintf(w, "decision_orig    : %s\n", diff.OriginalDecision)
			fmt.Fprintf(w, "decision_replay  : %s\n", diff.ReplayedDecision)
			fmt.Fprintf(w, "ranking_size     : %d -> %d\n", diff.OriginalRankingSize, diff.ReplayedRankingSize)
			if diff.Notes != "" {
				fmt.Fprintf(w, "notes            : %s\n", diff.Notes)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the default store path")
	return cmd
}

// rankingSize parses a stored auction_json blob and returns len(ranking).
// Returns 0 on any parse failure so a malformed historical row does not
// fail the replay.
func rankingSize(auctionJSON string) int {
	if auctionJSON == "" {
		return 0
	}
	var a map[string]any
	if err := json.Unmarshal([]byte(auctionJSON), &a); err != nil {
		return 0
	}
	if r, ok := a["ranking"].([]any); ok {
		return len(r)
	}
	return 0
}

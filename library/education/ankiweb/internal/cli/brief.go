// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/store"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

// briefTopDeck is one of the top-by-approval entries in a brief.
type briefTopDeck struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Approval string `json:"approval"`
	Votes    int    `json:"total_votes"`
}

// briefReport is the composed digest for a search term.
type briefReport struct {
	Term             string         `json:"term"`
	ResultCount      int            `json:"result_count"`
	TopByApproval    []briefTopDeck `json:"top_by_approval"`
	AudioCoverage    string         `json:"audio_coverage"`
	AudioCoveragePct float64        `json:"audio_coverage_pct"`
	FreshestDeck     *briefTopDeck  `json:"freshest_deck,omitempty"`
	FreshestDate     string         `json:"freshest_date,omitempty"`
	NewSinceLastSync *int           `json:"new_since_last_sync,omitempty"`
}

func newBriefCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "brief <term>",
		Short:       "One digest for a topic: top decks by approval, audio coverage, freshest deck, new-since-sync",
		Example:     "  ankiweb-pp-cli brief spanish --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			term := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), briefReport{Term: term, TopByApproval: []briefTopDeck{}}, flags)
			}

			decks, err := flags.loadSharedDecks(cmd, term)
			if err != nil {
				return err
			}

			report := briefReport{Term: term, ResultCount: len(decks), TopByApproval: []briefTopDeck{}}

			// Top 5 by approval rate among decks with >= 10 votes.
			eligible := make([]svc.SharedDeck, 0, len(decks))
			withAudio := 0
			var freshest svc.SharedDeck
			for _, d := range decks {
				if d.Audio > 0 {
					withAudio++
				}
				if d.Modified > freshest.Modified {
					freshest = d
				}
				if d.TotalVotes() >= 10 {
					eligible = append(eligible, d)
				}
			}
			sortByApproval(eligible)
			for i, d := range eligible {
				if i >= 5 {
					break
				}
				report.TopByApproval = append(report.TopByApproval, briefTopDeck{
					ID: d.ID, Title: d.Title, Approval: approvalPct(d), Votes: d.TotalVotes(),
				})
			}

			if len(decks) > 0 {
				pct := float64(withAudio) / float64(len(decks)) * 100
				report.AudioCoveragePct = pct
				report.AudioCoverage = fmt.Sprintf("%.1f%% (%d/%d)", pct, withAudio, len(decks))
				if freshest.ID != "" {
					report.FreshestDeck = &briefTopDeck{
						ID: freshest.ID, Title: freshest.Title, Approval: approvalPct(freshest), Votes: freshest.TotalVotes(),
					}
					report.FreshestDate = modifiedDate(freshest.Modified)
				}
			}

			// new-since-last-sync, only when a prior watch snapshot exists.
			if dbPath == "" {
				dbPath = defaultDBPath("ankiweb-pp-cli")
			}
			if db, derr := store.OpenWithContext(cmd.Context(), dbPath); derr == nil {
				rt := watchResourceType(term)
				prior := map[string]bool{}
				if rows, _ := db.List(rt, 100000); len(rows) > 0 {
					for _, raw := range rows {
						var d svc.SharedDeck
						if json.Unmarshal(raw, &d) == nil {
							prior[d.ID] = true
						}
					}
					n := 0
					for _, d := range decks {
						if !prior[d.ID] {
							n++
						}
					}
					report.NewSinceLastSync = &n
				}
				db.Close()
			}

			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ankiweb-pp-cli/data.db)")
	return cmd
}

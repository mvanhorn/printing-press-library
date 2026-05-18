// Copyright 2026 quoxientzero. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newLeaderboardCmd surfaces Skool's leaderboard data. Skool embeds the
// leaderboard inline on the community page response; we extract it and
// project a tight per-row shape suitable for agents.
func newLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var flagCommunity string
	var flagType string
	var flagTop int

	cmd := &cobra.Command{
		Use:   "leaderboard",
		Short: "Current leaderboard for a Skool community (top members by points)",
		Example: "  skool-pp-cli leaderboard --community bewarethedefault --type 30d --top 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			community := strings.TrimSpace(flagCommunity)
			if community == "" && c.Config != nil {
				community = c.Config.TemplateVars["community"]
			}
			if community == "" {
				return usageErr(fmt.Errorf("--community is required (or set template_vars.community in config)"))
			}

			// The leaderboard data lives inline on the community page response.
			path := "/_next/data/{buildId}/" + community + ".json"
			params := map[string]string{
				"g": community,
				"t": "leaderboard",
			}
			raw, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse just the bits we need.
			var envelope struct {
				PageProps struct {
					LeaderboardsData struct {
						Type      string  `json:"type"`
						UpdatedAt string  `json:"updatedAt"`
						Users     []struct {
							UserID string `json:"userId"`
							Rank   int    `json:"rank"`
							Points int    `json:"points"`
							User   struct {
								ID        string `json:"id"`
								Name      string `json:"name"`
								FirstName string `json:"firstName"`
								LastName  string `json:"lastName"`
								Metadata  struct {
									SPData       string `json:"spData"`
									Bio          string `json:"bio"`
									Location     string `json:"location"`
									LinkLinkedin string `json:"linkLinkedin"`
								} `json:"metadata"`
							} `json:"user"`
						} `json:"users"`
					} `json:"leaderboardsData"`
				} `json:"pageProps"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				return fmt.Errorf("parsing leaderboard response: %w", err)
			}

			rows := envelope.PageProps.LeaderboardsData.Users
			if flagTop > 0 && flagTop < len(rows) {
				rows = rows[:flagTop]
			}
			type out struct {
				Rank        int    `json:"rank"`
				Points      int    `json:"points"`
				UserID      string `json:"user_id"`
				Name        string `json:"name"`
				FullName    string `json:"full_name,omitempty"`
				Level       int    `json:"level,omitempty"`
				Bio         string `json:"bio,omitempty"`
				Location    string `json:"location,omitempty"`
				LinkLinkedin string `json:"link_linkedin,omitempty"`
			}
			results := make([]out, 0, len(rows))
			for _, r := range rows {
				lvl := 0
				if sp := r.User.Metadata.SPData; sp != "" {
					var sd struct {
						LV int `json:"lv"`
					}
					_ = json.Unmarshal([]byte(sp), &sd)
					lvl = sd.LV
				}
				full := strings.TrimSpace(r.User.FirstName + " " + r.User.LastName)
				results = append(results, out{
					Rank:        r.Rank,
					Points:      r.Points,
					UserID:      r.UserID,
					Name:        r.User.Name,
					FullName:    full,
					Level:       lvl,
					Bio:         r.User.Metadata.Bio,
					Location:    r.User.Metadata.Location,
					LinkLinkedin: r.User.Metadata.LinkLinkedin,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to template_vars.community)")
	cmd.Flags().StringVar(&flagType, "type", "30d", "Leaderboard window: 7d | 30d | all-time")
	cmd.Flags().IntVar(&flagTop, "top", 25, "Cap results to top N (0 = all)")
	return cmd
}

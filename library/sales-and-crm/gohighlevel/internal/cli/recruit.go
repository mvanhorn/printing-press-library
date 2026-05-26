// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `recruit hot` — composite scorecard combining production signals,
// engagement recency, and recruit-tag count.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newRecruitCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recruit",
		Short: "Recruiting reports and scorecards",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRecruitHotCmd(flags))
	return cmd
}

func newRecruitHotCmd(flags *rootFlags) *cobra.Command {
	var threshold int
	var tsv bool
	cmd := &cobra.Command{
		Use:         "hot",
		Short:       "Rank recruits by composite production + engagement + recruit-tag score",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openGHLStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := s.DB().QueryContext(ctx, `
                SELECT id, data FROM resources
                WHERE resource_type IN ('contacts', 'contacts_contacts')
            `)
			if err != nil {
				return apiErr(fmt.Errorf("query contacts: %w", err))
			}
			defer rows.Close()

			type out struct {
				Rank         int      `json:"rank"`
				ContactID    string   `json:"contact_id"`
				Email        string   `json:"email"`
				Name         string   `json:"name"`
				Score        int      `json:"score"`
				TopTags      []string `json:"top_tags"`
				LastActivity string   `json:"last_activity"`
			}

			var ranked []out
			for rows.Next() {
				var id sql.NullString
				var raw []byte
				if err := rows.Scan(&id, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}

				// Recruit-tag count.
				recruitTagCount := 0
				var topTags []string
				if tags, ok := obj["tags"].([]any); ok {
					for _, t := range tags {
						ts, _ := t.(string)
						if ts == "" {
							continue
						}
						l := strings.ToLower(ts)
						if strings.Contains(l, "recruit") || strings.Contains(l, "prospect") {
							recruitTagCount++
						}
						if len(topTags) < 5 {
							topTags = append(topTags, ts)
						}
					}
				}

				// Production custom-field signal.
				prodBonus := 0
				if cf, ok := obj["customFields"].([]any); ok {
					for _, item := range cf {
						m, _ := item.(map[string]any)
						if m == nil {
							continue
						}
						k, _ := m["key"].(string)
						if k == "" {
							k, _ = m["fieldKey"].(string)
						}
						l := strings.ToLower(k)
						if strings.Contains(l, "production") || strings.Contains(l, "deals") || strings.Contains(l, "volume") {
							prodBonus = 5
							break
						}
					}
				}

				email, _ := obj["email"].(string)
				cid := nullStr(id)

				// Engagement: messages count in last 14 days for this contact.
				engagement := 0
				var lastActivity sql.NullString
				_ = s.DB().QueryRowContext(ctx, `
                    SELECT COUNT(*), MAX(COALESCE(json_extract(data, '$.dateAdded'),
                                                  json_extract(data, '$.dateUpdated')))
                    FROM messages
                    WHERE json_extract(data, '$.contactId') = ?
                      AND COALESCE(json_extract(data, '$.dateAdded'),
                                   json_extract(data, '$.dateUpdated')) > datetime('now', '-14 days')
                `, cid).Scan(&engagement, &lastActivity)

				score := recruitTagCount*5 + engagement + prodBonus

				if score < threshold {
					continue
				}
				fn, _ := obj["firstName"].(string)
				ln, _ := obj["lastName"].(string)

				la := nullStr(lastActivity)
				if la == "" {
					if v, _ := obj["dateUpdated"].(string); v != "" {
						la = v
					}
				}
				ranked = append(ranked, out{
					ContactID:    cid,
					Email:        email,
					Name:         strings.TrimSpace(fn + " " + ln),
					Score:        score,
					TopTags:      topTags,
					LastActivity: la,
				})
			}

			sort.Slice(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })
			for i := range ranked {
				ranked[i].Rank = i + 1
			}

			if tsv && !flags.asJSON {
				w := cmd.OutOrStdout()
				fmt.Fprintln(w, "rank\tcontact_id\temail\tname\tscore\ttop_tags\tlast_activity")
				for _, r := range ranked {
					fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\t%s\n",
						r.Rank, r.ContactID, r.Email, r.Name, r.Score,
						strings.Join(r.TopTags, ","), r.LastActivity)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), ranked, flags)
		},
	}
	cmd.Flags().IntVar(&threshold, "threshold", 25, "Minimum composite score")
	cmd.Flags().BoolVar(&tsv, "tsv", false, "Emit TSV instead of JSON")
	return cmd
}

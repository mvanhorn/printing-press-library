// Copyright 2026 charles-garrison. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #5 — keyword gap (set difference across domains).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newKeywordGapCmd(flags *rootFlags) *cobra.Command {
	var database string
	var kdMax float64
	var mode string
	var limit int

	cmd := &cobra.Command{
		Use:   "gap [me] [them] [them2...]",
		Short: "Find organic keywords competitors rank for that you don't — or the intersection / unique-to-each.",
		Long: `gap reads organic keyword positions from the local store for two or
more domains in the same database, and emits the set difference,
intersection, or per-domain unique sets.

Run 'semrush-pp-cli sync --resource keyword' to populate the store first.`,
		Example:     "  semrush-pp-cli keyword gap mysite.com competitor.com --kd-max 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			recordBalanceSnapshotForCmd(ctx, db, flags, cmd.CommandPath(), cmd.ErrOrStderr())

			if !hintIfUnsynced(cmd, db, "keyword") {
				hintIfStale(cmd, db, "keyword", flags.maxAge)
			}

			type kwRow struct {
				Phrase   string  `json:"phrase"`
				Position float64 `json:"position"`
				Traffic  float64 `json:"traffic"`
				NQ       float64 `json:"nq"`
				KD       float64 `json:"kd"`
				URL      string  `json:"url"`
			}
			loadKeywords := func(domain string) (map[string]kwRow, error) {
				out := map[string]kwRow{}
				rows, err := db.DB().QueryContext(ctx,
					`SELECT json_extract(data, '$.Ph') AS phrase,
					        COALESCE(json_extract(data, '$.Po'), 0) AS position,
					        COALESCE(json_extract(data, '$.Tr'), 0) AS traffic,
					        COALESCE(json_extract(data, '$.Nq'), 0) AS nq,
					        COALESCE(json_extract(data, '$.Kd'), 0) AS kd,
					        COALESCE(json_extract(data, '$.Ur'), '') AS url
					 FROM resources
					 WHERE resource_type IN ('keyword', 'domain_keywords')
					   AND (json_extract(data, '$.domain') = ? OR json_extract(data, '$.Dn') = ?)
					   AND (? = '' OR json_extract(data, '$.database') = ? OR json_extract(data, '$.database') IS NULL)`,
					domain, domain, database, database)
				if err != nil {
					return nil, err
				}
				defer rows.Close()
				for rows.Next() {
					var r kwRow
					var phrase *string
					if err := rows.Scan(&phrase, &r.Position, &r.Traffic, &r.NQ, &r.KD, &r.URL); err != nil {
						return nil, err
					}
					if phrase == nil || strings.TrimSpace(*phrase) == "" {
						continue
					}
					if kdMax > 0 && r.KD > kdMax {
						continue
					}
					r.Phrase = *phrase
					if existing, ok := out[r.Phrase]; !ok || r.Position < existing.Position {
						out[r.Phrase] = r
					}
				}
				return out, rows.Err()
			}

			me := args[0]
			others := args[1:]
			myKws, err := loadKeywords(me)
			if err != nil {
				return fmt.Errorf("loading %s: %w", me, err)
			}
			theirSets := make(map[string]map[string]kwRow, len(others))
			for _, t := range others {
				kws, err := loadKeywords(t)
				if err != nil {
					return fmt.Errorf("loading %s: %w", t, err)
				}
				theirSets[t] = kws
			}

			type gapHit struct {
				Phrase             string             `json:"phrase"`
				DomainRankingForIt []string           `json:"domain_ranking_for_it"`
				TheirPositions     map[string]float64 `json:"their_positions"`
				TheirTraffic       map[string]float64 `json:"their_traffic"`
				NQ                 float64            `json:"nq"`
				KD                 float64            `json:"kd"`
			}

			var hits []gapHit
			switch mode {
			case "common":
				// Intersection: phrases me ranks for AND every competitor ranks for
				for phrase, my := range myKws {
					allHave := true
					theirPositions := map[string]float64{}
					theirTraffic := map[string]float64{}
					domains := []string{me}
					for _, t := range others {
						r, ok := theirSets[t][phrase]
						if !ok {
							allHave = false
							break
						}
						theirPositions[t] = r.Position
						theirTraffic[t] = r.Traffic
						domains = append(domains, t)
					}
					if !allHave {
						continue
					}
					theirPositions[me] = my.Position
					theirTraffic[me] = my.Traffic
					hits = append(hits, gapHit{
						Phrase:             phrase,
						DomainRankingForIt: domains,
						TheirPositions:     theirPositions,
						TheirTraffic:       theirTraffic,
						NQ:                 my.NQ,
						KD:                 my.KD,
					})
				}
			case "unique":
				// Phrases me ranks for but NONE of the competitors do
				for phrase, my := range myKws {
					anyTheirs := false
					for _, t := range others {
						if _, ok := theirSets[t][phrase]; ok {
							anyTheirs = true
							break
						}
					}
					if anyTheirs {
						continue
					}
					hits = append(hits, gapHit{
						Phrase:             phrase,
						DomainRankingForIt: []string{me},
						TheirPositions:     map[string]float64{me: my.Position},
						TheirTraffic:       map[string]float64{me: my.Traffic},
						NQ:                 my.NQ,
						KD:                 my.KD,
					})
				}
			default: // "gap"
				// Phrases competitors rank for but me does NOT
				seen := map[string]bool{}
				for _, t := range others {
					for phrase, theirs := range theirSets[t] {
						if _, ok := myKws[phrase]; ok {
							continue
						}
						if seen[phrase] {
							continue
						}
						seen[phrase] = true
						theirPositions := map[string]float64{}
						theirTraffic := map[string]float64{}
						var domains []string
						for _, tt := range others {
							if r, ok := theirSets[tt][phrase]; ok {
								theirPositions[tt] = r.Position
								theirTraffic[tt] = r.Traffic
								domains = append(domains, tt)
							}
						}
						hits = append(hits, gapHit{
							Phrase:             phrase,
							DomainRankingForIt: domains,
							TheirPositions:     theirPositions,
							TheirTraffic:       theirTraffic,
							NQ:                 theirs.NQ,
							KD:                 theirs.KD,
						})
					}
				}
			}

			// Deterministic order before truncating with --limit. All three
			// modes (gap/common/unique) build `hits` by ranging over Go maps,
			// which iterate non-deterministically — without this sort, two
			// `keyword gap --limit 50` invocations would return different
			// phrases, making the flag behave like a random sample. Sort by
			// search volume desc (most-impactful first), tiebreak by KD asc
			// (easier first), final tiebreak by phrase asc.
			sort.SliceStable(hits, func(i, j int) bool {
				if hits[i].NQ != hits[j].NQ {
					return hits[i].NQ > hits[j].NQ
				}
				if hits[i].KD != hits[j].KD {
					return hits[i].KD < hits[j].KD
				}
				return hits[i].Phrase < hits[j].Phrase
			})

			totalHitCount := len(hits)
			truncated := false
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
				truncated = true
			}
			out := map[string]any{
				"mode":            mode,
				"me":              me,
				"competitors":     others,
				"database":        database,
				"kd_max":          kdMax,
				"hit_count":       totalHitCount,
				"hit_count_shown": len(hits),
				"truncated":       truncated,
				"hits":            hits,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&database, "database", "us", "Semrush database/country code; empty matches all")
	cmd.Flags().Float64Var(&kdMax, "kd-max", 0, "Filter out keywords with KD greater than this (0 disables)")
	cmd.Flags().StringVar(&mode, "mode", "gap", "gap (competitors rank, you don't) | common (everyone ranks) | unique (only you rank)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum hits to return (0 disables)")
	return cmd
}

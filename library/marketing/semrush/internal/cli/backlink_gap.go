// Copyright 2026 charles-garrison. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #6 — backlink gap (referring domains they have, we don't).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newBacklinkGapCmd(flags *rootFlags) *cobra.Command {
	var minAscore int
	var limit int

	cmd := &cobra.Command{
		Use:         "gap [me] [them]",
		Short:       "List referring domains that link to a competitor but not to you, filtered by authority score.",
		Long:        "gap reads referring-domain rows from the local store and emits the left-anti-join: domains linking to <them> but not to <me>, filtered by --min-ascore. Run 'semrush-pp-cli sync --resource backlink_referring_domains' first.",
		Example:     "  semrush-pp-cli backlink gap mysite.com competitor.com --min-ascore 70",
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

			if !hintIfUnsynced(cmd, db, "backlink") {
				hintIfStale(cmd, db, "backlink", flags.maxAge)
			}

			type refRow struct {
				Domain    string  `json:"domain"`
				Ascore    float64 `json:"ascore"`
				Backlinks float64 `json:"backlinks"`
			}
			loadRefs := func(target string) (map[string]refRow, error) {
				out := map[string]refRow{}
				rows, err := db.DB().QueryContext(ctx,
					`SELECT COALESCE(json_extract(data, '$.domain'), json_extract(data, '$.Dn'), '') AS domain,
					        COALESCE(json_extract(data, '$.domain_ascore'), json_extract(data, '$.As'), 0) AS ascore,
					        COALESCE(json_extract(data, '$.backlinks_num'), json_extract(data, '$.Bn'), 0) AS backlinks
					 FROM resources
					 WHERE resource_type IN ('backlink', 'backlink_referring_domains', 'referring_domains', 'referring_domain')
					   AND (json_extract(data, '$.target') = ? OR json_extract(data, '$.Tg') = ?)`,
					target, target)
				if err != nil {
					return nil, err
				}
				defer rows.Close()
				for rows.Next() {
					var r refRow
					if err := rows.Scan(&r.Domain, &r.Ascore, &r.Backlinks); err != nil {
						return nil, err
					}
					if strings.TrimSpace(r.Domain) == "" {
						continue
					}
					out[strings.ToLower(r.Domain)] = r
				}
				return out, rows.Err()
			}

			me := args[0]
			them := args[1]

			myRefs, err := loadRefs(me)
			if err != nil {
				return fmt.Errorf("loading %s refs: %w", me, err)
			}
			theirRefs, err := loadRefs(them)
			if err != nil {
				return fmt.Errorf("loading %s refs: %w", them, err)
			}

			// If EITHER side has no target-scoped referring-domain data, we
			// cannot produce a meaningful gap. The naive loop below would
			// either (a) emit every theirRefs entry as a "gap" when myRefs
			// is empty (false positives — myRefs's "not present" filter
			// trivially passes), or (b) return zero hits when theirRefs is
			// empty (misleading — looks like no opportunities exist when
			// the user simply hasn't synced the competitor). `hintIfUnsynced`
			// upstream checks sync_state by resource type only, not by
			// domain, so a user who synced backlinks for ANY target won't
			// see the generic stale hint when they query for one they
			// HAVEN'T synced. Surface the specific missing side here.
			if len(myRefs) == 0 || len(theirRefs) == 0 {
				var missing []string
				if len(myRefs) == 0 {
					missing = append(missing, me)
				}
				if len(theirRefs) == 0 {
					missing = append(missing, them)
				}
				out := map[string]any{
					"me":              me,
					"them":            them,
					"min_ascore":      minAscore,
					"hit_count":       0,
					"hit_count_shown": 0,
					"truncated":       false,
					"hits":            []any{},
					"hint":            fmt.Sprintf("no target-scoped referring-domain data in local store for: %s. Run 'semrush-pp-cli sync --resources backlink' against %s before trusting this gap result.", strings.Join(missing, ", "), strings.Join(missing, " and ")),
				}
				raw, err := json.Marshal(out)
				if err != nil {
					return fmt.Errorf("encoding empty gap response: %w", err)
				}
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			type hit struct {
				Domain    string  `json:"domain"`
				Ascore    float64 `json:"ascore"`
				Backlinks float64 `json:"backlinks"`
			}
			var hits []hit
			for d, r := range theirRefs {
				if _, ok := myRefs[d]; ok {
					continue
				}
				if minAscore > 0 && r.Ascore < float64(minAscore) {
					continue
				}
				hits = append(hits, hit{Domain: r.Domain, Ascore: r.Ascore, Backlinks: r.Backlinks})
			}
			// Deterministic top-N. The build loop iterates a Go map
			// (theirRefs), so without this sort --limit would pick a random
			// subset of authority-filtered domains. Sort by Ascore desc
			// (authority first — most useful for outreach), tiebreak by
			// Backlinks desc, final tiebreak by Domain for stability.
			sort.SliceStable(hits, func(i, j int) bool {
				if hits[i].Ascore != hits[j].Ascore {
					return hits[i].Ascore > hits[j].Ascore
				}
				if hits[i].Backlinks != hits[j].Backlinks {
					return hits[i].Backlinks > hits[j].Backlinks
				}
				return hits[i].Domain < hits[j].Domain
			})
			totalHitCount := len(hits)
			truncated := false
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
				truncated = true
			}

			out := map[string]any{
				"me":              me,
				"them":            them,
				"min_ascore":      minAscore,
				"hit_count":       totalHitCount,
				"hit_count_shown": len(hits),
				"truncated":       truncated,
				"hits":       hits,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&minAscore, "min-ascore", 70, "Filter out referring domains with authority score below this")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum hits to return (0 disables)")
	return cmd
}

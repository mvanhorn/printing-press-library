// Copyright 2026 charles-garrison. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #6 — backlink gap (referring domains they have, we don't).

package cli

import (
	"encoding/json"
	"fmt"
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

			// If the per-target query returned nothing for either side, fall
			// back to the unscoped local set (typical when sync wrote rows
			// without an explicit target field). The hint helper has already
			// warned the user about local-store freshness, so this is an
			// "any referring domain we know about" answer.
			if len(myRefs) == 0 && len(theirRefs) == 0 {
				rows, err := db.DB().QueryContext(ctx,
					`SELECT COALESCE(json_extract(data, '$.domain'), json_extract(data, '$.Dn'), '') AS domain,
					        COALESCE(json_extract(data, '$.domain_ascore'), json_extract(data, '$.As'), 0) AS ascore,
					        COALESCE(json_extract(data, '$.backlinks_num'), json_extract(data, '$.Bn'), 0) AS backlinks
					 FROM resources
					 WHERE resource_type IN ('backlink', 'backlink_referring_domains', 'referring_domains', 'referring_domain')`)
				if err != nil {
					return fmt.Errorf("scan referring domains: %w", err)
				}
				defer rows.Close()
				for rows.Next() {
					var r refRow
					if err := rows.Scan(&r.Domain, &r.Ascore, &r.Backlinks); err != nil {
						return fmt.Errorf("scan referring row: %w", err)
					}
					if strings.TrimSpace(r.Domain) == "" {
						continue
					}
					theirRefs[strings.ToLower(r.Domain)] = r
				}
				if err := rows.Err(); err != nil {
					return fmt.Errorf("iterate referring rows: %w", err)
				}
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
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}

			out := map[string]any{
				"me":         me,
				"them":       them,
				"min_ascore": minAscore,
				"hit_count":  len(hits),
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

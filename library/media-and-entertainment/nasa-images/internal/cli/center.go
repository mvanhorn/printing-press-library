// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// nasaCenters is the canonical map of NASA center codes -> display names.
var nasaCenters = map[string]string{
	"HQ":   "NASA Headquarters",
	"JSC":  "Johnson Space Center",
	"KSC":  "Kennedy Space Center",
	"GSFC": "Goddard Space Flight Center",
	"JPL":  "Jet Propulsion Laboratory",
	"MSFC": "Marshall Space Flight Center",
	"ARC":  "Ames Research Center",
	"LRC":  "Langley Research Center",
	"AFRC": "Armstrong Flight Research Center",
	"GRC":  "Glenn Research Center",
	"SSC":  "Stennis Space Center",
}

func newCenterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "center",
		Short: "Inspect a specific NASA center's mirrored catalog (per-center counts, year histogram, top keywords)",
		Long: `Local aggregations over the synced cache, scoped to a specific NASA center.
Each center has a distinct content profile (JPL=Mars, JSC=human spaceflight,
KSC=launches, GSFC=Earth+Webb), but the upstream API doesn't expose per-center
breakdowns — only the local mirror can.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCenterProfileCmd(flags))
	return cmd
}

func newCenterProfileCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "profile [center_code]",
		Short: "Aggregate counts, year-bucket histogram, top keywords, and top photographers for a NASA center",
		Long: `Aggregate the local mirror for one NASA center. Returns counts by
media_type, a per-year histogram of date_created, top keywords by occurrence,
and top photographers — all the questions a journalist would ask before
running a search ("which center publishes this? when did they go quiet?").

Center codes (case-insensitive): HQ JSC KSC GSFC JPL MSFC ARC LRC AFRC GRC SSC.`,
		Example:     "  nasa-images-pp-cli center profile JPL",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			code := strings.ToUpper(args[0])
			if _, ok := nasaCenters[code]; !ok {
				return fmt.Errorf("unknown NASA center code %q; expected one of HQ JSC KSC GSFC JPL MSFC ARC LRC AFRC GRC SSC", code)
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would profile center %s\n", code)
				return nil
			}
			ctx := cmd.Context()
			s, err := openNasaStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			profile := map[string]any{
				"code":         code,
				"display_name": nasaCenters[code],
			}

			// Total + per-media_type counts.
			counts := map[string]int{}
			byType, terr := s.DB().QueryContext(ctx,
				`SELECT COALESCE(json_extract(data, '$.media_type'), 'unknown') AS mt, COUNT(*)
				 FROM resources
				 WHERE resource_type = 'asset' AND json_extract(data, '$.center') = ?
				 GROUP BY mt`, code)
			if terr != nil {
				return fmt.Errorf("counting by media_type: %w", terr)
			}
			total := 0
			for byType.Next() {
				var mt sql.NullString
				var n int
				if err := byType.Scan(&mt, &n); err == nil {
					counts[mt.String] = n
					total += n
				}
			}
			byType.Close()
			profile["total"] = total
			profile["by_media_type"] = counts

			if total == 0 {
				profile["note"] = "no assets for this center in the local mirror; run 'mirror search --center " + code + "' to populate"
				return flags.printJSON(cmd, profile)
			}

			// Year histogram.
			years := map[string]int{}
			byYear, yerr := s.DB().QueryContext(ctx,
				`SELECT substr(COALESCE(json_extract(data, '$.date_created'), ''), 1, 4) AS yr, COUNT(*)
				 FROM resources
				 WHERE resource_type = 'asset' AND json_extract(data, '$.center') = ?
				 GROUP BY yr ORDER BY yr`, code)
			if yerr == nil {
				for byYear.Next() {
					var yr sql.NullString
					var n int
					if err := byYear.Scan(&yr, &n); err == nil {
						if yr.Valid && yr.String != "" {
							years[yr.String] = n
						}
					}
				}
				byYear.Close()
			}
			profile["by_year"] = years

			// Top keywords.
			keywords := map[string]int{}
			kwRows, kerr := s.DB().QueryContext(ctx,
				`SELECT kw.value FROM resources r, json_each(json_extract(r.data, '$.keywords')) kw
				 WHERE r.resource_type = 'asset' AND json_extract(r.data, '$.center') = ?`, code)
			if kerr == nil {
				for kwRows.Next() {
					var v sql.NullString
					if err := kwRows.Scan(&v); err == nil && v.Valid {
						keywords[v.String]++
					}
				}
				kwRows.Close()
			}
			profile["top_keywords"] = topN(keywords, 10)

			// Top photographers.
			photographers := map[string]int{}
			pRows, perr := s.DB().QueryContext(ctx,
				`SELECT COALESCE(json_extract(data, '$.photographer'), '') AS ph
				 FROM resources
				 WHERE resource_type = 'asset' AND json_extract(data, '$.center') = ?`, code)
			if perr == nil {
				for pRows.Next() {
					var ph sql.NullString
					if err := pRows.Scan(&ph); err == nil && ph.Valid && ph.String != "" {
						photographers[ph.String]++
					}
				}
				pRows.Close()
			}
			profile["top_photographers"] = topN(photographers, 5)

			return flags.printJSON(cmd, profile)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nasa-images-pp-cli/data.db)")
	return cmd
}

type kvCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

func topN(m map[string]int, n int) []kvCount {
	out := make([]kvCount, 0, len(m))
	for k, v := range m {
		out = append(out, kvCount{Key: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

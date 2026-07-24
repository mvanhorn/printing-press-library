// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: server-side aggregates via outStatistics, no full pull.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// pp:client-call — hits the ArcGIS REST API via c.LayerMeta and c.QueryRaw (arcgis client), which the reimplementation regex does not recognize.
// pp:data-source live
func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var (
		groupBy string
		out     string
		where   string
		orderBy string
	)
	cmd := &cobra.Command{
		Use:   "stats <layer-url>",
		Short: "Compute server-side aggregates (count/sum/avg/min/max) without downloading rows",
		Long: `Run server-side statistics on a layer via outStatistics, optionally grouped by a
field. No feature rows are downloaded, so this is cheap even on huge layers.

--out is a comma-separated list of statistics. Each is one of:
  count            row count (uses the object-id field)
  count:<field>    non-null count of <field>
  sum:<field>      sum of <field>
  avg:<field>      average of <field>
  min:<field>      minimum of <field>
  max:<field>      maximum of <field>`,
		Example: strings.Trim(`
  arcgis-pp-cli stats <layer-url> --group-by LANDUSE --out count --agent
  arcgis-pp-cli stats <layer-url> --out "count,sum:ACRES,avg:TOTVAL"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute server-side statistics for the given layer")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a layer URL is required"))
			}
			if out == "" {
				out = "count"
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			// Need the object-id field for a bare count.
			meta, err := c.LayerMeta(ctx, args[0])
			if err != nil {
				return fmt.Errorf("reading layer metadata: %w", err)
			}
			oidField := meta.ObjectIDField
			if oidField == "" {
				oidField = "OBJECTID"
			}
			outStats, err := buildOutStatistics(out, oidField)
			if err != nil {
				return usageErr(err)
			}
			stJSON, _ := json.Marshal(outStats)

			p := url.Values{}
			p.Set("where", orDefault(where, "1=1"))
			p.Set("outStatistics", string(stJSON))
			p.Set("returnGeometry", "false")
			if groupBy != "" {
				p.Set("groupByFieldsForStatistics", groupBy)
			}
			if orderBy != "" {
				p.Set("orderByFields", orderBy)
			}
			res, err := c.QueryRaw(ctx, args[0], p)
			if err != nil {
				return fmt.Errorf("computing statistics: %w", err)
			}
			rows := make([]map[string]any, 0, len(res.Features))
			for _, f := range res.Features {
				rows = append(rows, f.Attributes)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"features": rows}, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no results")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&groupBy, "group-by", "", "field to group statistics by")
	cmd.Flags().StringVar(&out, "out", "count", "comma-separated stats: count, count:<f>, sum:<f>, avg:<f>, min:<f>, max:<f>")
	cmd.Flags().StringVar(&where, "where", "1=1", "SQL where clause to match features")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "orderByFields for grouped output")
	return cmd
}

type outStat struct {
	StatisticType         string `json:"statisticType"`
	OnStatisticField      string `json:"onStatisticField"`
	OutStatisticFieldName string `json:"outStatisticFieldName"`
}

func buildOutStatistics(spec, oidField string) ([]outStat, error) {
	var stats []outStat
	for _, raw := range strings.Split(spec, ",") {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		var kind, field string
		if i := strings.Index(s, ":"); i >= 0 {
			kind = strings.ToLower(strings.TrimSpace(s[:i]))
			field = strings.TrimSpace(s[i+1:])
		} else {
			kind = strings.ToLower(s)
		}
		var statType, onField, outName string
		switch kind {
		case "count":
			statType = "count"
			if field != "" {
				onField = field
				outName = "count_" + field
			} else {
				onField = oidField
				outName = "count"
			}
		case "sum", "avg", "min", "max":
			if field == "" {
				return nil, fmt.Errorf("%s requires a field, e.g. %s:ACRES", kind, kind)
			}
			statType = kind
			onField = field
			outName = kind + "_" + field
		default:
			return nil, fmt.Errorf("unknown statistic %q (use count, sum, avg, min, max)", kind)
		}
		stats = append(stats, outStat{StatisticType: statType, OnStatisticField: onField, OutStatisticFieldName: outName})
	}
	if len(stats) == 0 {
		return nil, fmt.Errorf("no statistics requested")
	}
	return stats, nil
}

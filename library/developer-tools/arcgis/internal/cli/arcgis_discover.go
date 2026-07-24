// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/arcgis"

	"github.com/spf13/cobra"
)

func newDiscoverCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover <service-url>",
		Short: "List the layers and tables published by an ArcGIS service",
		Long: `List the layers and tables published by an ArcGIS FeatureServer or MapServer.
Pass the service root URL (ending in /FeatureServer or /MapServer). Each layer's
id, name, and type is returned so you can pick a layer URL for query/fields.`,
		Example:     "  arcgis-pp-cli discover https://services.arcgis.com/P3ePLMYs2RVChkkx/arcgis/rest/services/USA_Counties/FeatureServer",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list services/layers at the given URL")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a service URL is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			info, err := c.ServiceMeta(ctx, args[0])
			if err != nil {
				return fmt.Errorf("reading service metadata: %w", err)
			}
			base := arcgis.NormalizeLayerURL(args[0])
			type row struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
				Kind string `json:"kind"`
				URL  string `json:"url"`
			}
			out := make([]row, 0, len(info.Layers)+len(info.Tables))
			for _, l := range info.Layers {
				out = append(out, row{l.ID, l.Name, l.Type, "layer", fmt.Sprintf("%s/%d", base, l.ID)})
			}
			for _, t := range info.Tables {
				out = append(out, row{t.ID, t.Name, t.Type, "table", fmt.Sprintf("%s/%d", base, t.ID)})
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no layers or tables found")
				return nil
			}
			rows := make([][]string, 0, len(out))
			for _, r := range out {
				rows = append(rows, []string{fmt.Sprintf("%d", r.ID), r.Kind, r.Name, r.URL})
			}
			return flags.printTable(cmd, []string{"ID", "KIND", "NAME", "URL"}, rows)
		},
	}
	return cmd
}

func newFieldsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fields <layer-url>",
		Short: "Show a layer's fields, types, geometry, and maxRecordCount",
		Long: `Introspect a single layer: its field names and types, geometry type,
object-id field, maxRecordCount, and whether it supports resultOffset pagination.
Pass a layer URL ending in a numeric layer id (e.g. .../FeatureServer/0).`,
		Example:     "  arcgis-pp-cli fields https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read layer schema at the given URL")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a layer URL is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			m, err := c.LayerMeta(ctx, args[0])
			if err != nil {
				return fmt.Errorf("reading layer metadata: %w", err)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				view := map[string]any{
					"name":                m.Name,
					"type":                m.Type,
					"geometry_type":       m.GeometryType,
					"object_id_field":     m.ObjectIDField,
					"max_record_count":    m.MaxRecordCount,
					"supports_pagination": m.Advanced.SupportsPagination,
					"fields":              m.Fields,
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", m.Name, m.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "geometry: %s   objectId: %s   maxRecordCount: %d   pagination: %v\n\n",
				m.GeometryType, m.ObjectIDField, m.MaxRecordCount, m.Advanced.SupportsPagination)
			rows := make([][]string, 0, len(m.Fields))
			for _, f := range m.Fields {
				rows = append(rows, []string{f.Name, f.Type, f.Alias})
			}
			return flags.printTable(cmd, []string{"FIELD", "TYPE", "ALIAS"}, rows)
		},
	}
	return cmd
}

func newCountCmd(flags *rootFlags) *cobra.Command {
	var where string
	cmd := &cobra.Command{
		Use:         "count <layer-url>",
		Short:       "Count features matching a where clause without pulling rows",
		Long:        "Return the number of features matching --where (default 1=1) via returnCountOnly. Cheap; no rows are downloaded.",
		Example:     "  arcgis-pp-cli count <layer-url> --where \"LANDUSE='VACANT'\"",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "url=https://services.arcgis.com/2gdL2gxYNFY2TOUb/arcgis/rest/services/FEMA_National_Flood_Hazard_Layer/FeatureServer/0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would count features at the given URL")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a layer URL is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newArcClient(flags)
			if err != nil {
				return err
			}
			n, err := c.Count(ctx, args[0], where)
			if err != nil {
				return fmt.Errorf("counting features: %w", err)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				b, _ := json.Marshal(map[string]any{"count": n, "where": orDefault(where, "1=1")})
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d\n", n)
			return nil
		},
	}
	cmd.Flags().StringVar(&where, "where", "1=1", "SQL where clause to match features")
	return cmd
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

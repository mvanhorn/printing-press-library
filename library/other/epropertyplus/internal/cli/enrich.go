// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// Hand-built novel command: `enrich --parcels` — emit parcelNumber + address +
// instance for each property, plus a top-level join_hint explaining how to join
// parcelNumber to county / Socrata condition & assessment data. This is an
// HONEST HOOK: it performs no external fetch.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

const parcelJoinHint = "Join `parcel` (parcelNumber) to county assessor / Socrata Open Data " +
	"records to enrich each parcel with building condition, current assessment, " +
	"and tax-delinquency signals. For Kansas City (kclb), the Jackson County " +
	"parcel id is the JA-prefixed APN; query the county assessor or a Socrata " +
	"dataset (e.g. data.kcmo.org) filtered by parcel id (often the `parcel_id` " +
	"or `apn` column) and left-join on parcelNumber. No external call is made " +
	"by this command — it emits the parcel keys and this join recipe only."

func newEnrichCmd(flags *rootFlags) *cobra.Command {
	var parcels bool
	var limit int

	cmd := &cobra.Command{
		Use:   "enrich --parcels",
		Short: "Emit parcel keys plus a join recipe for county/Socrata condition & assessment data",
		Long: strings.Trim(`
Emit, for each property in the active instance, {parcelNumber, address, city,
instance} plus a top-level "join_hint" describing how to join parcelNumber to
county / Socrata Open Data condition and market-value datasets.

This is an HONEST HOOK: it performs no external network fetch. It hands you the
parcel keys and the join recipe so you (or a downstream pipeline) can do the
enrichment. Prefers the synced local store; falls back to hydrating live up to
--limit.
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  epropertyplus-pp-cli enrich --parcels --json --limit 5
  epropertyplus-pp-cli --instance kclb enrich --parcels --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !parcels {
				// --parcels is the only mode today; show help when omitted so the
				// command is verify-friendly (no required-flag gate before RunE).
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			loaded, _, err := loadProperties(cmd.Context(), c, flags, limit, "all")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			slug := flags.activeSlug()
			items := make([]map[string]any, 0, len(loaded))
			for _, lp := range loaded {
				items = append(items, map[string]any{
					"parcelNumber": lp.Prop.ParcelNumber,
					"address":      lp.Prop.PropertyAddress1,
					"city":         lp.Prop.City,
					"instance":     slug,
				})
			}
			env := map[string]any{
				"join_hint": parcelJoinHint,
				"instance":  slug,
				"count":     len(items),
				"parcels":   items,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().BoolVar(&parcels, "parcels", false, "Emit parcel keys and the county/Socrata join recipe")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max properties to hydrate live (0 = default cap; ignored when reading the synced store)")
	return cmd
}

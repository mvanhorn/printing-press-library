// `catalog refresh` snapshots the live Categories and Areas reference data into
// the local store so subsequent reads can run offline. Subareas are flattened
// into cl_areas with parent_area_id wired so geo + watch commands can resolve
// them without an extra fetch.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"

	"github.com/spf13/cobra"
)

func newCatalogCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "catalog",
		Short:       "Manage the local catalog of Craigslist categories and sites",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newCatalogRefreshCmd(flags))
	return cmd
}

func newCatalogRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "refresh",
		Short:       "Refresh the local copy of the Craigslist catalog (categories + sites)",
		Long:        "Fetch categories and areas live from reference.craigslist.org and persist them to the local SQLite store. Idempotent — safe to run periodically.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			c := craigslist.New(1.0)
			cats, err := c.GetCategories(ctx)
			if err != nil {
				return fmt.Errorf("fetch categories: %w", err)
			}
			for _, k := range cats {
				if err := db.SaveCategory(ctx, k.CategoryID, k.Abbreviation, k.Description, k.Type); err != nil {
					return fmt.Errorf("save category %s: %w", k.Abbreviation, err)
				}
			}

			areas, err := c.GetAreas(ctx)
			if err != nil {
				return fmt.Errorf("fetch areas: %w", err)
			}
			subAreas := 0
			for _, a := range areas {
				if err := db.SaveArea(ctx, a.AreaID, a.Abbreviation, a.Hostname, a.Country, a.Region, a.Description, a.ShortDescription, a.Latitude, a.Longitude, a.Timezone, 0); err != nil {
					return fmt.Errorf("save area %s: %w", a.Hostname, err)
				}
				for _, sa := range a.SubAreas {
					subAreas++
					if err := db.SaveArea(ctx, sa.SubAreaID, sa.Abbreviation, a.Hostname+"/"+sa.Abbreviation, a.Country, a.Region, sa.Description, sa.ShortDescription, a.Latitude, a.Longitude, a.Timezone, a.AreaID); err != nil {
						return fmt.Errorf("save subarea %s: %w", sa.Abbreviation, err)
					}
				}
			}

			summary := map[string]any{
				"categories": len(cats),
				"areas":      len(areas),
				"subAreas":   subAreas,
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAutoTable(cmd.OutOrStdout(), []map[string]any{summary})
			}
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	return cmd
}

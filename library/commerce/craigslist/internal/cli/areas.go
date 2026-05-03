// `areas list` exposes the Craigslist area taxonomy (~707 hostnames + subareas).
// Live read against reference.craigslist.org by default, local read against
// cl_areas when --data-source local.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"

	"github.com/spf13/cobra"
)

// areaRow is the typed return value for `areas list`. Subareas are flattened
// out into individual rows with parent_area_id wiring so a downstream agent
// can group without a second call.
type areaRow struct {
	AreaID           int     `json:"areaId"`
	Hostname         string  `json:"hostname"`
	Country          string  `json:"country"`
	Region           string  `json:"region"`
	Description      string  `json:"description"`
	ShortDescription string  `json:"shortDescription,omitempty"`
	Latitude         float64 `json:"lat,omitempty"`
	Longitude        float64 `json:"lng,omitempty"`
	Timezone         string  `json:"timezone,omitempty"`
	ParentAreaID     int     `json:"parentAreaId,omitempty"`
	SubAreas         int     `json:"subAreas,omitempty"`
}

func newAreasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "areas",
		Short:       "Browse Craigslist sites and subareas",
		Long:        "Reference taxonomy for Craigslist sites (e.g. sfbay, nyc) and their subareas.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAreasListCmd(flags))
	return cmd
}

func newAreasListCmd(flags *rootFlags) *cobra.Command {
	var country, region, grep string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List Craigslist sites and subareas, with optional --country, --region, or --grep filters",
		Long:        "List Craigslist sites (and subareas) with optional country, region, or substring filtering.",
		Example:     "  craigslist-pp-cli areas list --country US\n  craigslist-pp-cli areas list --grep sfbay",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows, err := loadAreas(cmd.Context(), flags)
			if err != nil {
				return err
			}
			rows = filterAreas(rows, country, region, grep)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					items = append(items, map[string]any{
						"hostname":    r.Hostname,
						"country":     r.Country,
						"region":      r.Region,
						"description": r.Description,
						"subAreas":    r.SubAreas,
					})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&country, "country", "", "Filter by country code (US, CA, GB, AU, etc.)")
	cmd.Flags().StringVar(&region, "region", "", "Filter by region (case-insensitive substring)")
	cmd.Flags().StringVar(&grep, "grep", "", "Case-insensitive substring filter on hostname or description")
	return cmd
}

func loadAreas(ctx context.Context, flags *rootFlags) ([]areaRow, error) {
	if flags != nil && flags.dataSource == "local" {
		return loadAreasFromStore(ctx)
	}
	c := craigslist.New(1.0)
	areas, err := c.GetAreas(ctx)
	if err != nil {
		if flags != nil && flags.dataSource == "auto" {
			if rows, lerr := loadAreasFromStore(ctx); lerr == nil && len(rows) > 0 {
				return rows, nil
			}
		}
		return nil, fmt.Errorf("fetch areas: %w", err)
	}
	out := make([]areaRow, 0, len(areas))
	for _, a := range areas {
		out = append(out, areaRow{
			AreaID:           a.AreaID,
			Hostname:         a.Hostname,
			Country:          a.Country,
			Region:           a.Region,
			Description:      a.Description,
			ShortDescription: a.ShortDescription,
			Latitude:         a.Latitude,
			Longitude:        a.Longitude,
			Timezone:         a.Timezone,
			SubAreas:         len(a.SubAreas),
		})
	}
	return out, nil
}

func loadAreasFromStore(ctx context.Context) ([]areaRow, error) {
	db, err := openCLStore(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return queryAreasDB(ctx, db.DB())
}

func queryAreasDB(ctx context.Context, db *sql.DB) ([]areaRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT area_id, COALESCE(hostname,''), COALESCE(country,''), COALESCE(region,''),
		       COALESCE(description,''), COALESCE(short_description,''),
		       COALESCE(lat,0), COALESCE(lng,0), COALESCE(timezone,''), COALESCE(parent_area_id,0)
		FROM cl_areas
		ORDER BY country, region, hostname`)
	if err != nil {
		return nil, fmt.Errorf("query areas: %w", err)
	}
	defer rows.Close()
	var out []areaRow
	for rows.Next() {
		var r areaRow
		if err := rows.Scan(&r.AreaID, &r.Hostname, &r.Country, &r.Region, &r.Description, &r.ShortDescription, &r.Latitude, &r.Longitude, &r.Timezone, &r.ParentAreaID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// filterAreas applies the --country / --region / --grep filters.
func filterAreas(rows []areaRow, country, region, grep string) []areaRow {
	c := strings.ToUpper(strings.TrimSpace(country))
	r := strings.ToLower(strings.TrimSpace(region))
	g := strings.ToLower(strings.TrimSpace(grep))
	if c == "" && r == "" && g == "" {
		return rows
	}
	out := rows[:0]
	for _, row := range rows {
		if c != "" && !strings.EqualFold(row.Country, c) {
			continue
		}
		if r != "" && !strings.Contains(strings.ToLower(row.Region), r) {
			continue
		}
		if g != "" && !strings.Contains(strings.ToLower(row.Hostname), g) && !strings.Contains(strings.ToLower(row.Description), g) {
			continue
		}
		out = append(out, row)
	}
	return out
}

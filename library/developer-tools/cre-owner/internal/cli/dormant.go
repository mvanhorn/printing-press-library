package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

type dormantResult struct {
	ParcelID      string  `json:"parcel_id"`
	Address       string  `json:"address"`
	OwnerName     string  `json:"owner_name"`
	Market        string  `json:"market"`
	EntityName    string  `json:"entity_name"`
	EntityStatus  string  `json:"entity_status"`
	Jurisdiction  string  `json:"jurisdiction,omitempty"`
	FormationDate string  `json:"formation_date,omitempty"`
	YearsInactive int     `json:"years_inactive,omitempty"`
	AssessedValue float64 `json:"assessed_value,omitempty"`
}

func newDormantCmd(flags *rootFlags) *cobra.Command {
	var dbPath, market string
	var inactiveYears, limit int

	cmd := &cobra.Command{
		Use:   "dormant [--market market]",
		Short: "Properties held by dissolved or lapsed business entities",
		Example: strings.Trim(`
  cre-owner-pp-cli dormant --market lake-county-in --json
  cre-owner-pp-cli dormant --market lake-county-in --inactive-years 3
  cre-owner-pp-cli dormant --market lake-county-in --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if market == "" {
				return fmt.Errorf("--market is required")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cre-owner-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			sqlDB := db.DB()

			// Load entities with non-active status
			type entity struct {
				Name          string
				Status        string
				Jurisdiction  string
				FormationDate string
			}
			entityMap := map[string]entity{} // lowercase name -> entity

			entRows, err := sqlDB.QueryContext(cmd.Context(),
				`SELECT content FROM resources WHERE resource_type = ?`, "entities")
			if err != nil {
				return fmt.Errorf("querying entities: %w", err)
			}
			defer entRows.Close()

			for entRows.Next() {
				var content []byte
				if err := entRows.Scan(&content); err != nil {
					continue
				}
				var e struct {
					Name          string `json:"name"`
					Status        string `json:"status"`
					Jurisdiction  string `json:"jurisdiction"`
					FormationDate string `json:"formation_date"`
				}
				if err := json.Unmarshal(content, &e); err != nil {
					continue
				}
				status := strings.ToLower(e.Status)
				if status != "active" && status != "" {
					entityMap[strings.ToLower(e.Name)] = entity{
						Name:          e.Name,
						Status:        e.Status,
						Jurisdiction:  e.Jurisdiction,
						FormationDate: e.FormationDate,
					}
				}
			}

			// Load parcels for the market and cross-reference with entities
			parcelRows, err := sqlDB.QueryContext(cmd.Context(),
				`SELECT content FROM resources WHERE resource_type = ? AND json_extract(content, '$.market') = ?`,
				"parcels", market)
			if err != nil {
				return fmt.Errorf("querying parcels: %w", err)
			}
			defer parcelRows.Close()

			now := time.Now()
			var results []dormantResult
			for parcelRows.Next() {
				var content []byte
				if err := parcelRows.Scan(&content); err != nil {
					continue
				}
				var p struct {
					ParcelID      string  `json:"parcel_id"`
					Address       string  `json:"address"`
					OwnerName     string  `json:"owner_name"`
					Market        string  `json:"market"`
					AssessedValue float64 `json:"assessed_value"`
				}
				if err := json.Unmarshal(content, &p); err != nil {
					continue
				}

				ent, ok := entityMap[strings.ToLower(p.OwnerName)]
				if !ok {
					continue
				}

				// Calculate years inactive from formation date as a proxy
				yearsInactive := 0
				if ent.FormationDate != "" {
					if fd, err := time.Parse("2006-01-02", ent.FormationDate); err == nil {
						yearsInactive = int(now.Sub(fd).Hours() / 24 / 365)
					}
				}

				if inactiveYears > 0 && yearsInactive < inactiveYears {
					continue
				}

				results = append(results, dormantResult{
					ParcelID:      p.ParcelID,
					Address:       p.Address,
					OwnerName:     p.OwnerName,
					Market:        p.Market,
					EntityName:    ent.Name,
					EntityStatus:  ent.Status,
					Jurisdiction:  ent.Jurisdiction,
					FormationDate: ent.FormationDate,
					YearsInactive: yearsInactive,
					AssessedValue: p.AssessedValue,
				})
			}
			if err := parcelRows.Err(); err != nil {
				return fmt.Errorf("iterating parcels: %w", err)
			}

			// Sort by years inactive descending
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if results[j].YearsInactive > results[i].YearsInactive {
						results[i], results[j] = results[j], results[i]
					}
				}
			}

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&market, "market", "", "Market/county to search (e.g., lake-county-in)")
	cmd.Flags().IntVar(&inactiveYears, "inactive-years", 0, "Minimum years entity has been inactive")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

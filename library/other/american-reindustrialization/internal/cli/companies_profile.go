// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/internal/store"
	"github.com/spf13/cobra"
)

type profileOpening struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	WorkMode        string `json:"work_mode,omitempty"`
	ExperienceLevel string `json:"experience_level,omitempty"`
	SalaryMin       int64  `json:"salary_min,omitempty"`
	SalaryMax       int64  `json:"salary_max,omitempty"`
	LocationDisplay string `json:"location_display,omitempty"`
	PostedAt        string `json:"posted_at,omitempty"`
}

type profileSimilar struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Sector        string `json:"primary_sector,omitempty"`
	State         string `json:"hq_state,omitempty"`
	EmployeeRange string `json:"employee_range,omitempty"`
}

type companyProfile struct {
	Company  map[string]any   `json:"company"`
	Openings []profileOpening `json:"openings"`
	Similar  []profileSimilar `json:"similar"`
}

func newCompaniesProfileCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "profile <slug>",
		Short: "Single-shot rich profile: company + its open jobs + similar companies",
		Long: "Joins the locally synced companies, openings, and a similarity heuristic " +
			"(same primary_sector and employee_range) into one response. Pure local query.",
		Example:     "  american-reindustrialization-pp-cli companies profile harmony-ai --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("american-reindustrialization-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'american-reindustrialization-pp-cli sync' first.", err)
			}
			defer db.Close()

			var rawData, idStr sql.NullString
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT id, data FROM companies WHERE slug = ?`, slug,
			).Scan(&idStr, &rawData)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("company %q not found in local store; run sync or check spelling", slug))
			}
			if err != nil {
				return fmt.Errorf("query company: %w", err)
			}
			var company map[string]any
			if err := json.Unmarshal([]byte(rawData.String), &company); err != nil {
				return fmt.Errorf("parsing company JSON: %w", err)
			}

			openings, err := profileOpeningsForCompany(cmd, db, idStr.String)
			if err != nil {
				return err
			}

			var sector, employeeRange sql.NullString
			_ = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(primary_sector, ''), COALESCE(employee_range, '')
				 FROM companies WHERE slug = ?`, slug,
			).Scan(&sector, &employeeRange)
			similar, err := profileSimilarCompanies(cmd, db, slug, sector.String, employeeRange.String)
			if err != nil {
				return err
			}

			result := companyProfile{Company: company, Openings: openings, Similar: similar}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}

func profileOpeningsForCompany(cmd *cobra.Command, db *store.Store, companyID string) ([]profileOpening, error) {
	out := make([]profileOpening, 0)
	if companyID == "" {
		return out, nil
	}
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT slug, title, COALESCE(work_mode,''), COALESCE(experience_level,''),
		        COALESCE(salary_min,0), COALESCE(salary_max,0),
		        COALESCE(location_display,''), COALESCE(posted_at,'')
		 FROM openings
		 WHERE company_id = ?
		 ORDER BY COALESCE(posted_at,'') DESC`, companyID)
	if err != nil {
		return out, fmt.Errorf("query openings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var slug, title, wm, exp, loc, posted sql.NullString
		var smin, smax sql.NullInt64
		if err := rows.Scan(&slug, &title, &wm, &exp, &smin, &smax, &loc, &posted); err != nil {
			continue
		}
		out = append(out, profileOpening{
			Slug:            slug.String,
			Title:           title.String,
			WorkMode:        wm.String,
			ExperienceLevel: exp.String,
			SalaryMin:       smin.Int64,
			SalaryMax:       smax.Int64,
			LocationDisplay: loc.String,
			PostedAt:        posted.String,
		})
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("iterating profile openings rows: %w", err)
	}
	return out, nil
}

func profileSimilarCompanies(cmd *cobra.Command, db *store.Store, excludeSlug, sector, employeeRange string) ([]profileSimilar, error) {
	out := make([]profileSimilar, 0)
	if sector == "" {
		return out, nil
	}
	q := `SELECT slug, name, COALESCE(primary_sector,''), COALESCE(hq_state,''), COALESCE(employee_range,'')
	      FROM companies
	      WHERE slug != ? AND lower(primary_sector) = lower(?)`
	args := []any{excludeSlug, sector}
	if employeeRange != "" {
		q += " AND COALESCE(employee_range,'') = ?"
		args = append(args, employeeRange)
	}
	q += " ORDER BY COALESCE(jobs_count,0) DESC, name ASC LIMIT 10"

	rows, err := db.DB().QueryContext(cmd.Context(), q, args...)
	if err != nil {
		return out, fmt.Errorf("query similar: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var slug, name, sec, st, emp sql.NullString
		if err := rows.Scan(&slug, &name, &sec, &st, &emp); err != nil {
			continue
		}
		out = append(out, profileSimilar{
			Slug:          slug.String,
			Name:          name.String,
			Sector:        sec.String,
			State:         st.String,
			EmployeeRange: emp.String,
		})
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("iterating similar-companies rows: %w", err)
	}
	return out, nil
}

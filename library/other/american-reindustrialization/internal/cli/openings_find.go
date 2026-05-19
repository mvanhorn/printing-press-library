// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/internal/store"
	"github.com/spf13/cobra"
)

type openingRow struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	CompanyName     string `json:"company_name,omitempty"`
	CompanySlug     string `json:"company_slug,omitempty"`
	WorkMode        string `json:"work_mode,omitempty"`
	ExperienceLevel string `json:"experience_level,omitempty"`
	SalaryMin       int64  `json:"salary_min,omitempty"`
	SalaryMax       int64  `json:"salary_max,omitempty"`
	LocationCity    string `json:"location_city,omitempty"`
	LocationState   string `json:"location_state,omitempty"`
	LocationDisplay string `json:"location_display,omitempty"`
	PostedAt        string `json:"posted_at,omitempty"`
	ApplyURL        string `json:"apply_url,omitempty"`
	CompanySector   string `json:"company_sector,omitempty"`
	CompanyState    string `json:"company_state,omitempty"`
	EmployeeRange   string `json:"employee_range,omitempty"`
}

func newOpeningsFindCmd(flags *rootFlags) *cobra.Command {
	var workMode, experience, state, sector, companySize, postedSince, dbPath string
	var salaryMin int64
	var limit int

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Compose work-mode, experience, salary, state, company-size, sector, and posted-since filters",
		Long: "Filter openings by every available axis in a single SQL pass over the local " +
			"jobs × companies join. The website's /api/jobs silently ignores state filtering " +
			"and exposes no salary, company-size, or sector filters; this command does all of them.",
		Example:     "  american-reindustrialization-pp-cli openings find --work-mode remote --experience senior --salary-min 150000 --state TX --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			postedCutoff, err := parsePostedSince(postedSince)
			if err != nil {
				return usageErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("american-reindustrialization-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'american-reindustrialization-pp-cli sync' first.", err)
			}
			defer db.Close()

			q := `SELECT o.slug, o.title, COALESCE(c.name, ''), COALESCE(c.slug, ''),
			             COALESCE(o.work_mode,''), COALESCE(o.experience_level,''),
			             COALESCE(o.salary_min,0), COALESCE(o.salary_max,0),
			             COALESCE(o.location_city,''), COALESCE(o.location_state,''),
			             COALESCE(o.location_display,''), COALESCE(o.posted_at,''),
			             COALESCE(o.apply_url,''),
			             COALESCE(c.primary_sector,''), COALESCE(c.hq_state,''),
			             COALESCE(c.employee_range,'')
			      FROM openings o
			      LEFT JOIN companies c ON c.id = o.company_id
			      WHERE 1=1`
			args2 := []any{}
			if workMode != "" {
				q += " AND lower(o.work_mode) = lower(?)"
				args2 = append(args2, strings.TrimSpace(workMode))
			}
			if experience != "" {
				q += " AND lower(o.experience_level) = lower(?)"
				args2 = append(args2, strings.TrimSpace(experience))
			}
			if salaryMin > 0 {
				q += " AND COALESCE(o.salary_max, o.salary_min, 0) >= ?"
				args2 = append(args2, salaryMin)
			}
			if state != "" {
				q += " AND (upper(o.location_state) = upper(?) OR upper(c.hq_state) = upper(?))"
				args2 = append(args2, strings.TrimSpace(state), strings.TrimSpace(state))
			}
			if sector != "" {
				q += " AND lower(c.primary_sector) = lower(?)"
				args2 = append(args2, strings.TrimSpace(sector))
			}
			if companySize != "" {
				q += " AND COALESCE(c.employee_range,'') = ?"
				args2 = append(args2, strings.TrimSpace(companySize))
			}
			if !postedCutoff.IsZero() {
				q += " AND COALESCE(o.posted_at, o.created_at, '') >= ?"
				args2 = append(args2, postedCutoff.UTC().Format(time.RFC3339))
			}
			q += " ORDER BY COALESCE(o.posted_at, o.created_at, '') DESC"
			if limit > 0 {
				q += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			out := make([]openingRow, 0)
			for rows.Next() {
				var r openingRow
				var smin, smax sql.NullInt64
				if err := rows.Scan(&r.Slug, &r.Title, &r.CompanyName, &r.CompanySlug,
					&r.WorkMode, &r.ExperienceLevel, &smin, &smax,
					&r.LocationCity, &r.LocationState, &r.LocationDisplay, &r.PostedAt,
					&r.ApplyURL, &r.CompanySector, &r.CompanyState, &r.EmployeeRange); err != nil {
					continue
				}
				r.SalaryMin = smin.Int64
				r.SalaryMax = smax.Int64
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating openings rows: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&workMode, "work-mode", "", "Filter by work_mode (remote, hybrid, onsite)")
	cmd.Flags().StringVar(&experience, "experience", "", "Filter by experience_level (entry, mid, senior, lead)")
	cmd.Flags().Int64Var(&salaryMin, "salary-min", 0, "Minimum salary floor (matches against max-of-salary if available)")
	cmd.Flags().StringVar(&state, "state", "", "Filter by location state or company HQ state (2-letter code)")
	cmd.Flags().StringVar(&sector, "sector", "", "Filter by company's primary_sector")
	cmd.Flags().StringVar(&companySize, "company-size", "", "Filter by employee_range value (e.g. 11-50, 51-200)")
	cmd.Flags().StringVar(&postedSince, "posted-since", "", "Cutoff date (YYYY-MM-DD or RFC3339); openings posted/created at or after")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return (0 = no limit)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}

func parsePostedSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --posted-since %q: use YYYY-MM-DD or RFC3339", s)
}

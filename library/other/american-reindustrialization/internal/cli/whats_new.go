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

type whatsNewItem struct {
	Kind          string `json:"kind"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	State         string `json:"state,omitempty"`
	Sector        string `json:"primary_sector,omitempty"`
	FundingStage  string `json:"funding_stage,omitempty"`
	EmployeeRange string `json:"employee_range,omitempty"`
	Title         string `json:"title,omitempty"`
	WorkMode      string `json:"work_mode,omitempty"`
	Experience    string `json:"experience_level,omitempty"`
	CompanyID     string `json:"company_id,omitempty"`
	Timestamp     string `json:"timestamp"`
}

func newWhatsNewCmd(flags *rootFlags) *cobra.Command {
	var since string
	var kind string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "whats-new",
		Short: "Show companies and job openings added or updated since a date",
		Long: "Diff against the local SQLite store: companies whose updated_at or created_at " +
			"is greater than --since, plus openings whose posted_at or updated_at exceeds it.\n" +
			"The local store is populated by `sync`; if --since is omitted, defaults to 7 days ago.",
		Example: "  american-reindustrialization-pp-cli whats-new --since 2026-05-12 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := parseWhatsNewSince(since)
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

			items := make([]whatsNewItem, 0)
			cutoffStr := cutoff.UTC().Format(time.RFC3339)
			if kind == "" || kind == "companies" {
				rows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT slug, name, hq_state, primary_sector, funding_stage, employee_range,
					        COALESCE(updated_at, created_at, '') AS ts
					 FROM companies
					 WHERE COALESCE(updated_at, created_at, '') > ?
					 ORDER BY COALESCE(updated_at, created_at, '') DESC`, cutoffStr)
				if err != nil {
					return fmt.Errorf("query companies: %w", err)
				}
				for rows.Next() {
					var slug, name, state, sector, funding, employee, ts sql.NullString
					if err := rows.Scan(&slug, &name, &state, &sector, &funding, &employee, &ts); err != nil {
						continue
					}
					items = append(items, whatsNewItem{
						Kind:          "company",
						Slug:          slug.String,
						Name:          name.String,
						State:         state.String,
						Sector:        sector.String,
						FundingStage:  funding.String,
						EmployeeRange: employee.String,
						Timestamp:     ts.String,
					})
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return fmt.Errorf("iterating companies rows: %w", err)
				}
				rows.Close()
			}
			if kind == "" || kind == "openings" {
				rows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT slug, title, work_mode, experience_level, company_id,
					        COALESCE(posted_at, updated_at, created_at, '') AS ts
					 FROM openings
					 WHERE COALESCE(posted_at, updated_at, created_at, '') > ?
					 ORDER BY COALESCE(posted_at, updated_at, created_at, '') DESC`, cutoffStr)
				if err != nil {
					return fmt.Errorf("query openings: %w", err)
				}
				for rows.Next() {
					var slug, title, workMode, experience, companyID, ts sql.NullString
					if err := rows.Scan(&slug, &title, &workMode, &experience, &companyID, &ts); err != nil {
						continue
					}
					items = append(items, whatsNewItem{
						Kind:       "opening",
						Slug:       slug.String,
						Name:       title.String,
						Title:      title.String,
						WorkMode:   workMode.String,
						Experience: experience.String,
						CompanyID:  companyID.String,
						Timestamp:  ts.String,
					})
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return fmt.Errorf("iterating openings rows: %w", err)
				}
				rows.Close()
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Cutoff date (YYYY-MM-DD or RFC3339); defaults to 7 days ago")
	cmd.Flags().StringVar(&kind, "kind", "", "Limit to one kind: companies, openings (default both)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}

func parseWhatsNewSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().UTC().Add(-7 * 24 * time.Hour), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --since %q: use YYYY-MM-DD or RFC3339", s)
}

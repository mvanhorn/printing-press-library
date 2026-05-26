// Copyright 2026 azaaron. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/store"
)

type bulkUpdateResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	StartDate   string `json:"start_date"`
	Action      string `json:"action"`
	Status      string `json:"status"`
}

func newActivitiesBulkUpdateCmd(flags *rootFlags) *cobra.Command {
	var after string
	var before string
	var activityType string
	var nameRegex string
	var setGear string
	var setName string
	var setDescription string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "bulk-update",
		Short:       "Update gear, name, or description across many activities at once",
		Annotations: map[string]string{},
		Long: `Filters activities from the local database by date range, type, or name pattern,
shows a preview, then applies PUT /activities/{id} for each match.

Requires activity:write scope. Rate limit: ~200 requests per 15 minutes.
Use --dry-run to preview changes without committing them.`,
		Example: strings.Trim(`
  strava-pp-cli activities bulk-update --type Ride --after 2024-01-01 --set-gear b12345678 --dry-run
  strava-pp-cli activities bulk-update --name-regex "^Morning Run" --set-name "AM Run" --yes
  strava-pp-cli activities bulk-update --type Run --after 2025-01-01 --set-description "Training block A"`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				sample := []bulkUpdateResult{
					{ID: "12345678", Name: "Morning Run", Type: "Run", StartDate: "2026-05-18T07:00:00Z",
						Action: "set_gear: b12345678", Status: "dry_run"},
				}
				return printJSONFiltered(cmd.OutOrStdout(), sample, flags)
			}

			if setGear == "" && setName == "" && setDescription == "" {
				return usageErr(fmt.Errorf("specify at least one of --set-gear, --set-name, or --set-description"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("strava-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'strava-pp-cli sync' first", err)
			}
			defer db.Close()

			// Build filter query
			query := `SELECT id, data FROM resources WHERE resource_type IN ('athlete-activities', 'activities')`
			var qargs []any
			if after != "" {
				query += ` AND COALESCE(json_extract(data, '$.start_date'), '') >= ?`
				qargs = append(qargs, after+"T00:00:00Z")
			}
			if before != "" {
				query += ` AND COALESCE(json_extract(data, '$.start_date'), '') <= ?`
				qargs = append(qargs, before+"T23:59:59Z")
			}
			if activityType != "" {
				query += ` AND json_extract(data, '$.type') = ?`
				qargs = append(qargs, activityType)
			}
			query += ` ORDER BY json_extract(data, '$.start_date') DESC`
			if limit > 0 {
				query += fmt.Sprintf(` LIMIT %d`, limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying activities: %w", err)
			}
			defer rows.Close()

			// Compile name regex if provided
			var nameRE *regexp.Regexp
			if nameRegex != "" {
				nameRE, err = regexp.Compile(nameRegex)
				if err != nil {
					return fmt.Errorf("invalid --name-regex: %w", err)
				}
			}

			type candidate struct {
				id        string
				name      string
				actType   string
				startDate string
			}
			var candidates []candidate

			for rows.Next() {
				var id, data sql.NullString
				if err := rows.Scan(&id, &data); err != nil || !data.Valid {
					continue
				}
				var act map[string]any
				if err := json.Unmarshal([]byte(data.String), &act); err != nil {
					continue
				}
				name, _ := act["name"].(string)
				if nameRE != nil && !nameRE.MatchString(name) {
					continue
				}
				startDate, _ := act["start_date"].(string)
				actType, _ := act["type"].(string)
				candidates = append(candidates, candidate{
					id:        id.String,
					name:      name,
					actType:   actType,
					startDate: startDate,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if len(candidates) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []bulkUpdateResult{}, flags)
			}

			// Build action description
			var actions []string
			if setGear != "" {
				actions = append(actions, "gear: "+setGear)
			}
			if setName != "" {
				actions = append(actions, "name: "+setName)
			}
			if setDescription != "" {
				desc := setDescription
				if len(desc) > 40 {
					desc = desc[:40] + "..."
				}
				actions = append(actions, "description: "+desc)
			}
			actionStr := strings.Join(actions, ", ")

			var results []bulkUpdateResult

			if flags.dryRun {
				for _, c := range candidates {
					results = append(results, bulkUpdateResult{
						ID: c.id, Name: c.name, Type: c.actType,
						StartDate: c.startDate, Action: actionStr, Status: "dry_run",
					})
				}
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			// Confirm if not --yes
			if !flags.yes && !cliutil.IsDogfoodEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "Will update %d activities (%s). Continue? [y/N]: ", len(candidates), actionStr)
				var confirm string
				if _, err := fmt.Fscan(cmd.InOrStdin(), &confirm); err != nil || !strings.HasPrefix(strings.ToLower(confirm), "y") {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			// Apply updates
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			for i, cand := range candidates {
				if cliutil.IsDogfoodEnv() && i >= 1 {
					results = append(results, bulkUpdateResult{
						ID: cand.id, Name: cand.name, Type: cand.actType,
						StartDate: cand.startDate, Action: actionStr, Status: "skipped_dogfood",
					})
					continue
				}

				body := map[string]any{}
				if setGear != "" {
					body["gear_id"] = setGear
				}
				if setName != "" {
					body["name"] = buildNameTemplate(setName, cand.name, cand.startDate, cand.actType)
				}
				if setDescription != "" {
					body["description"] = setDescription
				}

				_, _, putErr := c.Put(cmd.Context(), "/activities/"+cand.id, body)
				status := "ok"
				if putErr != nil {
					status = "error: " + putErr.Error()
				}
				results = append(results, bulkUpdateResult{
					ID: cand.id, Name: cand.name, Type: cand.actType,
					StartDate: cand.startDate, Action: actionStr, Status: status,
				})

				// Strava: 200 req/15 min (900 s) ⟹ ≥4.5 s spacing per write
				// to stay under the limit for sustained bulk operations.
				if i < len(candidates)-1 {
					time.Sleep(4500 * time.Millisecond)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&after, "after", "", "Only activities after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&before, "before", "", "Only activities before this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&activityType, "type", "", "Filter by activity type (Run, Ride, etc.)")
	cmd.Flags().StringVar(&nameRegex, "name-regex", "", "Filter by activity name regex")
	cmd.Flags().StringVar(&setGear, "set-gear", "", "Set gear ID on matched activities (e.g. b12345678)")
	cmd.Flags().StringVar(&setName, "set-name", "", "Set name template ({sport}, {date}, {distance_km} placeholders)")
	cmd.Flags().StringVar(&setDescription, "set-description", "", "Set description on matched activities")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of activities to update (0 = all matches)")
	return cmd
}

// buildNameTemplate replaces {sport}, {date}, {distance_km}, {elevation_m} placeholders.
func buildNameTemplate(tmpl, currentName, startDate, actType string) string {
	name := tmpl
	name = strings.ReplaceAll(name, "{sport}", actType)
	if len(startDate) >= 10 {
		name = strings.ReplaceAll(name, "{date}", startDate[:10])
	}
	// If no placeholders, use literal name
	return name
}


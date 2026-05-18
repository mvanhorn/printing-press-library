package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/brainos/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/brainos/internal/store"
	"github.com/spf13/cobra"
)

func newSkillsGapCmd(flags *rootFlags) *cobra.Command {
	var dbPath, domain string
	cmd := &cobra.Command{
		Use:   "gap",
		Short: "Skill domains with low proficiency relative to tool usage",
		Example: strings.Trim(`
  brainos-pp-cli skills gap --json
  brainos-pp-cli skills gap --domain AI --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[]`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("brainos-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'brainos-pp-cli sync' first.", err)
			}
			defer db.Close()

			skillQuery := `
				SELECT
					json_extract(data, '$.skill_name') as skill,
					json_extract(data, '$.domain') as domain,
					CAST(json_extract(data, '$.proficiency') AS REAL) as proficiency,
					json_extract(data, '$.expertise_level') as expertise_level
				FROM resources WHERE resource_type = 'skills'`
			var sArgs []any
			if domain != "" {
				skillQuery += " AND json_extract(data, '$.domain') = ?"
				sArgs = append(sArgs, domain)
			}
			skillQuery += " ORDER BY proficiency ASC"

			rows, err := db.DB().QueryContext(cmd.Context(), skillQuery, sArgs...)
			if err != nil {
				return fmt.Errorf("querying skills: %w", err)
			}
			defer rows.Close()

			type skillRow struct {
				Skill          string  `json:"skill"`
				Domain         string  `json:"domain"`
				Proficiency    float64 `json:"proficiency"`
				ExpertiseLevel string  `json:"expertise_level"`
				ToolUsage      *int64  `json:"tool_usage"`
				TotalCalls     *int64  `json:"total_calls"`
			}
			var skills []skillRow
			for rows.Next() {
				var skill, dom, level sql.NullString
				var prof sql.NullFloat64
				if err := rows.Scan(&skill, &dom, &prof, &level); err == nil {
					skills = append(skills, skillRow{
						Skill:          skill.String,
						Domain:         dom.String,
						Proficiency:    prof.Float64,
						ExpertiseLevel: level.String,
					})
				}
			}

			// Load tool usage stats for correlation
			toolRows, _ := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.tool'), json_extract(data, '$.total_calls')
				FROM resources WHERE resource_type = 'tool-usage-stats'
			`)
			toolUsage := map[string]int64{}
			if toolRows != nil {
				defer toolRows.Close()
				for toolRows.Next() {
					var tool sql.NullString
					var calls sql.NullInt64
					if err := toolRows.Scan(&tool, &calls); err == nil {
						toolUsage[strings.ToLower(tool.String)] = calls.Int64
					}
				}
			}

			var results []skillRow
			for i := range skills {
				s := skills[i]
				// Fuzzy match skill name to tool names
				skillLower := strings.ToLower(s.Skill)
				for toolName, calls := range toolUsage {
					if strings.Contains(toolName, skillLower) || strings.Contains(skillLower, toolName) {
						c := calls
						s.TotalCalls = &c
						break
					}
				}
				results = append(results, s)
			}
			if results == nil {
				results = []skillRow{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&domain, "domain", "", "Filter by skill domain (e.g. AI, trading)")
	return cmd
}

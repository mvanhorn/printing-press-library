package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/trainual/internal/store"
)

type matrixCell struct {
	RoleName      string             `json:"role_name"`
	SubjectScores map[string]float64 `json:"subject_scores"`
	UserCount     int                `json:"user_count"`
}

func newCoverageMatrixCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "coverage-matrix",
		Short: "Role-by-subject completion matrix — the report ops managers build by hand",
		Example: strings.Trim(`
  trainual-pp-cli coverage-matrix --json
  trainual-pp-cli coverage-matrix`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("trainual-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			results, err := runCoverageMatrix(cmd.Context(), db)
			if err != nil {
				return err
			}
			jsonData, err := json.Marshal(results)
			if err != nil {
				return fmt.Errorf("marshaling results: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jsonData, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runCoverageMatrix(ctx context.Context, db *store.Store) ([]matrixCell, error) {
	roles, err := db.List("roles", 0)
	if err != nil {
		return nil, fmt.Errorf("querying roles: %w", err)
	}
	subjects, err := db.List("subjects", 0)
	if err != nil {
		return nil, fmt.Errorf("querying subjects: %w", err)
	}

	subjectNames := map[string]string{}
	for _, s := range subjects {
		var subj struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(s, &subj); err != nil {
			continue
		}
		subjectNames[subj.ID] = subj.Name
	}

	var results []matrixCell
	for _, r := range roles {
		var role struct {
			Name          string `json:"name"`
			AssignedUsers []struct {
				ID                  string `json:"id"`
				CurriculumsAssigned []struct {
					ID                   string  `json:"id"`
					CompletionPercentage float64 `json:"completion_percentage"`
				} `json:"curriculums_assigned"`
			} `json:"assigned_users"`
		}
		if err := json.Unmarshal(r, &role); err != nil {
			continue
		}
		if len(role.AssignedUsers) == 0 {
			continue
		}

		subjectTotals := map[string]float64{}
		subjectCounts := map[string]int{}
		for _, u := range role.AssignedUsers {
			for _, c := range u.CurriculumsAssigned {
				subjectTotals[c.ID] += c.CompletionPercentage
				subjectCounts[c.ID]++
			}
		}

		scores := map[string]float64{}
		for subID, total := range subjectTotals {
			name := subjectNames[subID]
			if name == "" {
				name = subID
			}
			scores[name] = total / float64(subjectCounts[subID])
		}

		results = append(results, matrixCell{
			RoleName:      role.Name,
			SubjectScores: scores,
			UserCount:     len(role.AssignedUsers),
		})
	}
	return results, nil
}

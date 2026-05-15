package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/trainual/internal/store"
)

type onboardingResult struct {
	UserID               string  `json:"user_id"`
	Name                 string  `json:"name"`
	Email                string  `json:"email"`
	CreatedAt            string  `json:"created_at"`
	DaysAgo              int     `json:"days_ago"`
	Role                 string  `json:"role,omitempty"`
	CompletionPercentage float64 `json:"completion_percentage"`
	SubjectsAssigned     int     `json:"subjects_assigned"`
}

func newOnboardingStatusCmd(flags *rootFlags) *cobra.Command {
	var days int
	var roleName string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "onboarding-status",
		Short: "Show new hires from the last N days with assignment completeness and completion",
		Example: strings.Trim(`
  trainual-pp-cli onboarding-status --days 30 --json
  trainual-pp-cli onboarding-status --days 14 --role "New Hire"`, "\n"),
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

			results, err := runOnboardingStatus(cmd.Context(), db, days, roleName)
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
	cmd.Flags().IntVar(&days, "days", 30, "Look back N days for new hires")
	cmd.Flags().StringVar(&roleName, "role", "", "Filter to a specific role")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runOnboardingStatus(ctx context.Context, db *store.Store, days int, roleName string) ([]onboardingResult, error) {
	users, err := db.List("users", 0)
	if err != nil {
		return nil, fmt.Errorf("querying users: %w", err)
	}
	roles, err := db.List("roles", 0)
	if err != nil {
		return nil, fmt.Errorf("querying roles: %w", err)
	}

	userRoles := map[string]string{}
	for _, r := range roles {
		var role struct {
			Name          string `json:"name"`
			AssignedUsers []struct {
				ID string `json:"id"`
			} `json:"assigned_users"`
		}
		if err := json.Unmarshal(r, &role); err != nil {
			continue
		}
		for _, u := range role.AssignedUsers {
			userRoles[u.ID] = role.Name
		}
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var results []onboardingResult
	for _, u := range users {
		var user struct {
			ID                   string            `json:"id"`
			Email                string            `json:"email"`
			FirstName            string            `json:"first_name"`
			LastName             string            `json:"last_name"`
			CompletionPercentage float64           `json:"completion_percentage"`
			CreatedAt            string            `json:"created_at"`
			Status               string            `json:"status"`
			CurriculumsAssigned  []json.RawMessage `json:"curriculums_assigned"`
		}
		if err := json.Unmarshal(u, &user); err != nil {
			continue
		}
		if user.Status != "" && user.Status != "active" {
			continue
		}

		created, err := time.Parse(time.RFC3339, user.CreatedAt)
		if err != nil {
			created, err = time.Parse("2006-01-02T15:04:05.000Z", user.CreatedAt)
			if err != nil {
				continue
			}
		}
		if created.Before(cutoff) {
			continue
		}

		role := userRoles[user.ID]
		if roleName != "" && !strings.EqualFold(role, roleName) {
			continue
		}

		results = append(results, onboardingResult{
			UserID:               user.ID,
			Name:                 strings.TrimSpace(user.FirstName + " " + user.LastName),
			Email:                user.Email,
			CreatedAt:            user.CreatedAt,
			DaysAgo:              int(time.Since(created).Hours() / 24),
			Role:                 role,
			CompletionPercentage: user.CompletionPercentage,
			SubjectsAssigned:     len(user.CurriculumsAssigned),
		})
	}
	return results, nil
}

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/trainual/internal/store"
)

type complianceResult struct {
	UserID               string  `json:"user_id"`
	Name                 string  `json:"name"`
	Email                string  `json:"email"`
	CompletionPercentage float64 `json:"completion_percentage"`
	Gap                  float64 `json:"gap"`
	Role                 string  `json:"role,omitempty"`
}

func newComplianceAuditCmd(flags *rootFlags) *cobra.Command {
	var threshold int
	var roleName string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compliance-audit",
		Short: "Surface every employee below a completion threshold, grouped by role",
		Example: strings.Trim(`
  trainual-pp-cli compliance-audit --threshold 80
  trainual-pp-cli compliance-audit --threshold 80 --role "Front Desk" --json`, "\n"),
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

			results, err := runComplianceAudit(cmd.Context(), db, threshold, roleName)
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
	cmd.Flags().IntVar(&threshold, "threshold", 100, "Completion percentage threshold (users below this are flagged)")
	cmd.Flags().StringVar(&roleName, "role", "", "Filter to a specific role")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runComplianceAudit(ctx context.Context, db *store.Store, threshold int, roleName string) ([]complianceResult, error) {
	users, err := db.List("users", 0)
	if err != nil {
		return nil, fmt.Errorf("querying users: %w", err)
	}
	roles, err := db.List("roles", 0)
	if err != nil {
		return nil, fmt.Errorf("querying roles: %w", err)
	}

	// Build role lookup from roles with assigned_users
	userRoles := map[string]string{}
	for _, r := range roles {
		var role struct {
			ID            string `json:"id"`
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

	var results []complianceResult
	for _, u := range users {
		var user struct {
			ID                   string  `json:"id"`
			Email                string  `json:"email"`
			FirstName            string  `json:"first_name"`
			LastName             string  `json:"last_name"`
			CompletionPercentage float64 `json:"completion_percentage"`
			Status               string  `json:"status"`
		}
		if err := json.Unmarshal(u, &user); err != nil {
			continue
		}
		if user.Status != "" && user.Status != "active" {
			continue
		}
		if user.CompletionPercentage >= float64(threshold) {
			continue
		}
		role := userRoles[user.ID]
		if roleName != "" && !strings.EqualFold(role, roleName) {
			continue
		}
		results = append(results, complianceResult{
			UserID:               user.ID,
			Name:                 strings.TrimSpace(user.FirstName + " " + user.LastName),
			Email:                user.Email,
			CompletionPercentage: user.CompletionPercentage,
			Gap:                  float64(threshold) - user.CompletionPercentage,
			Role:                 role,
		})
	}
	return results, nil
}

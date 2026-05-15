package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/trainual/internal/store"
)

type roleCompletionResult struct {
	RoleName      string  `json:"role_name"`
	AvgCompletion float64 `json:"avg_completion"`
	UserCount     int     `json:"user_count"`
	MinCompletion float64 `json:"min_completion"`
	MaxCompletion float64 `json:"max_completion"`
}

func newRoleCompletionCmd(flags *rootFlags) *cobra.Command {
	var sortBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "role-completion",
		Short: "Rank roles by average completion percentage",
		Example: strings.Trim(`
  trainual-pp-cli role-completion --sort avg_completion --json
  trainual-pp-cli role-completion`, "\n"),
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

			results, err := runRoleCompletion(cmd.Context(), db, sortBy)
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
	cmd.Flags().StringVar(&sortBy, "sort", "avg_completion", "Sort field: avg_completion, user_count, role_name")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runRoleCompletion(ctx context.Context, db *store.Store, sortBy string) ([]roleCompletionResult, error) {
	roles, err := db.List("roles", 0)
	if err != nil {
		return nil, fmt.Errorf("querying roles: %w", err)
	}
	users, err := db.List("users", 0)
	if err != nil {
		return nil, fmt.Errorf("querying users: %w", err)
	}

	userCompletion := map[string]float64{}
	for _, u := range users {
		var user struct {
			ID                   string  `json:"id"`
			CompletionPercentage float64 `json:"completion_percentage"`
		}
		if err := json.Unmarshal(u, &user); err != nil {
			continue
		}
		userCompletion[user.ID] = user.CompletionPercentage
	}

	var results []roleCompletionResult
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
		if len(role.AssignedUsers) == 0 {
			continue
		}

		var total, minC, maxC float64
		minC = 100
		for i, u := range role.AssignedUsers {
			c := userCompletion[u.ID]
			total += c
			if i == 0 || c < minC {
				minC = c
			}
			if c > maxC {
				maxC = c
			}
		}

		results = append(results, roleCompletionResult{
			RoleName:      role.Name,
			AvgCompletion: total / float64(len(role.AssignedUsers)),
			UserCount:     len(role.AssignedUsers),
			MinCompletion: minC,
			MaxCompletion: maxC,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		switch sortBy {
		case "user_count":
			return results[i].UserCount > results[j].UserCount
		case "role_name":
			return results[i].RoleName < results[j].RoleName
		default:
			return results[i].AvgCompletion < results[j].AvgCompletion
		}
	})

	return results, nil
}

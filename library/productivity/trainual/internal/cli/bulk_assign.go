package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/trainual/internal/store"
)

type bulkAssignPlan struct {
	UserID     string   `json:"user_id"`
	UserName   string   `json:"user_name"`
	SubjectIDs []string `json:"subject_ids"`
	Action     string   `json:"action"`
}

func newBulkAssignCmd(flags *rootFlags) *cobra.Command {
	var roleName string
	var subjectIDs string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "bulk-assign",
		Short: "Assign subjects to all users in a role with one command",
		Example: strings.Trim(`
  trainual-pp-cli bulk-assign --role "Kitchen" --subjects 101,102,103 --dry-run
  trainual-pp-cli bulk-assign --role "New Hire" --subjects 1340738 --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if roleName == "" || subjectIDs == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "Both --role and --subjects are required")
				return cmd.Help()
			}
			if dbPath == "" {
				dbPath = defaultDBPath("trainual-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			sids := strings.Split(subjectIDs, ",")
			for i := range sids {
				sids[i] = strings.TrimSpace(sids[i])
			}

			plan, err := buildBulkAssignPlan(cmd.Context(), db, roleName, sids)
			if err != nil {
				return err
			}

			if dryRunOK(flags) {
				jsonData, err := json.Marshal(plan)
				if err != nil {
					return fmt.Errorf("marshaling plan: %w", err)
				}
				return printOutputWithFlags(cmd.OutOrStdout(), jsonData, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var results []bulkAssignPlan
			for _, p := range plan {
				body := map[string]string{"subject_ids": strings.Join(p.SubjectIDs, ",")}
				bodyJSON, _ := json.Marshal(body)
				_, _, err := c.Post(fmt.Sprintf("/users/%s/assign_subjects", p.UserID), bodyJSON)
				if err != nil {
					p.Action = fmt.Sprintf("FAILED: %v", err)
				} else {
					p.Action = "ASSIGNED"
				}
				results = append(results, p)
			}

			jsonData, err := json.Marshal(results)
			if err != nil {
				return fmt.Errorf("marshaling results: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jsonData, flags)
		},
	}
	cmd.Flags().StringVar(&roleName, "role", "", "Role name to target (required)")
	cmd.Flags().StringVar(&subjectIDs, "subjects", "", "Comma-separated subject IDs to assign (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func buildBulkAssignPlan(ctx context.Context, db *store.Store, roleName string, subjectIDs []string) ([]bulkAssignPlan, error) {
	roles, err := db.List("roles", 0)
	if err != nil {
		return nil, fmt.Errorf("querying roles: %w", err)
	}

	var plan []bulkAssignPlan
	for _, r := range roles {
		var role struct {
			Name          string `json:"name"`
			AssignedUsers []struct {
				ID        string `json:"id"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
			} `json:"assigned_users"`
		}
		if err := json.Unmarshal(r, &role); err != nil {
			continue
		}
		if !strings.EqualFold(role.Name, roleName) {
			continue
		}
		for _, u := range role.AssignedUsers {
			plan = append(plan, bulkAssignPlan{
				UserID:     u.ID,
				UserName:   strings.TrimSpace(u.FirstName + " " + u.LastName),
				SubjectIDs: subjectIDs,
				Action:     "PLANNED",
			})
		}
	}

	if len(plan) == 0 {
		return []bulkAssignPlan{}, nil
	}
	return plan, nil
}

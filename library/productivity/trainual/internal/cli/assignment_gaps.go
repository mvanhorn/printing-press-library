package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/trainual/internal/store"
)

type assignmentGap struct {
	UserID          string   `json:"user_id"`
	UserName        string   `json:"user_name"`
	RoleName        string   `json:"role_name"`
	MissingSubjects []string `json:"missing_subjects"`
}

func newAssignmentGapsCmd(flags *rootFlags) *cobra.Command {
	var byRole bool
	var roleName string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "assignment-gaps",
		Short: "Detect users missing subject assignments that peers in their role have",
		Example: strings.Trim(`
  trainual-pp-cli assignment-gaps --by-role --json
  trainual-pp-cli assignment-gaps --role "Kitchen" --json`, "\n"),
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

			results, err := runAssignmentGaps(cmd.Context(), db, byRole, roleName)
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
	cmd.Flags().BoolVar(&byRole, "by-role", false, "Group gaps by role")
	cmd.Flags().StringVar(&roleName, "role", "", "Filter to a specific role")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runAssignmentGaps(ctx context.Context, db *store.Store, byRole bool, roleName string) ([]assignmentGap, error) {
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

	type roleData struct {
		Name          string
		UserSubjects  map[string]map[string]bool // userID -> set of subjectIDs
		UserNames     map[string]string
		AllSubjectIDs map[string]bool
	}

	roleMap := map[string]*roleData{}
	for _, r := range roles {
		var role struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			AssignedUsers []struct {
				ID                  string `json:"id"`
				FirstName           string `json:"first_name"`
				LastName            string `json:"last_name"`
				CurriculumsAssigned []struct {
					ID string `json:"id"`
				} `json:"curriculums_assigned"`
				SubjectsAssigned []struct {
					ID string `json:"id"`
				} `json:"subjects_assigned"`
			} `json:"assigned_users"`
		}
		if err := json.Unmarshal(r, &role); err != nil {
			continue
		}
		if roleName != "" && !strings.EqualFold(role.Name, roleName) {
			continue
		}
		rd := &roleData{
			Name:          role.Name,
			UserSubjects:  map[string]map[string]bool{},
			UserNames:     map[string]string{},
			AllSubjectIDs: map[string]bool{},
		}
		for _, u := range role.AssignedUsers {
			subs := map[string]bool{}
			for _, c := range u.CurriculumsAssigned {
				subs[c.ID] = true
				rd.AllSubjectIDs[c.ID] = true
			}
			for _, s := range u.SubjectsAssigned {
				subs[s.ID] = true
				rd.AllSubjectIDs[s.ID] = true
			}
			rd.UserSubjects[u.ID] = subs
			rd.UserNames[u.ID] = strings.TrimSpace(u.FirstName + " " + u.LastName)
		}
		roleMap[role.ID] = rd
	}

	var results []assignmentGap
	for _, rd := range roleMap {
		for userID, userSubs := range rd.UserSubjects {
			var missing []string
			for subID := range rd.AllSubjectIDs {
				if !userSubs[subID] {
					name := subjectNames[subID]
					if name == "" {
						name = subID
					}
					missing = append(missing, name)
				}
			}
			if len(missing) > 0 {
				results = append(results, assignmentGap{
					UserID:          userID,
					UserName:        rd.UserNames[userID],
					RoleName:        rd.Name,
					MissingSubjects: missing,
				})
			}
		}
	}
	return results, nil
}

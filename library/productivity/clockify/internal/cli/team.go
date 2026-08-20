// Copyright 2026 melanson633 and contributors. Licensed under Apache-2.0. See LICENSE.
// Transcendence feature: team submission tracker — diffs the workspace
// user list against the week's approval requests to surface non-submitters.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/store"
	"github.com/spf13/cobra"
)

func newTeamCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Team-wide timesheet and approval views",
		Long: `Commands for a team lead reviewing the whole workspace's weekly
timesheets from the local store.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTeamTimesheetsCmd(flags))
	return cmd
}

// rawApproval is the subset of a Clockify approval request team timesheets
// needs. Owner and date shapes are read defensively.
type rawApproval struct {
	ID    string `json:"id"`
	Owner struct {
		UserID   string `json:"userId"`
		UserName string `json:"userName"`
	} `json:"owner"`
	UserID    string `json:"userId"`
	DateRange struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"dateRange"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
}

func newTeamTimesheetsCmd(flags *rootFlags) *cobra.Command {
	var dateFlag, workspace, dbPath string

	cmd := &cobra.Command{
		Use:   "timesheets",
		Short: "Who has submitted their weekly timesheet — and who has not",
		Long: `Diff the full workspace user list against the week's synced approval
requests so the people who never submitted are visible, not invisible.
Each member's tracked hours for the week are shown alongside.

Reads the local store — run 'clockify-pp-cli sync' for fresh data.`,
		Example: `  # This week's team submission status
  clockify-pp-cli team timesheets

  # A specific week, as JSON
  clockify-pp-cli team timesheets --date 2026-05-11 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("clockify-pp-cli")
			}
			ref, err := parseDateFlag(dateFlag, time.Now())
			if err != nil {
				return usageErr(err)
			}
			ws := weekStart(ref)
			weekEnd := ws.AddDate(0, 0, 7)

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clockify-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Users.
			userRaws, _ := loadRaw(db, []string{"users"}, []string{"workspaces_users", "users", "workspaces-users"})
			type userInfo struct {
				ID, Name string
			}
			var users []userInfo
			userByID := map[string]string{}
			for _, r := range userRaws {
				var u struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Email string `json:"email"`
				}
				if json.Unmarshal(r, &u) != nil || u.ID == "" {
					continue
				}
				name := u.Name
				if name == "" {
					name = u.Email
				}
				if name == "" {
					name = u.ID
				}
				users = append(users, userInfo{u.ID, name})
				userByID[u.ID] = name
			}

			// Approval requests for this week.
			apRaws, _ := loadRaw(db, []string{"approval_requests"}, []string{"approval-requests", "approval_requests"})
			// With a non-admin key the approvals endpoint returns 403, so the
			// store has no approval rows — submission status is then unknown,
			// not "not submitted".
			approvalDataAvailable := len(apRaws) > 0
			stateByUser := map[string]string{}
			for _, r := range apRaws {
				var a rawApproval
				if json.Unmarshal(r, &a) != nil {
					continue
				}
				uid := a.Owner.UserID
				if uid == "" {
					uid = a.UserID
				}
				if uid == "" {
					continue
				}
				if a.DateRange.Start == "" {
					continue // no Start — can't place this approval in a week, exclude rather than credit every week queried
				}
				t, err := time.Parse(time.RFC3339, a.DateRange.Start)
				if err != nil {
					continue // unparseable Start — same exclusion
				}
				t = t.Local()
				if t.Before(ws) || !t.Before(weekEnd) {
					continue // a different week
				}
				stateByUser[uid] = strings.ToUpper(a.Status.State)
			}

			// Week hours per user.
			entries, err := ensureTimeEntries(db, flags, ws, weekEnd, workspace)
			if err != nil {
				return fmt.Errorf("loading time entries: %w", err)
			}
			hoursByUser := map[string]time.Duration{}
			for _, e := range entries {
				if e.Start.IsZero() || e.Start.Before(ws) || !e.Start.Before(weekEnd) {
					continue
				}
				if workspace != "" && e.WorkspaceID != workspace {
					continue
				}
				hoursByUser[e.UserID] += e.Duration
			}

			type member struct {
				UserID       string  `json:"user_id"`
				Name         string  `json:"name"`
				TrackedHours float64 `json:"tracked_hours"`
				Status       string  `json:"status"`
				Submitted    bool    `json:"submitted"`
			}
			var members []member
			submitted, missing := 0, 0
			for _, u := range users {
				var label string
				var isSub bool
				if approvalDataAvailable {
					label, isSub = submissionLabel(stateByUser[u.ID])
					if isSub {
						submitted++
					} else {
						missing++
					}
				} else {
					label = "unknown"
				}
				members = append(members, member{
					UserID: u.ID, Name: u.Name,
					TrackedHours: round2(hoursByUser[u.ID].Hours()),
					Status:       label, Submitted: isSub,
				})
			}
			sort.Slice(members, func(i, j int) bool {
				if members[i].Submitted != members[j].Submitted {
					return !members[i].Submitted // non-submitters first
				}
				return members[i].Name < members[j].Name
			})

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"week_start":              ws.Format("2006-01-02"),
					"members":                 members,
					"submitted_count":         submitted,
					"not_submitted_count":     missing,
					"approval_data_available": approvalDataAvailable,
				})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Team timesheets — week of %s\n\n", ws.Format("Mon Jan 2, 2006"))
			if len(members) == 0 {
				fmt.Fprintf(out, "No workspace users synced. (%s)\n", emptyStoreHint)
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "MEMBER\tTRACKED\tSTATUS")
			for _, m := range members {
				fmt.Fprintf(tw, "%s\t%.2fh\t%s\n", truncate(m.Name, 28), m.TrackedHours, m.Status)
			}
			tw.Flush()
			if approvalDataAvailable {
				fmt.Fprintf(out, "\n%d submitted, %d not submitted.\n", submitted, missing)
			} else {
				fmt.Fprintln(out, "\nSubmission status is unavailable — no approval-request data is accessible")
				fmt.Fprintln(out, "with this API key. Workspace approvals need an admin/manager key plus a sync.")
			}
			fmt.Fprintln(out, "Tracked hours reflect only time entries visible to your API key.")
			return nil
		},
	}

	cmd.Flags().StringVar(&dateFlag, "date", "", "Any date in the target week (YYYY-MM-DD, default: today)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter to one workspace id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// submissionLabel maps an approval-request state to a readable label and
// whether it counts as "submitted".
func submissionLabel(state string) (label string, submitted bool) {
	switch state {
	case "":
		return "NOT SUBMITTED", false
	case "PENDING", "SUBMITTED":
		return "submitted (pending)", true
	case "APPROVED":
		return "approved", true
	case "REJECTED":
		return "rejected — needs rework", false
	case "WITHDRAWN_SUBMISSION", "WITHDRAWN_APPROVAL", "WITHDRAWN":
		return "withdrawn", false
	default:
		return strings.ToLower(state), true
	}
}

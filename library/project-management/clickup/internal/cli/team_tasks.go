// Copyright 2026 rben-gaspar. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: team-tasks. Not press-generated.
// PATCH(amend-2026-05-24: add team-tasks read command wrapping GET /team/{team_id}/task)

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// teamTaskRow is the shaped output item emitted by team-tasks.
// All fields are always present regardless of --compact so agents
// get a stable schema without stripping.
type teamTaskRow struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Status      teamTaskStatus      `json:"status"`
	Priority    teamTaskPriority    `json:"priority"`
	DueDate     string              `json:"due_date"`
	DateUpdated string              `json:"date_updated"`
	DateCreated string              `json:"date_created"`
	URL         string              `json:"url"`
	Assignees   []teamTaskAssignee  `json:"assignees"`
	List        teamTaskListRef     `json:"list"`
	Folder      teamTaskFolderRef   `json:"folder"`
	Space       teamTaskSpaceRef    `json:"space"`
}

type teamTaskStatus struct {
	Status string `json:"status"`
	Type   string `json:"type"`
}

type teamTaskPriority struct {
	Priority   string `json:"priority"`
	OrderIndex string `json:"orderindex"`
}

type teamTaskAssignee struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type teamTaskListRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type teamTaskFolderRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type teamTaskSpaceRef struct {
	ID string `json:"id"`
}

func newTeamTasksCmd(flags *rootFlags) *cobra.Command {
	var teamID int
	var assigneeID int
	var includeClosed bool
	var subtasks bool

	cmd := &cobra.Command{
		Use:   "team-tasks",
		Short: "List tasks across a workspace (filtered by assignee)",
		Long: `Fetch tasks from a workspace via GET /v2/team/{team_id}/task.

Defaults to the token's first workspace when --team-id is not given. Always
includes subtasks (--subtasks=true by default) and excludes closed tasks
(--include-closed=false by default). The --assignee-id flag is required to
scope the result to one person.

Output shape is {"tasks":[...]} — each task exposes id, name, status
(status+type), priority (priority+orderindex), due_date, date_updated,
date_created, url, assignees (id+username), list (id+name), folder
(id+name), and space (id). These fields are never stripped under --compact.

Only live data is returned; --data-source local and auto-fallback are not
supported for this command because the underlying query is team-scoped and
not reproducible from the flat local store.`,
		Example: strings.Trim(`
  # All open tasks assigned to user 12345678 in the default workspace
  clickup-pp-cli team-tasks --assignee-id 12345678

  # Include closed tasks
  clickup-pp-cli team-tasks --assignee-id 12345678 --include-closed

  # Target a specific workspace
  clickup-pp-cli team-tasks --assignee-id 12345678 --team-id 9876543

  # Agent-friendly JSON
  clickup-pp-cli team-tasks --assignee-id 12345678 --agent

  # Skip subtasks
  clickup-pp-cli team-tasks --assignee-id 12345678 --subtasks=false
`, "\n"),
		Annotations: map[string]string{
			"pp:endpoint":  "task.get-filtered-team",
			"pp:method":    "GET",
			"pp:path":      "/v2/team/{team_Id}/task",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// team-tasks is live-only: the query is team-scoped and cannot be
			// reproduced from the flat local store.
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("team-tasks does not support --data-source local; use 'live' or 'auto'"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Resolve team ID: use --team-id when provided; otherwise call
			// GET /v2/team and pick the first workspace returned.
			resolvedTeamID := teamID
			if resolvedTeamID == 0 {
				raw, err := c.Get("/v2/team", nil)
				if err != nil {
					return fmt.Errorf("resolving workspace: %w\nhint: set CLICKUP_TOKEN and run 'clickup-pp-cli doctor'", err)
				}
				// Response shape: {"teams":[{"id":"12345","name":"..."},...]}
				var envelope struct {
					Teams []struct {
						ID string `json:"id"`
					} `json:"teams"`
				}
				if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Teams) == 0 {
					return fmt.Errorf("could not parse workspace list from /v2/team; use --team-id to specify explicitly")
				}
				n, err := strconv.Atoi(envelope.Teams[0].ID)
				if err != nil {
					return fmt.Errorf("workspace ID %q is not an integer; use --team-id to specify explicitly", envelope.Teams[0].ID)
				}
				resolvedTeamID = n
			}

			path := fmt.Sprintf("/v2/team/%d/task", resolvedTeamID)
			params := map[string]string{
				"archived":       "false",
				"subtasks":       strconv.FormatBool(subtasks),
				"include_closed": strconv.FormatBool(includeClosed),
			}
			if assigneeID != 0 {
				params["assignees[]"] = strconv.Itoa(assigneeID)
			}

			// Suppress data-source routing: we always call the live API for
			// this command. Use the raw paginated helper directly.
			raw, err := paginatedGet(c, path, params, nil, false, "page", "", "")
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// The ClickUp response envelope is {"tasks":[...]}. Unwrap the
			// array so we can shape each item.
			var envelope struct {
				Tasks []json.RawMessage `json:"tasks"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				// Try direct array (in case the API changes shape)
				var arr []json.RawMessage
				if err2 := json.Unmarshal(raw, &arr); err2 != nil {
					return fmt.Errorf("unexpected response shape from /v2/team/.../task: %w", err)
				}
				envelope.Tasks = arr
			}

			rows := make([]teamTaskRow, 0, len(envelope.Tasks))
			for _, raw := range envelope.Tasks {
				var t map[string]any
				if json.Unmarshal(raw, &t) != nil {
					continue
				}
				rows = append(rows, shapeTeamTask(t))
			}

			type output struct {
				Tasks []teamTaskRow `json:"tasks"`
			}
			result := output{Tasks: rows}
			_ = ctx // context used implicitly by paginatedGet → client

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				// Never run compactFields on the tasks slice — the spec requires
				// all fields to be present regardless of --compact. Emit the
				// {"tasks":[...]} envelope directly (no provenance wrapper) so
				// callers can rely on a stable top-level "tasks" key.
				inner, err := json.Marshal(result)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), inner, true)
			}

			// Human table: show a trimmed view for readability.
			out := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(out, "No tasks found.")
				return nil
			}
			fmt.Fprintf(out, "%d task(s) (workspace %d):\n\n", len(rows), resolvedTeamID)
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "ID\tNAME\tSTATUS\tPRIORITY\tDUE\tLIST")
			for _, r := range rows {
				due := r.DueDate
				if due == "" {
					due = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.ID,
					truncate(r.Name, 48),
					r.Status.Status,
					r.Priority.Priority,
					due,
					r.List.Name,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&assigneeID, "assignee-id", 0, "Filter tasks to this assignee's ClickUp user ID")
	cmd.Flags().IntVar(&teamID, "team-id", 0, "Workspace (team) ID; defaults to the token's first workspace")
	cmd.Flags().BoolVar(&includeClosed, "include-closed", false, "Include closed tasks (default: open only)")
	cmd.Flags().BoolVar(&subtasks, "subtasks", true, "Include subtasks (default: true)")

	return cmd
}

// shapeTeamTask converts a raw ClickUp task JSON map into the stable teamTaskRow
// output shape required by the spec. All fields are always populated (empty
// strings / zero values for absent data) so the output schema is invariant.
func shapeTeamTask(t map[string]any) teamTaskRow {
	str := func(key string) string {
		if v, ok := t[key]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	// status: {status: "in progress", type: "custom"}
	var status teamTaskStatus
	if s, ok := t["status"].(map[string]any); ok {
		if v, ok := s["status"].(string); ok {
			status.Status = v
		}
		if v, ok := s["type"].(string); ok {
			status.Type = v
		}
	}

	// priority: {priority: "high", orderindex: "1"}
	var priority teamTaskPriority
	if p, ok := t["priority"].(map[string]any); ok {
		if v, ok := p["priority"].(string); ok {
			priority.Priority = v
		}
		// orderindex may be a number or string
		if v, ok := p["orderindex"]; ok && v != nil {
			priority.OrderIndex = fmt.Sprintf("%v", v)
		}
	}

	// assignees: [{id: 12345, username: "alice", ...}, ...]
	var assignees []teamTaskAssignee
	if arr, ok := t["assignees"].([]any); ok {
		for _, a := range arr {
			if am, ok := a.(map[string]any); ok {
				var row teamTaskAssignee
				if id, ok := am["id"]; ok && id != nil {
					switch v := id.(type) {
					case float64:
						row.ID = int(v)
					case json.Number:
						if n, err := v.Int64(); err == nil {
							row.ID = int(n)
						}
					}
				}
				if u, ok := am["username"].(string); ok {
					row.Username = u
				}
				assignees = append(assignees, row)
			}
		}
	}
	if assignees == nil {
		assignees = []teamTaskAssignee{}
	}

	// list, folder: {id: "...", name: "..."}
	listRef := func(key string) (string, string) {
		if v, ok := t[key].(map[string]any); ok {
			id := ""
			name := ""
			if i, ok := v["id"]; ok && i != nil {
				id = fmt.Sprintf("%v", i)
			}
			if n, ok := v["name"].(string); ok {
				name = n
			}
			return id, name
		}
		return "", ""
	}

	listID, listName := listRef("list")
	folderID, folderName := listRef("folder")
	spaceID := ""
	if s, ok := t["space"].(map[string]any); ok {
		if i, ok := s["id"]; ok && i != nil {
			spaceID = fmt.Sprintf("%v", i)
		}
	}

	return teamTaskRow{
		ID:          str("id"),
		Name:        str("name"),
		Status:      status,
		Priority:    priority,
		DueDate:     str("due_date"),
		DateUpdated: str("date_updated"),
		DateCreated: str("date_created"),
		URL:         str("url"),
		Assignees:   assignees,
		List:        teamTaskListRef{ID: listID, Name: listName},
		Folder:      teamTaskFolderRef{ID: folderID, Name: folderName},
		Space:       teamTaskSpaceRef{ID: spaceID},
	}
}

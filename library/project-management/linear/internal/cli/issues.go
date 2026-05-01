package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// issueRow is the shared projection used by `issues list` and table rendering.
// It mirrors the shape the sync writes to the `data` JSON column.
type issueRow struct {
	ID         string  `json:"id"`
	Identifier string  `json:"identifier"`
	Title      string  `json:"title"`
	Priority   int     `json:"priority"`
	Estimate   float64 `json:"estimate,omitempty"`
	DueDate    string  `json:"dueDate,omitempty"`
	State      struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
	Team struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	} `json:"team"`
	Project *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project,omitempty"`
	Assignee *struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
	} `json:"assignee,omitempty"`
	UpdatedAt string `json:"updatedAt"`
	URL       string `json:"url,omitempty"`
}

func newIssuesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "issues <ID>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Get or list Linear issues",
		Long: `Get a single issue by identifier (e.g. ESP-1155), or list issues with filters.

Single-issue get resolution order (with --data-source auto, the default):
  1. local sqlite store, matched by identifier
  2. live Linear GraphQL query
  3. on live failure with a fresh store, return the store miss as not found

Use 'issues list' for filtered listing against the local sqlite store.`,
		Example: `  linear-pp-cli issues ESP-1155
  linear-pp-cli issues list
  linear-pp-cli issues list --assignee me
  linear-pp-cli issues list --assignee me --state started
  linear-pp-cli issues list --team ESP --state started --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runIssuesGet(cmd, flags, resolveDBPath(dbPath), args[0])
		},
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Database path")

	cmd.AddCommand(newIssuesListCmd(flags, &dbPath))
	return cmd
}

func resolveDBPath(override string) string {
	if override != "" {
		return override
	}
	return defaultDBPath("linear-pp-cli")
}

// openStoreAt opens the sqlite store at the given path. Returns (nil, nil) when
// the file does not exist — callers interpret this as "no sync yet" and decide
// whether to fall back to live.
func openStoreAt(dbPath string) (*store.Store, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	return store.Open(dbPath)
}

func newIssuesListCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	var (
		assignee  string
		stateFlag string
		team      string
		project   string
		limit     int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List issues from the local sqlite store with filters",
		Long: `List issues from the local sqlite store. Requires a prior 'linear-pp-cli sync'.

Filters compose with AND. --state is matched against state.type (not state.name)
so values like 'started', 'backlog', 'completed', 'canceled', 'triage' work across
teams that customize state names. Use --state all to include completed and canceled.

--assignee accepts 'me' (resolves the authenticated viewer via a live GraphQL query),
a user id, a user's display name, or a user's email.

--team and --project accept either a team/project key or a UUID.`,
		Example: `  linear-pp-cli issues list --assignee me
  linear-pp-cli issues list --assignee me --state started --json
  linear-pp-cli issues list --team ESP --state all --limit 500`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIssuesList(cmd, flags, resolveDBPath(*dbPath), assignee, stateFlag, team, project, limit)
		},
	}
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee (me, user id, display name, or email)")
	cmd.Flags().StringVar(&stateFlag, "state", "active", "Filter by state type: active (default), started, backlog, unstarted, completed, canceled, triage, all")
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key or ID")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project key or ID")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum results to return")
	return cmd
}

func runIssuesGet(cmd *cobra.Command, flags *rootFlags, dbPath, identifier string) error {
	db, openErr := openStoreAt(dbPath)
	if db != nil {
		defer db.Close()
	}

	// Local first when allowed by --data-source
	if flags.dataSource != "live" && db != nil {
		rows, err := db.ListIssues(map[string]string{"identifier": identifier}, 1)
		if err == nil && len(rows) > 0 {
			return renderIssue(cmd, flags, rows[0], DataProvenance{Source: "local", ResourceType: "issues"})
		}
	}

	if flags.dataSource == "local" {
		if openErr != nil {
			return notFoundErr(fmt.Errorf("issue %q not found in local store (and store unavailable: %v). Run 'linear-pp-cli sync' first", identifier, openErr))
		}
		return notFoundErr(fmt.Errorf("issue %q not found in local store. Run 'linear-pp-cli sync' first", identifier))
	}

	// Live GraphQL fetch
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, liveErr := fetchIssueLive(c, identifier)
	if liveErr == nil {
		return renderIssue(cmd, flags, data, DataProvenance{Source: "live", ResourceType: "issues"})
	}

	// Fall back to local if live failed (auto mode only — live mode bubbles the error up)
	if flags.dataSource != "live" && db != nil {
		rows, err := db.ListIssues(map[string]string{"identifier": identifier}, 1)
		if err == nil && len(rows) > 0 {
			fmt.Fprintf(os.Stderr, "live fetch failed, serving from local: %v\n", liveErr)
			return renderIssue(cmd, flags, rows[0], DataProvenance{Source: "local", ResourceType: "issues", Reason: "api_unreachable"})
		}
	}
	return classifyAPIError(liveErr)
}

// fetchIssueLive fetches a single issue by identifier via the Linear GraphQL API.
// Parses "ESP-1155" into team key "ESP" and number 1155, then filters. This avoids
// relying on Linear accepting the identifier string in the top-level issue(id:) arg,
// which behaves inconsistently across workspaces.
func fetchIssueLive(c *client.Client, identifier string) (json.RawMessage, error) {
	teamKey, number, ok := parseIssueIdentifier(identifier)
	if !ok {
		return nil, fmt.Errorf("invalid issue identifier %q (expected TEAM-NUMBER, e.g. ESP-1155)", identifier)
	}
	query := `query($teamKey: String!, $number: Float!) {
		issues(filter: { team: { key: { eq: $teamKey } }, number: { eq: $number } }, first: 1) {
			nodes {
				id identifier title description priority estimate dueDate url updatedAt createdAt
				state { name type }
				team { id key name }
				project { id name }
				assignee { id name displayName email }
			}
		}
	}`
	var resp struct {
		Issues struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"issues"`
	}
	if err := c.QueryInto(query, map[string]any{"teamKey": teamKey, "number": number}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Issues.Nodes) == 0 {
		return nil, notFoundErr(fmt.Errorf("issue %q not found", identifier))
	}
	return resp.Issues.Nodes[0], nil
}

func parseIssueIdentifier(identifier string) (string, float64, bool) {
	idx := strings.LastIndex(identifier, "-")
	if idx <= 0 || idx == len(identifier)-1 {
		return "", 0, false
	}
	teamKey := identifier[:idx]
	var number int
	if _, err := fmt.Sscanf(identifier[idx+1:], "%d", &number); err != nil || number <= 0 {
		return "", 0, false
	}
	return teamKey, float64(number), true
}

func runIssuesList(cmd *cobra.Command, flags *rootFlags, dbPath, assignee, stateFlag, team, project string, limit int) error {
	db, err := openStoreAt(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first", err)
	}
	if db == nil {
		return fmt.Errorf("no local data. Run 'linear-pp-cli sync' first")
	}
	defer db.Close()

	filter := map[string]string{}

	if assignee != "" {
		userID, err := resolveAssigneeFilter(flags, db, assignee)
		if err != nil {
			return err
		}
		filter["assignee_id"] = userID
	}

	if team != "" {
		teamID, err := resolveTeamFilter(db, team)
		if err != nil {
			return err
		}
		filter["team_id"] = teamID
	}

	if project != "" {
		projectID, err := resolveProjectFilter(db, project)
		if err != nil {
			return err
		}
		filter["project_id"] = projectID
	}

	raw, err := db.ListIssues(filter, limit)
	if err != nil {
		return err
	}

	rows := make([]issueRow, 0, len(raw))
	for _, r := range raw {
		var row issueRow
		if err := json.Unmarshal(r, &row); err != nil {
			continue
		}
		if !matchesStateFilter(row.State.Type, stateFlag) {
			continue
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Priority != rows[j].Priority {
			pi, pj := rows[i].Priority, rows[j].Priority
			if pi == 0 {
				pi = 99 // unprioritized sorts last
			}
			if pj == 0 {
				pj = 99
			}
			return pi < pj
		}
		return rows[i].Identifier < rows[j].Identifier
	})

	prov := localProvenance(db, "issues", "user_requested")
	printProvenance(cmd, len(rows), prov)

	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No issues found.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-4s %-16s %-10s %s\n", "ID", "PRI", "STATE", "TEAM", "TITLE")
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 80))
	for _, row := range rows {
		title := row.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-4s %-16s %-10s %s\n",
			row.Identifier, priorityLabel(row.Priority), row.State.Name, row.Team.Key, title)
	}
	return nil
}

func matchesStateFilter(stateType, stateFlag string) bool {
	switch strings.ToLower(strings.TrimSpace(stateFlag)) {
	case "all", "":
		return true
	case "active":
		return stateType != "completed" && stateType != "canceled"
	default:
		return strings.EqualFold(stateType, stateFlag)
	}
}

// resolveAssigneeFilter maps --assignee input to a user UUID.
// Accepts: "me" (queries viewer), a UUID, a display name, or an email.
func resolveAssigneeFilter(flags *rootFlags, db *store.Store, input string) (string, error) {
	if strings.EqualFold(input, "me") {
		c, err := flags.newClient()
		if err != nil {
			return "", err
		}
		var viewer struct {
			Viewer struct {
				ID string `json:"id"`
			} `json:"viewer"`
		}
		if err := c.QueryInto(`query { viewer { id } }`, nil, &viewer); err != nil {
			return "", fmt.Errorf("resolving --assignee me: %w\nhint: run 'linear-pp-cli doctor' to check auth status", err)
		}
		if viewer.Viewer.ID == "" {
			return "", fmt.Errorf("viewer id empty — is LINEAR_API_KEY set?")
		}
		return viewer.Viewer.ID, nil
	}
	if store.IsUUID(input) {
		return input, nil
	}
	users, err := db.ListUsers()
	if err != nil {
		return "", fmt.Errorf("listing users: %w", err)
	}
	for _, raw := range users {
		var u struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		}
		if err := json.Unmarshal(raw, &u); err != nil {
			continue
		}
		if strings.EqualFold(u.Email, input) || strings.EqualFold(u.DisplayName, input) || strings.EqualFold(u.Name, input) {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("no user matching %q in local store. Use 'me', a user id, display name, or email. Run 'linear-pp-cli sync' if the user was added recently", input)
}

// resolveTeamFilter maps --team input to a team UUID. Accepts key or UUID.
func resolveTeamFilter(db *store.Store, input string) (string, error) {
	if store.IsUUID(input) {
		return input, nil
	}
	teams, err := db.ListTeams()
	if err != nil {
		return "", fmt.Errorf("listing teams: %w", err)
	}
	for _, raw := range teams {
		var t struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		if strings.EqualFold(t.Key, input) || strings.EqualFold(t.Name, input) {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("no team matching %q in local store", input)
}

// resolveProjectFilter maps --project input to a project UUID. Accepts name or UUID.
func resolveProjectFilter(db *store.Store, input string) (string, error) {
	if store.IsUUID(input) {
		return input, nil
	}
	projects, err := db.ListProjects(nil)
	if err != nil {
		return "", fmt.Errorf("listing projects: %w", err)
	}
	for _, raw := range projects {
		var p struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slugId"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			continue
		}
		if strings.EqualFold(p.Name, input) || strings.EqualFold(p.Slug, input) {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("no project matching %q in local store", input)
}

func renderIssue(cmd *cobra.Command, flags *rootFlags, data json.RawMessage, prov DataProvenance) error {
	printProvenance(cmd, 1, prov)
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		if flags.compact {
			data = compactFields(data)
		}
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		}
		wrapped, err := wrapWithProvenance(data, prov)
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

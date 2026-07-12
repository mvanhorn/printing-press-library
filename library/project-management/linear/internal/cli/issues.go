package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

var errTeamFilterNotFound = errors.New("team not found")

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
		Use:   "issues [ID]",
		Short: "Get, list, or create Linear issues",
		Long: `Get a single issue by identifier (e.g. ESP-1155), or list issues with filters.

Single-issue get resolution order (with --data-source auto, the default):
  1. local sqlite store, matched by identifier
  2. live Linear GraphQL query
  3. on live failure with a fresh store, return the store miss as not found

Pass comma-separated identifiers to fetch several issues in one ordered response.
Use 'issues list' for filtered listing against the local sqlite store.
Use 'issues create --parent' or 'issues edit --parent/--no-parent' to manage
parent and sub-issue links.`,
		Example: `  linear-pp-cli issues ESP-1155
  linear-pp-cli issues ESP-1155,ESP-1156 --agent
  linear-pp-cli issues list
  linear-pp-cli issues list --assignee me
  linear-pp-cli issues list --assignee me --state started
  linear-pp-cli issues list --team ESP --state started --json
  linear-pp-cli issues create --title "child" --team ESP --parent ESP-1155 --agent
  linear-pp-cli issues edit ESP-1156 --no-parent --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Verify mode: short-circuit so identifier-shape probes
			// (TEAM-NUMBER) don't fail the mechanical verify pass.
			if cliutil.IsVerifyEnv() {
				return nil
			}
			commaList := strings.Contains(args[0], ",")
			identifiers, err := parseIssueIdentifiers(args[0])
			if err != nil {
				return err
			}
			if !commaList {
				return runIssuesGet(cmd, flags, resolveDBPath(dbPath), identifiers[0])
			}
			return runIssuesMultiGet(cmd, flags, resolveDBPath(dbPath), identifiers)
		},
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Database path")

	cmd.AddCommand(newIssuesListCmd(flags, &dbPath))
	cmd.AddCommand(newIssuesSearchCmd(flags, &dbPath))
	cmd.AddCommand(newIssuesCreateCmd(flags))
	cmd.AddCommand(newIssuesEditCmd(flags, &dbPath))
	return cmd
}

func parseIssueIdentifiers(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	identifiers := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		identifier := strings.TrimSpace(part)
		if identifier == "" {
			return nil, usageErr(fmt.Errorf("issue identifier list contains an empty value"))
		}
		key := strings.ToUpper(identifier)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		identifiers = append(identifiers, identifier)
	}
	return identifiers, nil
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
		Use:         "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List issues from the local sqlite store with filters",
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
	return classifyLiveReadError(liveErr, flags)
}

func runIssuesMultiGet(cmd *cobra.Command, flags *rootFlags, dbPath string, identifiers []string) error {
	db, openErr := openStoreAt(dbPath)
	if db != nil {
		defer db.Close()
	}

	if flags.dataSource != "live" && db != nil {
		rows, missing, err := loadIssuesFromStore(db, identifiers)
		if err != nil {
			return err
		}
		if missing == "" {
			return renderIssues(cmd, flags, rows, DataProvenance{Source: "local", ResourceType: "issues"})
		}
		if flags.dataSource == "local" {
			return notFoundErr(fmt.Errorf("issue %q not found in local store. Run 'linear-pp-cli sync' first", missing))
		}
	}

	if flags.dataSource == "local" {
		if openErr != nil {
			return notFoundErr(fmt.Errorf("one or more requested issues were not found in local store (and store unavailable: %v). Run 'linear-pp-cli sync' first", openErr))
		}
		return notFoundErr(fmt.Errorf("one or more requested issues were not found in local store. Run 'linear-pp-cli sync' first"))
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	rows := make([]json.RawMessage, 0, len(identifiers))
	for _, identifier := range identifiers {
		row, fetchErr := fetchIssueLive(c, identifier)
		if fetchErr != nil {
			return classifyLiveReadError(fetchErr, flags)
		}
		rows = append(rows, row)
	}
	return renderIssues(cmd, flags, rows, DataProvenance{Source: "live", ResourceType: "issues"})
}

func loadIssuesFromStore(db *store.Store, identifiers []string) ([]json.RawMessage, string, error) {
	rows := make([]json.RawMessage, 0, len(identifiers))
	for _, identifier := range identifiers {
		matches, err := db.ListIssues(map[string]string{"identifier": identifier}, 1)
		if err != nil {
			return nil, "", err
		}
		if len(matches) == 0 {
			return nil, identifier, nil
		}
		rows = append(rows, matches[0])
	}
	return rows, "", nil
}

// fetchIssueLive fetches a single issue by UUID or identifier via the Linear
// GraphQL API. For identifiers it parses "ESP-1155" into team key "ESP" and
// number 1155, then filters. This avoids relying on Linear accepting the
// identifier string in the top-level issue(id:) arg, which behaves
// inconsistently across workspaces.
func fetchIssueLive(c graphqlQueryer, identifier string) (json.RawMessage, error) {
	if store.IsUUID(identifier) {
		return fetchIssueByIDLive(c, identifier)
	}
	teamKey, number, ok := parseIssueIdentifier(identifier)
	if !ok {
		return nil, fmt.Errorf("invalid issue identifier %q (expected TEAM-NUMBER, e.g. ESP-1155)", identifier)
	}
	query := `query($teamKey: String!, $number: Float!) {
		issues(filter: { team: { key: { eq: $teamKey } }, number: { eq: $number } }, first: 1) {
			nodes {
				id identifier title description priority estimate dueDate url updatedAt createdAt
				state { id name type }
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

func fetchIssueByIDLive(c graphqlQueryer, id string) (json.RawMessage, error) {
	query := `query($id: String!) {
		issue(id: $id) {
			id identifier title description priority estimate dueDate url updatedAt createdAt
			state { id name type }
			team { id key name }
			project { id name }
			assignee { id name displayName email }
		}
	}`
	var resp struct {
		Issue json.RawMessage `json:"issue"`
	}
	if err := c.QueryInto(query, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Issue) == 0 || string(resp.Issue) == "null" {
		return nil, notFoundErr(fmt.Errorf("issue %q not found", id))
	}
	return resp.Issue, nil
}

func resolveIssueID(c graphqlQueryer, identifier string) (string, error) {
	if store.IsUUID(identifier) {
		return identifier, nil
	}
	teamKey, number, ok := parseIssueIdentifier(identifier)
	if !ok {
		return "", fmt.Errorf("invalid issue identifier %q (expected TEAM-NUMBER, e.g. ESP-1155)", identifier)
	}
	query := `query($teamKey: String!, $number: Float!) {
		issues(filter: { team: { key: { eq: $teamKey } }, number: { eq: $number } }, first: 1) {
			nodes { id }
		}
	}`
	var resp struct {
		Issues struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"issues"`
	}
	if err := c.QueryInto(query, map[string]any{"teamKey": teamKey, "number": number}, &resp); err != nil {
		return "", err
	}
	if len(resp.Issues.Nodes) == 0 || resp.Issues.Nodes[0].ID == "" {
		return "", notFoundErr(fmt.Errorf("issue %q not found", identifier))
	}
	return resp.Issues.Nodes[0].ID, nil
}

func resolveParentIssueID(c graphqlQueryer, parent string) (string, error) {
	parent, err := validateParentIssueRef(parent)
	if err != nil {
		return "", err
	}
	if store.IsUUID(parent) {
		return parent, nil
	}
	return resolveIssueID(c, parent)
}

func validateParentIssueRef(parent string) (string, error) {
	parent = strings.TrimSpace(parent)
	if parent == "" {
		return "", usageErr(fmt.Errorf("--parent requires an issue identifier (TEAM-NUMBER) or issue UUID"))
	}
	if store.IsUUID(parent) {
		return parent, nil
	}
	if _, _, ok := parseIssueIdentifier(parent); !ok {
		return "", usageErr(fmt.Errorf("--parent expects an issue identifier (TEAM-NUMBER, e.g. MOB-123) or issue UUID; got %q", parent))
	}
	return parent, nil
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

	// Key→UUID resolution always goes through the local store. These
	// reference tables (teams, projects, users) are small, change rarely,
	// and resolving via API would burn complexity budget on every list
	// invocation. The user must `sync` at least once before filter keys
	// resolve; the live-first branch below still calls the API for the
	// actual issue collection.
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

	// Honor --data-source for the actual issue fetch. `auto` (default)
	// tries live-first per the framework's resolveRead pattern; `local`
	// pins to the store (budget-conscious); `live` errors instead of
	// falling back. Without this, the v3-ported command silently ignored
	// the flag and always read local — see the data-source-reasoning
	// retro candidate for the full story.
	var raw []json.RawMessage
	useLive := flags.dataSource != "local"
	servedFromLive := false
	fellBackOnNetErr := false
	if useLive {
		raw, err = fetchIssuesLive(cmd.Context(), flags, db, filter, stateFlag, limit)
		if err != nil {
			if flags.dataSource == "live" {
				return err
			}
			// auto: fall back to local on network error only — propagate
			// 4xx/5xx so auth/permission failures don't silently use stale
			// data.
			if !isNetworkError(err) {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "  (live API unreachable — falling back to local store)")
			raw = nil
			fellBackOnNetErr = true
		} else {
			servedFromLive = true
		}
	}
	if raw == nil {
		raw, err = db.ListIssues(filter, limit)
		if err != nil {
			return err
		}
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

	// Provenance must reflect where `rows` actually came from, not where
	// the default code path would read. When fetchIssuesLive succeeded
	// (servedFromLive), the data is from Linear's GraphQL API and was
	// write-through'd into the store; the source is "live". When the
	// store served the read (default --data-source local, or auto's
	// network-error fallback), it's "local" with the store's sync
	// timestamp. The stale-hint only makes sense for store reads — a
	// fresh live response can't be stale by definition.
	var prov DataProvenance
	switch {
	case servedFromLive:
		reason := "user_requested"
		prov = DataProvenance{Source: "live", ResourceType: "issues", Reason: reason}
	case fellBackOnNetErr:
		prov = localProvenance(db, "issues", "api_unreachable")
	default:
		prov = localProvenance(db, "issues", "user_requested")
	}
	prov = attachFreshness(prov, flags)
	printProvenance(cmd, len(rows), prov)

	if !servedFromLive {
		if len(rows) == 0 {
			hintIfUnsynced(cmd, db, "issues")
		} else {
			hintIfStale(cmd, db, "issues", flags.maxAge)
		}
	}

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
	return "", fmt.Errorf("%w: no team matching %q in local store", errTeamFilterNotFound, input)
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
	return renderPayloadWithProvenance(cmd, flags, data, prov, true)
}

func renderIssues(cmd *cobra.Command, flags *rootFlags, issues []json.RawMessage, prov DataProvenance) error {
	data, err := json.Marshal(issues)
	if err != nil {
		return fmt.Errorf("marshaling issues: %w", err)
	}
	printProvenance(cmd, len(issues), prov)
	return renderPayloadWithProvenance(cmd, flags, data, prov, true)
}

// fetchIssuesLive queries Linear's `issues(filter:...)` GraphQL endpoint
// using the filter map produced by runIssuesList. On success, it
// write-throughs the response into the local store via UpsertIssue so the
// next --data-source local read sees fresh data. The cliutil/store imports
// already in this file's import block cover the helpers used here.
func fetchIssuesLive(ctx context.Context, flags *rootFlags, db *store.Store, filter map[string]string, stateFlag string, limit int) ([]json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	gqlFilter := map[string]any{}
	if v, ok := filter["assignee_id"]; ok && v != "" {
		gqlFilter["assignee"] = map[string]any{"id": map[string]any{"eq": v}}
	}
	if v, ok := filter["team_id"]; ok && v != "" {
		gqlFilter["team"] = map[string]any{"id": map[string]any{"eq": v}}
	}
	if v, ok := filter["project_id"]; ok && v != "" {
		gqlFilter["project"] = map[string]any{"id": map[string]any{"eq": v}}
	}
	switch stateFlag {
	case "", "all":
		// no filter — return everything
	case "active":
		// "active" is the v3 semantic: not completed AND not canceled. Linear's
		// state.type enum is {backlog, unstarted, started, completed, canceled,
		// triage}; the live filter uses nin.
		gqlFilter["state"] = map[string]any{"type": map[string]any{"nin": []string{"completed", "canceled"}}}
	default:
		gqlFilter["state"] = map[string]any{"type": map[string]any{"eq": stateFlag}}
	}
	// Linear's GraphQL `issues` query caps `first` at 100 per page. To
	// honor a user-supplied --limit greater than 100, paginate via
	// pageInfo.endCursor until we have enough rows or there's no next
	// page. This keeps live and local result sets consistent for the same
	// --limit (the local path's db.ListIssues handles arbitrary limits
	// against the snapshot). When --limit is 0 or negative, the user
	// asked for "everything" — paginate until pageInfo.hasNextPage flips.
	want := limit
	all := want <= 0
	const pageMax = 100
	collected := make([]json.RawMessage, 0)
	cursor := ""
	for {
		first := pageMax
		if !all {
			remaining := want - len(collected)
			if remaining <= 0 {
				break
			}
			if remaining < pageMax {
				first = remaining
			}
		}
		vars := map[string]any{"first": first, "filter": gqlFilter}
		if cursor != "" {
			vars["after"] = cursor
		}
		var resp struct {
			Issues struct {
				Nodes    []json.RawMessage `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"issues"`
		}
		if err := c.QueryInto(client.IssuesQuery, vars, &resp); err != nil {
			return nil, err
		}
		// Write-through: upsert each fetched issue into the local store so
		// follow-up `--data-source local` reads in the same session are fresh.
		for _, n := range resp.Issues.Nodes {
			var meta struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
				Title      string `json:"title"`
			}
			if err := json.Unmarshal(n, &meta); err != nil || meta.ID == "" {
				continue
			}
			_ = db.UpsertIssue(meta.ID, meta.Identifier, meta.Title, n)
		}
		collected = append(collected, resp.Issues.Nodes...)
		if !resp.Issues.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Issues.PageInfo.EndCursor
		if cursor == "" {
			break
		}
		if !all && len(collected) >= want {
			break
		}
	}
	if !all && len(collected) > want {
		collected = collected[:want]
	}
	return collected, nil
}

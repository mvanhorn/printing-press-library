// Copyright 2026 carlf01. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel commands for the authentik CLI.
// These implement transcendence features that require aggregating across
// multiple API endpoints or performing local cross-table joins.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/auth/authentik/internal/store"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// health — aggregate system status snapshot
// ---------------------------------------------------------------------------

func newHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "health",
		Short:       "Operator health snapshot: system, tasks, workers, and version in one call",
		Long:        `Joins admin/system + tasks + workers + version into a single agent-readable summary. No single authentik endpoint returns this; health aggregates locally from the API.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Full health snapshot
  authentik-pp-cli health

  # JSON for agents
  authentik-pp-cli health --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Fetch system info
			systemData, _, sysErr := resolveRead(ctx, c, flags, "admin", false, "/admin/system/", nil, nil, cmd.ErrOrStderr())
			// Fetch version
			versionData, _, verErr := resolveRead(ctx, c, flags, "admin", false, "/admin/version/", nil, nil, cmd.ErrOrStderr())
			// Fetch tasks
			tasksData, _, taskErr := resolveRead(ctx, c, flags, "tasks", true, "/admin/system/tasks/", nil, nil, cmd.ErrOrStderr())
			// Fetch workers
			workersData, _, workerErr := resolveRead(ctx, c, flags, "tasks", true, "/admin/workers/", nil, nil, cmd.ErrOrStderr())

			snapshot := map[string]any{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}

			if sysErr == nil {
				var sys map[string]any
				if json.Unmarshal(systemData, &sys) == nil {
					snapshot["system"] = sys
				}
			} else {
				snapshot["system_error"] = sysErr.Error()
			}

			if verErr == nil {
				var ver map[string]any
				if json.Unmarshal(versionData, &ver) == nil {
					snapshot["version"] = ver
				}
			} else {
				snapshot["version_error"] = verErr.Error()
			}

			if taskErr == nil {
				var tasks []any
				if json.Unmarshal(tasksData, &tasks) == nil {
					snapshot["tasks"] = tasks
					snapshot["task_count"] = len(tasks)
				} else {
					var taskObj map[string]any
					if json.Unmarshal(tasksData, &taskObj) == nil {
						snapshot["tasks"] = taskObj
					}
				}
			} else {
				snapshot["tasks_error"] = taskErr.Error()
			}

			if workerErr == nil {
				var workers []any
				if json.Unmarshal(workersData, &workers) == nil {
					snapshot["workers"] = workers
					snapshot["worker_count"] = len(workers)
				} else {
					var workerObj map[string]any
					if json.Unmarshal(workersData, &workerObj) == nil {
						snapshot["workers"] = workerObj
					}
				}
			} else {
				snapshot["workers_error"] = workerErr.Error()
			}

			out, err := json.MarshalIndent(snapshot, "", "  ")
			if err != nil {
				return err
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return err
			}

			// Human-readable summary
			fmt.Fprintln(cmd.OutOrStdout(), "=== Authentik Health ===")
			if ver, ok := snapshot["version"].(map[string]any); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Version: %v\n", ver["version_current"])
				if latest, ok := ver["version_latest"].(string); ok && latest != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Latest:  %v\n", latest)
				}
			}
			if sys, ok := snapshot["system"].(map[string]any); ok {
				if env, ok := sys["env"].(map[string]any); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "Environment: %v\n", env)
				}
			}
			if wc, ok := snapshot["worker_count"]; ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Workers: %v\n", wc)
			}
			if tc, ok := snapshot["task_count"]; ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Tasks:   %v\n", tc)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Snapshot: %v\n", snapshot["timestamp"])
			fmt.Fprintln(cmd.OutOrStdout(), "\nRun with --json for full machine-readable output.")
			return nil
		},
	}
	return cmd
}

// ---------------------------------------------------------------------------
// tokens — parent command with novel subcommands
// ---------------------------------------------------------------------------

func newTokensCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Manage and audit API tokens",
		Long:  "Token management and security auditing commands. Requires data to be synced first with 'sync' or 'workflow archive'.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTokensStaleCmd(flags))
	return cmd
}

func newTokensStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Find API tokens whose owners have not used them in N days",
		Long:        `Queries the local SQLite store to find API tokens with a last_used date older than --days. Requires data synced first with 'authentik-pp-cli sync' or 'authentik-pp-cli workflow archive'.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Tokens unused for 90+ days
  authentik-pp-cli tokens stale --days 90

  # JSON output for scripting
  authentik-pp-cli tokens stale --days 90 --json

  # Conservative threshold for service account hygiene
  authentik-pp-cli tokens stale --days 180 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("authentik-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'authentik-pp-cli sync' first.", err)
			}
			defer db.Close()

			cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02T15:04:05")

			// Tokens are stored in the core table with intent = "api"
			// last_used tracks the last time the token was authenticated with
			rows, err := db.Query(`
				SELECT id, data
				FROM core
				WHERE (intent = 'api' OR intent IS NULL)
				  AND (last_used IS NULL OR last_used < ?)
				ORDER BY last_used ASC
			`, cutoff)
			if err != nil {
				return fmt.Errorf("querying tokens: %w", err)
			}
			defer rows.Close()

			type TokenSummary struct {
				ID           string `json:"id"`
				Identifier   string `json:"identifier"`
				User         any    `json:"user,omitempty"`
				Intent       string `json:"intent,omitempty"`
				LastUsed     string `json:"last_used,omitempty"`
				Expires      string `json:"expires,omitempty"`
				DaysSinceUse int    `json:"days_since_use,omitempty"`
			}

			var stale []TokenSummary
			for rows.Next() {
				var id string
				var rawData []byte
				if err := rows.Scan(&id, &rawData); err != nil {
					continue
				}
				var d map[string]any
				if err := json.Unmarshal(rawData, &d); err != nil {
					continue
				}
				ts := TokenSummary{ID: id}
				if v, ok := d["identifier"].(string); ok {
					ts.Identifier = v
				}
				if v, ok := d["intent"].(string); ok {
					ts.Intent = v
				}
				if v, ok := d["user"].(float64); ok {
					ts.User = int(v)
				} else if v, ok := d["user_obj"]; ok {
					ts.User = v
				}
				if v, ok := d["last_used"].(string); ok {
					ts.LastUsed = v
					if t, err := time.Parse(time.RFC3339, v); err == nil {
						ts.DaysSinceUse = int(time.Since(t).Hours() / 24)
					}
				} else {
					ts.DaysSinceUse = -1 // never used
				}
				if v, ok := d["expires"].(string); ok {
					ts.Expires = v
				}
				stale = append(stale, ts)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if len(stale) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), `{"stale_tokens":[],"count":0,"threshold_days":`+fmt.Sprint(days)+`}`)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No stale tokens found (threshold: %d days).\n", days)
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				result := map[string]any{
					"stale_tokens":   stale,
					"count":          len(stale),
					"threshold_days": days,
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Stale tokens (unused > %d days): %d\n\n", days, len(stale))
			fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-30s  %-10s  %s\n", "ID", "Identifier", "DaysSince", "LastUsed")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 90))
			for _, t := range stale {
				daysStr := fmt.Sprint(t.DaysSinceUse)
				if t.DaysSinceUse < 0 {
					daysStr = "never"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-30s  %-10s  %s\n", t.ID, t.Identifier, daysStr, t.LastUsed)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 90, "Threshold in days since last use")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/authentik-pp-cli/data.db)")
	return cmd
}

// ---------------------------------------------------------------------------
// apps — parent command with novel subcommands
// ---------------------------------------------------------------------------

func newAppsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Application management and security auditing",
		Long:  "Application auditing commands. Requires data to be synced first with 'sync' or 'workflow archive'.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAppsUnusedCmd(flags))
	return cmd
}

func newAppsUnusedCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "unused",
		Short:       "List applications with no successful login in N days",
		Long:        `Queries the local SQLite store to find applications that have no login events in the events table within the last --days. Requires data synced first with 'authentik-pp-cli sync' or 'authentik-pp-cli workflow archive'.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Apps with no logins in 30 days
  authentik-pp-cli apps unused --days 30

  # JSON output for review
  authentik-pp-cli apps unused --days 30 --json

  # Conservative threshold for cleanup candidates
  authentik-pp-cli apps unused --days 60 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("authentik-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'authentik-pp-cli sync' first.", err)
			}
			defer db.Close()

			cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02T15:04:05")

			// Get all application slugs from core table (model_name = application or has slug+launch_url)
			appRows, err := db.Query(`
				SELECT id, data, slug, name
				FROM core
				WHERE slug IS NOT NULL
				  AND (model_name = 'application' OR launch_url IS NOT NULL OR meta_launch_url IS NOT NULL)
			`)
			if err != nil {
				return fmt.Errorf("querying applications: %w", err)
			}
			defer appRows.Close()

			type AppInfo struct {
				ID   string
				Slug string
				Name string
			}
			var apps []AppInfo
			for appRows.Next() {
				var id, slug, name string
				var rawData []byte
				if err := appRows.Scan(&id, &rawData, &slug, &name); err != nil {
					continue
				}
				apps = append(apps, AppInfo{ID: id, Slug: slug, Name: name})
			}
			appRows.Close()

			if len(apps) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), `{"unused_apps":[],"count":0,"threshold_days":`+fmt.Sprint(days)+`}`)
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No applications found in local store. Run 'authentik-pp-cli sync' first.")
				return nil
			}

			// For each app slug, check if there are any login events within the window
			// Events table has action = "login" and context->app
			type AppSummary struct {
				ID             string `json:"id"`
				Slug           string `json:"slug"`
				Name           string `json:"name"`
				LastLoginEvent string `json:"last_login_event,omitempty"`
				DaysSinceLogin int    `json:"days_since_login,omitempty"`
			}

			var unused []AppSummary
			for _, app := range apps {
				var lastLogin string
				var loginTime time.Time

				// Check events table for this app's login events
				evtRow := db.DB().QueryRow(`
					SELECT MAX(created) FROM events
					WHERE action LIKE '%login%'
					  AND (app = ? OR data LIKE ?)
					LIMIT 1
				`, app.Slug, `%"`+app.Slug+`"%`)
				_ = evtRow.Scan(&lastLogin)

				if lastLogin != "" {
					loginTime, _ = time.Parse(time.RFC3339, lastLogin)
				}

				// Include if last login was before cutoff OR no login event found
				if lastLogin == "" || lastLogin < cutoff {
					summary := AppSummary{
						ID:             app.ID,
						Slug:           app.Slug,
						Name:           app.Name,
						LastLoginEvent: lastLogin,
					}
					if !loginTime.IsZero() {
						summary.DaysSinceLogin = int(time.Since(loginTime).Hours() / 24)
					} else {
						summary.DaysSinceLogin = -1 // no login found
					}
					unused = append(unused, summary)
				}
			}

			// Sort by days since login descending
			sort.Slice(unused, func(i, j int) bool {
				return unused[i].DaysSinceLogin > unused[j].DaysSinceLogin
			})

			if len(unused) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), `{"unused_apps":[],"count":0,"threshold_days":`+fmt.Sprint(days)+`}`)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No unused apps found (threshold: %d days).\n", days)
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				result := map[string]any{
					"unused_apps":    unused,
					"count":          len(unused),
					"threshold_days": days,
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Unused apps (no login > %d days): %d\n\n", days, len(unused))
			fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-30s  %-12s  %s\n", "ID", "Name", "DaysSince", "LastLogin")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 90))
			for _, a := range unused {
				daysStr := fmt.Sprint(a.DaysSinceLogin)
				if a.DaysSinceLogin < 0 {
					daysStr = "never"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-30s  %-12s  %s\n", a.ID, a.Name, daysStr, a.LastLoginEvent)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 30, "Threshold in days since last login event")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/authentik-pp-cli/data.db)")
	return cmd
}

// ---------------------------------------------------------------------------
// users — parent command with novel subcommands
// ---------------------------------------------------------------------------

func newUsersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "User management and security auditing",
		Long:  "User auditing commands. Requires data to be synced first with 'sync' or 'workflow archive'.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newUsersGroupsCmd(flags))
	return cmd
}

func newUsersGroupsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "groups [username]",
		Short:       "Recursively expand a user's group memberships including inherited roles",
		Long:        `Walks the local SQLite store to recursively expand all group memberships for a user, including nested parent groups. The authentik API returns direct groups only; this command walks parents locally. Requires data synced first.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Show all groups for a user
  authentik-pp-cli users groups alice

  # JSON for agents
  authentik-pp-cli users groups alice --json

  # Pipe to jq for just group names
  authentik-pp-cli users groups alice --json | jq '[.groups[].name]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			username := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("authentik-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'authentik-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Find the user by username
			userRow := db.DB().QueryRow(`
				SELECT id, data, num_pk FROM core
				WHERE username = ?
				  AND is_superuser IS NOT NULL
				LIMIT 1
			`, username)
			var userID string
			var userPK int
			var rawUserData []byte
			if err := userRow.Scan(&userID, &rawUserData, &userPK); err != nil {
				// Try alternative: search by email or username in data JSON
				userRow2 := db.DB().QueryRow(`
					SELECT id, data, num_pk FROM core
					WHERE username = ? OR email = ?
					LIMIT 1
				`, username, username)
				if err2 := userRow2.Scan(&userID, &rawUserData, &userPK); err2 != nil {
					return fmt.Errorf("user %q not found in local store. Run 'authentik-pp-cli sync' first.", username)
				}
			}

			// Find direct groups for this user
			// In authentik, groups are stored in the core table with parent references
			// Users belong to groups which are also in core; the relationship is via
			// the data JSON field "users_obj" or via the group's "users" list
			type GroupInfo struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Parent string `json:"parent,omitempty"`
				Depth  int    `json:"depth"`
			}

			// Collect all groups from the store
			groupRows, err := db.Query(`
				SELECT id, data, name, parent, parent_name FROM core
				WHERE is_superuser IS NULL
				  AND username IS NULL
				  AND name IS NOT NULL
			`)
			if err != nil {
				return fmt.Errorf("querying groups: %w", err)
			}
			defer groupRows.Close()

			type RawGroup struct {
				ID         string
				Name       string
				Parent     string
				ParentName string
				Data       map[string]any
			}
			allGroups := map[string]RawGroup{}
			for groupRows.Next() {
				var id, name string
				var parent, parentName *string
				var rawData []byte
				if err := groupRows.Scan(&id, &rawData, &name, &parent, &parentName); err != nil {
					continue
				}
				g := RawGroup{ID: id, Name: name}
				if parent != nil {
					g.Parent = *parent
				}
				if parentName != nil {
					g.ParentName = *parentName
				}
				if err := json.Unmarshal(rawData, &g.Data); err == nil {
					// Check if this group has our user in its users list
					allGroups[id] = g
				}
			}
			groupRows.Close()

			// Find groups this user directly belongs to by checking group data
			var directGroupIDs []string
			for gid, g := range allGroups {
				if users, ok := g.Data["users"].([]any); ok {
					for _, u := range users {
						switch v := u.(type) {
						case float64:
							if int(v) == userPK {
								directGroupIDs = append(directGroupIDs, gid)
							}
						case string:
							if v == userID {
								directGroupIDs = append(directGroupIDs, gid)
							}
						}
					}
				}
			}

			// Recursively expand group hierarchy
			visited := map[string]bool{}
			var result []GroupInfo

			var walk func(groupID string, depth int)
			walk = func(groupID string, depth int) {
				if visited[groupID] {
					return
				}
				visited[groupID] = true
				g, ok := allGroups[groupID]
				if !ok {
					return
				}
				result = append(result, GroupInfo{
					ID:     g.ID,
					Name:   g.Name,
					Parent: g.ParentName,
					Depth:  depth,
				})
				if g.Parent != "" {
					walk(g.Parent, depth+1)
				}
			}

			for _, gid := range directGroupIDs {
				walk(gid, 0)
			}

			// Sort by depth then name
			sort.Slice(result, func(i, j int) bool {
				if result[i].Depth != result[j].Depth {
					return result[i].Depth < result[j].Depth
				}
				return result[i].Name < result[j].Name
			})

			var userData map[string]any
			_ = json.Unmarshal(rawUserData, &userData)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(map[string]any{
					"user":   username,
					"groups": result,
					"count":  len(result),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			if len(result) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "User %q belongs to no groups (in local store).\n", username)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Groups for user %q (%d total, including inherited):\n\n", username, len(result))
			for _, g := range result {
				indent := strings.Repeat("  ", g.Depth)
				if g.Depth > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "%s↳ %s (inherited via parent)\n", indent, g.Name)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s• %s\n", indent, g.Name)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/authentik-pp-cli/data.db)")
	return cmd
}

// ---------------------------------------------------------------------------
// flows map — subcommand added to the existing flows group
// ---------------------------------------------------------------------------

func newFlowsMapCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "map [flow-slug]",
		Short:       "Render a flow with its ordered stage bindings as a tree",
		Long:        `Joins flow_stage_bindings and stages tables from the local SQLite store to render a flow and its ordered stage bindings. Requires data synced first with 'authentik-pp-cli sync' or 'authentik-pp-cli workflow archive'.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Map the default authentication flow
  authentik-pp-cli flows map default-authentication-flow

  # JSON output for agent processing
  authentik-pp-cli flows map default-authentication-flow --json

  # All flows with their stage counts
  authentik-pp-cli flows map --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("authentik-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'authentik-pp-cli sync' first.", err)
			}
			defer db.Close()

			if len(args) == 0 {
				// List all flows with stage counts
				flowRows, err := db.Query(`
					SELECT id, slug, name, designation, title FROM flows
					WHERE slug IS NOT NULL
					ORDER BY name
				`)
				if err != nil {
					return fmt.Errorf("querying flows: %w", err)
				}
				defer flowRows.Close()

				type FlowBrief struct {
					ID          string `json:"id"`
					Slug        string `json:"slug"`
					Name        string `json:"name"`
					Title       string `json:"title,omitempty"`
					Designation string `json:"designation,omitempty"`
				}
				var flows []FlowBrief
				for flowRows.Next() {
					var id, slug, name string
					var design, title *string
					if err := flowRows.Scan(&id, &slug, &name, &design, &title); err != nil {
						continue
					}
					f := FlowBrief{ID: id, Slug: slug, Name: name}
					if design != nil {
						f.Designation = *design
					}
					if title != nil {
						f.Title = *title
					}
					flows = append(flows, f)
				}

				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					out, _ := json.MarshalIndent(map[string]any{"flows": flows, "count": len(flows)}, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Available flows (%d):\n\n", len(flows))
				for _, f := range flows {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  (%s)\n", f.Slug, f.Name)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nRun 'authentik-pp-cli flows map <slug>' to render a specific flow's stages.")
				return nil
			}

			slug := args[0]

			// Find the flow
			flowRow := db.DB().QueryRow(`
				SELECT id, slug, name, title, designation, data FROM flows
				WHERE slug = ?
				LIMIT 1
			`, slug)
			var flowID, flowSlug, flowName string
			var flowTitle, flowDesig *string
			var rawFlowData []byte
			if err := flowRow.Scan(&flowID, &flowSlug, &flowName, &flowTitle, &flowDesig, &rawFlowData); err != nil {
				return fmt.Errorf("flow %q not found in local store. Run 'authentik-pp-cli sync' first.", slug)
			}

			// Find stage bindings for this flow
			// Bindings are in flows table with target = flow pk and stage info
			bindingRows, err := db.Query(`
				SELECT id, data, "order", stage, stage_obj, evaluate_on_plan, re_evaluate_policies, policy_engine_mode
				FROM flows
				WHERE target = ? OR (data LIKE ?)
				  AND stage IS NOT NULL
				ORDER BY "order" ASC
			`, flowID, `%"`+flowID+`"%`)
			if err != nil {
				return fmt.Errorf("querying stage bindings: %w", err)
			}
			defer bindingRows.Close()

			type StageBinding struct {
				ID                 string `json:"id"`
				Order              int    `json:"order"`
				Stage              string `json:"stage"`
				StageName          string `json:"stage_name,omitempty"`
				EvaluateOnPlan     bool   `json:"evaluate_on_plan"`
				ReEvaluatePolicies bool   `json:"re_evaluate_policies"`
				PolicyEngineMode   string `json:"policy_engine_mode,omitempty"`
			}

			var bindings []StageBinding
			for bindingRows.Next() {
				var id string
				var order int
				var stage string
				var stageObj *string
				var rawData []byte
				var evalOnPlan, reEval *bool
				var policyMode *string
				if err := bindingRows.Scan(&id, &rawData, &order, &stage, &stageObj, &evalOnPlan, &reEval, &policyMode); err != nil {
					continue
				}
				b := StageBinding{ID: id, Order: order, Stage: stage}
				if stageObj != nil {
					// Try to parse stage_obj for name
					var so map[string]any
					if json.Unmarshal([]byte(*stageObj), &so) == nil {
						if name, ok := so["name"].(string); ok {
							b.StageName = name
						}
					}
				}
				if evalOnPlan != nil {
					b.EvaluateOnPlan = *evalOnPlan
				}
				if reEval != nil {
					b.ReEvaluatePolicies = *reEval
				}
				if policyMode != nil {
					b.PolicyEngineMode = *policyMode
				}
				bindings = append(bindings, b)
			}

			title := flowName
			if flowTitle != nil && *flowTitle != "" {
				title = *flowTitle
			}
			desig := ""
			if flowDesig != nil {
				desig = *flowDesig
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(map[string]any{
					"flow": map[string]any{
						"id":          flowID,
						"slug":        flowSlug,
						"name":        flowName,
						"title":       title,
						"designation": desig,
					},
					"stages":      bindings,
					"stage_count": len(bindings),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			// Human-readable tree
			fmt.Fprintf(cmd.OutOrStdout(), "Flow: %s (%s)\n", title, flowSlug)
			if desig != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Designation: %s\n", desig)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nStages (%d):\n", len(bindings))
			if len(bindings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  (no stage bindings found in local store)")
				fmt.Fprintln(cmd.OutOrStdout(), "\nTip: Run 'authentik-pp-cli sync' to refresh data, then try again.")
			} else {
				for _, b := range bindings {
					stageName := b.StageName
					if stageName == "" {
						stageName = b.Stage
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %2d. %s\n", b.Order, stageName)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/authentik-pp-cli/data.db)")
	return cmd
}

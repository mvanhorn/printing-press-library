package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/config"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/groups"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// resourceTypeStateGroups labels the provenance envelope of the groups
// commands. The answer comes from disk, never from the API.
const resourceTypeStateGroups = "state_groups"

// resourceTypeWorkflowStates is the store resource `groups check` reads to
// show which concrete states a group actually hits.
const resourceTypeWorkflowStates = "workflow_states"

// stateGroupFlagUsage is the shared help text for every --state flag that
// takes a group token. Kept in one place so the nine former hand-written
// predicates cannot drift apart again.
const stateGroupFlagUsage = "State group or type to include: a group name from 'groups list' (active, all, ...), " +
	"a raw type (" + validLinearWorkflowStateTypeList + "), type:<type> to bypass a shadowing group, or name:<state name>"

// completedGroupFlagUsage is the help text for the completed-side group used
// by burndown-style arithmetic, where "done" means "counted as delivered".
const completedGroupFlagUsage = "State group or type that counts as delivered for this calculation (see 'groups list')"

// loadStateGroups resolves the group registry for this invocation and returns
// the config path it was anchored to. Loading is lazy and memoized, so a
// malformed groups.toml never breaks commands that take no state filter.
func loadStateGroups(flags *rootFlags) (*groups.Registry, string, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, "", configErr(err)
	}
	reg, err := groups.Load(cfg.Path)
	if err != nil {
		return nil, cfg.Path, configErr(err)
	}
	return reg, cfg.Path, nil
}

// resolveStateSet turns a --state token into the single predicate every state
// filter in this CLI is allowed to use. A broken file is a config problem the
// user fixes once (exit 10). A bad token is a usage problem on this
// invocation (exit 2).
func resolveStateSet(flags *rootFlags, teamKey, token string) (groups.Set, error) {
	reg, _, err := loadStateGroups(flags)
	if err != nil {
		return groups.Set{}, err
	}
	set, err := reg.Resolve(teamKey, token)
	if err != nil {
		if errors.Is(err, groups.ErrUnknownToken) {
			return groups.Set{}, usageErr(err)
		}
		return groups.Set{}, configErr(err)
	}
	return set, nil
}

// teamKeyForGroups maps whatever the user passed to --team (a key or a UUID)
// to the team key that team-scoped group tables are keyed by. Team scope only
// applies when a team is unambiguously in scope, so an empty input stays
// empty and the workspace group applies.
func teamKeyForGroups(db *store.Store, input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if db != nil {
		if team, ok := resolveTeam(db, trimmed); ok && team.Key != "" {
			return team.Key
		}
	}
	return trimmed
}

// projectTeamKeyForGroups reports the team key a project's state predicates
// must resolve against, given the project's stored payload. A Linear project
// can span several teams, so a team-scoped override is only unambiguous when
// the project belongs to exactly one team: anything else (no teams recorded,
// or more than one) resolves at workspace scope, which is what the empty key
// selects.
func projectTeamKeyForGroups(raw json.RawMessage) string {
	var payload struct {
		Teams struct {
			Nodes []struct {
				Key string `json:"key"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if len(payload.Teams.Nodes) != 1 {
		return ""
	}
	return strings.TrimSpace(payload.Teams.Nodes[0].Key)
}

func newGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "Inspect the workflow-state groups that --state resolves against",
		Long: `Show and test the state groups every --state flag resolves through.

A group names a set of workflow states. Built-in groups ship with the binary.
You can add or override any of them by declaring them in groups.toml next to
your config file (the path is printed by 'groups list'). Declarations there
survive token rotation, which is why they do not live in config.toml.

Grammar:

  schema_version = 1

  [state_groups.wip]
  description = "Actually being worked, plus our review column"
  types = ["started"]
  names = ["In Review"]

  [team_state_groups.ENG.wip]
  types = ["started"]
  names = ["In Review", "QA"]

Membership is the union of the two keys: a state belongs to the group when its
type is listed in types OR its name matches one of names case-insensitively.
There is no negation and no nesting, on purpose.`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,10"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newGroupsListCmd(flags))
	cmd.AddCommand(newGroupsCheckCmd(flags))
	return cmd
}

func newGroupsListCmd(flags *rootFlags) *cobra.Command {
	var team string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every effective state group and where it is declared",
		Long: `List every state group visible right now, including shadowed ones, so you can
see why a token resolves the way it does.

source is one of:
  builtin            shipped with this binary
  config:workspace   declared in [state_groups.<name>] in your groups.toml
  config:team:<KEY>  declared in [team_state_groups.<KEY>.<name>]

Precedence is team, then workspace, then builtin. A shadowed row names the
higher-precedence entries that win over it in shadowed_by. Use
'--state type:<type>' when you have shadowed a raw type and still need it.`,
		Example: `  linear-pp-cli groups list
  linear-pp-cli groups list --team ENG --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,10",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, cfgPath, err := loadStateGroups(flags)
			if err != nil {
				return err
			}
			warnMisplacedGroups(cmd, reg, cfgPath)

			rows := reg.Effective(team)
			var teamScope any
			if scope := strings.ToUpper(strings.TrimSpace(team)); scope != "" {
				teamScope = scope
			}
			envelope := map[string]any{
				"results": rows,
				"meta": map[string]any{
					"source":              "local",
					"resource_type":       resourceTypeStateGroups,
					"reason":              "user_requested",
					"groups_path":         reg.Path(),
					"groups_file_present": reg.Present(),
					"team_scope":          teamScope,
				},
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			table := make([][]string, 0, len(rows))
			for _, g := range rows {
				table = append(table, []string{
					g.Name,
					g.Source,
					strings.Join(g.Types, ","),
					strings.Join(g.Names, ","),
					g.Description,
				})
			}
			if err := flags.printTable(cmd, []string{"NAME", "SOURCE", "TYPES", "NAMES", "DESCRIPTION"}, table); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\ndeclarations: %s (present: %t)\n", reg.Path(), reg.Present())
			return nil
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Team key whose team-scoped groups to include (e.g. ENG)")
	return cmd
}

func newGroupsCheckCmd(flags *rootFlags) *cobra.Command {
	var team, dbPath string
	cmd := &cobra.Command{
		Use:   "check <group-or-type>",
		Short: "Resolve one state token and show exactly what it matches",
		Long: `Resolve a --state token through the full resolution order and print the
membership predicate it produces, the live WorkflowStateFilter it emits, and,
when workflow states have been synced, the concrete states in your workspace
that it actually hits.

unmatched_names is the payoff: it is how you discover that you wrote
names = ["Staging"] but your column is called "On Staging". Group names are
never validated at load time precisely because they are validated here,
against live data.

The command never fails just because the local store is cold. It drops
matching_states and tells you to sync.`,
		Example: `  linear-pp-cli groups check active
  linear-pp-cli groups check wip --team ENG --json
  linear-pp-cli groups check type:duplicate`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,10",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, cfgPath, err := loadStateGroups(flags)
			if err != nil {
				return err
			}
			warnMisplacedGroups(cmd, reg, cfgPath)

			set, resolveErr := reg.Resolve(team, args[0])
			if resolveErr != nil {
				if errors.Is(resolveErr, groups.ErrUnknownToken) {
					return usageErr(resolveErr)
				}
				return configErr(resolveErr)
			}

			result := map[string]any{
				"token":       set.Token,
				"resolved_as": set.ResolvedAs,
				"source":      set.Group.Source,
				"scope":       set.Group.Scope,
				"types":       set.Group.Types,
				"names":       set.Group.Names,
			}
			liveFilter := set.GraphQLFilter()
			if liveFilter == nil {
				result["live_filter"] = nil
			} else {
				result["live_filter"] = map[string]any{"state": liveFilter}
			}

			meta := map[string]any{
				"source":        "local",
				"resource_type": resourceTypeStateGroups,
				"reason":        "user_requested",
			}

			db, _ := openStoreAt(resolveDBPath(dbPath))
			if db != nil {
				defer db.Close()
				prov := localProvenance(db, resourceTypeWorkflowStates, "user_requested")
				if prov.SyncedAt != nil {
					meta["synced_at"] = prov.SyncedAt.UTC().Format(time.RFC3339)
				}
				matching, unmatched, ok := matchStatesForGroup(db, set, team)
				if ok {
					result["matching_states"] = matching
					result["unmatched_names"] = unmatched
				} else {
					hintIfUnsynced(cmd, db, resourceTypeWorkflowStates)
				}
			}

			envelope := map[string]any{"results": result, "meta": meta}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			return renderGroupCheck(cmd, result)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Team key to resolve team-scoped groups against (e.g. ENG)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to list the concrete states the group hits")
	return cmd
}

// warnMisplacedGroups points at the one file the declarations must live in.
// config.save rewrites config.toml wholesale from a fixed struct, so group
// tables left there are deleted the first time the user rotates a token.
func warnMisplacedGroups(cmd *cobra.Command, reg *groups.Registry, cfgPath string) {
	if !groups.MisplacedInConfig(cfgPath) {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"warning: %s declares state_groups or team_state_groups. Those keys are ignored there and any 'auth' write will delete them. Move them to %s\n",
		cfgPath, reg.Path())
}

// matchStatesForGroup lists the concrete workflow states in the local
// snapshot that the resolved set matches. The third return is false when the
// store holds no workflow states at all, which is a cold cache and not an
// error.
func matchStatesForGroup(db *store.Store, set groups.Set, teamKey string) ([]map[string]any, []string, bool) {
	teamID := ""
	if trimmed := strings.TrimSpace(teamKey); trimmed != "" {
		if team, ok := resolveTeam(db, trimmed); ok {
			teamID = team.ID
		}
	}
	raw, err := db.ListWorkflowStates(teamID)
	if err != nil || len(raw) == 0 {
		return nil, nil, false
	}

	matched := make([]map[string]any, 0, len(raw))
	hit := map[string]bool{}
	for _, item := range raw {
		var ws workflowStateRow
		if err := json.Unmarshal(item, &ws); err != nil || ws.ID == "" {
			continue
		}
		byType := set.MatchesType(ws.Type)
		byName := set.MatchesName(ws.Name)
		if !byType && !byName {
			continue
		}
		matchedBy := "name"
		if byType {
			matchedBy = "type"
		}
		for _, want := range set.Group.Names {
			if strings.EqualFold(strings.TrimSpace(want), strings.TrimSpace(ws.Name)) {
				hit[want] = true
			}
		}
		matched = append(matched, map[string]any{
			"id":         ws.ID,
			"name":       ws.Name,
			"type":       ws.Type,
			"team_key":   ws.Team.Key,
			"matched_by": matchedBy,
		})
	}
	sort.Slice(matched, func(i, j int) bool {
		a, _ := matched[i]["team_key"].(string)
		b, _ := matched[j]["team_key"].(string)
		if a != b {
			return a < b
		}
		an, _ := matched[i]["name"].(string)
		bn, _ := matched[j]["name"].(string)
		return an < bn
	})

	unmatched := []string{}
	for _, want := range set.Group.Names {
		if !hit[want] {
			unmatched = append(unmatched, want)
		}
	}
	return matched, unmatched, true
}

func renderGroupCheck(cmd *cobra.Command, result map[string]any) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%-14s %v\n", "TOKEN", result["token"])
	fmt.Fprintf(out, "%-14s %v\n", "RESOLVED AS", result["resolved_as"])
	fmt.Fprintf(out, "%-14s %v\n", "SOURCE", result["source"])
	fmt.Fprintf(out, "%-14s %v\n", "SCOPE", result["scope"])
	if types, ok := result["types"].([]string); ok {
		fmt.Fprintf(out, "%-14s %s\n", "TYPES", strings.Join(types, ", "))
	}
	if names, ok := result["names"].([]string); ok {
		fmt.Fprintf(out, "%-14s %s\n", "NAMES", strings.Join(names, ", "))
	}
	if filter, err := json.Marshal(result["live_filter"]); err == nil {
		fmt.Fprintf(out, "%-14s %s\n", "LIVE FILTER", string(filter))
	}
	states, ok := result["matching_states"].([]map[string]any)
	if !ok {
		fmt.Fprintln(cmd.ErrOrStderr(), "no synced workflow states. Run 'linear-pp-cli sync' to see which states this group hits")
		return nil
	}
	fmt.Fprintf(out, "\n%-10s %-24s %-12s %s\n", "TEAM", "STATE", "TYPE", "MATCHED BY")
	fmt.Fprintln(out, strings.Repeat("-", 62))
	for _, s := range states {
		fmt.Fprintf(out, "%-10v %-24v %-12v %v\n", s["team_key"], s["name"], s["type"], s["matched_by"])
	}
	if unmatched, ok := result["unmatched_names"].([]string); ok && len(unmatched) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "\nnames matching no synced state: %s\n", strings.Join(unmatched, ", "))
	}
	return nil
}

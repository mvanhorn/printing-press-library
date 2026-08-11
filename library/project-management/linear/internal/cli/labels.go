package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
	"github.com/spf13/cobra"
)

type issueLabelTeam struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type issueLabelInfo struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Color    string          `json:"color"`
	Global   bool            `json:"global"`
	TeamID   string          `json:"teamId,omitempty"`
	TeamKey  string          `json:"teamKey,omitempty"`
	TeamName string          `json:"teamName,omitempty"`
	Team     *issueLabelTeam `json:"team"`
}

type issueTeamInfo struct {
	ID  string
	Key string
}

func newLabelsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "labels",
		Short:       "Manage Linear issue labels: list, create, update, delete, retire, restore",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newLabelsListCmd(flags))
	cmd.AddCommand(newLabelsCreateCmd(flags))
	cmd.AddCommand(newLabelsUpdateCmd(flags))
	cmd.AddCommand(newLabelsDeleteCmd(flags))
	cmd.AddCommand(newLabelsRetireCmd(flags))
	cmd.AddCommand(newLabelsRestoreCmd(flags))
	return cmd
}

func newLabelsListCmd(flags *rootFlags) *cobra.Command {
	var team string
	var includeGlobal bool
	var noGlobal bool
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List issue labels, optionally filtered to labels safe for a team",
		Example: `  linear-pp-cli labels list --team SYMPH --agent
  linear-pp-cli labels list --team HSUI --no-global --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				limit = 100
			}
			includeGlobals := includeGlobal && !noGlobal
			var labels []json.RawMessage
			prov := DataProvenance{ResourceType: "issue_labels"}
			fetchLocal := func(reason string) ([]json.RawMessage, DataProvenance, error) {
				db, err := store.Open(resolveDBPath(dbPath))
				if err != nil {
					return nil, DataProvenance{}, fmt.Errorf("opening local database: %w", err)
				}
				defer db.Close()
				if team != "" {
					labels, err = db.ListIssueLabelsForTeam(limit, team, includeGlobals)
				} else {
					labels, err = db.ListIssueLabels(limit)
				}
				if err != nil {
					return nil, DataProvenance{}, fmt.Errorf("listing issue labels: %w", err)
				}
				return labels, DataProvenance{Source: "local", ResourceType: "issue_labels", Reason: reason}, nil
			}
			switch flags.dataSource {
			case "local":
				var err error
				labels, prov, err = fetchLocal("user_requested")
				if err != nil {
					return err
				}
			default:
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				nodes, err := c.PaginatedQueryMax(client.IssueLabelsQuery, map[string]any{"first": limit}, "issueLabels", limit, 10)
				if err != nil {
					if flags.dataSource == "live" || !isNetworkError(err) {
						return classifyAPIError(err, flags)
					}
					var fallbackErr error
					labels, prov, fallbackErr = fetchLocal("api_unreachable")
					if fallbackErr != nil {
						return fmt.Errorf("API unreachable and no local issue labels. Run 'linear-pp-cli sync' to enable offline access.\n\nOriginal error: %w", err)
					}
					break
				}
				labels = nodes
				prov.Source = "live"
				prov.Reason = "user_requested"
			}
			filtered := filterIssueLabelsForTeam(labels, team, includeGlobals)
			out, err := json.Marshal(filtered)
			if err != nil {
				return err
			}
			return renderPayloadWithProvenance(cmd, flags, out, prov, true)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Target team key, name, or UUID; returns global labels plus labels owned by this team")
	cmd.Flags().BoolVar(&includeGlobal, "global", true, "Include global labels when --team is set")
	cmd.Flags().BoolVar(&noGlobal, "no-global", false, "Exclude global labels when --team is set")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum labels per live API page")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for --data-source local")
	return cmd
}

func renderPayloadWithProvenance(cmd *cobra.Command, flags *rootFlags, data json.RawMessage, prov DataProvenance, compact bool) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		} else if compact && flags.compact {
			data = compactFields(data)
		}
		wrapped, err := wrapWithProvenance(data, prov)
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func filterIssueLabelsForTeam(raw []json.RawMessage, team string, includeGlobal bool) []issueLabelInfo {
	target := strings.ToLower(strings.TrimSpace(team))
	out := make([]issueLabelInfo, 0, len(raw))
	for _, node := range raw {
		var label issueLabelInfo
		if err := json.Unmarshal(node, &label); err != nil || label.ID == "" {
			continue
		}
		label.normalizeTeam()
		if target == "" {
			out = append(out, label)
			continue
		}
		if label.Team == nil || (label.Team.ID == "" && label.Team.Key == "" && label.Team.Name == "") {
			if includeGlobal {
				out = append(out, label)
			}
			continue
		}
		if strings.ToLower(label.Team.ID) == target || strings.ToLower(label.Team.Key) == target || strings.ToLower(label.Team.Name) == target {
			out = append(out, label)
		}
	}
	return out
}

func (label *issueLabelInfo) normalizeTeam() {
	if label.Team != nil && (label.Team.ID != "" || label.Team.Key != "" || label.Team.Name != "") {
		label.Global = false
		return
	}
	if label.TeamID == "" && label.TeamKey == "" && label.TeamName == "" {
		label.Global = true
		return
	}
	label.Team = &issueLabelTeam{ID: label.TeamID, Key: label.TeamKey, Name: label.TeamName}
	label.Global = false
}

func validateIssueLabelTeams(c *client.Client, labelIDs []string, target issueTeamInfo) error {
	if len(labelIDs) == 0 {
		return nil
	}
	targetID := strings.ToLower(strings.TrimSpace(target.ID))
	targetKey := strings.ToLower(strings.TrimSpace(target.Key))
	if targetID == "" && targetKey == "" {
		return fmt.Errorf("cannot validate labels without target issue team")
	}
	labels, err := fetchIssueLabelsByIDsLive(c, labelIDs)
	if err != nil {
		return err
	}
	byID := make(map[string]issueLabelInfo, len(labels))
	for _, label := range labels {
		byID[strings.ToLower(label.ID)] = label
	}
	for _, id := range labelIDs {
		label, ok := byID[strings.ToLower(strings.TrimSpace(id))]
		if !ok {
			return notFoundErr(fmt.Errorf("issue label %q not found", id))
		}
		if label.Team == nil || (label.Team.ID == "" && label.Team.Key == "") {
			continue
		}
		labelID := strings.ToLower(label.Team.ID)
		labelKey := strings.ToLower(label.Team.Key)
		if (targetID != "" && labelID == targetID) || (targetKey != "" && labelKey == targetKey) {
			continue
		}
		return usageErr(fmt.Errorf("label %q (%s) belongs to team %s; target issue team is %s", label.ID, label.Name, labelTeamName(label.Team), issueTeamName(target)))
	}
	return nil
}

// fetchIssueLabelsByIDsLive resolves all requested label UUIDs in a single
// batched GraphQL call. The previous shape issued one round-trip per label,
// so a multi-label edit paid N sequential API calls before the mutation fired.
func fetchIssueLabelsByIDsLive(c *client.Client, ids []string) ([]issueLabelInfo, error) {
	unique := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, strings.TrimSpace(id))
	}
	const query = `query($ids: [ID!]!, $first: Int!) {
		issueLabels(filter: { id: { in: $ids } }, first: $first) {
			nodes {
				id name color
				team { id key name }
			}
		}
	}`
	var resp struct {
		IssueLabels struct {
			Nodes []issueLabelInfo `json:"nodes"`
		} `json:"issueLabels"`
	}
	if err := c.QueryInto(query, map[string]any{"ids": unique, "first": len(unique)}, &resp); err != nil {
		return nil, err
	}
	return resp.IssueLabels.Nodes, nil
}

func labelTeamName(team *issueLabelTeam) string {
	if team == nil {
		return "global"
	}
	return firstNonEmpty(team.Key, team.ID, "unknown")
}

func issueTeamName(team issueTeamInfo) string {
	return firstNonEmpty(team.Key, team.ID, "unknown")
}

// Label write surface (GAP-026). All five issueLabel mutations are live in
// api-inventory.json. Retire and restore are the modern soft-delete pair and are
// deliberately separate commands from the hard delete.

// newLabelsCreateCmd creates a team label when --team is given and a workspace
// label when it is not. IssueLabelCreateInput.teamId is optional and the API
// documents the omitted case as "associated with the entire workspace".
func newLabelsCreateCmd(flags *rootFlags) *cobra.Command {
	var nameFlag, teamFlag, colorFlag, descFlag, parentFlag, dbPath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an issue label, workspace-wide or scoped to one team",
		Long: `Create a Linear issue label via the issueLabelCreate mutation.

Omit --team to create a workspace label visible to every team. Pass --team with
a team key or UUID to create a label only that team can apply.`,
		Example: `  linear-pp-cli labels create --name "needs-repro" --team ENG --agent
  linear-pp-cli labels create --name "customer-reported" --color "#4ea7fc" --agent
  linear-pp-cli labels create --name "child" --parent <label-uuid> --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(nameFlag) == "" {
				return usageErr(fmt.Errorf("--name is required"))
			}
			// buildInput takes the client so the live path can fall back to a
			// server-side team lookup when the local store has not been synced.
			// The dry-run path passes nil and stays offline.
			buildInput := func(c graphqlQueryer) (map[string]any, error) {
				input := map[string]any{"name": nameFlag}
				setOptionalString(input, "color", colorFlag)
				setOptionalString(input, "description", descFlag)
				setOptionalString(input, "parentId", parentFlag)
				if teamFlag == "" {
					return input, nil
				}
				teamID, err := resolveWriteTeamID(c, dbPath, teamFlag)
				if err != nil {
					return nil, err
				}
				input["teamId"] = teamID
				return input, nil
			}
			if flags.dryRun {
				input, err := buildInput(nil)
				if err != nil {
					return err
				}
				return renderMutationDryRun(cmd, flags, "would_create_label", "issueLabelCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			input, err := buildInput(c)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			resp, err := c.Mutate(client.IssueLabelCreateMutation, map[string]any{"input": input})
			if err != nil {
				return classifyGraphQLCreateError("issueLabelCreate", err, flags)
			}
			label, err := extractMutationObject(resp, "issueLabelCreate", "issueLabel")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, label, "issue_labels")
		},
	}
	cmd.Flags().StringVar(&nameFlag, "name", "", "Label name (required)")
	cmd.Flags().StringVar(&teamFlag, "team", "", "Team key or UUID, omit to create a workspace label")
	cmd.Flags().StringVar(&colorFlag, "color", "", "Label color as a hex string (e.g. #4ea7fc)")
	cmd.Flags().StringVar(&descFlag, "description", "", "Label description")
	cmd.Flags().StringVar(&parentFlag, "parent", "", "Parent label UUID, nests this label under a label group")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (for team-key resolution)")
	return cmd
}

// newLabelsUpdateCmd offers no --team flag on purpose: IssueLabelUpdateInput
// has no teamId field, so a label cannot be moved between teams by mutation.
func newLabelsUpdateCmd(flags *rootFlags) *cobra.Command {
	var nameFlag, colorFlag, descFlag, parentFlag string
	cmd := &cobra.Command{
		Use:   "update <label-id>",
		Short: "Update an issue label's name, color, description, or parent",
		Long: `Update a Linear issue label via the issueLabelUpdate mutation.

Only the fields you pass are sent. There is no --team flag: IssueLabelUpdateInput
has no teamId field, so a label's team ownership is fixed at creation. Delete and
recreate the label to change it.`,
		Example: `  linear-pp-cli labels update <label-uuid> --name "needs-repro" --agent
  linear-pp-cli labels update <label-uuid> --color "#f2994a" --dry-run --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := map[string]any{}
			setChangedString(cmd, input, "name", "name", nameFlag)
			setChangedString(cmd, input, "color", "color", colorFlag)
			setChangedString(cmd, input, "description", "description", descFlag)
			setChangedString(cmd, input, "parent", "parentId", parentFlag)
			if len(input) == 0 {
				return usageErr(fmt.Errorf("no label fields supplied, pass --name, --color, --description, or --parent"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_label", "issueLabelUpdate", map[string]any{"label": args[0], "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.IssueLabelUpdateMutation, map[string]any{"id": args[0], "input": input})
			if err != nil {
				return classifyGraphQLMutationError("issueLabelUpdate", err, flags)
			}
			label, err := extractMutationObject(resp, "issueLabelUpdate", "issueLabel")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, label, "issue_labels")
		},
	}
	cmd.Flags().StringVar(&nameFlag, "name", "", "New label name")
	cmd.Flags().StringVar(&colorFlag, "color", "", "New label color as a hex string")
	cmd.Flags().StringVar(&descFlag, "description", "", "New label description (pass an empty string to clear)")
	cmd.Flags().StringVar(&parentFlag, "parent", "", "New parent label UUID")
	return cmd
}

// newLabelsDeleteCmd hard-deletes a label. Retire is the reversible option and
// the one to reach for when the label is still on existing issues.
func newLabelsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <label-id>",
		Short: "Permanently delete an issue label",
		Long: `Delete a Linear issue label via the issueLabelDelete mutation.

This is not reversible and removes the label from every issue that carries it.
Prefer 'labels retire' when the label should stop being applied to new issues
but stay readable on old ones. Requires confirmation unless --yes is set.`,
		Example: `  linear-pp-cli labels delete <label-uuid> --yes --agent
  linear-pp-cli labels delete <label-uuid> --ignore-missing --yes --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_label", "issueLabelDelete", map[string]any{"input": map[string]any{"id": args[0]}})
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Permanently delete label %s and remove it from every issue?", args[0])); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.IssueLabelDeleteMutation, map[string]any{"id": args[0]})
			if err != nil {
				return classifyGraphQLMutationError("issueLabelDelete", err, flags)
			}
			entityID, err := extractDeletedEntityID(resp, "issueLabelDelete")
			if err != nil {
				return err
			}
			return renderMutationEvent(cmd, flags, "label_deleted", map[string]any{"entity_id": firstNonEmpty(entityID, args[0])})
		},
	}
	return cmd
}

// newLabelsRetireCmd and newLabelsRestoreCmd are the soft-delete pair. Retired
// labels stay on the issues that already carry them and cannot be applied to
// new ones. Both declare their own Use/Short/Example literally and share only
// the RunE body, so the command surface stays greppable from this file.
func newLabelsRetireCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "retire <label-id>",
		Short:   "Retire an issue label so it can no longer be applied to new issues",
		Long:    "Retire a Linear issue label via the issueLabelRetire mutation. Existing issues keep the label and stay searchable by it. Reverse with 'labels restore'.",
		Example: "  linear-pp-cli labels retire <label-uuid> --agent",
		Args:    cobra.ExactArgs(1),
		RunE: labelStateChangeRunE(flags, labelStateChange{
			event:    "would_retire_label",
			mutation: "issueLabelRetire",
			document: client.IssueLabelRetireMutation,
		}),
	}
}

func newLabelsRestoreCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "restore <label-id>",
		Short:   "Restore a retired issue label so it can be applied again",
		Long:    "Restore a previously retired Linear issue label via the issueLabelRestore mutation.",
		Example: "  linear-pp-cli labels restore <label-uuid> --agent",
		Args:    cobra.ExactArgs(1),
		RunE: labelStateChangeRunE(flags, labelStateChange{
			event:    "would_restore_label",
			mutation: "issueLabelRestore",
			document: client.IssueLabelRestoreMutation,
		}),
	}
}

// labelStateChange describes the two id-only IssueLabelPayload mutations.
// Retire and restore differ only in their document and their dry-run event, so
// they share one RunE rather than two near-identical bodies.
type labelStateChange struct {
	event    string
	mutation string
	document string
}

func labelStateChangeRunE(flags *rootFlags, spec labelStateChange) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if flags.dryRun {
			return renderMutationDryRun(cmd, flags, spec.event, spec.mutation, map[string]any{"input": map[string]any{"id": args[0]}})
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		resp, err := c.Mutate(spec.document, map[string]any{"id": args[0]})
		if err != nil {
			return classifyGraphQLMutationError(spec.mutation, err, flags)
		}
		label, err := extractMutationObject(resp, spec.mutation, "issueLabel")
		if err != nil {
			return err
		}
		return renderLiveObject(cmd, flags, label, "issue_labels")
	}
}

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
		Short:       "List Linear issue labels with team ownership",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newLabelsListCmd(flags))
	return cmd
}

func newLabelsListCmd(flags *rootFlags) *cobra.Command {
	var team string
	var includeGlobal bool
	var noGlobal bool
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issue labels, optionally filtered to labels safe for a team",
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

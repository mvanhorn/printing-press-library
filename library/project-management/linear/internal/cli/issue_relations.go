package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

type relationIssue struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

type issueRelationNode struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	CreatedAt    string        `json:"createdAt"`
	Issue        relationIssue `json:"issue"`
	RelatedIssue relationIssue `json:"relatedIssue"`
}

type issueRelationRow struct {
	ID        string        `json:"id"`
	Relation  string        `json:"relation"`
	Issue     relationIssue `json:"issue"`
	CreatedAt string        `json:"createdAt,omitempty"`
}

var relationLabels = map[string][2]string{
	"blocks":    {"blocks", "blocked-by"},
	"related":   {"related", "related"},
	"duplicate": {"duplicate-of", "duplicated-by"},
	"similar":   {"similar", "similar"},
}

func relationRow(node issueRelationNode, namedIssueID string) issueRelationRow {
	labels, known := relationLabels[node.Type]
	if !known {
		labels = [2]string{node.Type, "inverse:" + node.Type}
	}
	row := issueRelationRow{ID: node.ID, CreatedAt: node.CreatedAt}
	if node.Issue.ID == namedIssueID {
		row.Relation, row.Issue = labels[0], node.RelatedIssue
	} else {
		row.Relation, row.Issue = labels[1], node.Issue
	}
	return row
}

func relationCreateInput(relation, issue, other string) map[string]any {
	if relation == "blocked-by" {
		return map[string]any{"issueId": other, "relatedIssueId": issue, "type": "blocks"}
	}
	return map[string]any{"issueId": issue, "relatedIssueId": other, "type": relation}
}

type relationTargetFlags struct {
	blocks    string
	blockedBy string
	related   string
}

func (t *relationTargetFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&t.blockedBy, "blocked-by", "", "Issue identifier or UUID that blocks the named issue")
	cmd.Flags().StringVar(&t.blocks, "blocks", "", "Issue identifier or UUID that the named issue blocks")
	cmd.Flags().StringVar(&t.related, "related", "", "Issue identifier or UUID related to the named issue")
}

func (t relationTargetFlags) pick() (relation, other string, err error) {
	count := 0
	for _, candidate := range [][2]string{{"blocked-by", t.blockedBy}, {"blocks", t.blocks}, {"related", t.related}} {
		if candidate[1] != "" {
			count++
			relation, other = candidate[0], candidate[1]
		}
	}
	if count != 1 {
		return "", "", usageErr(fmt.Errorf("pass exactly one of --blocked-by, --blocks, or --related"))
	}
	return relation, other, nil
}

func newIssuesRelationsCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relations",
		Short: "List, add, and remove issue relations",
		Long: `List, add, and remove relations between Linear issues. Every relation is
reported from the named issue's side: blocks, blocked-by, related,
duplicate-of, duplicated-by, or similar.`,
		Example: `  linear-pp-cli issues relations list ENG-124 --agent
  linear-pp-cli issues relations add ENG-124 --blocked-by ENG-123 --agent
  linear-pp-cli issues relations remove ENG-124 --blocked-by ENG-123 --agent`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newIssuesRelationsListCmd(flags))
	cmd.AddCommand(newIssuesRelationsAddCmd(flags, dbPath))
	cmd.AddCommand(newIssuesRelationsRemoveCmd(flags, dbPath))
	return cmd
}

// pp:data-source live
func newIssuesRelationsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list <issue>",
		Short:       "List every relation of an issue, from that issue's side",
		Example:     `  linear-pp-cli issues relations list ENG-124 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			issue, nodes, err := fetchIssueRelations(c, args[0])
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			rows := make([]issueRelationRow, 0, len(nodes))
			for _, node := range nodes {
				rows = append(rows, relationRow(node, issue.ID))
			}
			return renderRelations(cmd, flags, issue, "relations", rows)
		},
	}
}

func newIssuesRelationsAddCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	targets := relationTargetFlags{}
	cmd := &cobra.Command{
		Use:     "add <issue>",
		Aliases: []string{"create"},
		Short:   "Add a blocked-by, blocks, or related relation to an issue",
		Example: `  linear-pp-cli issues relations add ENG-124 --blocked-by ENG-123 --agent
  linear-pp-cli issues relations add ENG-124 --related ENG-200 --dry-run --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			relation, other, err := targets.pick()
			if err != nil {
				return err
			}
			if err := requireRelationLedger(flags, *dbPath, args[0], other); err != nil {
				return err
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_create_issue_relation", "issueRelationCreate", map[string]any{"input": relationCreateInput(relation, args[0], other)})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			issueID, otherID, err := resolveIssuePair(c, args[0], other)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			mutation := `mutation($input: IssueRelationCreateInput!) {
				issueRelationCreate(input: $input) {
					success
					issueRelation {` + client.IssueRelationSelection + `
					}
				}
			}`
			resp, err := c.Mutate(mutation, map[string]any{"input": relationCreateInput(relation, issueID, otherID)})
			if err != nil {
				return classifyMutationError("issueRelationCreate", err, flags, nil)
			}
			raw, err := extractMutationObject(resp, "issueRelationCreate", "issueRelation")
			if err != nil {
				return err
			}
			var node issueRelationNode
			if err := json.Unmarshal(raw, &node); err != nil {
				return fmt.Errorf("parsing issueRelationCreate relation: %w", err)
			}
			writeRelationsBack(*dbPath, func(db *store.Store) error {
				return db.UpsertIssueRelation(store.IssueRelationRecord{ID: node.ID, IssueID: node.Issue.ID, RelatedIssueID: node.RelatedIssue.ID, Type: node.Type, Data: raw})
			})
			issue := node.Issue
			if issue.ID != issueID {
				issue = node.RelatedIssue
			}
			return renderRelations(cmd, flags, issue, "relations", []issueRelationRow{relationRow(node, issueID)})
		},
	}
	targets.bind(cmd)
	return cmd
}

func newIssuesRelationsRemoveCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	targets := relationTargetFlags{}
	cmd := &cobra.Command{
		Use:     "remove <issue>",
		Aliases: []string{"delete"},
		Short:   "Remove a blocked-by, blocks, or related relation from an issue",
		Long: `Remove every relation that matches the named issue, the relation flag, and the
other issue. The flags read the same way as 'issues relations add'.`,
		Example: `  linear-pp-cli issues relations remove ENG-124 --blocked-by ENG-123 --agent
  linear-pp-cli issues relations remove ENG-124 --related ENG-200 --ignore-missing --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			relation, other, err := targets.pick()
			if err != nil {
				return err
			}
			if err := requireRelationLedger(flags, *dbPath, args[0], other); err != nil {
				return err
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_issue_relation", "issueRelationDelete", map[string]any{"issue": args[0], "relation": relation, "other_issue": other})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			issueID, otherID, err := resolveIssuePair(c, args[0], other)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			issue, nodes, err := fetchIssueRelations(c, issueID)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			var matches []issueRelationRow
			for _, node := range nodes {
				if row := relationRow(node, issueID); row.Relation == relation && row.Issue.ID == otherID {
					matches = append(matches, row)
				}
			}
			if len(matches) == 0 {
				if flags.ignoreMissing {
					return writeNoop(flags, "already_deleted", "no matching relation (no-op)")
				}
				return notFoundErr(fmt.Errorf("issue %s has no %s relation with %s", args[0], relation, other))
			}
			var removed []string
			defer func() {
				writeRelationsBack(*dbPath, func(db *store.Store) error {
					for _, id := range removed {
						if _, err := db.DeleteIssueRelation(id); err != nil {
							return err
						}
					}
					return nil
				})
			}()
			for _, row := range matches {
				if err := deleteIssueRelation(c, row.ID, flags); err != nil {
					if len(removed) > 0 {
						return fmt.Errorf("%w (already removed: %s)", err, strings.Join(removed, ", "))
					}
					return err
				}
				removed = append(removed, row.ID)
			}
			return renderRelations(cmd, flags, issue, "removed", matches)
		},
	}
	targets.bind(cmd)
	return cmd
}

func deleteIssueRelation(c *client.Client, id string, flags *rootFlags) error {
	resp, err := c.Mutate(`mutation($id: String!) { issueRelationDelete(id: $id) { success entityId } }`, map[string]any{"id": id})
	if err != nil {
		return classifyMutationError("issueRelationDelete", err, flags, nil)
	}
	_, err = extractMutationObject(resp, "issueRelationDelete", "entityId")
	return err
}

func resolveIssuePair(c graphqlQueryer, issue, other string) (string, string, error) {
	issueID, err := resolveIssueID(c, issue)
	if err != nil {
		return "", "", err
	}
	otherID, err := resolveIssueID(c, other)
	if err != nil {
		return "", "", err
	}
	return issueID, otherID, nil
}

func fetchIssueRelations(c graphqlQueryer, ref string) (relationIssue, []issueRelationNode, error) {
	var issue relationIssue
	var nodes []issueRelationNode
	for _, connection := range []string{"relations", "inverseRelations"} {
		query := `query($id: String!, $first: Int!, $after: String) {
			issue(id: $id) {
				id identifier title
				connection: ` + connection + `(first: $first, after: $after) {
					nodes {` + client.IssueRelationSelection + `
					}
					pageInfo { hasNextPage endCursor }
				}
			}
		}`
		var after any
		for {
			var resp struct {
				Issue *struct {
					relationIssue
					Connection struct {
						Nodes    []issueRelationNode `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"connection"`
				} `json:"issue"`
			}
			if err := c.QueryInto(query, map[string]any{"id": ref, "first": 50, "after": after}, &resp); err != nil {
				return relationIssue{}, nil, err
			}
			if resp.Issue == nil {
				return relationIssue{}, nil, notFoundErr(fmt.Errorf("issue %q not found", ref))
			}
			issue = resp.Issue.relationIssue
			nodes = append(nodes, resp.Issue.Connection.Nodes...)
			if !resp.Issue.Connection.PageInfo.HasNextPage {
				break
			}
			after = resp.Issue.Connection.PageInfo.EndCursor
		}
	}
	return issue, nodes, nil
}

func requireRelationLedger(flags *rootFlags, dbPath string, refs ...string) error {
	if flags.trustMode != "strict" {
		return nil
	}
	path := resolveDBPath(dbPath)
	db, err := openStoreAt(path)
	if err != nil {
		return fmt.Errorf("trust-mode=strict: cannot open ledger at %s: %w", path, err)
	}
	if db == nil {
		return usageErr(fmt.Errorf("trust-mode=strict: no pp_created ledger at %s; refusing to change issue relations", path))
	}
	defer db.Close()
	fixtures, err := db.ListPPFixtures("")
	if err != nil {
		return fmt.Errorf("trust-mode=strict: reading ledger at %s: %w", path, err)
	}
	for _, ref := range refs {
		found := false
		for _, f := range fixtures {
			if f.IssueID == ref || strings.EqualFold(f.Identifier, ref) {
				found = true
				break
			}
		}
		if !found {
			return usageErr(fmt.Errorf("trust-mode=strict: %s was not created by this CLI (not in the pp_created ledger); refusing to change its relations", ref))
		}
	}
	return nil
}

func writeRelationsBack(dbPath string, write func(*store.Store) error) {
	path := resolveDBPath(dbPath)
	db, err := openStoreAt(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open local store at %s: %v\n", path, err)
		return
	}
	if db == nil {
		return
	}
	defer db.Close()
	if err := write(db); err != nil {
		fmt.Fprintf(os.Stderr, "warning: local store write-back failed: %v\n", err)
	}
}

func renderRelations(cmd *cobra.Command, flags *rootFlags, issue relationIssue, key string, rows []issueRelationRow) error {
	out, err := json.Marshal(map[string]any{"issue": issue, key: rows})
	if err != nil {
		return err
	}
	return renderLivePayload(cmd, flags, out, "issue_relations", false)
}

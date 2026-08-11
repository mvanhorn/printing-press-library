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

// Issue relations.
//
// Linear models every issue-to-issue link as one IssueRelation carrying an
// IssueRelationType. The enum has exactly four values (blocks, duplicate,
// related, similar) and they are schema values, not workspace vocabulary, so
// naming them here is API data rather than a compiled-in workspace literal.
//
// "blocked by" is NOT a fifth type. A single IssueRelation{type: blocks,
// issue: A, relatedIssue: B} is outgoing on A and incoming on B. The incoming
// direction over a `blocks` relation is what a human calls "blocked by".
// Every row this file emits carries `type` (the enum, verbatim) and
// `direction` (outgoing or incoming) as separate fields, and no code path
// ever writes "blocked_by" into a type.

const (
	relationDirectionOutgoing = "outgoing"
	relationDirectionIncoming = "incoming"
	relationDirectionBoth     = "both"
)

// issueRelationTypes are the IssueRelationType enum values, verified against
// the introspected schema. Order is the enum's own order.
var issueRelationTypes = []string{"blocks", "duplicate", "related", "similar"}

// symmetricRelationTypes are the types whose meaning does not depend on which
// side of the pair the relation was created from. `blocks` is directional (A
// blocks B is not B blocks A) and `duplicate` is directional (A is a
// duplicate of B), so both are excluded. Used only by the --idempotent
// existing-relation check.
var symmetricRelationTypes = map[string]bool{"related": true, "similar": true}

func normalizeIssueRelationType(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, t := range issueRelationTypes {
		if normalized == t {
			return normalized, nil
		}
	}
	return "", usageErr(fmt.Errorf("--type %q is not a valid IssueRelationType. Valid types: %s", value, strings.Join(issueRelationTypes, ", ")))
}

// relationIssueRef is the flattened issue projection used on both sides of a
// relation row. State is flattened to state_name / state_type so a consumer
// can evaluate a state group without walking a nested object.
type relationIssueRef struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	URL        string `json:"url,omitempty"`
	ArchivedAt string `json:"archived_at,omitempty"`
	StateName  string `json:"state_name,omitempty"`
	StateType  string `json:"state_type,omitempty"`
}

func relationIssueRefOf(src client.IssueRelationIssue) relationIssueRef {
	return relationIssueRef{
		ID:         src.ID,
		Identifier: src.Identifier,
		Title:      src.Title,
		URL:        src.URL,
		ArchivedAt: src.ArchivedAt,
		StateName:  src.State.Name,
		StateType:  src.State.Type,
	}
}

// relationRow is one rendered relation. Direction is relative to the issue
// the command was asked about, never a property of the relation itself.
type relationRow struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Direction    string           `json:"direction"`
	Issue        relationIssueRef `json:"issue"`
	RelatedIssue relationIssueRef `json:"related_issue"`
	CreatedAt    string           `json:"created_at,omitempty"`
	// ArchivedAt is the relation's own archivedAt, empty for a live relation.
	// It is populated only when the read asked for archived rows.
	ArchivedAt string `json:"archived_at,omitempty"`
}

// counterpart returns the issue on the far side of the relation from the
// subject, which is the one worth showing in a table.
func (r relationRow) counterpart() relationIssueRef {
	if r.Direction == relationDirectionIncoming {
		return r.Issue
	}
	return r.RelatedIssue
}

// relationLabel renders type plus direction as the phrase a human uses. This
// is a presentation-layer string only: `blocked by` never appears in the
// `type` field of any payload.
func relationLabel(relType, direction string) string {
	incoming := direction == relationDirectionIncoming
	switch relType {
	case "blocks":
		if incoming {
			return "blocked by"
		}
		return "blocks"
	case "duplicate":
		if incoming {
			return "duplicated by"
		}
		return "duplicate of"
	case "related":
		return "related to"
	case "similar":
		return "similar to"
	}
	if incoming {
		return relType + " (incoming)"
	}
	return relType
}

func relationRowsFrom(res client.IssueRelationsResult) []relationRow {
	rows := make([]relationRow, 0, len(res.Relations)+len(res.InverseRelations))
	for _, n := range res.Relations {
		rows = append(rows, relationRowOf(n, relationDirectionOutgoing))
	}
	for _, n := range res.InverseRelations {
		rows = append(rows, relationRowOf(n, relationDirectionIncoming))
	}
	sortRelationRows(rows)
	return rows
}

func relationRowOf(n client.IssueRelationNode, direction string) relationRow {
	return relationRow{
		ID:           n.ID,
		Type:         n.Type,
		Direction:    direction,
		Issue:        relationIssueRefOf(n.Issue),
		RelatedIssue: relationIssueRefOf(n.RelatedIssue),
		CreatedAt:    n.CreatedAt,
		ArchivedAt:   n.ArchivedAt,
	}
}

func sortRelationRows(rows []relationRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Type != rows[j].Type {
			return rows[i].Type < rows[j].Type
		}
		if rows[i].Direction != rows[j].Direction {
			return rows[i].Direction < rows[j].Direction
		}
		return rows[i].counterpart().Identifier < rows[j].counterpart().Identifier
	})
}

func newRelationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relations",
		Aliases: []string{"links"},
		Short:   "Read and manage issue relations (blocks, duplicate, related, similar)",
		Long: `Manage the IssueRelation graph.

Linear has exactly four relation types: blocks, duplicate, related and
similar. "blocked by" is not one of them. It is the incoming direction over a
blocks relation, so every row carries a separate 'direction' field
(outgoing or incoming) alongside the verbatim 'type'.`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,6,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRelationsListCmd(flags))
	cmd.AddCommand(newRelationsCreateCmd(flags))
	cmd.AddCommand(newRelationsDeleteCmd(flags))
	return cmd
}

func newRelationsListCmd(flags *rootFlags) *cobra.Command {
	var typeFilter, direction, dbPath string
	var includeArchived bool
	cmd := &cobra.Command{
		Use:         "list <issue>",
		Aliases:     []string{"ls"},
		Short:       "List every relation touching one issue, in both directions",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List an issue's relations.

Reads Issue.relations (outgoing) and Issue.inverseRelations (incoming) and
renders them as one list. Query.issueRelations takes no filter argument, so a
relation read is always per-issue: there is no server-side way to ask for
"every blocks relation in the workspace".

Rows carry 'type' (the IssueRelationType enum value) and 'direction'
(outgoing or incoming) as separate fields. An incoming blocks relation is
what a human calls "blocked by". It is a direction, never a type.

With --data-source live (the default via auto) relations are fetched from the
API and written through to the local issue_relations table, so a later
--data-source local read of the same issue works offline.

--include-archived passes includeArchived: true to both relation connections,
so relations that were archived alongside their issues come back too. Archived
rows carry archived_at, and the issue projections on either side carry their
own archived_at when the issue itself is archived. Without the flag an
archived relation is invisible on both the live and the local path, even if a
previous --include-archived read cached one.`,
		Example: `  linear-pp-cli relations list ESP-1155
  linear-pp-cli relations list ESP-1155 --type blocks --direction incoming --agent
  linear-pp-cli relations list ESP-1155 --data-source local --json
  linear-pp-cli relations list ESP-1155 --include-archived --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if typeFilter != "" {
				normalized, err := normalizeIssueRelationType(typeFilter)
				if err != nil {
					return err
				}
				typeFilter = normalized
			}
			switch direction {
			case relationDirectionBoth, relationDirectionOutgoing, relationDirectionIncoming:
			default:
				return usageErr(fmt.Errorf("--direction %q is invalid. Valid values: %s, %s, %s", direction, relationDirectionBoth, relationDirectionOutgoing, relationDirectionIncoming))
			}
			return runRelationsList(cmd, flags, resolveDBPath(dbPath), args[0], typeFilter, direction, includeArchived)
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "Only show relations of this IssueRelationType ("+strings.Join(issueRelationTypes, ", ")+")")
	cmd.Flags().StringVar(&direction, "direction", relationDirectionBoth, "Which side to show: both, outgoing, or incoming (incoming blocks == blocked by)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "Include archived relations (maps to the includeArchived connection argument)")
	return cmd
}

func runRelationsList(cmd *cobra.Command, flags *rootFlags, dbPath, issueRef, typeFilter, direction string, includeArchived bool) error {
	db, openErr := openStoreAt(dbPath)
	if db != nil {
		defer db.Close()
	}

	var rows []relationRow
	var prov DataProvenance

	fetchLocal := func(reason string) ([]relationRow, DataProvenance, error) {
		if openErr != nil {
			return nil, DataProvenance{}, fmt.Errorf("opening local database: %w\nRun 'linear-pp-cli relations list %s' with --data-source live first", openErr, issueRef)
		}
		if db == nil {
			return nil, DataProvenance{}, notFoundErr(fmt.Errorf("no local relation snapshot. Relations are not part of 'sync', so run 'linear-pp-cli relations list %s --data-source live' once to populate it", issueRef))
		}
		subjectID, err := resolveIssueIDLocal(db, issueRef)
		if err != nil {
			return nil, DataProvenance{}, err
		}
		raw, err := db.ListIssueRelationsForIssue(subjectID)
		if err != nil {
			return nil, DataProvenance{}, err
		}
		local := make([]relationRow, 0, len(raw))
		for _, r := range raw {
			var node client.IssueRelationNode
			if err := json.Unmarshal(r, &node); err != nil || node.ID == "" {
				continue
			}
			dir := relationDirectionIncoming
			if node.Issue.ID == subjectID {
				dir = relationDirectionOutgoing
			}
			local = append(local, relationRowOf(node, dir))
		}
		sortRelationRows(local)
		return local, localProvenance(db, "issue_relations", reason), nil
	}

	switch flags.dataSource {
	case "local":
		var err error
		rows, prov, err = fetchLocal("user_requested")
		if err != nil {
			return err
		}
	default:
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		issueID, err := resolveIssueID(c, issueRef)
		if err != nil {
			if flags.dataSource == "live" || !isNetworkError(err) {
				return classifyLiveReadError(err, flags)
			}
			rows, prov, err = fetchLocal("api_unreachable")
			if err != nil {
				return err
			}
			break
		}
		res, err := c.FetchIssueRelationsWith(issueID, 0, includeArchived)
		if err != nil {
			if flags.dataSource == "live" || !isNetworkError(err) {
				return classifyLiveReadError(err, flags)
			}
			var fallbackErr error
			rows, prov, fallbackErr = fetchLocal("api_unreachable")
			if fallbackErr != nil {
				return fmt.Errorf("API unreachable and no local relation snapshot for %s.\n\nOriginal error: %w", issueRef, err)
			}
			break
		}
		rows = relationRowsFrom(res)
		prov = DataProvenance{Source: "live", ResourceType: "issue_relations", Reason: "user_requested"}
		if db != nil {
			writeThroughRelations(db, issueID, res)
		}
	}

	filtered := make([]relationRow, 0, len(rows))
	for _, r := range rows {
		if typeFilter != "" && r.Type != typeFilter {
			continue
		}
		if direction != relationDirectionBoth && r.Direction != direction {
			continue
		}
		// The live path already excluded archived relations server-side, but
		// the local snapshot may hold rows cached by an earlier
		// --include-archived read. Re-applying the predicate here keeps the
		// two paths answering the same question.
		if !includeArchived && r.ArchivedAt != "" {
			continue
		}
		filtered = append(filtered, r)
	}

	prov = attachFreshness(prov, flags)
	printProvenance(cmd, len(filtered), prov)
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printRelationTable(cmd, filtered, includeArchived)
	}
	payload, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	return renderPayloadWithProvenance(cmd, flags, payload, prov, false)
}

func printRelationTable(cmd *cobra.Command, rows []relationRow, showArchived bool) error {
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No relations.")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	if showArchived {
		fmt.Fprintln(tw, "RELATION\tISSUE\tSTATE\tARCHIVED\tTITLE\tRELATION-ID")
	} else {
		fmt.Fprintln(tw, "RELATION\tISSUE\tSTATE\tTITLE\tRELATION-ID")
	}
	for _, r := range rows {
		other := r.counterpart()
		if showArchived {
			// A relation can be archived on its own, and either endpoint can
			// be archived independently. Show whichever stamp exists, relation
			// first, so the column is never silently empty for an archived row.
			stamp := r.ArchivedAt
			if stamp == "" {
				stamp = other.ArchivedAt
			}
			if stamp == "" {
				stamp = "-"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				relationLabel(r.Type, r.Direction),
				other.Identifier,
				other.StateName,
				stamp,
				truncate(other.Title, 48),
				r.ID,
			)
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			relationLabel(r.Type, r.Direction),
			other.Identifier,
			other.StateName,
			truncate(other.Title, 48),
			r.ID,
		)
	}
	return tw.Flush()
}

// writeThroughRelations mirrors a live relation read into the local snapshot.
// Failures are advisory: a read must not fail because the cache write did.
func writeThroughRelations(db *store.Store, issueID string, res client.IssueRelationsResult) {
	records := make([]store.IssueRelationRecord, 0, len(res.Relations)+len(res.InverseRelations))
	appendRecord := func(n client.IssueRelationNode) {
		data, err := json.Marshal(n)
		if err != nil {
			return
		}
		records = append(records, store.IssueRelationRecord{
			ID:             n.ID,
			IssueID:        n.Issue.ID,
			RelatedIssueID: n.RelatedIssue.ID,
			Type:           n.Type,
			Data:           data,
		})
	}
	for _, n := range res.Relations {
		appendRecord(n)
	}
	for _, n := range res.InverseRelations {
		appendRecord(n)
	}
	if err := db.ReplaceIssueRelationsForIssue(issueID, records); err != nil {
		fmt.Fprintf(os.Stderr, "warning: local relation write-through failed: %v\n", err)
	}
}

// resolveIssueIDLocal maps an identifier to a UUID using the local snapshot,
// so a --data-source local relation read never touches the network.
func resolveIssueIDLocal(db *store.Store, issueRef string) (string, error) {
	if store.IsUUID(issueRef) {
		return issueRef, nil
	}
	rows, err := db.ListIssues(map[string]string{"identifier": issueRef}, 1)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", notFoundErr(fmt.Errorf("issue %q not found in the local store. Run 'linear-pp-cli sync' or use --data-source live", issueRef))
	}
	var row struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rows[0], &row); err != nil || row.ID == "" {
		return "", notFoundErr(fmt.Errorf("issue %q has no id in the local store. Run 'linear-pp-cli sync'", issueRef))
	}
	return row.ID, nil
}

func newRelationsCreateCmd(flags *rootFlags) *cobra.Command {
	var relType, to, dbPath string
	cmd := &cobra.Command{
		Use:   "create <issue> --type <type> --to <issue>",
		Short: "Create an issue relation (blocks, duplicate, related, similar)",
		Long: `Create one IssueRelation from <issue> to --to.

Direction matters for the two directional types. 'create A --type blocks --to
B' means A blocks B, which makes B blocked by A. 'create A --type duplicate
--to B' means A is a duplicate of B, so B is the canonical issue. 'related'
and 'similar' are symmetric.

To close A as a duplicate of B, prefer 'issues close-duplicate A --of B',
which creates the relation and then sets the duplicate state in that order.

--idempotent turns an already-existing identical relation into a successful
no-op. For blocks and duplicate the existing-relation check is
direction-sensitive. For the symmetric types related and similar it matches
in either direction.`,
		Example: `  linear-pp-cli relations create ESP-1155 --type blocks --to ESP-1160
  linear-pp-cli relations create ESP-1155 --type related --to ESP-1161 --idempotent --agent
  linear-pp-cli relations create ESP-1155 --type blocks --to ESP-1160 --dry-run --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return usageErr(fmt.Errorf("--to is required (the issue on the other side of the relation)"))
			}
			if relType == "" {
				return usageErr(fmt.Errorf("--type is required. Valid types: %s", strings.Join(issueRelationTypes, ", ")))
			}
			normalizedType, err := normalizeIssueRelationType(relType)
			if err != nil {
				return err
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_create_relation", "issueRelationCreate", map[string]any{
					"input": map[string]any{
						"type":           normalizedType,
						"issueId":        args[0],
						"relatedIssueId": to,
					},
				})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			issueID, err := resolveIssueID(c, args[0])
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			relatedID, err := resolveIssueID(c, to)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			if issueID == relatedID {
				return usageErr(fmt.Errorf("cannot relate %s to itself", args[0]))
			}
			if err := enforceIssueTrustMode(flags, resolveDBPath(dbPath), issueID, args[0]); err != nil {
				return err
			}

			if flags.idempotent {
				existing, err := findExistingRelation(c, issueID, relatedID, normalizedType)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				if existing != nil {
					return writeNoop(flags, "already_exists", fmt.Sprintf("relation already exists (%s, id %s)", relationLabel(existing.Type, existing.Direction), existing.ID))
				}
			}

			node, err := createIssueRelation(c, issueID, relatedID, normalizedType)
			if err != nil {
				return classifyMutationError("issueRelationCreate", err, flags, nil)
			}

			row := relationRowOf(node, relationDirectionOutgoing)
			if db, dbErr := store.Open(resolveDBPath(dbPath)); dbErr == nil {
				defer db.Close()
				if data, mErr := json.Marshal(node); mErr == nil {
					_ = db.UpsertIssueRelation(store.IssueRelationRecord{
						ID:             node.ID,
						IssueID:        node.Issue.ID,
						RelatedIssueID: node.RelatedIssue.ID,
						Type:           node.Type,
						Data:           data,
					})
				}
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"event":         "relation_created",
					"id":            row.ID,
					"type":          row.Type,
					"direction":     row.Direction,
					"issue":         row.Issue,
					"related_issue": row.RelatedIssue,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", row.Issue.Identifier, relationLabel(row.Type, row.Direction), row.RelatedIssue.Identifier)
			fmt.Fprintf(cmd.OutOrStdout(), "  relation id: %s\n", row.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&relType, "type", "", "Relation type: "+strings.Join(issueRelationTypes, ", ")+" (required)")
	cmd.Flags().StringVar(&to, "to", "", "The issue on the other side of the relation, identifier or UUID (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for the trust-mode ledger and relation write-through")
	return cmd
}

// createIssueRelation issues the mutation and returns the created node.
func createIssueRelation(c *client.Client, issueID, relatedID, relType string) (client.IssueRelationNode, error) {
	resp, err := c.Mutate(client.IssueRelationCreateMutation, map[string]any{
		"input": map[string]any{
			"type":           relType,
			"issueId":        issueID,
			"relatedIssueId": relatedID,
		},
	})
	if err != nil {
		return client.IssueRelationNode{}, err
	}
	node, ok, err := client.DecodeIssueRelationCreate(resp)
	if err != nil {
		return client.IssueRelationNode{}, err
	}
	if !ok {
		return client.IssueRelationNode{}, apiErr(fmt.Errorf("Linear reported issueRelationCreate success=false"))
	}
	return node, nil
}

// findExistingRelation looks for a relation of relType between the two
// issues. Direction-sensitive for blocks and duplicate, direction-insensitive
// for the symmetric types. Returns nil when there is no match.
func findExistingRelation(c *client.Client, issueID, relatedID, relType string) (*relationRow, error) {
	res, err := c.FetchIssueRelations(issueID, 0)
	if err != nil {
		return nil, err
	}
	for _, n := range res.Relations {
		if n.Type == relType && n.RelatedIssue.ID == relatedID {
			row := relationRowOf(n, relationDirectionOutgoing)
			return &row, nil
		}
	}
	if !symmetricRelationTypes[relType] {
		return nil, nil
	}
	for _, n := range res.InverseRelations {
		if n.Type == relType && n.Issue.ID == relatedID {
			row := relationRowOf(n, relationDirectionIncoming)
			return &row, nil
		}
	}
	return nil, nil
}

func newRelationsDeleteCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "delete <relation-id>",
		Aliases: []string{"rm"},
		Short:   "Delete an issue relation by its relation UUID",
		Long: `Delete one IssueRelation.

The argument is the relation's own UUID, not an issue identifier. Get it from
the RELATION-ID column of 'relations list <issue>'.

Deleting is confirmed interactively unless --yes is passed. With
--ignore-missing an already-deleted relation is a successful no-op.`,
		Example: `  linear-pp-cli relations delete 9f1c2a5e-2b1d-4d2f-9a44-1f0c3f6a77aa --yes
  linear-pp-cli relations delete 9f1c... --ignore-missing --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			relationID := strings.TrimSpace(args[0])
			if !store.IsUUID(relationID) {
				return usageErr(fmt.Errorf("expected a relation UUID (got %q). Run 'linear-pp-cli relations list <issue>' and use the RELATION-ID column", args[0]))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_relation", "issueRelationDelete", map[string]any{
					"id": relationID,
				})
			}
			if err := confirmRelationMutation(cmd, flags, fmt.Sprintf("Delete issue relation %s?", relationID)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.IssueRelationDeleteMutation, map[string]any{"id": relationID})
			if err != nil {
				if flags.ignoreMissing && isMissingEntityError(err) {
					deleteRelationLocally(resolveDBPath(dbPath), relationID)
					return writeNoop(flags, "already_deleted", "relation already deleted (no-op)")
				}
				return classifyDeleteError(err, flags)
			}
			entityID, ok, err := client.DecodeIssueRelationDelete(resp)
			if err != nil {
				return err
			}
			if !ok {
				return apiErr(fmt.Errorf("Linear reported issueRelationDelete success=false for %s", relationID))
			}
			deleteRelationLocally(resolveDBPath(dbPath), relationID)
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"event": "relation_deleted",
					"id":    firstNonEmpty(entityID, relationID),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted relation %s\n", firstNonEmpty(entityID, relationID))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for the local relation snapshot")
	return cmd
}

func deleteRelationLocally(dbPath, relationID string) {
	db, err := store.Open(dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	_, _ = db.DeleteIssueRelation(relationID)
}

// isMissingEntityError reports whether a GraphQL failure means the target no
// longer exists. Linear answers a delete of an unknown id with a 200 plus an
// errors array, so the HTTP-404 branch in classifyDeleteError never fires for
// the GraphQL transport and --ignore-missing needs this shape check too.
func isMissingEntityError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "could not find") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "entity not found")
}

// confirmRelationMutation is the shared confirmation gate for the destructive
// relation paths. --yes skips it, --no-input without --yes is a usage error
// rather than a hang.
func confirmRelationMutation(cmd *cobra.Command, flags *rootFlags, prompt string) error {
	if flags.yes {
		return nil
	}
	if flags.noInput {
		return usageErr(fmt.Errorf("%s this needs explicit confirmation: pass --yes (or drop --no-input)", prompt))
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N] ", prompt)
	var resp string
	fmt.Fscanln(cmd.InOrStdin(), &resp)
	if !strings.EqualFold(resp, "y") && !strings.EqualFold(resp, "yes") {
		return usageErr(fmt.Errorf("aborted"))
	}
	return nil
}

// newIssuesCloseDuplicateCmd closes an issue as a duplicate of another, in
// the only order that does not lose information.
//
// Linear removed the per-team markedAsDuplicateWorkflowStateId model
// (deprecated on Team, TeamCreateInput and TeamUpdateInput, "Duplicates are
// now system-managed via the native duplicate state"), so duplication is
// carried by the IssueRelation graph. Setting a duplicate-typed state without
// first creating the duplicate relation produces an orphaned close: the issue
// reads as closed-as-duplicate with nothing recording what it duplicates.
//
// The ordering is therefore fixed and not configurable:
//
//	step 1  issueRelationCreate{type: duplicate, issue: <issue>, relatedIssue: <canonical>}
//	step 2  issueUpdate{stateId: <the team's duplicate-typed state>}
//
// Step 1 failing leaves nothing done. Step 1 succeeding and step 2 failing
// leaves the relation recorded and the issue open, which is recoverable and
// exits 6 (partial) naming the step that failed. The reverse order has no
// recoverable failure mode, which is why it is not offered.
func newIssuesCloseDuplicateCmd(flags *rootFlags) *cobra.Command {
	var canonical, duplicateState, dbPath string
	cmd := &cobra.Command{
		Use:     "close-duplicate <issue> --of <canonical-issue>",
		Aliases: []string{"dupe"},
		Short:   "Close an issue as a duplicate: create the duplicate relation, then set the duplicate state",
		Long: `Close <issue> as a duplicate of --of.

Two steps, in this order and no other:

  1. issueRelationCreate with type duplicate, from <issue> to the canonical
     issue. This is what records WHAT the issue duplicates.
  2. issueUpdate setting <issue> to its team's duplicate-typed workflow state.

Doing step 2 first (which 'issues edit --state-type duplicate' does on its
own) produces an orphaned close. If step 1 fails nothing is changed. If step 2
fails the relation still stands and the command exits 6 (partial) telling you
which step failed and how to finish.

The duplicate state is resolved at runtime from the issue's own team, by
querying workflowStates filtered on type eq duplicate. A team with no
duplicate-typed state is exit 3: nothing is guessed and no state name is
compiled into this binary. Pass --duplicate-state when a team has more than
one duplicate-typed state.`,
		Example: `  linear-pp-cli issues close-duplicate ESP-1160 --of ESP-1155 --yes
  linear-pp-cli issues close-duplicate ESP-1160 --of ESP-1155 --dry-run --json
  linear-pp-cli issues close-duplicate ESP-1160 --of ESP-1155 --idempotent --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if canonical == "" {
				return usageErr(fmt.Errorf("--of is required (the canonical issue this one duplicates)"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_close_duplicate", "issueRelationCreate+issueUpdate", map[string]any{
					"steps": []map[string]any{
						{
							"step":     1,
							"mutation": "issueRelationCreate",
							"input": map[string]any{
								"type":           "duplicate",
								"issueId":        args[0],
								"relatedIssueId": canonical,
							},
						},
						{
							"step":       2,
							"mutation":   "issueUpdate",
							"issue":      args[0],
							"state_type": "duplicate",
						},
					},
				})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := fetchIssueLive(c, args[0])
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			var subject struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
				Title      string `json:"title"`
				Team       struct {
					ID  string `json:"id"`
					Key string `json:"key"`
				} `json:"team"`
			}
			if err := json.Unmarshal(raw, &subject); err != nil || subject.ID == "" {
				return notFoundErr(fmt.Errorf("issue %q not found", args[0]))
			}
			canonicalID, err := resolveIssueID(c, canonical)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			if canonicalID == subject.ID {
				return usageErr(fmt.Errorf("cannot close %s as a duplicate of itself", args[0]))
			}
			if err := enforceIssueTrustMode(flags, resolveDBPath(dbPath), subject.ID, args[0]); err != nil {
				return err
			}

			// Resolve the duplicate-typed state BEFORE the relation is
			// created. A team with no duplicate state must fail before any
			// mutation goes out, otherwise step 1 lands and step 2 can never
			// succeed, turning a plain exit 3 into a pointless partial.
			team := issueTeamInfo{ID: subject.Team.ID, Key: subject.Team.Key}
			stateID, err := resolveDuplicateStateID(c, team, duplicateState)
			if err != nil {
				return err
			}

			if err := confirmRelationMutation(cmd, flags, fmt.Sprintf("Close %s as a duplicate of %s?", firstNonEmpty(subject.Identifier, args[0]), canonical)); err != nil {
				return err
			}

			// Step 1: the relation. Always first.
			relationID := ""
			relationStatus := "created"
			if flags.idempotent {
				existing, findErr := findExistingRelation(c, subject.ID, canonicalID, "duplicate")
				if findErr != nil {
					return classifyLiveReadError(findErr, flags)
				}
				if existing != nil {
					relationID = existing.ID
					relationStatus = "already_existed"
				}
			}
			if relationID == "" {
				node, createErr := createIssueRelation(c, subject.ID, canonicalID, "duplicate")
				if createErr != nil {
					return classifyMutationError("issueRelationCreate", createErr, flags, nil)
				}
				relationID = node.ID
			}

			// Step 2: the state. A failure here is partial, not total: the
			// relation from step 1 stands and the issue is still open.
			if err := setIssueState(c, subject.ID, stateID); err != nil {
				return partialFailureErr(fmt.Errorf(
					"step 1 (issueRelationCreate) succeeded, relation %s records %s as a duplicate of %s, but step 2 (issueUpdate to the duplicate state) failed: %v\n%s is still OPEN. Finish with 'linear-pp-cli issues edit %s --state %s'",
					relationID, firstNonEmpty(subject.Identifier, args[0]), canonical, err,
					firstNonEmpty(subject.Identifier, args[0]), firstNonEmpty(subject.Identifier, args[0]), stateID,
				))
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"event":        "issue_closed_as_duplicate",
					"identifier":   subject.Identifier,
					"id":           subject.ID,
					"canonical":    canonical,
					"canonical_id": canonicalID,
					"relation_id":  relationID,
					"relation":     relationStatus,
					"state_id":     stateID,
					"steps":        []string{"issueRelationCreate", "issueUpdate"},
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s closed as a duplicate of %s\n", firstNonEmpty(subject.Identifier, args[0]), canonical)
			fmt.Fprintf(cmd.OutOrStdout(), "  relation %s (%s), then duplicate state %s\n", relationID, relationStatus, stateID)
			return nil
		},
	}
	cmd.Flags().StringVar(&canonical, "of", "", "The canonical issue this one duplicates, identifier or UUID (required)")
	cmd.Flags().StringVar(&duplicateState, "duplicate-state", "", "Disambiguate when the team has several duplicate-typed states: pass the state UUID or its exact name")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for the trust-mode ledger")
	return cmd
}

// resolveDuplicateStateID finds the team's duplicate-typed workflow state.
// The state's NAME is workspace vocabulary and is never assumed: resolution
// goes through WorkflowState.type, which is API-documented schema data.
// override accepts a UUID or an exact state name for teams carrying more
// than one duplicate-typed state.
func resolveDuplicateStateID(c graphqlQueryer, team issueTeamInfo, override string) (string, error) {
	if override != "" {
		if store.IsUUID(override) {
			return override, nil
		}
		return resolveWorkflowState(c, team, override, "")
	}
	stateID, err := resolveWorkflowState(c, team, "", "duplicate")
	if err != nil {
		return "", fmt.Errorf("%w\nA duplicate-typed state cannot be created by mutation (WorkflowStateCreateInput does not accept the duplicate type), so it has to already exist in Linear for this team", err)
	}
	return stateID, nil
}

// setIssueState is step 2 of the duplicate close.
func setIssueState(c *client.Client, issueID, stateID string) error {
	resp, err := c.Mutate(client.IssueUpdateMutation, map[string]any{
		"id":    issueID,
		"input": map[string]any{"stateId": stateID},
	})
	if err != nil {
		return err
	}
	var parsed struct {
		IssueUpdate struct {
			Success bool `json:"success"`
		} `json:"issueUpdate"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return fmt.Errorf("parsing issueUpdate response: %w", err)
	}
	if !parsed.IssueUpdate.Success {
		return fmt.Errorf("Linear reported issueUpdate success=false")
	}
	return nil
}

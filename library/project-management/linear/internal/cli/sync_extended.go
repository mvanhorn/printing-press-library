package cli

// Sync fetchers for the shell tables the store created and never filled
// (GAP-038).
//
// documents, templates, custom_views, favorites, project_milestones,
// project_statuses, initiatives and issue_relations all existed as
// id/data/synced_at tables from the first migration, and every
// `--data-source local` read of them answered empty because no code path ever
// wrote a row. These fetchers close that hole: each one crawls a
// workspace-wide root connection to exhaustion and writes what it saw.
//
// Every fetcher returns the wave-1 syncPass so the generic reconcile pass in
// newSyncCmd applies unchanged: complete is read off pagination exhaustion,
// and a crawl cut short by --max-pages prunes nothing.

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

// extendedSyncPageSize is the page size for every shell-table crawl. These
// nodes are wider than an id-only enumeration and narrower than an issue, so
// 100 keeps the page count low without pushing response sizes into the
// complexity limit.
const extendedSyncPageSize = 100

// mirrorShellRows writes the same nodes into the generic resources table under
// the resource type the promoted read commands use. The typed shell table is
// what the reconcile pass prunes, but `--data-source local` for these
// resources goes through resolveLocal, which reads resources by resource_type,
// so filling only the typed table would leave every offline read still
// answering empty. resourceType is the hyphenated command-level name, not the
// snake_case table name, and an empty string means the resource has no
// generic read path to serve (documents and custom views own hand-written
// live-only commands).
//
// Failures here are best effort: the typed table already has the row, and the
// resources table is a cache that sync never prunes.
func mirrorShellRows(db *store.Store, resourceType string, nodes []json.RawMessage) {
	if resourceType == "" || len(nodes) == 0 {
		return
	}
	if err := db.UpsertBatch(resourceType, nodes); err != nil {
		fmt.Fprintf(os.Stderr, "%s cache mirror error: %v\n", resourceType, err)
	}
}

// syncShellResource is the body every shell-table fetcher shares: page the
// connection to exhaustion, write each node under its id, and report what was
// seen. Errors on individual rows are printed and skipped rather than
// aborting, matching the wave-1 fetchers: a single malformed node must not
// cost the whole pass, but it does leave the row absent locally.
func syncShellResource(c *client.Client, db *store.Store, maxPages int, table, resourceType, query, fieldPath string) (syncPass, error) {
	nodes, complete, pages, err := c.PaginatedQueryComplete(query, nil, fieldPath, extendedSyncPageSize, maxPages)
	if err != nil {
		return syncPass{}, err
	}
	written := 0
	for _, node := range nodes {
		var n struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(node, &n); err != nil || n.ID == "" {
			continue
		}
		if err := db.UpsertShellRow(table, n.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "%s upsert error: %v\n", table, err)
			continue
		}
		written++
	}
	mirrorShellRows(db, resourceType, nodes)
	return syncPass{count: written, liveIDs: client.NodeIDs(nodes), complete: complete, pages: pages}, nil
}

func syncDocuments(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	return syncShellResource(c, db, maxPages, "documents", "", client.DocumentsSyncQuery, "documents")
}

func syncCustomViews(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	return syncShellResource(c, db, maxPages, "custom_views", "", client.CustomViewsSyncQuery, "customViews")
}

func syncFavorites(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	return syncShellResource(c, db, maxPages, "favorites", "favorites", client.FavoritesSyncQuery, "favorites")
}

func syncProjectMilestones(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	return syncShellResource(c, db, maxPages, "project_milestones", "project-milestones", client.ProjectMilestonesSyncQuery, "projectMilestones")
}

func syncProjectStatuses(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	return syncShellResource(c, db, maxPages, "project_statuses", "project-statuses", client.ProjectStatusesSyncQuery, "projectStatuses")
}

func syncInitiatives(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	return syncShellResource(c, db, maxPages, "initiatives", "initiatives", client.InitiativesSyncQuery, "initiatives")
}

// syncTemplates is the one shell resource whose root field is a plain list
// rather than a connection: Query.templates is [Template!]! and carries no
// pageInfo. One response therefore holds the whole workspace, which makes the
// crawl complete by construction and the pass prunable at one page.
func syncTemplates(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	data, err := c.Query(client.TemplatesQuery, nil)
	if err != nil {
		return syncPass{}, err
	}
	if len(data) == 0 {
		// Dry-run transport or an empty body. Nothing was enumerated, so this
		// is never a complete crawl and must not prune.
		return syncPass{}, nil
	}
	var result struct {
		Templates []json.RawMessage `json:"templates"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return syncPass{}, fmt.Errorf("parsing templates: %w", err)
	}
	written := 0
	for _, node := range result.Templates {
		var t struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(node, &t); err != nil || t.ID == "" {
			continue
		}
		if err := db.UpsertShellRow("templates", t.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "templates upsert error: %v\n", err)
			continue
		}
		written++
	}
	mirrorShellRows(db, "templates", result.Templates)
	return syncPass{count: written, liveIDs: client.NodeIDs(result.Templates), complete: true, pages: 1}, nil
}

// syncIssueRelations crawls the workspace-wide relation connection. Unlike the
// other shell tables, issue_relations has typed columns (issue_id,
// related_issue_id, type) that the relation reads index on, so it writes
// through the typed helper in the store rather than the generic shell writer.
func syncIssueRelations(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	nodes, complete, pages, err := c.PaginatedQueryComplete(client.IssueRelationsSyncQuery, nil, "issueRelations", extendedSyncPageSize, maxPages)
	if err != nil {
		return syncPass{}, err
	}
	written := 0
	for _, node := range nodes {
		var rel struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Issue struct {
				ID string `json:"id"`
			} `json:"issue"`
			RelatedIssue struct {
				ID string `json:"id"`
			} `json:"relatedIssue"`
		}
		if err := json.Unmarshal(node, &rel); err != nil || rel.ID == "" {
			continue
		}
		record := store.IssueRelationRecord{
			ID:             rel.ID,
			IssueID:        rel.Issue.ID,
			RelatedIssueID: rel.RelatedIssue.ID,
			Type:           rel.Type,
			Data:           node,
		}
		if err := db.UpsertIssueRelation(record); err != nil {
			fmt.Fprintf(os.Stderr, "issue_relations upsert error: %v\n", err)
			continue
		}
		written++
	}
	return syncPass{count: written, liveIDs: client.NodeIDs(nodes), complete: complete, pages: pages}, nil
}

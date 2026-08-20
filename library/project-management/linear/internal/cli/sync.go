package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

const linearProjectsSyncPageSize = 25

// linearIDsPageSize is the page size for the slim id-only enumerations that
// back the reconcile pass. One field per node, so a large page is cheap.
const linearIDsPageSize = 250

// syncIncrementalOverlap is subtracted from a stored cursor before it becomes
// an updatedAt lower bound. Two clocks are involved (this machine's and
// Linear's) and a row can be mutated in the seconds between the server
// evaluating the filter and the crawl finishing, so the window is deliberately
// re-walked at its leading edge. Re-fetching a handful of already-current rows
// costs one upsert each. Missing one loses it until the next --full.
const syncIncrementalOverlap = 5 * time.Minute

// syncPass is what one resource fetch saw.
//
// liveIDs is the set of upstream ids observed during this run and complete
// says whether that set is a COMPLETE ENUMERATION of the resource. The
// reconcile pass refuses to delete anything unless complete is true, because a
// partial live set cannot tell a deleted row from an unseen one.
//
// exhausted is the weaker fact: the crawl reached the last page of whatever it
// asked for. A full crawl that ran out of pages is both exhausted and
// complete. An incremental crawl that ran to the last page is exhausted and
// NEVER complete, because it only ever asked for the rows that changed. The
// two are separate fields precisely so an incremental pass can advance its
// cursor (which needs exhausted) without ever authorising a prune (which needs
// complete).
type syncPass struct {
	count     int
	liveIDs   []string
	complete  bool
	exhausted bool
	pages     int
}

// syncTable pairs a resource with the local table its reconcile pass prunes.
//
// fn is the full fetch. inc is the optional incremental fetch, narrowed to
// rows updated after the given instant. A nil inc means the resource has no
// filterable query and `sync --incremental` falls back to fn for it.
//
// mirror is the hyphenated resource type this resource is also written to in
// the generic resources cache, empty when it is not mirrored. A reconcile pass
// that prunes the typed table prunes the mirror under the same live set, so
// the promoted local reads (which go through the cache) cannot keep answering
// with an entity that was deleted upstream.
type syncTable struct {
	name   string
	table  string
	mirror string
	fn     func(*client.Client, *store.Store, int) (syncPass, error)
	inc    func(*client.Client, *store.Store, int, time.Time) (syncPass, error)
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var incremental bool
	var dbPath string
	var maxPages int
	var noPrune bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Linear data to local SQLite store",
		Long: "Pull issues, projects, teams, cycles, users, labels, and workflow states from Linear into the local store for offline search and analytics, then the workflow shell resources: documents, templates, custom views, favorites, project milestones, project statuses, initiatives, and issue relations.\n\n" +
			"After a resource is fetched to its last page, sync reconciles: local rows whose id was not seen upstream are deleted, so issues deleted or archived in Linear stop showing up in --data-source local reads. A fetch cut short by --max-pages never prunes.\n\n" +
			"--incremental narrows every resource whose query accepts an updatedAt filter to the rows changed since that resource's last recorded sync, minus a five minute overlap. Resources without a filterable query are fetched in full. An incremental fetch is not an enumeration of the workspace, so it NEVER reconciles: the rows it did not ask for are indistinguishable from rows that were deleted. Run a plain sync (or --full) to prune.",
		Example: `  linear-pp-cli sync
  linear-pp-cli sync --full
  linear-pp-cli sync --incremental
  linear-pp-cli sync --max-pages 0
  linear-pp-cli sync --no-prune`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if full && incremental {
				// Checked before anything else: --full's first act is to wipe
				// the very cursors --incremental would read.
				return usageErr(fmt.Errorf("--full and --incremental are mutually exclusive: --full clears the cursors that --incremental reads"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if full {
				if err := db.ClearSyncCursors(); err != nil {
					return fmt.Errorf("clearing sync state: %w", err)
				}
				fmt.Fprintln(os.Stderr, "Full sync requested, cleared all cursors")
			}
			if incremental && flags.dryRun {
				// PaginatedQueryComplete reports a dry-run transport as an
				// incomplete crawl, so no cursor would move anyway. Say so
				// instead of pretending the pass happened.
				fmt.Fprintln(os.Stderr, "sync: --dry-run fetches nothing, so --incremental has no window to advance")
			}

			start := time.Now()
			total := 0

			syncs := []syncTable{
				{name: "teams", table: "teams", fn: syncTeams},
				{name: "users", table: "users", fn: syncUsers},
				{name: "workflow states", table: "workflow_states", fn: syncWorkflowStates},
				{name: "labels", table: "issue_labels", fn: syncLabels},
				{name: "projects", table: "projects", fn: syncProjects},
				// Only the two documents that already declare a $filter variable
				// carry an incremental fetch. Everything else falls back to fn,
				// which is correct rather than merely convenient: a resource
				// whose query takes no filter cannot be narrowed server-side,
				// and narrowing it client-side would fetch every row anyway.
				{name: "cycles", table: "cycles", fn: syncCycles, inc: syncCyclesSince},
				{name: "issues", table: "issues", fn: syncIssues, inc: syncIssuesSince},
				// The shell tables (GAP-038). They come last because they are
				// the widest crawls and the least likely to be what a user
				// interrupted a sync for.
				{name: "documents", table: "documents", fn: syncDocuments},
				{name: "templates", table: "templates", mirror: "templates", fn: syncTemplates},
				{name: "custom views", table: "custom_views", fn: syncCustomViews},
				{name: "favorites", table: "favorites", mirror: "favorites", fn: syncFavorites},
				{name: "project milestones", table: "project_milestones", mirror: "project-milestones", fn: syncProjectMilestones},
				{name: "project statuses", table: "project_statuses", mirror: "project-statuses", fn: syncProjectStatuses},
				{name: "initiatives", table: "initiatives", mirror: "initiatives", fn: syncInitiatives},
				{name: "issue relations", table: "issue_relations", fn: syncIssueRelations},
			}

			type prunedResource struct {
				name    string
				removed int
			}
			var pruned []prunedResource
			var truncated []string
			var problems []string
			var narrowed []string
			var widened []string
			totalPruned := 0

			for _, s := range syncs {
				// The cursor is stamped with the instant the fetch STARTED.
				// Anything mutated upstream while this crawl was in flight has
				// to land inside the next window, not fall between the two.
				passStart := time.Now()

				var since time.Time
				if incremental && s.inc != nil {
					state, cerr := db.SyncCursorState(s.table)
					switch {
					case cerr != nil:
						fmt.Fprintf(os.Stderr, "sync: %s cursor unreadable, falling back to a full fetch: %v\n", s.name, cerr)
					case state.HasSynced:
						since = state.LastSyncedAt.Add(-syncIncrementalOverlap)
					}
				}
				incrementalPass := !since.IsZero()

				var pass syncPass
				var err error
				if incrementalPass {
					fmt.Fprintf(os.Stderr, "Syncing %s since %s... ", s.name, since.UTC().Format(time.RFC3339))
					pass, err = s.inc(c, db, maxPages, since)
					narrowed = append(narrowed, s.name)
				} else {
					if incremental {
						widened = append(widened, s.name)
					}
					fmt.Fprintf(os.Stderr, "Syncing %s... ", s.name)
					pass, err = s.fn(c, db, maxPages)
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
					continue
				}
				fmt.Fprintf(os.Stderr, "%d\n", pass.count)
				total += pass.count

				// Cursor. Advanced only off a crawl that reached its last
				// page: stamping a truncated fetch would permanently skip
				// every row past the cut. A full pass proves that through
				// complete, an incremental pass through exhausted.
				if pass.complete || pass.exhausted {
					// total_count is read back as "rows we hold", so it is the
					// local row count, not the fetch count. An incremental pass
					// that found nothing changed must not report an empty
					// store: hintIfUnsynced reads exactly this number.
					recorded := pass.count
					if n, cerr := db.CountRows(s.table); cerr == nil {
						recorded = n
					}
					if cerr := db.RecordSyncPass(s.table, passStart, recorded); cerr != nil {
						problems = append(problems, cerr.Error())
					}
				}

				if noPrune {
					continue
				}
				if incrementalPass {
					// The load-bearing half of --incremental. This pass asked
					// the server for changed rows only, so the ids it did not
					// see are overwhelmingly rows that simply did not change.
					// Reconciling against that set would delete the entire
					// unchanged workspace. pass.complete is false for exactly
					// this reason and the guard below would already catch it.
					// This branch exists so the reason printed is the true one
					// rather than "truncated".
					continue
				}
				if !pass.complete {
					// GAP-020: say so out loud. With the default --max-pages 10
					// an issues fetch tops out at 500 rows, and silently
					// pruning everything past that would delete live work.
					fmt.Fprintf(os.Stderr, "sync: %s truncated at %d pages, prune skipped\n", s.name, pass.pages)
					truncated = append(truncated, s.name)
					continue
				}
				if len(pass.liveIDs) == 0 {
					// Guardrail carried over from linear-store-prune: an empty
					// live set is far more likely to be a broken response than
					// an empty workspace.
					fmt.Fprintf(os.Stderr, "sync: %s returned an empty live set, prune skipped\n", s.name)
					continue
				}

				if flags.dryRun {
					stale, err := db.CountMissingWithMirror(s.table, s.mirror, pass.liveIDs)
					if err != nil {
						problems = append(problems, fmt.Sprintf("%s: %v", s.name, err))
						continue
					}
					if stale > 0 {
						fmt.Fprintf(os.Stderr, "sync: %s would prune %d stale row(s) (dry run)\n", s.name, stale)
					}
					continue
				}

				// One transaction covers the typed row and the mirrored copy
				// the promoted local reads answer from, so the two can never
				// disagree about what upstream still has.
				removed, err := db.PruneMissingWithMirror(s.table, s.mirror, pass.liveIDs)
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s: %v", s.name, err))
					continue
				}
				if removed > 0 {
					fmt.Fprintf(os.Stderr, "sync: pruned %d stale %s row(s)\n", removed, s.name)
					pruned = append(pruned, prunedResource{s.name, removed})
					totalPruned += removed
				}
				if s.table == "issues" && removed > 0 {
					// The issues_ad trigger is supposed to keep the FTS index in
					// step with the delete. Assert it, because a silent drift
					// here means local search keeps answering with pruned rows.
					issueRows, ftsRows, err := db.VerifyIssuesFTS()
					if err != nil {
						problems = append(problems, fmt.Sprintf("issues fts verification: %v", err))
					} else if issueRows != ftsRows {
						problems = append(problems, fmt.Sprintf("issues and issues_fts disagree after prune (issues=%d, issues_fts=%d), run: linear-pp-cli sync --full", issueRows, ftsRows))
					}
				}
			}

			fmt.Fprintf(os.Stderr, "\nSynced %d items in %s\n", total, time.Since(start).Round(time.Millisecond))
			if len(pruned) > 0 {
				parts := make([]string, 0, len(pruned))
				for _, p := range pruned {
					parts = append(parts, fmt.Sprintf("%s %d", p.name, p.removed))
				}
				fmt.Fprintf(os.Stderr, "Pruned %d stale rows: %s\n", totalPruned, strings.Join(parts, ", "))
			}
			if len(narrowed) > 0 {
				fmt.Fprintf(os.Stderr, "Incremental (updatedAt window, prune skipped): %s\n", strings.Join(narrowed, ", "))
				fmt.Fprintln(os.Stderr, "An incremental fetch is not an enumeration, so nothing was reconciled. Run 'linear-pp-cli sync' without --incremental to prune.")
			}
			if len(widened) > 0 {
				fmt.Fprintf(os.Stderr, "Fetched in full (no updatedAt filter, or never synced before): %s\n", strings.Join(widened, ", "))
			}
			if len(truncated) > 0 {
				fmt.Fprintf(os.Stderr, "Prune skipped (fetch truncated by --max-pages %d): %s\n", maxPages, strings.Join(truncated, ", "))
				fmt.Fprintln(os.Stderr, "Re-run with --max-pages 0 to enumerate everything and reconcile.")
			}
			if len(problems) > 0 {
				for _, p := range problems {
					fmt.Fprintf(os.Stderr, "sync: %s\n", p)
				}
				return partialFailureErr(fmt.Errorf("sync finished with %d reconcile problem(s)", len(problems)))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Full sync (ignore cursors, re-fetch everything)")
	cmd.Flags().BoolVar(&incremental, "incremental", false, "Fetch only rows updated since the last recorded sync, minus a five minute overlap. Never prunes")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/linear-pp-cli/store.db)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Maximum pages to fetch per resource (0 = unlimited)")
	cmd.Flags().BoolVar(&noPrune, "no-prune", false, "Keep local rows that no longer exist upstream (skip reconciliation)")
	return cmd
}

// Archived issues: sync deliberately does NOT pass includeArchived. The issues
// query excludes archived issues by default, so an archived issue stops
// appearing in the live set and its local row is pruned like a deleted one,
// which is the behaviour the local store wants. GAP-021 exposes archived rows
// on the read side instead, where the caller opts in per invocation:
// `issues list --include-archived` and `relations list --include-archived`,
// both carrying archivedAt. Threading it into sync would make the reconcile
// pass keep archived rows alive forever, which is the bug prune exists to fix.

func syncTeams(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	data, err := c.Query(client.TeamsQuery, nil)
	if err != nil {
		return syncPass{}, err
	}
	var result struct {
		Teams struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return syncPass{}, err
	}
	for _, node := range result.Teams.Nodes {
		var t struct {
			ID string `json:"id"`
		}
		json.Unmarshal(node, &t)
		if err := db.UpsertTeam(t.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "team upsert error: %v\n", err)
		}
	}
	pass := syncPass{count: len(result.Teams.Nodes)}
	enumerateLive(c, "teams", client.TeamIDsQuery, "teams", maxPages, &pass)
	return pass, nil
}

func syncUsers(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	data, err := c.Query(client.UsersQuery, map[string]any{"first": 200})
	if err != nil {
		return syncPass{}, err
	}
	var result struct {
		Users struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"users"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return syncPass{}, err
	}
	for _, node := range result.Users.Nodes {
		var u struct {
			ID string `json:"id"`
		}
		json.Unmarshal(node, &u)
		if err := db.UpsertUser(u.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "user upsert error: %v\n", err)
		}
	}
	pass := syncPass{count: len(result.Users.Nodes)}
	enumerateLive(c, "users", client.UserIDsQuery, "users", maxPages, &pass)
	return pass, nil
}

func syncWorkflowStates(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	data, err := c.Query(client.WorkflowStatesQuery, nil)
	if err != nil {
		return syncPass{}, err
	}
	var result struct {
		WorkflowStates struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return syncPass{}, err
	}
	for _, node := range result.WorkflowStates.Nodes {
		var s struct {
			ID string `json:"id"`
		}
		json.Unmarshal(node, &s)
		if err := db.UpsertWorkflowState(s.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "state upsert error: %v\n", err)
		}
	}
	pass := syncPass{count: len(result.WorkflowStates.Nodes)}
	enumerateLive(c, "workflow states", client.WorkflowStateIDsQuery, "workflowStates", maxPages, &pass)
	return pass, nil
}

// enumerateLive fills the live-id half of a pass for the three resources whose
// sync document carries no pageInfo, so completeness cannot be read off the
// fetch itself. A failed enumeration is not fatal: the upserts already landed,
// the pass just stays incomplete and nothing gets pruned.
//
// The enumeration is a separate crawl from the fat fetch that upserted the
// rows, and that is safe: prune deletes only ids the enumeration did not see,
// so a fat fetch that stopped short of the enumeration leaves those rows alone.
// What matters is that the ID crawl itself ran to the last page.
func enumerateLive(c *client.Client, name, query, fieldPath string, maxPages int, pass *syncPass) {
	ids, complete, pages, err := c.LiveIDs(query, nil, fieldPath, linearIDsPageSize, maxPages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync: %s live id enumeration failed, prune skipped: %v\n", name, err)
		return
	}
	pass.liveIDs = ids
	pass.complete = complete
	pass.pages = pages
}

func syncLabels(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	nodes, complete, pages, err := c.PaginatedQueryComplete(client.IssueLabelsQuery, map[string]any{"first": 100}, "issueLabels", 100, maxPages)
	if err != nil {
		return syncPass{}, err
	}
	for _, node := range nodes {
		var l struct {
			ID string `json:"id"`
		}
		json.Unmarshal(node, &l)
		if err := db.UpsertIssueLabel(l.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "label upsert error: %v\n", err)
		}
	}
	return syncPass{count: len(nodes), liveIDs: client.NodeIDs(nodes), complete: complete, pages: pages}, nil
}

func syncProjects(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	nodes, complete, pages, err := c.PaginatedQueryComplete(client.ProjectsQuery, nil, "projects", linearProjectsSyncPageSize, maxPages)
	if err != nil {
		return syncPass{}, err
	}
	for _, node := range nodes {
		var p struct {
			ID string `json:"id"`
		}
		json.Unmarshal(node, &p)
		if err := db.UpsertProject(p.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "project upsert error: %v\n", err)
		}
	}
	return syncPass{count: len(nodes), liveIDs: client.NodeIDs(nodes), complete: complete, pages: pages}, nil
}

func syncCycles(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	nodes, complete, pages, err := c.PaginatedQueryComplete(client.CyclesQuery, nil, "cycles", 50, maxPages)
	if err != nil {
		return syncPass{}, err
	}
	for _, node := range nodes {
		var cy struct {
			ID string `json:"id"`
		}
		json.Unmarshal(node, &cy)
		if err := db.UpsertCycle(cy.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "cycle upsert error: %v\n", err)
		}
	}
	return syncPass{count: len(nodes), liveIDs: client.NodeIDs(nodes), complete: complete, pages: pages}, nil
}

func syncIssues(c *client.Client, db *store.Store, maxPages int) (syncPass, error) {
	nodes, complete, pages, err := c.PaginatedQueryComplete(client.IssuesQuery, nil, "issues", 50, maxPages)
	if err != nil {
		return syncPass{}, err
	}
	liveIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		var issue struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
			Title      string `json:"title"`
		}
		json.Unmarshal(node, &issue)
		db.UpsertIssue(issue.ID, issue.Identifier, issue.Title, node)
		if issue.ID != "" {
			liveIDs = append(liveIDs, issue.ID)
		}
	}
	// Cursor writes are the orchestration loop's job, keyed by table name, so
	// every resource gets one instead of just this one.
	return syncPass{count: len(nodes), liveIDs: liveIDs, complete: complete, pages: pages}, nil
}

// updatedAfterFilter is the filter fragment that narrows a crawl to rows
// touched since `since`. IssueFilter.updatedAt and CycleFilter.updatedAt are
// both DateComparator, and DateComparator carries gt taking a
// DateTimeOrDuration, all verified live in api-inventory.json. RFC3339 is the
// DateTime half of that scalar.
func updatedAfterFilter(since time.Time) map[string]any {
	return map[string]any{
		"updatedAt": map[string]any{"gt": since.UTC().Format(time.RFC3339)},
	}
}

// syncIssuesSince is the incremental issues fetch.
//
// liveIDs is left empty on purpose. The ids this pass saw are the ids that
// CHANGED, never the ids that exist, so handing them to a reconcile pass would
// delete every unchanged issue in the workspace. complete stays false for the
// same reason. The orchestration loop already refuses to prune an incremental
// pass, and the empty live set is the second lock on that door.
func syncIssuesSince(c *client.Client, db *store.Store, maxPages int, since time.Time) (syncPass, error) {
	vars := map[string]any{"filter": updatedAfterFilter(since)}
	nodes, exhausted, pages, err := c.PaginatedQueryComplete(client.IssuesQuery, vars, "issues", 50, maxPages)
	if err != nil {
		return syncPass{}, err
	}
	for _, node := range nodes {
		var issue struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
			Title      string `json:"title"`
		}
		json.Unmarshal(node, &issue)
		db.UpsertIssue(issue.ID, issue.Identifier, issue.Title, node)
	}
	return syncPass{count: len(nodes), exhausted: exhausted, pages: pages}, nil
}

// syncCyclesSince is the incremental cycles fetch. Same contract as
// syncIssuesSince: no live ids, never complete, prune never runs off it.
func syncCyclesSince(c *client.Client, db *store.Store, maxPages int, since time.Time) (syncPass, error) {
	vars := map[string]any{"filter": updatedAfterFilter(since)}
	nodes, exhausted, pages, err := c.PaginatedQueryComplete(client.CyclesQuery, vars, "cycles", 50, maxPages)
	if err != nil {
		return syncPass{}, err
	}
	for _, node := range nodes {
		var cy struct {
			ID string `json:"id"`
		}
		json.Unmarshal(node, &cy)
		if err := db.UpsertCycle(cy.ID, node); err != nil {
			fmt.Fprintf(os.Stderr, "cycle upsert error: %v\n", err)
		}
	}
	return syncPass{count: len(nodes), exhausted: exhausted, pages: pages}, nil
}

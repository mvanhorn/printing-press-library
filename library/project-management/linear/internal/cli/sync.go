package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

const linearProjectsSyncPageSize = 25

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var dbPath string
	var maxPages int
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Linear data to local SQLite store",
		Long:  "Pull issues, projects, teams, cycles, users, labels, and workflow states from Linear into the local store for offline search and analytics.",
		Example: `  linear-pp-cli sync
  linear-pp-cli sync --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
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
				fmt.Fprintln(os.Stderr, "Full sync requested — cleared all cursors")
			}

			start := time.Now()
			total := 0

			syncs := []struct {
				name string
				fn   func(*client.Client, *store.Store, int) (int, error)
			}{
				{"teams", syncTeams},
				{"users", syncUsers},
				{"workflow states", syncWorkflowStates},
				{"labels", syncLabels},
				{"projects", syncProjects},
				{"cycles", syncCycles},
				{"issues", syncIssues},
			}

			for _, s := range syncs {
				fmt.Fprintf(os.Stderr, "Syncing %s... ", s.name)
				n, err := s.fn(c, db, maxPages)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
					continue
				}
				fmt.Fprintf(os.Stderr, "%d\n", n)
				total += n
			}

			fmt.Fprintf(os.Stderr, "\nSynced %d items in %s\n", total, time.Since(start).Round(time.Millisecond))
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Full sync (ignore cursors, re-fetch everything)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/linear-pp-cli/store.db)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Maximum pages to fetch per resource (0 = unlimited)")
	return cmd
}

func syncTeams(c *client.Client, db *store.Store, _ int) (int, error) {
	data, err := c.Query(client.TeamsQuery, nil)
	if err != nil {
		return 0, err
	}
	var result struct {
		Teams struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
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
	return len(result.Teams.Nodes), nil
}

func syncUsers(c *client.Client, db *store.Store, _ int) (int, error) {
	data, err := c.Query(client.UsersQuery, map[string]any{"first": 200})
	if err != nil {
		return 0, err
	}
	var result struct {
		Users struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"users"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
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
	return len(result.Users.Nodes), nil
}

func syncWorkflowStates(c *client.Client, db *store.Store, _ int) (int, error) {
	data, err := c.Query(client.WorkflowStatesQuery, nil)
	if err != nil {
		return 0, err
	}
	var result struct {
		WorkflowStates struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
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
	return len(result.WorkflowStates.Nodes), nil
}

func syncLabels(c *client.Client, db *store.Store, maxPages int) (int, error) {
	nodes, err := c.PaginatedQueryMax(client.IssueLabelsQuery, map[string]any{"first": 100}, "issueLabels", 100, maxPages)
	if err != nil {
		return 0, err
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
	return len(nodes), nil
}

func syncProjects(c *client.Client, db *store.Store, maxPages int) (int, error) {
	nodes, err := c.PaginatedQueryMax(client.ProjectsQuery, nil, "projects", linearProjectsSyncPageSize, maxPages)
	if err != nil {
		return 0, err
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
	return len(nodes), nil
}

func syncCycles(c *client.Client, db *store.Store, maxPages int) (int, error) {
	nodes, err := c.PaginatedQueryMax(client.CyclesQuery, nil, "cycles", 50, maxPages)
	if err != nil {
		return 0, err
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
	return len(nodes), nil
}

func syncIssues(c *client.Client, db *store.Store, maxPages int) (int, error) {
	nodes, err := c.PaginatedQueryMax(client.IssuesQuery, nil, "issues", 50, maxPages)
	if err != nil {
		return 0, err
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
	db.UpdateSyncCursor("issues", "", len(nodes))
	return len(nodes), nil
}

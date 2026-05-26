package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/nlm"
	"notebooklm-pp-cli/internal/store"
)

func newSyncCmd() *cobra.Command {
	var notebookID string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Cache notebook content to local SQLite for offline search",
		Long: `Fetch notebook source content from NotebookLM and store in a local
SQLite database with FTS5 full-text search. After syncing, use 'search'
for instant offline queries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()

			now := time.Now().UTC().Format(time.RFC3339)

			notebooks, err := listNotebooks(r)
			if err != nil {
				return err
			}

			totalSources := 0
			totalContent := 0

			for _, nb := range notebooks {
				if notebookID != "" && nb.ID != notebookID {
					continue
				}
				if err := s.UpsertNotebook(store.NotebookRow{
					ID: nb.ID, Title: nb.Title, SourceCount: nb.SourceCount,
					UpdatedAt: nb.UpdatedAt.Format(time.RFC3339), SyncedAt: now,
				}); err != nil {
					fmt.Printf("WARN notebook %s: %v\n", nb.ID, err)
					continue
				}

				sources, err := listSources(r, nb.ID)
				if err != nil {
					fmt.Printf("WARN sources for %s: %v\n", nb.ID, err)
					continue
				}
				totalSources += len(sources)

				for _, src := range sources {
					if err := s.UpsertSource(store.SourceRow{
						ID: src.ID, NotebookID: nb.ID, Title: src.Title,
						Type: src.Type, SyncedAt: now,
					}); err != nil {
						fmt.Printf("WARN source %s: %v\n", src.ID, err)
						continue
					}

					content, err := getSourceContent(r, src.ID)
					if err != nil {
						fmt.Printf("WARN content %s: %v\n", src.ID, err)
						continue
					}
					if content != "" {
						if err := s.UpsertContent(src.ID, nb.ID, src.Title, content); err != nil {
							fmt.Printf("WARN store %s: %v\n", src.ID, err)
							continue
						}
						totalContent++
					}
				}
				if !flags.json {
					fmt.Printf("synced %s (%d sources, %d with content)\n", nb.Title, len(sources), totalContent)
				}
				totalContent = 0
			}

			if flags.json {
				nbCount, _ := s.NotebookCount()
				srcCount, _ := s.SourceCount()
				return printJSON(map[string]any{
					"notebooks":       nbCount,
					"sources":         srcCount,
					"database":        s.Path(),
				})
			}
			nbCount, _ := s.NotebookCount()
			srcCount, _ := s.SourceCount()
			fmt.Printf("\nDone: %d notebooks, %d sources in %s\n", nbCount, srcCount, s.Path())
			return nil
		},
	}
	cmd.Flags().StringVar(&notebookID, "notebook", "", "Sync a single notebook by ID")
	return cmd
}

func listNotebooks(r *nlm.Runner) ([]nlm.Notebook, error) {
	var notebooks []nlm.Notebook
	if err := r.RunJSON(&notebooks, "notebook", "list"); err != nil {
		return nil, err
	}
	return notebooks, nil
}

func listSources(r *nlm.Runner, notebookID string) ([]nlm.Source, error) {
	var sources []nlm.Source
	if err := r.RunJSON(&sources, "source", "list", notebookID); err != nil {
		return nil, err
	}
	return sources, nil
}

func getSourceContent(r *nlm.Runner, sourceID string) (string, error) {
	out, err := r.Run("source", "content", sourceID, "--json")
	if err != nil {
		return "", err
	}
	var resp nlm.SourceContent
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", err
	}
	return resp.Value.Content, nil
}

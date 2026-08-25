// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/store"
	"github.com/spf13/cobra"
)

func fetchNotebookPage(ctx context.Context, client *nlm.Client) ([]nlm.Notebook, error) {
	return client.ListNotebooks(ctx)
}

func filterNotebooksByResources(batch []nlm.Notebook, resources []string) []nlm.Notebook {
	if len(resources) == 0 {
		return batch
	}
	want := make(map[string]bool, len(resources))
	for _, r := range resources {
		want[r] = true
	}
	filtered := make([]nlm.Notebook, 0, len(batch))
	for _, nb := range batch {
		if want["notebooks"] || want[nb.ID] {
			filtered = append(filtered, nb)
		}
	}
	return filtered
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var resources []string
	var full bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Refresh local notebook cache from live API",
		Example: `  notebooklm-pp-cli sync --json
  notebooklm-pp-cli sync --db ~/.local/share/notebooklm-pp-cli/cache.db --resources notebooks`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]any{"synced": 0, "dry_run": true})
				}
				dryRunMessage("sync notebooks to local cache")
				return nil
			}
			if len(resources) == 0 {
				resources = defaultSyncResources()
			}
			st, err := store.Open(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()

			client, err := newAPIClient(context.Background(), flags)
			if err != nil {
				return err
			}
			_ = full
			pageCursor := ""
			total := 0
			for page := 0; page < 100; page++ {
				batch, err := fetchNotebookPage(context.Background(), client)
				if err != nil {
					return wrapAPIError(err)
				}
				if pageCursor != "" {
					break
				}
				if len(batch) == 0 {
					break
				}
				if len(resources) > 0 {
					batch = filterNotebooksByResources(batch, resources)
				}
				now := time.Now().UTC().Format(time.RFC3339)
				for _, nb := range batch {
					if err := st.UpsertNotebook(nb, now); err != nil {
						return err
					}
				}
				total += len(batch)
				pageCursor = "done"
				if err := st.SaveSyncState(store.SyncState{
					ResourceType: "notebooks",
					LastCursor:   pageCursor,
					LastSyncedAt: time.Now().UTC(),
					TotalCount:   int64(total),
				}); err != nil {
					return err
				}
				break
			}
			if flags.asJSON {
				return printJSON(map[string]any{
					"synced": total, "resources": resources,
					"events": []map[string]any{{"type": "page_fetch", "count": total}},
					"ndjson": false,
				})
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "synced\t%d\n", total)
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite cache path (default: ~/.local/share/notebooklm-pp-cli/cache.db)")
	cmd.Flags().StringSliceVar(&resources, "resources", nil, "Resources to sync (default: notebooks)")
	cmd.Flags().BoolVar(&full, "full", false, "Full refresh (default behavior)")
	return cmd
}

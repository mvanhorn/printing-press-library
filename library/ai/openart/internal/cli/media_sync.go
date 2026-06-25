package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/store"
)

func newMediaSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		projectID string
		maxPages  int
	)
	cmd := &cobra.Command{
		Use:   "media-sync",
		Short: "Sync your full OpenArt media library into the local store",
		Long: `Walks /suite/api/resources for the active project and writes every
generation + upload into the local SQLite store.

The framework's generic 'sync' command skips this resource because the
spec marks projectId as required and it has no default. media-sync resolves
the default project automatically (override with --project-id).

Run this once to populate the local store, then 'prompts find', 'credits
burn', 'stats' all work offline.`,
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_sync": "media", "project_id": projectID}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if projectID == "" {
				resolved, perr := resolveDefaultProject(c)
				if perr != nil {
					return fmt.Errorf("resolve default project: %w", perr)
				}
				projectID = resolved
			}

			db, err := openLocalStoreRW()
			if err != nil {
				return fmt.Errorf("open local store: %w", err)
			}
			defer db.Close()

			cursor := ""
			page := 0
			total := 0
			for {
				params := map[string]string{
					"projectId":     projectID,
					"folderIdNull":  "true",
					"limit":         "50",
				}
				if cursor != "" {
					params["cursor"] = cursor
				}
				path := "/resources?" + encodeParams(params)
				raw, err := c.Get(path, nil)
				if err != nil {
					return fmt.Errorf("page %d: %w", page, err)
				}
				var resp struct {
					Success bool              `json:"success"`
					Data    []json.RawMessage `json:"data"`
					Cursor  string            `json:"cursor"`
					HasMore bool              `json:"hasMore"`
				}
				if err := json.Unmarshal(raw, &resp); err != nil {
					return fmt.Errorf("page %d: parse: %w", page, err)
				}
				for _, item := range resp.Data {
					var withID struct {
						ID string `json:"id"`
					}
					_ = json.Unmarshal(item, &withID)
					if withID.ID == "" {
						continue
					}
					if err := db.UpsertMedia(item); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "  upsert %s: %v\n", withID.ID, err)
					}
					total++
				}
				page++
				fmt.Fprintf(cmd.ErrOrStderr(), "  page %d: %d items, total=%d\n", page, len(resp.Data), total)
				if !resp.HasMore || resp.Cursor == "" {
					break
				}
				cursor = resp.Cursor
				if maxPages > 0 && page >= maxPages {
					fmt.Fprintf(cmd.ErrOrStderr(), "  reached --max-pages cap (%d)\n", maxPages)
					break
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"project_id":   projectID,
				"pages":        page,
				"total_items":  total,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID (default: workspace's default)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Stop after N pages (0 = unlimited)")
	return cmd
}

// openLocalStoreRW opens the local SQLite DB read-write so media-sync can
// upsert. The other novel commands open it read-only.
func openLocalStoreRW() (*store.Store, error) {
	return store.Open(localStorePath())
}

func encodeParams(params map[string]string) string {
	v := url.Values{}
	for k, val := range params {
		v.Set(k, val)
	}
	return v.Encode()
}
